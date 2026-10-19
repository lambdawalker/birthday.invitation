package app

import (
	"errors"
	"net/http"
)

func (s *Server) deleteFamily(w http.ResponseWriter, r *http.Request) {
	actor, ok := s.actor(r)
	if !ok {
		fail(w, 401, "Sign in before deleting a family")
		return
	}
	f, ok := s.family(w, r)
	if !ok {
		return
	}
	if actor.ID != "admin" && f.AssignedTo != actor.ID {
		fail(w, 403, "Only the assigned user or administrator can delete this family")
		return
	}
	if f.Status == "sending" || f.Status == "uncertain" || f.Status == "whatsapp_opened" {
		fail(w, 409, "Resolve the pending send before deleting this family")
		return
	}
	err := s.store.Delete(r.Context(), f)
	if errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) {
		fail(w, 409, "The family changed. Refresh the roster before deleting")
		return
	}
	if err != nil {
		fail(w, 503, "Could not confirm deletion. Refresh the roster before retrying")
		return
	}
	send(w, 200, map[string]bool{"deleted": true})
}
