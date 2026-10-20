package server

import (
	"encoding/json"
	"net/http"
	"time"

	"mamdani-chess/names"
)

// matchStream puts the caller in the quick-match queue for as long as the
// stream is open. It sends "queued" at once, then "matched" with the new
// game's code. The caller gets a name first, so the game can show it.
func (s *Server) matchStream(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		headSSE(w) // no place in line, and no name
		return
	}
	guest := guestID(w, r)
	// One game at a time: a guest already playing is sent back to it.
	if code := s.games.Active(guest); code != "" {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "you're already in a game", "code": code})
		return
	}
	if _, err := s.store.EnsureGuest(r.Context(), guest, names.Random); err != nil {
		s.internalError(w, "guest name", err)
		return
	}
	fl := startSSE(w)
	started := time.Now()
	others := s.match.LookingFor(guest)
	// A playtest's game is marked and its search isn't logged, so the
	// stats leave both out.
	playtest := isRobot(r)
	t := s.match.Join(guest, playtest)
	matched := false
	defer func() {
		t.Leave()
		if !playtest {
			s.logSearch(started, matched, others)
		}
	}()
	queued, _ := json.Marshal(map[string]int{"looking": s.match.Looking()})
	if writeEvent(w, fl, "queued", queued) != nil {
		return
	}
	pump(s, r, w, fl, t.C, func(code string) bool {
		// The stream ends here. Without the long retry, EventSource would
		// reconnect and queue the guest again before the page has gone to
		// the game.
		matched = true
		data, _ := json.Marshal(map[string]string{"code": code})
		writeLastEvent(w, fl, "matched", data, time.Hour)
		return false
	})
}
