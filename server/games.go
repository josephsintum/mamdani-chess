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
	sub, err := g.Join(guest)
	if err != nil { // the game stopped since Get
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	defer g.Leave(sub)
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
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
		case v, open := <-sub.C:
			if !open { // the game stopped
				return
			}
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
	Seq *int `json:"seq"` // required: a move must say which position it was made from
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Seq == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
		return
	}
	m, ok := game.ParseMove(req.MoveJSON)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad move"})
		return
	}
	writeGameResult(w, g, guest, g.Move(guest, m, *req.Seq))
}

// gameResign ends the game; the caller's opponent wins.
func (s *Server) gameResign(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.games.Get(r.PathValue("code"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	writeGameResult(w, g, guest, g.Resign(guest))
}

// writeGameResult maps the outcome of a move or resignation to a response.
// Success is 204: the new state arrives on the stream. A conflict carries
// the caller's current view, so the client can resync at once.
func writeGameResult(w http.ResponseWriter, g *game.Game, guest string, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, game.ErrNotPlayer):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, game.ErrGone):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
	case errors.Is(err, rules.ErrBadDie), errors.Is(err, game.ErrInternal):
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	default:
		body := map[string]any{"error": err.Error()}
		if v, verr := g.View(guest); verr == nil {
			body["state"] = v
		}
		writeJSON(w, http.StatusConflict, body)
	}
}
