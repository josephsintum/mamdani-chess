package game

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

// sameGame compares two views of a game, leaving out the clock's times,
// which a restore restarts.
func sameGame(t *testing.T, got, want *View) {
	t.Helper()
	g, w := *got, *want
	g.Clock, w.Clock = ClockJSON{}, ClockJSON{}
	g.data, w.data = nil, nil
	gj, _ := json.Marshal(g)
	wj, _ := json.Marshal(w)
	if string(gj) != string(wj) {
		t.Fatalf("restored view differs:\n got %s\nwant %s", gj, wj)
	}
}

func TestRestoreRebuildsAGameInProgress(t *testing.T) {
	st := openStore(t)
	// Pothole rolls on every turn, so the restore must replay the dice.
	dice := &script{rolls: []int{2, 4, 4, 6, 3, 5, 1, 8, 2, 7, 1, 2, 5, 5, 3}}
	h := NewHub(dice, st)
	g, _, _ := seated(t, h)
	for i, m := range []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1c4"} {
		guest := []string{"alice", "bob"}[i%2]
		play(t, g, guest, m, i)
	}
	want := recvView(t, g, "bob")
	opened := 0
	for _, e := range want.Log {
		opened += strings.Count(e.Dice, "→")
	}
	if opened == 0 {
		t.Fatalf("test setup: no pothole opened, so the dice aren't tested: %+v", want.Log)
	}

	h2 := NewHub(odd{}, st) // a restart: new dice, same database
	if n := h2.Restore(load(t, st)); n != 1 {
		t.Fatalf("restored %d games, want 1", n)
	}
	g2, ok := h2.Get(g.Code())
	if !ok {
		t.Fatal("restored game isn't in the hub")
	}
	got := recvView(t, g2, "bob")
	want.Online = OnlineJSON{} // nobody has reconnected yet
	sameGame(t, got, want)
	if got.Clock.WhiteMS != want.Clock.WhiteMS || got.Clock.BlackMS != want.Clock.BlackMS {
		t.Errorf("clocks %+v, want %+v", got.Clock, want.Clock)
	}
	if got.Clock.Running != "black" || got.Clock.Since < time.Now().Add(RestoreGrace-time.Second).UnixMilli() {
		t.Errorf("clock %+v: black's clock should restart after the grace period", got.Clock)
	}
	// The game goes on, and its next turn is saved too.
	join(t, g2, "bob")
	play(t, g2, "bob", "g8f6", 5)
	if saved := load(t, st); len(saved[0].Turns) != 6 {
		t.Fatalf("saved %d turns after the restore, want 6", len(saved[0].Turns))
	}
}

func TestRestoreKeepsAFinishedGamesResult(t *testing.T) {
	st := openStore(t)
	h := NewHub(odd{}, st)
	g, _, _ := seated(t, h)
	openings(t, g)
	g.Resign("bob")
	want := recvView(t, g, "alice")

	h2 := NewHub(odd{}, st)
	h2.Restore(load(t, st))
	g2, ok := h2.Get(g.Code())
	if !ok {
		t.Fatal("a game that ended today should be restored")
	}
	got := recvView(t, g2, "alice")
	want.Online = OnlineJSON{}
	sameGame(t, got, want)
	if got.Result == nil || got.Result.Winner != "white" || got.Result.Reason != Resignation {
		t.Fatalf("result %+v", got.Result)
	}
}

func TestRestoreRestartsTheFirstMoveDeadline(t *testing.T) {
	st := openStore(t)
	h := NewHub(odd{}, st)
	g, _, _ := seated(t, h)
	play(t, g, "alice", "e2e4", 0)
	h2 := NewHub(odd{}, st)
	h2.Restore(load(t, st))
	g2, _ := h2.Get(g.Code())
	v := recvView(t, g2, "bob")
	if v.Clock.Running != "" || v.Clock.FirstMoveDeadline < time.Now().Add(FirstMoveTime-time.Second).UnixMilli() {
		t.Fatalf("clock %+v: Black should get a full minute again", v.Clock)
	}
}

func TestRestoreSkipsAGameThatWontReplay(t *testing.T) {
	saved := []store.SavedGame{
		{Game: store.Game{Code: "BAD001", White: "a", Black: "b"},
			Turns: []store.Turn{{Ply: 0, Move: "e2e5"}}}, // illegal
		{Game: store.Game{Code: "OK0001", White: "a"}},
	}
	h := NewHub(odd{}, nil)
	if n := h.Restore(saved); n != 1 {
		t.Fatalf("restored %d, want 1", n)
	}
	if _, ok := h.Get("BAD001"); ok {
		t.Error("a game that won't replay was restored")
	}
}
