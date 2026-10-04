package server

import (
	"net/http"
	"time"
)

// The honk counter proves the full loop (POST → SQLite → SSE → every tab).
// Throwaway: removed by the game-server plan.

type honkCount struct {
	Count int64 `json:"count"`
}

func (s *Server) honk(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.Honk(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "honk failed"})
		return
	}
	s.honks.publish(n)
	writeJSON(w, http.StatusOK, honkCount{n})
}

func (s *Server) honkStream(w http.ResponseWriter, r *http.Request) {
	// Subscribe before reading the current value so no honk slips between.
	updates, cancel := s.honks.subscribe()
	defer cancel()
	current, err := s.store.Honks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read failed"})
		return
	}
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if writeEvent(w, fl, "honk", honkCount{current}) != nil {
		return
	}
	last := current
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case n := <-updates:
			if n <= last { // already sent a newer count
				continue
			}
			last = n
			if writeEvent(w, fl, "honk", honkCount{n}) != nil {
				return
			}
		case <-ticker.C:
			if writeHeartbeat(w, fl) != nil {
				return
			}
		}
	}
}
