package main

import (
	"net/http/httptest"
	"path/filepath"
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

// TestLoadAgainstTheRealServer runs a small load through the real HTTP
// server, so the tool stays in step with the API it measures.
func TestLoadAgainstTheRealServer(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "load.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	s := server.New(st, game.NewHub(odd{}, st), fstest.MapFS{"index.html": {Data: []byte("app")}})
	ts := httptest.NewServer(s)
	defer ts.Close()
	defer s.Close()

	const games, spectators, plies = 5, 3, 10
	r := load(config{url: ts.URL, games: games, spectators: spectators, plies: plies, stall: 5 * time.Second})

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
