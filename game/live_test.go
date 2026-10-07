package game

import (
	"strings"
	"testing"
)

// playing starts a game between white and black on h.
func playing(t *testing.T, h *Hub, white, black string) *Game {
	t.Helper()
	g, err := h.CreatePair(white, black)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func codes(games []*Live) []string {
	var out []string
	for _, l := range games {
		out = append(out, l.Code)
	}
	return out
}

func TestListShowsOnlyGamesBeingPlayed(t *testing.T) {
	h := NewHub(odd{}, nil)
	create(t, h, "alice") // waiting for a friend
	ended := playing(t, h, "carol", "dave")
	if err := ended.Resign("carol"); err != nil {
		t.Fatal(err)
	}
	live := playing(t, h, "erin", "frank")
	if got := codes(h.List(12)); len(got) != 1 || got[0] != live.Code() {
		t.Fatalf("listed %v, want only %s", got, live.Code())
	}
}

func TestListIsMostWatchedFirstThenNewest(t *testing.T) {
	h := NewHub(odd{}, nil)
	first := playing(t, h, "a1", "a2")
	second := playing(t, h, "b1", "b2")
	third := playing(t, h, "c1", "c2")
	if got := codes(h.List(12)); got[0] != third.Code() || got[1] != second.Code() || got[2] != first.Code() {
		t.Fatalf("with nobody watching: %v, want newest first", got)
	}
	join(t, first, "watcher")
	if got := codes(h.List(12)); got[0] != first.Code() {
		t.Fatalf("with one watcher on the oldest: %v, want it first", got)
	}
	if got := h.List(2); len(got) != 2 {
		t.Fatalf("List(2) returned %d games", len(got))
	}
}

func TestLiveShowsTheGame(t *testing.T) {
	h := NewHub(odd{}, nil)
	g := playing(t, h, "alice", "bob")
	join(t, g, "carol")
	join(t, g, "carol") // a second tab
	join(t, g, "dave")
	join(t, g, "alice") // a player isn't a watcher
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	l := h.List(12)[0]
	if l.Watching != 2 || l.Move != 1 || l.Board[28] != "wP" || l.Last == nil || *l.Last != (MoveJSON{From: "e2", To: "e4"}) {
		t.Fatalf("live %+v", l)
	}
	if err := g.Move("bob", mv(t, "e7e5"), 1); err != nil {
		t.Fatal(err)
	}
	if l := h.List(12)[0]; l.Move != 2 {
		t.Fatalf("after e4 e5: move %d, want 2", l.Move)
	}
}

func TestWatchingFollowsStreamsClosing(t *testing.T) {
	h := NewHub(odd{}, nil)
	g := playing(t, h, "alice", "bob")
	c1, c2 := join(t, g, "carol"), join(t, g, "carol") // two tabs
	d := join(t, g, "dave")
	a := join(t, g, "alice")
	watching := func() int { return h.List(12)[0].Watching }
	if n := watching(); n != 2 {
		t.Fatalf("watching %d, want 2", n)
	}
	g.Leave(c1)
	if n := watching(); n != 2 {
		t.Errorf("carol still has a tab open: watching %d, want 2", n)
	}
	g.Leave(a)
	if n := watching(); n != 2 {
		t.Errorf("a player leaving isn't a watcher leaving: watching %d, want 2", n)
	}
	g.Leave(c2)
	g.Leave(d)
	if n := watching(); n != 0 {
		t.Errorf("everyone left: watching %d, want 0", n)
	}
}

func TestActiveIsYourGameBeingPlayed(t *testing.T) {
	h := NewHub(odd{}, nil)
	create(t, h, "alice") // waiting for a friend
	if code := h.Active("alice"); code != "" {
		t.Fatalf("a game still waiting for a friend counts as active: %s", code)
	}
	g := playing(t, h, "bob", "carol")
	if h.Active("bob") != g.Code() || h.Active("carol") != g.Code() {
		t.Fatalf("Active(bob)=%q Active(carol)=%q, want %s", h.Active("bob"), h.Active("carol"), g.Code())
	}
	join(t, g, "dave") // watching
	if code := h.Active("dave"); code != "" {
		t.Fatalf("a spectator has an active game: %s", code)
	}
	if err := g.Resign("bob"); err != nil {
		t.Fatal(err)
	}
	if code := h.Active("bob"); code != "" {
		t.Fatalf("a finished game counts as active: %s", code)
	}
}

func TestGetIgnoresCase(t *testing.T) {
	h := NewHub(odd{}, nil)
	g := create(t, h, "alice")
	if _, ok := h.Get(strings.ToLower(g.Code())); !ok {
		t.Fatalf("Get(%q) found nothing", strings.ToLower(g.Code()))
	}
}
