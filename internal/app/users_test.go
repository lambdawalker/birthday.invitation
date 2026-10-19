package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func createPINUser(t *testing.T, s *Server) (string, string) {
	t.Helper()
	w := req(s, "POST", "/api/admin/users", `{"username":"David"}`, admin(s))
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body.String())
	}
	var result struct {
		User userView `json:"user"`
		PIN  string   `json:"pin"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.User.Username != "david" || len(result.PIN) != 8 {
		t.Fatal(result)
	}
	return result.User.ID, result.PIN
}
func pinLogin(s *Server, pin string) (int, string) {
	w := req(s, "POST", "/api/user/login", fmt.Sprintf(`{"username":"DAVID","pin":%q}`, pin), "")
	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	return w.Code, result["token"]
}
func TestUserPINLifecycle(t *testing.T) {
	s, st, f := setup(t)
	id, pin := createPINUser(t, s)
	if w := req(s, "POST", "/api/admin/users", `{"username":" david "}`, admin(s)); w.Code != 409 {
		t.Fatal(w.Code)
	}
	if req(s, "GET", "/api/admin/users", "", "").Code != 401 {
		t.Fatal("public users")
	}
	raw := req(s, "GET", "/api/admin/users", "", admin(s)).Body.String()
	if strings.Contains(raw, pin) || strings.Contains(raw, "pinHash") || strings.Contains(raw, "sessionHash") {
		t.Fatal("secret leaked")
	}
	stored, _ := st.GetUser(t.Context(), id)
	if stored.PINHash == pin || stored.PINHash == HashToken(pin) {
		t.Fatal("PIN needs keyed hash")
	}
	status, token := pinLogin(s, pin)
	if status != 200 {
		t.Fatal(status)
	}
	if status, _ = pinLogin(s, pin); status != 401 {
		t.Fatal("PIN reused")
	}
	if req(s, "POST", "/api/admin/users", `{"username":"other"}`, token).Code != 401 {
		t.Fatal("scope escalation")
	}
	if req(s, "POST", "/api/families/"+f.ID+"/actions", `{"action":"claim"}`, token).Code != 200 {
		t.Fatal("claim failed")
	}
	assigned, _ := st.Get(t.Context(), f.ID)
	if assigned.AssignedTo != id {
		t.Fatal("unstable assignment")
	}
	w := req(s, "POST", "/api/admin/users/"+id+"/pin", "", admin(s))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var renewed map[string]json.RawMessage
	json.Unmarshal(w.Body.Bytes(), &renewed)
	var newPIN string
	json.Unmarshal(renewed["pin"], &newPIN)
	if req(s, "GET", "/api/families", "", token).Code != 401 {
		t.Fatal("old session survived reset")
	}
	if status, _ = pinLogin(s, pin); status != 401 {
		t.Fatal("old PIN survived reset")
	}
	status, newToken := pinLogin(s, newPIN)
	if status != 200 {
		t.Fatal(status)
	}
	assigned, _ = st.Get(t.Context(), f.ID)
	if assigned.AssignedTo != id {
		t.Fatal("assignment lost")
	}
	if req(s, "GET", "/api/families", "", newToken).Code != 200 {
		t.Fatal("new login failed")
	}
	if req(s, "POST", "/api/user/logout", "", newToken).Code != 204 {
		t.Fatal("logout failed")
	}
	if req(s, "GET", "/api/families", "", newToken).Code != 401 {
		t.Fatal("logout token still valid")
	}
}
func TestPINExpiryLockoutAndRevocation(t *testing.T) {
	for _, mode := range []string{"expiry", "attempts", "revoke"} {
		t.Run(mode, func(t *testing.T) {
			s, st, _ := setup(t)
			id, pin := createPINUser(t, s)
			switch mode {
			case "expiry":
				u, _ := st.GetUser(t.Context(), id)
				rev := u.Revision
				u.Revision++
				u.PINExpires = time.Now().Add(-time.Minute).Unix()
				st.SaveUser(t.Context(), u, rev)
			case "attempts":
				bad := "00000000"
				if pin == bad {
					bad = "11111111"
				}
				for i := 0; i < maxPINAttempts; i++ {
					if status, _ := pinLogin(s, bad); status != 401 {
						t.Fatal(status)
					}
				}
			case "revoke":
				if req(s, "DELETE", "/api/admin/users/"+id, "", admin(s)).Code != 204 {
					t.Fatal("revoke failed")
				}
			}
			if status, _ := pinLogin(s, pin); status != 401 {
				t.Fatal("unusable PIN accepted", status)
			}
		})
	}
}
func TestPINConcurrentSingleUse(t *testing.T) {
	s, _, _ := setup(t)
	_, pin := createPINUser(t, s)
	var wg sync.WaitGroup
	codes := make(chan int, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); status, _ := pinLogin(s, pin); codes <- status }()
	}
	wg.Wait()
	close(codes)
	success := 0
	for status := range codes {
		if status == 200 {
			success++
		} else if status != 401 && status != 429 {
			t.Fatal(status)
		}
	}
	if success != 1 {
		t.Fatalf("PIN accepted %d times", success)
	}
}
func TestPINConcurrentAttemptLimit(t *testing.T) {
	s, st, _ := setup(t)
	id, pin := createPINUser(t, s)
	bad := "00000000"
	if bad == pin {
		bad = "11111111"
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); pinLogin(s, bad) }()
	}
	wg.Wait()
	u, _ := st.GetUser(t.Context(), id)
	if u.Attempts != maxPINAttempts {
		t.Fatal(u.Attempts)
	}
	if status, _ := pinLogin(s, pin); status != 401 {
		t.Fatal(status)
	}
}

func TestRegenerateUnusedPINAndRestoreRevokedUser(t *testing.T) {
	s, st, _ := setup(t)
	id, oldPIN := createPINUser(t, s)
	if req(s, "DELETE", "/api/admin/users/"+id, "", admin(s)).Code != 204 {
		t.Fatal("revoke")
	}
	w := req(s, "POST", "/api/admin/users/"+id+"/pin", "", admin(s))
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	var result struct {
		PIN string `json:"pin"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	// A fresh Lambda process must see the same state, including revocation/reset.
	next := NewServer(st, s.key, s.origin)
	if status, _ := pinLogin(next, oldPIN); status != 401 {
		t.Fatal("old PIN accepted")
	}
	if status, _ := pinLogin(next, result.PIN); status != 200 {
		t.Fatal("restored login failed", status)
	}
}
