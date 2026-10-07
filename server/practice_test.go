package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// A practice game: the caller moves for both sides; nobody else can, and
// the live games list doesn't show it.
func TestPracticeOverHTTP(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	status, body := alice.post("/api/practice", "")
	var out struct{ Code string }
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &out) != nil || len(out.Code) != 6 {
		t.Fatalf("POST /api/practice: %d %s", status, body)
	}
	moves := "/api/games/" + out.Code + "/move"
	if status, body := alice.post(moves, `{"from":"e2","to":"e4","seq":0}`); status != http.StatusNoContent {
		t.Fatalf("White's move: %d %s", status, body)
	}
	if status, body := bob.post(moves, `{"from":"e7","to":"e5","seq":1}`); status != http.StatusForbidden {
		t.Errorf("another guest's move: %d %s, want 403", status, body)
	}
	if status, body := alice.post(moves, `{"from":"e7","to":"e5","seq":1}`); status != http.StatusNoContent {
		t.Fatalf("Black's move by the same guest: %d %s", status, body)
	}
	if _, list := alice.get("/api/games"); strings.Contains(list, out.Code) {
		t.Errorf("the live games list shows the practice game: %s", list)
	}
	if _, view := alice.get("/api/games/" + out.Code); !strings.Contains(view, `"practice":true`) {
		t.Errorf("view lacks practice: %s", view)
	}
}
