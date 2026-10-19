package app

import "testing"

func TestPartialRSVPLifecycle(t *testing.T) {
	s, st, f := setup(t)
	path := "/api/invitations/" + f.ID + "/rsvp"
	post := func(body string) {
		t.Helper()
		w := req(s, "POST", path, body, "")
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	post(`{"rsvp":"yes"}`)
	got, _ := st.Get(t.Context(), f.ID)
	if got.RSVP != "yes" || !got.HeadcountUnknown || got.Adults != 0 || got.Children != 0 {
		t.Fatal(got)
	}
	post(`{"rsvp":"yes","adults":2,"children":1}`)
	post(`{"rsvp":"yes"}`)
	got, _ = st.Get(t.Context(), f.ID)
	if got.HeadcountUnknown || got.Adults != 2 || got.Children != 1 {
		t.Fatal("partial retry erased completed response", got)
	}
	post(`{"rsvp":"no"}`)
	got, _ = st.Get(t.Context(), f.ID)
	if got.HeadcountUnknown || got.RSVP != "no" || got.Adults != 0 || got.Children != 0 {
		t.Fatal(got)
	}
	post(`{"rsvp":"yes"}`)
	got, _ = st.Get(t.Context(), f.ID)
	if !got.HeadcountUnknown || got.RSVP != "yes" {
		t.Fatal(got)
	}
	for _, body := range []string{`{"rsvp":"yes","adults":0,"children":0}`, `{"rsvp":"yes","adults":1}`, `{"rsvp":"yes","adults":-1,"children":1}`} {
		if w := req(s, "POST", path, body, ""); w.Code != 400 {
			t.Fatal(body, w.Code)
		}
	}
}
