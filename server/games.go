package server

import (
	"errors"
	"net/http"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/rules"
)

// createGame starts a friend game with the caller as White.
func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	g, err := s.games.Create(guestID(w, r))
	if err != nil {
		s.internalError(w, "create game", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"code": g.Code()})
}

// createPractice starts a practice game, the caller playing both sides,
// and ends their previous one.
func (s *Server) createPractice(w http.ResponseWriter, r *http.Request) {
	g := s.games.CreatePractice(guestID(w, r))
	writeJSON(w, http.StatusCreated, map[string]string{"code": g.Code()})
}

// liveGamesMax is how many games the home page lists.
const liveGamesMax = 12

// liveGames lists games being played, most watched first, and how many
// guests are waiting in quick match.
func (s *Server) liveGames(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"games":   s.games.List(liveGamesMax),
		"looking": s.match.LookingFor(guestID(w, r)),
	})
}

// findGame returns the game named in the path, or answers 404 and reports
// false.
func (s *Server) findGame(w http.ResponseWriter, r *http.Request) (*game.Game, bool) {
	g, ok := s.games.Get(r.PathValue("code"))
	if !ok {
		writeError(w, http.StatusNotFound, "game not found")
	}
	return g, ok
}

// gameView returns the caller's view of the game without opening a stream
// or taking a seat. The page uses it to tell "game not found" from a
// server that is restarting.
func (s *Server) gameView(w http.ResponseWriter, r *http.Request) {
	g, ok := s.findGame(w, r)
	if !ok {
		return
	}
	v, err := g.View(guestID(w, r))
	if err != nil {
		writeError(w, http.StatusNotFound, "game not found")
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// gameStream sends the caller's view of the game on connect and after every
// change. Opening it takes Black's seat if that is still free.
func (s *Server) gameStream(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.findGame(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodHead {
		headSSE(w)
		return
	}
	sub, err := g.Join(guest)
	if err != nil { // the game stopped since Get
		writeError(w, http.StatusNotFound, "game not found")
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
			if writeEvent(w, fl, "state", v.JSON()) != nil {
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
	g, ok := s.findGame(w, r)
	if !ok {
		return
	}
	var req moveRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Seq == nil {
		writeError(w, http.StatusBadRequest, "bad request body")
		return
	}
	m, ok := game.ParseMove(req.MoveJSON)
	if !ok {
		writeError(w, http.StatusBadRequest, "bad move")
		return
	}
	writeGameResult(w, g, guest, g.Move(guest, m, *req.Seq))
}

// gameResign ends the game; the caller's opponent wins.
func (s *Server) gameResign(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.findGame(w, r)
	if !ok {
		return
	}
	writeGameResult(w, g, guest, g.Resign(guest))
}

type rematchRequest struct {
	Decline bool `json:"decline"`
}

// gameRematch offers a rematch, accepts the opponent's offer, or declines
// it. The outcome (and the new game's code) arrives on the stream.
func (s *Server) gameRematch(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.findGame(w, r)
	if !ok {
		return
	}
	var req rematchRequest
	if !decodeBody(w, r, &req) {
		return
	}
	writeGameResult(w, g, guest, g.Rematch(guest, req.Decline))
}

// writeGameResult maps the outcome of a move, resignation or rematch call
// to a response. Success is 204: the new state arrives on the stream. A
// conflict carries the caller's current view, so the client can resync at
// once.
func writeGameResult(w http.ResponseWriter, g *game.Game, guest string, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, game.ErrNotPlayer):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, game.ErrGone):
		writeError(w, http.StatusNotFound, "game not found")
	case errors.Is(err, rules.ErrBadDie), errors.Is(err, game.ErrInternal):
		writeError(w, http.StatusInternalServerError, "internal error")
	default:
		body := map[string]any{"error": err.Error()}
		if v, verr := g.View(guest); verr == nil {
			body["state"] = v
		}
		writeJSON(w, http.StatusConflict, body)
	}
}
