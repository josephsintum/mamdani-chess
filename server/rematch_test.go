package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"mamdani-chess/game"
)

func TestGameViewDoesNotTakeASeat(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	status, body := bob.get("/api/games/" + code)
	var v game.View
	if err := json.Unmarshal([]byte(body), &v); status != http.StatusOK || err != nil || v.You != "spectator" || v.Status != game.Waiting {
		t.Fatalf("view: %d %v you=%s status=%s", status, err, v.You, v.Status)
	}
	if v := bob.stream(code).state(); v.You != "black" {
		t.Fatalf("bob's stream: you=%s; looking at the game shouldn't have used up the seat", v.You)
	}
}

func TestStateCarriesTheClock(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	a := alice.stream(code)
	a.state()
	bob.stream(code).state()
	if v := a.state(); v.Clock.FirstMoveDeadline == 0 || v.Clock.Running != "" || v.Clock.WhiteMS != 600_000 {
		t.Fatalf("after bob joins: clock %+v", v.Clock)
	}
	alice.post("/api/games/"+code+"/move", `{"from":"e2","to":"e4","seq":0}`)
	bob.post("/api/games/"+code+"/move", `{"from":"e7","to":"e5","seq":1}`)
	a.state()
	if v := a.state(); v.Clock.Running != "white" || v.Clock.Since <= v.Clock.Now {
		t.Fatalf("after both first moves: clock %+v, want white's clock starting after the dice pause", v.Clock)
	}
}
