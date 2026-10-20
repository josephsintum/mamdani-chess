package server

import (
	"net/http"
	"testing"
)

func TestLookingAroundNeedsNoName(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	bob.stream(code).state()

	if status, body := carol.get("/api/games"); status != http.StatusOK {
		t.Fatalf("GET /api/games: %d %s", status, body)
	}
	if v := carol.stream(code).state(); v.You != "spectator" {
		t.Fatalf("carol: you=%s", v.You)
	}
	if carol.name() != "" {
		t.Fatal("carol got a name by looking around")
	}
	if alice.name() == "" || bob.name() == "" {
		t.Fatal("creating a game and taking a seat should each give a name")
	}
}
