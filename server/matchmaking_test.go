package server

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Every search is counted: matched or not, and whether anyone else was
// looking when it started.
func TestSearchLogged(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	// Alice searches alone and gives up.
	ctx, cancel := context.WithCancel(t.Context())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/match", nil)
	resp, err := alice.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sseReader{t: t, sc: bufio.NewScanner(resp.Body)}.next() // queued
	time.Sleep(20 * time.Millisecond)
	cancel()
	// The log is written as each stream ends.
	for deadline := time.Now().Add(2 * time.Second); counted(t, s)["search"] < 1 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	// Bob and Carol find each other.
	bs, _ := bob.queue()
	cs, _ := carol.queue()
	if matchedCode(bs) == "" || matchedCode(cs) == "" {
		t.Fatal("no match")
	}
	var counts map[string]int
	for deadline := time.Now().Add(2 * time.Second); counts["search"] < 3 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
		counts = counted(t, s)
	}
	// Carol saw Bob looking; Alice and Bob saw nobody.
	if counts["search"] != 3 || counts["search.matched"] != 2 || counts["search.gaveup"] != 1 || counts["search.alone"] != 2 {
		t.Fatalf("counters %v", counts)
	}
}

func TestLiveGamesIsEmptyJSONList(t *testing.T) {
	_, ts := newTestServer(t)
	status, body := newPlayer(t, ts).get("/api/games")
	if status != http.StatusOK || body != "{\"games\":[],\"looking\":0}\n" {
		t.Fatalf("GET /api/games: %d %q", status, body)
	}
}

func TestClosingTheStreamLeavesTheQueue(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	resp, err := alice.c.Get(ts.URL + "/api/match")
	if err != nil {
		t.Fatal(err)
	}
	r := sseReader{t: t, sc: bufio.NewScanner(resp.Body)}
	if ev, _ := r.next(); ev != "queued" {
		t.Fatalf("got %q", ev)
	}
	resp.Body.Close() // Cancel, or a closed tab
	deadline := time.Now().Add(2 * time.Second)
	for s.match.Looking() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("still looking 2s after the stream closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLowercaseCodeFindsTheGame(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	code := alice.create()
	if status, body := alice.get("/api/games/" + strings.ToLower(code)); status != http.StatusOK {
		t.Fatalf("GET with a lowercase code: %d %s", status, body)
	}
}
