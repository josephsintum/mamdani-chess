package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/rules"
)

// createGame starts a friend game with the caller as White.
func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	g := s.games.Create(guestID(w, r))
	writeJSON(w, http.StatusCreated, map[string]string{"code": g.Code()})
}

// gameStream sends the caller's view of the game on connect and after every
// change. Opening it takes Black's seat if that is still free.
func (s *Server) gameStream(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.games.Get(r.PathValue("code"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	sub := g.Join(guest)
	defer g.Leave(sub)
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case v := <-sub.C:
			if writeEvent(w, fl, "state", v) != nil {
				return
			}
		case <-ticker.C:
			if writeHeartbeat(w, fl) != nil {
				return
			}
		}
	}
}

type moveRequest struct {
	game.MoveJSON
	Seq int `json:"seq"`
}

// gameMove plays the caller's move. The new state arrives on the stream.
func (s *Server) gameMove(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.games.Get(r.PathValue("code"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	var req moveRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
		return
	}
	m, ok := game.ParseMove(req.MoveJSON)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad move"})
		return
	}
	switch err := g.Move(guest, m, req.Seq); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, game.ErrNotPlayer):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, rules.ErrBadDie), errors.Is(err, game.ErrInternal):
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dice failed"})
	default:
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	}
}
