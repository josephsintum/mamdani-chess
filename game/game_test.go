package game

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"mamdani-chess/rules"
)

// odd always rolls 1: no potholes, so moves are plain chess.
type odd struct{}

func (odd) D8() int { return 1 }

// script rolls the given values in order, then 1 forever.
type script struct{ rolls []int }

func (s *script) D8() int {
	if len(s.rolls) == 0 {
		return 1
	}
	r := s.rolls[0]
	s.rolls = s.rolls[1:]
	return r
}

func recv(t *testing.T, sub *Sub) *View {
	t.Helper()
	select {
	case v := <-sub.C:
		return v
	case <-time.After(time.Second):
		t.Fatal("no view within 1s")
		return nil
	}
}

func mv(t *testing.T, uci string) rules.Move {
	t.Helper()
	m, err := rules.ParseMove(uci)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSeats(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := g.Join("alice")
	if v := recv(t, a); v.You != "white" || v.Status != Waiting || len(v.Legal) != 0 {
		t.Fatalf("creator: you=%s status=%s legal=%d", v.You, v.Status, len(v.Legal))
	}
	b := g.Join("bob")
	if v := recv(t, b); v.You != "black" || v.Status != Playing || len(v.Legal) != 0 {
		t.Fatalf("second guest: you=%s status=%s legal=%d", v.You, v.Status, len(v.Legal))
	}
	if v := recv(t, a); v.Status != Playing || len(v.Legal) != 33 {
		t.Fatalf("white after black joins: status=%s legal=%d, want playing and 33", v.Status, len(v.Legal))
	}
	c := g.Join("carol")
	if v := recv(t, c); v.You != "spectator" || len(v.Legal) != 0 {
		t.Fatalf("third guest: you=%s legal=%d", v.You, len(v.Legal))
	}
	a2 := g.Join("alice") // a second tab, or a reconnect
	if v := recv(t, a2); v.You != "white" {
		t.Fatalf("reconnect: you=%s, want white", v.You)
	}
}

func TestMoveBroadcastsToEveryone(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a, b, c := g.Join("alice"), g.Join("bob"), g.Join("carol")
	recv(t, a)
	recv(t, b)
	recv(t, c)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	for name, sub := range map[string]*Sub{"alice": a, "bob": b, "carol": c} {
		v := recv(t, sub)
		if v.Seq != 1 || v.Turn != "black" || v.Board[rules.E4] != "wP" || v.Board[rules.E2] != "" {
			t.Errorf("%s: seq=%d turn=%s e4=%q e2=%q", name, v.Seq, v.Turn, v.Board[rules.E4], v.Board[rules.E2])
		}
		if !slices.Equal(v.Log, []LogEntry{{SAN: "e4", Color: "white", Dice: "d8 1"}}) {
			t.Errorf("%s: log %q", name, v.Log)
		}
		// Black: 19 normal moves (a7a5 is blocked by the Mamdani) + 13 Mamdani moves.
		wantLegal := map[string]int{"alice": 0, "bob": 32, "carol": 0}[name]
		if len(v.Legal) != wantLegal {
			t.Errorf("%s: %d legal moves, want %d", name, len(v.Legal), wantLegal)
		}
	}
}

func TestMoveErrors(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrWaiting) {
		t.Errorf("before black joins: %v", err)
	}
	g.Join("bob")
	cases := []struct {
		guest, uci string
		seq        int
		want       error
	}{
		{"carol", "e2e4", 0, ErrNotPlayer},
		{"bob", "e7e5", 0, ErrNotYourTurn},
		{"alice", "e2e4", 3, ErrStale},
		{"alice", "e2e5", 0, ErrIllegalMove},
	}
	for _, c := range cases {
		if err := g.Move(c.guest, mv(t, c.uci), c.seq); !errors.Is(err, c.want) {
			t.Errorf("%s %s seq %d: got %v, want %v", c.guest, c.uci, c.seq, err, c.want)
		}
	}
}

func TestPotholeShowsInView(t *testing.T) {
	// Even roll, then file 4 rank 4: a pothole opens on d4.
	g := NewHub(&script{rolls: []int{2, 4, 4}}).Create("alice")
	a := g.Join("alice")
	g.Join("bob")
	recv(t, a)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	v := recv(t, a)
	if !slices.Equal(v.Potholes, []Pothole{{Sq: "d4", By: "white"}}) {
		t.Errorf("potholes %v", v.Potholes)
	}
	kinds := []rules.EventKind{}
	for _, e := range v.Last {
		kinds = append(kinds, e.Kind)
	}
	want := []rules.EventKind{rules.Moved, rules.RolledPothole, rules.Target, rules.PotholeOpened}
	if !slices.Equal(kinds, want) {
		t.Errorf("last %v, want %v", kinds, want)
	}
	if !strings.HasPrefix(v.Log[0].Dice, "d8 2 → d4") {
		t.Errorf("log %q", v.Log[0])
	}
}

