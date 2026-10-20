package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
)

func TestPracticeAndReconnectEvents(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, _ := alice.post("/api/practice", ""); status != http.StatusCreated {
		t.Fatalf("practice: %d", status)
	}
	code := alice.create()
	alice.stream(code).state()
	resp, err := alice.c.Get(ts.URL + "/api/games/" + code + "/stream?again=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	counts := counted(t, s)
	if counts["event.practice"] != 1 || counts["event.reconnect"] != 1 || counts["event.restart"] != 0 {
		t.Fatalf("counters %v", counts)
	}
}

// A playtest's games are marked, its searches and events aren't logged,
// and the stats count none of it.
func TestPlaytestGamesAreNotCounted(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob := newPlaytester(t, ts), newPlaytester(t, ts)
	code := matchPair(t, alice, bob)
	if kind, playtest := kindOf(t, s, code); kind != "quick" || !playtest {
		t.Fatalf("quick match: kind %q playtest %v, want quick, true", kind, playtest)
	}
	if status, body := alice.post("/api/games/"+code+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign: %d %s", status, body)
	}
	next := rematchOf(t, code, alice, bob)
	if kind, playtest := kindOf(t, s, next); kind != "quick" || !playtest {
		t.Fatalf("rematch: kind %q playtest %v, want quick, true", kind, playtest)
	}
	friend := newPlaytester(t, ts).create()
	if kind, playtest := kindOf(t, s, friend); kind != "friend" || !playtest {
		t.Fatalf("friend link: kind %q playtest %v, want friend, true", kind, playtest)
	}
	if status, _ := alice.post("/api/practice", ""); status != http.StatusCreated {
		t.Fatalf("practice: %d", status)
	}
	resp, err := bob.c.Get(ts.URL + "/api/games/" + friend + "/stream?again=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	// A playtest writes no counters at all, and every search and event
	// that is logged is counted, so none was logged either.
	if counts := counted(t, s); len(counts) != 0 {
		t.Fatalf("the stats count a playtest: %v", counts)
	}
}

// A game that ends saves its stats row with the result; every game is
// saved with its kind, and a rematch keeps it. (An aborted game's missing
// row is in game's TestAbortedGameSavesNoStatsRow.)
func TestGameEndSavesStatsAndKind(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := matchPair(t, alice, bob)
	if status, body := alice.post("/api/games/"+code+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign: %d %s", status, body)
	}
	if kind, playtest := kindOf(t, s, code); kind != "quick" || playtest || !hasStats(t, s, code) {
		t.Fatalf("quick match: kind %q playtest %v stats %v, want quick, false, true", kind, playtest, hasStats(t, s, code))
	}
	next := rematchOf(t, code, alice, bob)
	if kind, _ := kindOf(t, s, next); kind != "quick" {
		t.Fatalf("rematch of a quick match: kind %q", kind)
	}
	carol := newPlayer(t, ts)
	if kind, playtest := kindOf(t, s, carol.create()); kind != "friend" || playtest {
		t.Fatalf("friend link: kind %q playtest %v", kind, playtest)
	}

	// A practice game is never saved, so it has no row of either kind.
	status, body := carol.post("/api/practice", "")
	var out struct{ Code string }
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &out) != nil {
		t.Fatalf("practice: %d %s", status, body)
	}
	practice := out.Code
	if status, body := carol.post("/api/games/"+practice+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign practice: %d %s", status, body)
	}
	if _, _, err := s.store.GameKind(t.Context(), practice); !errors.Is(err, sql.ErrNoRows) || hasStats(t, s, practice) {
		t.Fatalf("practice game saved: %v", err)
	}
}
