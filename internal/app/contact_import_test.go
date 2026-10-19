package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
)

func contactRequest(s *Server, contacts []ContactInput, token string) (int, []ContactImportResult) {
	b, _ := json.Marshal(map[string]any{"contacts": contacts})
	w := req(s, "POST", "/api/families/import", string(b), token)
	var out struct {
		Results []ContactImportResult `json:"results"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out.Results
}
func TestContactImportAndPreserveExisting(t *testing.T) {
	s, st, old := setup(t)
	id, pin := createPINUser(t, s)
	_, token := pinLogin(s, pin)
	old.RSVP = "yes"
	old.Status = "sent"
	old.AssignedTo = "someone-else"
	old.Revision++
	if err := st.Save(t.Context(), old, 1); err != nil {
		t.Fatal(err)
	}
	contacts := []ContactInput{
		{Name: "New family", Phone: "+52 (656) 222-3344", Email: "family@example.com"},
		{Name: "Do not overwrite", Phone: old.Phone},
		{Name: "Repeated", Phone: "+5216562223344"},
		{Name: "Invalid", Phone: "123"},
		{Name: "Wrong email", Phone: "+526562223355", Email: "not an email"},
	}
	status, results := contactRequest(s, contacts, token)
	if status != 200 || len(results) != 5 {
		t.Fatal(status, results)
	}
	for i, want := range []string{"added", "existing", "existing", "invalid", "invalid"} {
		if results[i].Index != i || results[i].Status != want {
			t.Fatal(results)
		}
	}
	added, err := st.Get(t.Context(), results[0].FamilyID)
	if err != nil || added.AssignedTo != id || added.CreatedBy != id || added.AssignedName != "david" || added.Phone != "+526562223344" || added.RSVP != "pending" || added.Status != "pending" {
		t.Fatal(added, err)
	}
	if !idPattern.MatchString(added.ID) || len(added.History) != 1 || added.History[0].Status != "added" {
		t.Fatal(added)
	}
	preserved, _ := st.Get(t.Context(), old.ID)
	if preserved.Name != old.Name || preserved.Status != "sent" || preserved.RSVP != "yes" || preserved.AssignedTo != "someone-else" || preserved.Revision != old.Revision {
		t.Fatal("existing family changed", preserved)
	}
	status, again := contactRequest(s, contacts[:1], token)
	if status != 200 || again[0].Status != "existing" || again[0].FamilyID != added.ID {
		t.Fatal(again)
	}
}
func TestContactImportAuthorizationAndLimits(t *testing.T) {
	s, _, _ := setup(t)
	c := []ContactInput{{Name: "Test", Phone: "+526561110000"}}
	if status, _ := contactRequest(s, c, ""); status != 401 {
		t.Fatal(status)
	}
	if status, _ := contactRequest(s, nil, admin(s)); status != 400 {
		t.Fatal(status)
	}
	if status, _ := contactRequest(s, make([]ContactInput, 21), admin(s)); status != 400 {
		t.Fatal(status)
	}
	if req(s, "POST", "/api/families/import", `{"contacts":[{"name":"Bad","phone":"+526561110000","assignedTo":"admin"}]}`, admin(s)).Code != 400 {
		t.Fatal("accepted injected ownership")
	}
}
func TestConcurrentContactImports(t *testing.T) {
	s, st, _ := setup(t)
	id, pin := createPINUser(t, s)
	_, token := pinLogin(s, pin)
	c := []ContactInput{{Name: "Same family", Phone: "+526561110000"}}
	codes := make(chan string, 12)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status, res := contactRequest(s, c, token)
			if status != 200 {
				codes <- fmt.Sprint(status)
				return
			}
			codes <- res[0].Status
		}()
	}
	wg.Wait()
	close(codes)
	count := 0
	for status := range codes {
		if status == "added" {
			count++
		} else if status != "existing" {
			t.Fatal(status)
		}
	}
	if count != 1 {
		t.Fatal(count)
	}
	fs, _ := st.List(t.Context())
	for _, f := range fs {
		if f.Phone == c[0].Phone && f.AssignedTo != id {
			t.Fatal(f)
		}
	}
}
func TestNormalizePhone(t *testing.T) {
	for raw, want := range map[string]string{
		"+52 (656) 123-4567": "+526561234567",
		"+5216561234567":     "+526561234567",
		"00526561234567":     "+526561234567",
		"+1 (915) 555-1234":  "+19155551234",
	} {
		got, err := NormalizePhone(raw)
		if err != nil || got != want {
			t.Fatal(raw, got, err)
		}
	}
	for _, raw := range []string{"6561234567", "+52abc6561234567", "+526561234567 ext 3", "++526561234567", "+012345678", ""} {
		if _, err := NormalizePhone(raw); err == nil {
			t.Fatal(raw)
		}
	}
}

type uncertainCreateStore struct {
	*MemoryStore
	once bool
}

func (st *uncertainCreateStore) Create(ctx context.Context, f Family) error {
	err := st.MemoryStore.Create(ctx, f)
	if err == nil && !st.once {
		st.once = true
		return errors.New("response lost")
	}
	return err
}
func TestImportRetryAfterCommittedButLostResponse(t *testing.T) {
	st := &uncertainCreateStore{MemoryStore: NewMemoryStore()}
	s := NewServer(st, "a-very-long-test-admin-secret-32bytes", "https://xitlaliabigail.com")
	contacts := []ContactInput{{Name: "Family", Phone: "+526561234567"}}
	_, first := contactRequest(s, contacts, admin(s))
	if first[0].Status != "error" {
		t.Fatal(first)
	}
	_, second := contactRequest(s, contacts, admin(s))
	if second[0].Status != "existing" {
		t.Fatal(second)
	}
	fs, _ := st.List(t.Context())
	if len(fs) != 1 {
		t.Fatal(len(fs))
	}
}