func TestCheckmateEndsTheGame(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := g.Join("alice")
	g.Join("bob")
	for i, m := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		guest := []string{"alice", "bob"}[i%2]
		if err := g.Move(guest, mv(t, m), i); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}
	var v *View
	for range 5 { // drain to the newest view
		v = recv(t, a)
		if v.Seq == 4 {
			break
		}
	}
	if v.Status != Over || v.Result == nil || v.Result.Winner != "black" || v.Result.Reason != rules.Checkmate {
		t.Fatalf("status %s result %+v", v.Status, v.Result)
	}
	if err := g.Move("alice", mv(t, "a2a3"), 4); !errors.Is(err, ErrGameOver) {
		t.Errorf("move after mate: %v", err)
	}
}

func TestLeaveStopsUpdates(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := g.Join("alice")
	recv(t, a)
	g.Leave(a)
	g.Join("bob") // would broadcast to alice if she were still subscribed
	select {
	case v := <-a.C:
		t.Fatalf("got a view after Leave: seq %d", v.Seq)
	case <-time.After(50 * time.Millisecond):
	}
}

// panicky panics on its first roll, then rolls 1 forever.
type panicky struct{ rolled bool }

func (p *panicky) D8() int {
	if !p.rolled {
		p.rolled = true
		panic("dice exploded")
	}
	return 1
}

func TestPanicInGameIsContained(t *testing.T) {
	g := NewHub(&panicky{}).Create("alice")
	a := g.Join("alice")
	g.Join("bob")
	recv(t, a)
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrInternal) {
		t.Fatalf("got %v, want ErrInternal", err)
	}
	// The game goroutine survives and the position is unchanged.
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatalf("the game should keep working after a panic: %v", err)
	}
}

func TestResign(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	if err := g.Resign("alice"); !errors.Is(err, ErrWaiting) {
		t.Errorf("resign before black joins: %v", err)
	}
	b := g.Join("bob")
	recv(t, b)
	if err := g.Resign("carol"); !errors.Is(err, ErrNotPlayer) {
		t.Errorf("spectator resigns: %v", err)
	}
	if err := g.Resign("alice"); err != nil {
		t.Fatal(err)
	}
	v := recv(t, b)
	if v.Status != Over || v.Result == nil || v.Result.Winner != "black" || v.Result.Reason != Resignation || len(v.Legal) != 0 {
		t.Fatalf("status %s result %+v legal %d", v.Status, v.Result, len(v.Legal))
	}
	if err := g.Resign("bob"); !errors.Is(err, ErrGameOver) {
		t.Errorf("resign after the game ended: %v", err)
	}
	if err := g.Move("bob", mv(t, "e7e5"), 0); !errors.Is(err, ErrGameOver) {
		t.Errorf("move after resignation: %v", err)
	}
}

func TestStatsAndLostPieces(t *testing.T) {
	g := NewHub(&script{rolls: []int{
		2, 7, 8, // e4: g8 is hit; the Mamdani on a5 has no line to it, so the knight falls
		2, 4, 2, 5, // e5: d2 is hit; a5-b4-c3-d2 is clear, so White rolls to save: 5 saves it
		2, 2, 4, // Nf3: b4 is next to the Mamdani, so the new pothole is repaired at once
	}}).Create("alice")
	a := g.Join("alice")
	g.Join("bob")
	recv(t, a)
	for i, m := range []string{"e2e4", "e7e5", "g1f3"} {
		if err := g.Move([]string{"alice", "bob"}[i%2], mv(t, m), i); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}
	var v *View
	for v == nil || v.Seq < 3 {
		v = recv(t, a)
	}
	if !slices.Equal(v.Lost.White, []string{}) || !slices.Equal(v.Lost.Black, []string{"bN"}) {
		t.Errorf("lost %+v, want white none and black [bN]", v.Lost)
	}
	if want := (StatsJSON{SavingRolls: 1, Saved: 1, Repaired: 1}); v.Stats != want {
		t.Errorf("stats %+v, want %+v", v.Stats, want)
	}
	if want := (LogEntry{SAN: "e4", Color: "white", Dice: "d8 2 → g8 · bN falls"}); v.Log[0] != want {
		t.Errorf("log[0] %+v, want %+v", v.Log[0], want)
	}
}
