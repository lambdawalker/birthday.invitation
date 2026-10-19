package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func HashToken(t string) string { h := sha256.Sum256([]byte(t)); return hex.EncodeToString(h[:]) }
func (s *Server) sign(payload string) string {
	m := hmac.New(sha256.New, []byte(s.key))
	m.Write([]byte(payload))
	return hex.EncodeToString(m.Sum(nil))
}
func (s *Server) session() string {
	p := "admin." + strconv.FormatInt(time.Now().Add(12*time.Hour).Unix(), 10)
	return p + "." + s.sign(p)
}
func (s *Server) isAdmin(r *http.Request) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if c, e := r.Cookie("session"); e == nil && token == "" {
		token = c.Value
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "admin" {
		return false
	}
	exp, e := strconv.ParseInt(parts[1], 10, 64)
	if e != nil || time.Now().Unix() >= exp {
		return false
	}
	return hmac.Equal([]byte(parts[2]), []byte(s.sign(parts[0]+"."+parts[1])))
}
func (s *Server) actor(r *http.Request) (Device, bool) {
	if s.isAdmin(r) {
		return Device{ID: "admin", Name: "Administrador"}, true
	}
	t := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !strings.HasPrefix(t, "device_") {
		return Device{}, false
	}
	d, e := s.store.GetDevice(r.Context(), HashToken(t))
	return d, e == nil && !d.Revoked
}
func equal(a, b string) bool {
	aa := sha256.Sum256([]byte(a))
	bb := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aa[:], bb[:]) == 1
}
