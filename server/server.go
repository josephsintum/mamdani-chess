// Package server is the HTTP layer: routes, SSE and the static frontend.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/store"
)

// Server routes API and frontend requests.
type Server struct {
	store     *store.Store
	games     *game.Hub
	assets    fs.FS
	heartbeat time.Duration
	mux       *http.ServeMux
	done      chan struct{} // closed by Close to end SSE streams
	closeOnce sync.Once
}

// New returns a Server for the games in hub that serves the frontend from
// assets. st is unused until games are saved (a later milestone).
func New(st *store.Store, hub *game.Hub, assets fs.FS) *Server {
	s := &Server{
		store:     st,
		games:     hub,
		assets:    assets,
		heartbeat: 15 * time.Second,
		mux:       http.NewServeMux(),
		done:      make(chan struct{}),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/games", s.createGame)
	s.mux.HandleFunc("GET /api/games/{code}/stream", s.gameStream)
	s.mux.HandleFunc("POST /api/games/{code}/move", s.gameMove)
	s.mux.HandleFunc("POST /api/games/{code}/resign", s.gameResign)
	s.mux.HandleFunc("/api/", s.apiNotFound)
	s.mux.HandleFunc("/", s.static)
	return s
}

// Close ends every open SSE stream. Other in-flight requests are unaffected.
// Safe to call more than once.
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.done) }) }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
