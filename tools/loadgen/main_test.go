package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/server"
	"mamdani-chess/store"
)

// odd always rolls 1: no potholes, so every game runs its full length.
type odd struct{}

func (odd) D8() int { return 1 }

// newServer runs the real HTTP server, behind wrap if it isn't nil, until
// the test ends, and returns its URL.
func newServer(t *testing.T, wrap func(http.Handler) http.Handler) string {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "load.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := server.New(st, game.NewHub(odd{}, st), fstest.MapFS{"index.html": {Data: []byte("app")}})
	var h http.Handler = s
	if wrap != nil {
		h = wrap(s)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	t.Cleanup(s.Close) // runs first: ends the streams, which ts.Close waits on
	return ts.URL
}

// TestLoadAgainstTheRealServer runs a small load through the real HTTP
// server, so the tool stays in step with the API it measures.
func TestLoadAgainstTheRealServer(t *testing.T) {
	url := newServer(t, nil)

	const games, spectators, plies = 5, 3, 10
	r := load(config{url: url, games: games, spectators: spectators, plies: plies, stall: 5 * time.Second})

	if r.errors != 0 {
		t.Fatalf("%d errors", r.errors)
	}
	if r.moves != games*plies {
		t.Errorf("moves %d, want %d", r.moves, games*plies)
	}
	// Every viewer sees every move.
	if want := games * (spectators + 2) * plies; len(r.viewLatency) < want {
		t.Errorf("viewer deliveries %d, want at least %d", len(r.viewLatency), want)
	}
	if r.peakStreams == 0 || r.peakStreams > games*(spectators+2) {
		t.Errorf("peak open streams %d, want 1..%d", r.peakStreams, games*(spectators+2))
	}
}

// A game loadgen gives up on (a move refused, then its resignation too)
// never ends, so its streams stay open. The run must still finish, and
// report the failures, instead of waiting on those streams for good.
func TestAGameThatNeverEndsDoesNotHangTheRun(t *testing.T) {
	url := newServer(t, func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && (strings.HasSuffix(r.URL.Path, "/move") || strings.HasSuffix(r.URL.Path, "/resign")) {
				http.Error(w, "refused for the test", http.StatusInternalServerError)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	done := make(chan result, 1)
	go func() {
		done <- load(config{url: url, games: 2, spectators: 2, plies: 10, stall: 300 * time.Millisecond})
	}()
	select {
	case r := <-done:
		if r.errors == 0 {
			t.Error("the refused moves weren't reported as errors")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the run hung on a game that never ended")
	}
}
