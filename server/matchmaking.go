package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"mamdani-chess/names"
	"mamdani-chess/store"
)

// logSearch writes one quick-match search. A failure is logged, never
// shown: the search itself has already happened.
func (s *Server) logSearch(guest string, started time.Time, matched bool, others int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.store.AddSearch(ctx, store.Search{StartedAt: started, EndedAt: time.Now(), Guest: visitorOf(guest), Matched: matched, Others: others}); err != nil {
		s.log.Error("log search", "err", err)
	}
}

// logEvent counts one event (a practice game, a reopened game stream) that
// r caused, unless a playtest sent r. Failures are logged, never shown.
func (s *Server) logEvent(r *http.Request, kind string) {
	if isRobot(r) {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.store.AddEvent(ctx, kind, time.Now()); err != nil {
		s.log.Error("log event", "kind", kind, "err", err)
	}
}

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
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
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
			s.logSearch(guest, started, matched, others)
		}
	}()
	queued, _ := json.Marshal(map[string]int{"looking": s.match.Looking()})
	if writeEvent(w, fl, "queued", queued) != nil {
		return
	}
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case code := <-t.C:
			// The stream ends here. Without the long retry, EventSource
			// would reconnect and queue the guest again before the page
			// has gone to the game.
			matched = true
			data, _ := json.Marshal(map[string]string{"code": code})
			writeLastEvent(w, fl, "matched", data, time.Hour)
			return
		case <-ticker.C:
			if writeHeartbeat(w, fl) != nil {
				return
			}
		}
	}
}
