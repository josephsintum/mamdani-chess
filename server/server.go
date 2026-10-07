// Package server is the HTTP layer: routes, SSE and the static frontend.
package server

import (
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/match"
	"mamdani-chess/store"
)

// Server routes API and frontend requests.
type Server struct {
	store     *store.Store
	games     *game.Hub
	match     *match.Queue
	assets    fs.FS
	etags     sync.Map // etagKey → ETag of a static file
	heartbeat time.Duration
	slow      time.Duration // a request taking longer logs at WARN
	log       *slog.Logger
	mux       *http.ServeMux
	done      chan struct{} // closed by Close to end SSE streams
	closeOnce sync.Once
	// Version is the deployed build (a commit), reported by /healthz so a
	// deploy can be checked without touching game data. "" reads as "dev".
	Version string
}

// New returns a Server for the games in hub that serves the frontend from
// assets. st is the database the hub saves games to.
func New(st *store.Store, hub *game.Hub, assets fs.FS) *Server {
	s := &Server{
		store:     st,
		games:     hub,
		assets:    assets,
		heartbeat: 15 * time.Second,
		slow:      time.Second,
		log:       slog.Default(),
		mux:       http.NewServeMux(),
		done:      make(chan struct{}),
	}
	s.match = match.New(func(white, black string) (string, error) {
		g, err := hub.CreatePair(white, black)
		if err != nil {
			return "", err
		}
		return g.Code(), nil
	})
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /api/me", s.me)
	s.mux.HandleFunc("GET /api/me/names", s.nameOffers)
	s.mux.HandleFunc("POST /api/me/name", s.chooseName)
	s.mux.HandleFunc("GET /api/match", s.matchStream)
	s.mux.HandleFunc("GET /api/games", s.liveGames)
	s.mux.HandleFunc("POST /api/games", s.createGame)
	s.mux.HandleFunc("POST /api/practice", s.createPractice)
	s.mux.HandleFunc("GET /api/games/{code}", s.gameView)
	s.mux.HandleFunc("GET /api/games/{code}/stream", s.gameStream)
	s.mux.HandleFunc("POST /api/games/{code}/move", s.gameMove)
	s.mux.HandleFunc("POST /api/games/{code}/resign", s.gameResign)
	s.mux.HandleFunc("POST /api/games/{code}/rematch", s.gameRematch)
	s.mux.HandleFunc("/api/", s.apiNotFound)
	s.mux.HandleFunc("/", s.static)
	return s
}

// Close ends every open SSE stream. Other in-flight requests are unaffected.
// Safe to call more than once.
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.done) }) }

// ServeHTTP routes the request and logs one line for it: method, path,
// status and how long it took.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	d := time.Since(start)
	// Successful requests log at DEBUG: at INFO, a first visit's 30 or so
	// files would use Railway's 500 log lines/s with about 17 new visitors
	// a second. Slow requests log at WARN, failed or not (a stream stays
	// open by design, so it is never slow); refusals and errors at INFO.
	// The game logs its own events (created, ended…) at INFO.
	level := slog.LevelDebug
	switch {
	case d > s.slow && rec.Header().Get("Content-Type") != "text/event-stream":
		level = slog.LevelWarn
	case rec.status >= 400:
		level = slog.LevelInfo
	}
	s.log.Log(r.Context(), level, "request",
		"method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", d)
}

// statusRecorder remembers the status a handler wrote. It passes Flush
// through, so SSE streams still work behind it.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(b)
}

// WriteString passes strings through without converting them to bytes, so
// writing an SSE event doesn't allocate.
func (r *statusRecorder) WriteString(s string) (int, error) {
	r.wrote = true
	return io.WriteString(r.ResponseWriter, s)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	version := s.Version
	if version == "" {
		version = "dev"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
}

func (s *Server) apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes {"error": msg}.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// decodeBody reads a small JSON body into v, or answers 400 and reports
// false.
func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "bad request body")
		return false
	}
	return true
}

func (s *Server) internalError(w http.ResponseWriter, what string, err error) {
	s.log.Error(what, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}
