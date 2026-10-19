package app

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
)

type Server struct {
	store       Store
	key, origin string
	mux         *http.ServeMux
}

func NewServer(st Store, key, origin string) *Server {
	if len(key) < 32 {
		panic("ADMIN_KEY must be at least 32 characters")
	}
	s := &Server{store: st, key: key, origin: strings.TrimRight(origin, "/"), mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/admin/users", s.users)
	s.mux.HandleFunc("POST /api/admin/users", s.users)
	s.mux.HandleFunc("POST /api/admin/users/{id}/pin", s.userPIN)
	s.mux.HandleFunc("DELETE /api/admin/users/{id}", s.revokeUser)
	s.mux.HandleFunc("POST /api/user/login", s.userLogin)
	s.mux.HandleFunc("POST /api/user/logout", s.userLogout)
	s.mux.HandleFunc("POST /api/login", s.login)
	s.mux.HandleFunc("POST /api/logout", s.logout)
	s.mux.HandleFunc("GET /api/invitations/{id}", s.invitation)
	s.mux.HandleFunc("POST /api/invitations/{id}/rsvp", s.rsvp)
	s.mux.HandleFunc("GET /api/admin/families", s.adminList)
	s.mux.HandleFunc("GET /api/families", s.list)
	s.mux.HandleFunc("POST /api/families/import", s.importContacts)
	s.mux.HandleFunc("DELETE /api/families/{id}", s.deleteFamily)
	s.mux.HandleFunc("POST /api/families/{id}/actions", s.action)
	s.mux.HandleFunc("GET /api/admin/devices", s.devices)
	s.mux.HandleFunc("POST /api/admin/devices", s.devices)
	s.mux.HandleFunc("DELETE /api/admin/devices/{id}", s.revoke)
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { send(w, 200, map[string]string{"status": "ok"}) })
	return s
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if o := r.Header.Get("Origin"); o != "" && o != s.origin {
		fail(w, 403, "Origen no permitido")
		return
	}
	s.mux.ServeHTTP(w, r)
}
func send(w http.ResponseWriter, c int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(c)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}
func fail(w http.ResponseWriter, c int, m string) { send(w, c, map[string]string{"error": m}) }
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return decodeLimit(w, r, v, 8192)
}
func decodeLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		fail(w, 400, "Datos inválidos")
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		fail(w, 400, "Datos inválidos")
		return false
	}
	return true
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Key string `json:"key"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !equal(s.key, b.Key) {
		fail(w, 401, "Clave incorrecta")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: s.session(), Path: "/api", HttpOnly: true, Secure: !strings.HasPrefix(s.origin, "http://localhost"), SameSite: http.SameSiteStrictMode, MaxAge: 43200})
	send(w, 200, map[string]bool{"ok": true})
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/api", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	send(w, 204, nil)
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (s *Server) family(w http.ResponseWriter, r *http.Request) (Family, bool) {
	id := r.PathValue("id")
	if !idPattern.MatchString(id) {
		fail(w, 404, "Invitación no encontrada")
		return Family{}, false
	}
	f, e := s.store.Get(r.Context(), id)
	if e != nil {
		if errors.Is(e, ErrNotFound) {
			fail(w, 404, "Invitación no encontrada")
		} else {
			fail(w, 503, "Servicio no disponible")
		}
		return f, false
	}
	return f, true
}
func public(f Family) any {
	return struct {
		Name     string `json:"name"`
		RSVP     string `json:"rsvp"`
		Adults   int    `json:"adults"`
		Children int    `json:"children"`
	}{f.Name, f.RSVP, f.Adults, f.Children}
}
func (s *Server) invitation(w http.ResponseWriter, r *http.Request) {
	if f, ok := s.family(w, r); ok {
		send(w, 200, public(f))
	}
}
func (s *Server) rsvp(w http.ResponseWriter, r *http.Request) {
	var b struct {
		RSVP     string `json:"rsvp"`
		Adults   int    `json:"adults"`
		Children int    `json:"children"`
	}
	if !decode(w, r, &b) {
		return
	}
	if (b.RSVP != "yes" && b.RSVP != "no") || b.Adults < 0 || b.Children < 0 || (b.RSVP == "yes" && b.Adults == 0 && b.Children == 0) {
		fail(w, 400, "Indica si asistirás y cuántas personas vendrán")
		return
	}
	f, ok := s.family(w, r)
	if !ok {
		return
	}
	rev := f.Revision
	f.RSVP = b.RSVP
	f.Adults = b.Adults
	f.Children = b.Children
	if b.RSVP == "no" {
		f.Adults = 0
		f.Children = 0
	}
	f.Revision++
	f.UpdatedAt = now()
	if s.save(w, r, f, rev) {
		send(w, 200, public(f))
	}
}
func (s *Server) save(w http.ResponseWriter, r *http.Request, f Family, rev int) bool {
	e := s.store.Save(r.Context(), f, rev)
	if errors.Is(e, ErrConflict) {
		fail(w, 409, "La información cambió. Actualiza e intenta de nuevo.")
		return false
	}
	if e != nil {
		fail(w, 503, "No se pudo guardar. Intenta de nuevo.")
		return false
	}
	return true
}
func (s *Server) adminList(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		fail(w, 401, "Inicia sesión")
		return
	}
	s.list(w, r)
}
func (s *Server) list(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.actor(r); !ok {
		fail(w, 401, "Acceso no autorizado")
		return
	}
	fs, e := s.store.List(r.Context())
	if e != nil {
		fail(w, 503, "No se pudo cargar la lista")
		return
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Name < fs[j].Name })
	send(w, 200, map[string]any{"families": fs, "baseUrl": s.origin})
}
func (s *Server) devices(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		fail(w, 401, "Inicia sesión")
		return
	}
	if r.Method == "GET" {
		ds, e := s.store.Devices(r.Context())
		if e != nil {
			fail(w, 503, "Error de lectura")
			return
		}
		send(w, 200, ds)
		return
	}
	var b struct {
		Name string `json:"name"`
	}
	if !decode(w, r, &b) {
		return
	}
	b.Name = strings.TrimSpace(b.Name)
	if len(b.Name) < 1 || len(b.Name) > 80 {
		fail(w, 400, "Escribe un nombre de dispositivo")
		return
	}
	token := "device_" + RandomID() + RandomID()
	d := Device{ID: HashToken(token), Name: b.Name, CreatedAt: now()}
	if s.store.PutDevice(r.Context(), d) != nil {
		fail(w, 503, "No se pudo crear")
		return
	}
	send(w, 201, map[string]string{"id": d.ID, "name": d.Name, "token": token, "baseUrl": s.origin})
}
func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		fail(w, 401, "Inicia sesión")
		return
	}
	d, e := s.store.GetDevice(r.Context(), r.PathValue("id"))
	if e != nil {
		fail(w, 404, "Dispositivo no encontrado")
		return
	}
	d.Revoked = true
	if s.store.PutDevice(r.Context(), d) != nil {
		fail(w, 503, "No se pudo revocar")
		return
	}
	send(w, 204, nil)
}
