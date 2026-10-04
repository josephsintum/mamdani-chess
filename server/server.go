// Package server is the HTTP layer: routes, SSE and the static frontend.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"mamdani-chess/store"
)

// Server routes API and frontend requests.
type Server struct {
	store     *store.Store
	assets    fs.FS
	honks     *broadcaster
	heartbeat time.Duration
	mux       *http.ServeMux
}

// New returns a Server backed by st that serves the frontend from assets.
func New(st *store.Store, assets fs.FS) *Server {
	s := &Server{
		store:     st,
		assets:    assets,
		honks:     newBroadcaster(),
		heartbeat: 15 * time.Second,
		mux:       http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/honk", s.honk)
	s.mux.HandleFunc("GET /api/honk/stream", s.honkStream)
	s.mux.HandleFunc("/api/", s.apiNotFound)
	s.mux.HandleFunc("/", s.static)
	return s
}

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
