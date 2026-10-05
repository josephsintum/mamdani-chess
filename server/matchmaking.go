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
	guest := guestID(w, r)
	if _, err := s.store.EnsureGuest(r.Context(), guest, names.Random); err != nil {
		s.log.Error("guest name", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	t := s.match.Join(guest)
	defer t.Leave()
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
