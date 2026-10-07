package server

import (
	"errors"
	"net/http"
	"time"

	"mamdani-chess/names"
	"mamdani-chess/store"
)

// MeJSON is the caller's name and name changes on the wire. ChangesResetAt
// is when they get store.NameChanges again (Unix ms), or null while none
// are used.
type MeJSON struct {
	Name           *string `json:"name"`
	ChangesLeft    int     `json:"changesLeft"`
	ChangesResetAt *int64  `json:"changesResetAt"`
	// Game is the code of the game the caller is playing right now, absent
	// when they aren't, so a page can offer to take them back to it.
	Game *string `json:"game,omitempty"`
}

func newMeJSON(name string, a store.Allowance) MeJSON {
	out := MeJSON{ChangesLeft: a.Left}
	if name != "" {
		out.Name = &name
	}
	if !a.ResetAt.IsZero() {
		ms := a.ResetAt.UnixMilli()
		out.ChangesResetAt = &ms
	}
	return out
}

// me returns the caller's name, or null if they have none yet, and their
// name changes left today. It never creates a name: a guest gets one when
// they first play.
func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	name, a, err := s.store.Guest(r.Context(), guest, time.Now())
	if err != nil {
		s.internalError(w, "guest name", err)
		return
	}
	out := newMeJSON(name, a)
	if code := s.games.Active(guest); code != "" {
		out.Game = &code
	}
	writeJSON(w, http.StatusOK, out)
}

// nameOffers returns the names the caller may change to. They stay the same
// until one is chosen. 409 before the caller has played; 429 once today's
// changes are used, with when they come back.
func (s *Server) nameOffers(w http.ResponseWriter, r *http.Request) {
	offers, a, err := s.store.NameOffers(r.Context(), guestID(w, r), names.Random, time.Now())
	if s.nameRefused(w, a, err) {
		return
	}
	out := newMeJSON("", a)
	writeJSON(w, http.StatusOK, map[string]any{
		"names": offers, "changesLeft": out.ChangesLeft, "changesResetAt": out.ChangesResetAt,
	})
}

type nameRequest struct {
	Name string `json:"name"`
}

// chooseName changes the caller's name to one of their offers, using one of
// today's changes. 409 for a name that isn't on offer; 429 once today's
// changes are used.
func (s *Server) chooseName(w http.ResponseWriter, r *http.Request) {
	var req nameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	a, err := s.store.ChooseName(r.Context(), guestID(w, r), req.Name, time.Now())
	if s.nameRefused(w, a, err) {
		return
	}
	writeJSON(w, http.StatusOK, newMeJSON(req.Name, a))
}

// nameRefused writes the answer for a refused name change and reports
// whether it did.
func (s *Server) nameRefused(w http.ResponseWriter, a store.Allowance, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, store.ErrNoChanges):
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error": err.Error(), "changesResetAt": a.ResetAt.UnixMilli(),
		})
	case errors.Is(err, store.ErrNoName), errors.Is(err, store.ErrNotOffered):
		writeError(w, http.StatusConflict, err.Error())
	default:
		s.internalError(w, "name change", err)
	}
	return true
}
