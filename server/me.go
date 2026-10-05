package server

import (
	"net/http"

	"mamdani-chess/names"
)

// me returns the caller's name, or null if they have none yet. It never
// creates one: a guest gets a name when they first play.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	name, err := s.store.GuestName(r.Context(), guestID(w, r))
	if err != nil {
		s.log.Error("guest name", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	var out struct {
		Name *string `json:"name"`
	}
	if name != "" {
		out.Name = &name
	}
	writeJSON(w, http.StatusOK, out)
}

// rerollName gives the caller a new name (their first, if they had none).
func (s *Server) rerollName(w http.ResponseWriter, r *http.Request) {
	name, err := s.store.RerollGuest(r.Context(), guestID(w, r), names.Random)
	if err != nil {
		s.log.Error("reroll name", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"name": name})
}
