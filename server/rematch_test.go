package server

import (
	"encoding/json"
	"net/http"
	"testing"

	"mamdani-chess/game"
)

func TestRematchOverHTTP(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	a := alice.stream(code)
	a.state()
	b := bob.stream(code)
	b.state()
	a.state()
	c := carol.stream(code)
	c.state()
	rematch := "/api/games/" + code + "/rematch"

	if status, body := alice.post(rematch, `{}`); status != http.StatusConflict {
		t.Fatalf("rematch during the game: %d %s, want 409", status, body)
	}
	if status, body := alice.post("/api/games/"+code+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign: %d %s", status, body)
	}
	a.state()
	b.state()
	c.state()
	for _, tc := range []struct {
		who  *player
		body string
		want int
	}{
		{carol, `{}`, http.StatusForbidden},
		{alice, `not json`, http.StatusBadRequest},
		{alice, ``, http.StatusBadRequest},
	} {
		if status, body := tc.who.post(rematch, tc.body); status != tc.want {
			t.Errorf("rematch %q: %d %s, want %d", tc.body, status, body, tc.want)
		}
	}

	if status, body := alice.post(rematch, `{}`); status != http.StatusNoContent {
		t.Fatalf("offer: %d %s", status, body)
	}
	if v := b.state(); v.Rematch.Offer != "white" {
		t.Fatalf("bob sees rematch %+v", v.Rematch)
	}
	if status, body := bob.post(rematch, `{"decline":true}`); status != http.StatusNoContent {
		t.Fatalf("decline: %d %s", status, body)
	}
	a.state() // alice's own offer
	if v := a.state(); v.Rematch != (game.RematchJSON{Declined: true}) {
		t.Fatalf("alice sees rematch %+v", v.Rematch)
	}
	bob.post(rematch, `{}`)
	alice.post(rematch, `{}`) // accepts bob's offer
	var next string
	for next == "" { // a slow reader skips stale views, so read until the code shows
		next = c.state().Rematch.Code
	}
	if next == code {
		t.Fatal("the rematch reused the old code")
	}
	if v := bob.stream(next).state(); v.You != "white" || v.Status != game.Playing {
		t.Fatalf("bob in the rematch: you=%s status=%s", v.You, v.Status)
	}
}

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
	if status, _ := bob.get("/api/games/NOPE99"); status != http.StatusNotFound {
		t.Fatalf("unknown game: %d, want 404", status)
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
