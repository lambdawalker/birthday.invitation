package app

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
)

const pinLifetime = 15 * time.Minute
const maxPINAttempts = 5

var usernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,39}$`)
var pinPattern = regexp.MustCompile(`^[0-9]{8}$`)

// User is persisted internally. Only userView is returned to administrators.
// A stable ID keeps assignments intact when a PIN or session is replaced.
type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	CreatedAt   string `json:"createdAt"`
	Revision    int    `json:"revision"`
	PINHash     string `json:"pinHash"`
	PINExpires  int64  `json:"pinExpires"`
	Attempts    int    `json:"attempts"`
	SessionHash string `json:"sessionHash"`
	Revoked     bool   `json:"revoked"`
}
type userView struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Status     string `json:"status"`
	PINExpires int64  `json:"pinExpires,omitempty"`
}

func viewUser(u User) userView {
	status := "needs_pin"
	if u.Revoked {
		status = "revoked"
	} else if u.SessionHash != "" {
		status = "signed_in"
	} else if u.PINHash != "" && u.PINExpires > time.Now().Unix() && u.Attempts < maxPINAttempts {
		status = "pin_ready"
	}
	return userView{u.ID, u.Username, status, u.PINExpires}
}
func canonicalUsername(name string) string { return strings.ToLower(strings.TrimSpace(name)) }
func (s *Server) issuePIN(u *User) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(100000000))
	if err != nil {
		return "", err
	}
	pin := fmt.Sprintf("%08d", n)
	u.PINHash = s.sign("pin:" + u.ID + ":" + pin)
	u.PINExpires = time.Now().Add(pinLifetime).Unix()
	u.Attempts = 0
	u.SessionHash = ""
	u.Revoked = false
	return pin, nil
}
func (s *Server) users(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		fail(w, 401, "Sign in as administrator")
		return
	}
	if r.Method == "GET" {
		users, err := s.store.Users(r.Context())
		if err != nil {
			fail(w, 503, "Unable to load users")
			return
		}
		sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
		result := []userView{}
		for _, u := range users {
			result = append(result, viewUser(u))
		}
		send(w, 200, result)
		return
	}
	var body struct {
		Username string `json:"username"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := canonicalUsername(body.Username)
	if !usernamePattern.MatchString(name) {
		fail(w, 400, "Use 3–40 letters, numbers, dots, underscores or hyphens for the username")
		return
	}
	u := User{ID: HashToken(name), Username: name, CreatedAt: now(), Revision: 1}
	pin, err := s.issuePIN(&u)
	if err != nil {
		fail(w, 503, "Unable to generate PIN")
		return
	}
	if err = s.store.SaveUser(r.Context(), u, 0); err != nil {
		if errors.Is(err, ErrConflict) {
			fail(w, 409, "Username already exists. Generate a new PIN for that user instead.")
		} else {
			fail(w, 503, "Unable to create user")
		}
		return
	}
	send(w, 201, map[string]any{"user": viewUser(u), "pin": pin, "baseUrl": s.origin})
}
func (s *Server) userPIN(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		fail(w, 401, "Sign in as administrator")
		return
	}
	u, err := s.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			fail(w, 404, "User not found")
		} else {
			fail(w, 503, "Unable to load user")
		}
		return
	}
	rev := u.Revision
	u.Revision++
	pin, err := s.issuePIN(&u)
	if err != nil {
		fail(w, 503, "Unable to generate PIN")
		return
	}
	if err = s.store.SaveUser(r.Context(), u, rev); err != nil {
		if errors.Is(err, ErrConflict) {
			fail(w, 409, "User changed. Refresh and try again.")
		} else {
			fail(w, 503, "Unable to save user. Try again shortly.")
		}
		return
	}
	send(w, 200, map[string]any{"user": viewUser(u), "pin": pin, "baseUrl": s.origin})
}
func (s *Server) revokeUser(w http.ResponseWriter, r *http.Request) {
	if !s.isAdmin(r) {
		fail(w, 401, "Sign in as administrator")
		return
	}
	u, err := s.store.GetUser(r.Context(), r.PathValue("id"))
	if err != nil {
		fail(w, 404, "User not found")
		return
	}
	rev := u.Revision
	u.Revision++
	u.Revoked = true
	u.PINHash = ""
	u.SessionHash = ""
	u.PINExpires = 0
	if err = s.store.SaveUser(r.Context(), u, rev); err != nil {
		if errors.Is(err, ErrConflict) {
			fail(w, 409, "User changed. Refresh and try again.")
		} else {
			fail(w, 503, "Unable to save user. Try again shortly.")
		}
		return
	}
	send(w, 204, nil)
}
func (s *Server) userLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		PIN      string `json:"pin"`
	}
	if !decode(w, r, &body) {
		return
	}
	name := canonicalUsername(body.Username)
	if !usernamePattern.MatchString(name) || !pinPattern.MatchString(body.PIN) {
		fail(w, 401, "Invalid or expired PIN. Ask the administrator for a new PIN.")
		return
	}
	// Conditional writes enforce attempts and single use across concurrent Lambda instances.
	for tries := 0; tries < 8; tries++ {
		u, err := s.store.GetUser(r.Context(), HashToken(name))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				fail(w, 401, "Invalid or expired PIN. Ask the administrator for a new PIN.")
			} else {
				fail(w, 503, "Sign-in temporarily unavailable")
			}
			return
		}
		if u.Revoked || u.PINHash == "" || u.PINExpires <= time.Now().Unix() || u.Attempts >= maxPINAttempts {
			fail(w, 401, "Invalid or expired PIN. Ask the administrator for a new PIN.")
			return
		}
		valid := equal(u.PINHash, s.sign("pin:"+u.ID+":"+body.PIN))
		rev := u.Revision
		u.Revision++
		u.Attempts++
		token := ""
		if valid {
			token = "user_" + u.ID + "." + RandomID() + RandomID()
			u.SessionHash = HashToken(token)
			u.PINHash = ""
			u.PINExpires = 0
		}
		if err = s.store.SaveUser(r.Context(), u, rev); errors.Is(err, ErrConflict) {
			continue
		} else if err != nil {
			fail(w, 503, "Sign-in temporarily unavailable")
			return
		}
		if !valid {
			fail(w, 401, "Invalid or expired PIN. Ask the administrator for a new PIN.")
			return
		}
		send(w, 200, map[string]string{"token": token, "id": u.ID, "username": u.Username, "baseUrl": s.origin})
		return
	}
	fail(w, 429, "Too many simultaneous attempts. Try again shortly.")
}
func (s *Server) userLogout(w http.ResponseWriter, r *http.Request) {
	d, ok := s.actor(r)
	if !ok || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer user_") {
		fail(w, 401, "Sign in first")
		return
	}
	u, err := s.store.GetUser(r.Context(), d.ID)
	if err != nil {
		fail(w, 503, "Unable to sign out")
		return
	}
	// A reset/re-login between reads must not let an old session revoke a new one.
	if !equal(u.SessionHash, HashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))) {
		fail(w, 401, "Session has changed")
		return
	}
	rev := u.Revision
	u.Revision++
	u.SessionHash = ""
	if err = s.store.SaveUser(r.Context(), u, rev); err != nil {
		if errors.Is(err, ErrConflict) {
			fail(w, 409, "Session changed. Try again.")
		} else {
			fail(w, 503, "Unable to sign out. Try again shortly.")
		}
		return
	}
	send(w, 204, nil)
}
