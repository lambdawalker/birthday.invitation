package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"sync"
	"testing"
)

func setup(t *testing.T) (*Server, *MemoryStore, Family) {
	t.Helper()
	st := NewMemoryStore()
	f := Family{ID: "0123456789abcdef0123456789abcdef", Name: "Familia García", Phone: "+526561234567", Email: "private@example.com", RSVP: "pending", Status: "pending", Revision: 1}
	if err := st.Create(t.Context(), f); err != nil {
		t.Fatal(err)
	}
	return NewServer(st, "a-very-long-test-admin-secret-32bytes", "https://xitlaliabigail.com"), st, f
}
func req(s *Server, method, path, body, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func admin(s *Server) string { return s.session() }
func TestPublicPrivacyAndRSVP(t *testing.T) {
	s, st, f := setup(t)
	w := req(s, "GET", "/api/invitations/"+f.ID, "", "")
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte(f.Phone)) || bytes.Contains(w.Body.Bytes(), []byte(f.Email)) {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, b := range []string{`{"rsvp":"yes","adults":-1,"children":2}`, `{"rsvp":"yes","adults":0,"children":0}`, `{"rsvp":"unknown"}`, `{"rsvp":"yes","adults":1.5}`} {
		if req(s, "POST", "/api/invitations/"+f.ID+"/rsvp", b, "").Code != 400 {
			t.Fatal(b)
		}
	}
	w = req(s, "POST", "/api/invitations/"+f.ID+"/rsvp", `{"rsvp":"yes","adults":20,"children":25}`, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	got, _ := st.Get(t.Context(), f.ID)
	if got.Adults != 20 || got.Children != 25 {
		t.Fatal(got)
	}
}
func TestAdminAuthAndDeviceScope(t *testing.T) {
	s, _, _ := setup(t)
	if req(s, "GET", "/api/admin/families", "", "").Code != 401 {
		t.Fatal("unprotected")
	}
	w := req(s, "POST", "/api/admin/devices", `{"name":"David phone"}`, admin(s))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var d map[string]string
	json.Unmarshal(w.Body.Bytes(), &d)
	if req(s, "GET", "/api/families", "", d["token"]).Code != 200 {
		t.Fatal("device denied")
	}
	if req(s, "POST", "/api/admin/devices", `{"name":"attacker"}`, d["token"]).Code != 401 {
		t.Fatal("scope escalation")
	}
	if req(s, "DELETE", "/api/admin/devices/"+d["id"], "", admin(s)).Code != 204 {
		t.Fatal("revoke failed")
	}
	if req(s, "GET", "/api/families", "", d["token"]).Code != 401 {
		t.Fatal("revoked allowed")
	}
}
func TestClaimAndAttemptIdempotency(t *testing.T) {
	s, st, f := setup(t)
	token := func(name string) string {
		w := req(s, "POST", "/api/admin/devices", `{"name":"`+name+`"}`, admin(s))
		var d map[string]string
		json.Unmarshal(w.Body.Bytes(), &d)
		return d["token"]
	}
	a, b := token("A"), token("B")
	path := "/api/families/" + f.ID + "/actions"
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, tok := range []string{a, b} {
		wg.Add(1)
		go func(tok string) { defer wg.Done(); codes <- req(s, "POST", path, `{"action":"claim"}`, tok).Code }(tok)
	}
	wg.Wait()
	close(codes)
	ok, conf := 0, 0
	for c := range codes {
		if c == 200 {
			ok++
		}
		if c == 409 {
			conf++
		}
	}
	if ok != 1 || conf != 1 {
		t.Fatal(ok, conf)
	}
	got, _ := st.Get(t.Context(), f.ID)
	owner := a
	if got.AssignedTo != HashToken(a) {
		owner = b
	}
	body := `{"action":"begin","channel":"sms","attemptId":"attempt-123"}`
	if w := req(s, "POST", path, body, owner); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if req(s, "POST", path, `{"action":"begin","channel":"sms","attemptId":"another"}`, owner).Code != 409 {
		t.Fatal("duplicate send allowed")
	}
	result := `{"action":"result","attemptId":"attempt-123","status":"sent"}`
	for i := 0; i < 2; i++ {
		if w := req(s, "POST", path, result, owner); w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	got, _ = st.Get(t.Context(), f.ID)
	if got.Status != "sent" || len(got.History) != 2 {
		t.Fatal(got)
	}
	if req(s, "POST", path, `{"action":"release"}`, owner).Code != 200 {
		t.Fatal("release")
	}
}
func TestLoginCookieAndOrigin(t *testing.T) {
	s, _, _ := setup(t)
	w := req(s, "POST", "/api/login", `{"key":"a-very-long-test-admin-secret-32bytes"}`, "")
	if w.Code != 200 || len(w.Result().Cookies()) == 0 {
		t.Fatal(w.Code)
	}
	c := w.Result().Cookies()[0]
	if !c.HttpOnly || !c.Secure {
		t.Fatal(c)
	}
	r := httptest.NewRequest("POST", "/api/admin/devices", bytes.NewBufferString(`{"name":"x"}`))
	r.Header.Set("Origin", "https://evil.example")
	r.AddCookie(c)
	out := httptest.NewRecorder()
	s.ServeHTTP(out, r)
	if out.Code != 403 {
		t.Fatal(out.Code)
	}
}
func TestAdminCanReconcileStuckDeviceAttempt(t *testing.T) {
	s, _, f := setup(t)
	w := req(s, "POST", "/api/admin/devices", `{"name":"Phone"}`, admin(s))
	var d map[string]string
	json.Unmarshal(w.Body.Bytes(), &d)
	p := "/api/families/" + f.ID + "/actions"
	req(s, "POST", p, `{"action":"claim"}`, d["token"])
	req(s, "POST", p, `{"action":"begin","channel":"sms","attemptId":"lost"}`, d["token"])
	w = req(s, "POST", p, `{"action":"result","attemptId":"lost","status":"uncertain"}`, admin(s))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if req(s, "POST", p, `{"action":"release"}`, admin(s)).Code != 200 {
		t.Fatal("cannot release reconciled attempt")
	}
}
