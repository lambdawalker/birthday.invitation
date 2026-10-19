package app

import (
	"errors"
	"testing"
)

func TestDeleteFamily(t *testing.T) {
	s, st, f := setup(t)
	id, pin := createPINUser(t, s)
	_, token := pinLogin(s, pin)
	path := "/api/families/" + f.ID
	if w := req(s, "DELETE", path, "", ""); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := req(s, "DELETE", path, "", token); w.Code != 403 {
		t.Fatal(w.Code)
	}
	f.AssignedTo = id
	f.Revision++
	st.Save(t.Context(), f, 1)
	for _, status := range []string{"sending", "uncertain", "whatsapp_opened"} {
		old := f.Revision
		f.Status = status
		f.Revision++
		st.Save(t.Context(), f, old)
		if w := req(s, "DELETE", path, "", token); w.Code != 409 {
			t.Fatal(status, w.Code)
		}
	}
	old := f.Revision
	f.Status = "sent"
	f.Revision++
	st.Save(t.Context(), f, old)
	if w := req(s, "DELETE", path, "", token); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if _, err := st.Get(t.Context(), f.ID); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if w := req(s, "GET", "/api/invitations/"+f.ID, "", ""); w.Code != 404 {
		t.Fatal(w.Code)
	}
	_, results := contactRequest(s, []ContactInput{{Name: "Re-added", Phone: f.Phone}}, token)
	if len(results) != 1 || results[0].Status != "added" || results[0].FamilyID == f.ID {
		t.Fatal(results)
	}
}

func TestDeleteRevisionConflict(t *testing.T) {
	_, st, stale := setup(t)
	updated := stale
	updated.Revision++
	updated.RSVP = "yes"
	if err := st.Save(t.Context(), updated, stale.Revision); err != nil {
		t.Fatal(err)
	}
	if err := st.Delete(t.Context(), stale); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := st.Get(t.Context(), stale.ID); err != nil {
		t.Fatal(err)
	}
}
