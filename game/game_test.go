package game

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
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

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler)) // game events log at INFO
	os.Exit(m.Run())
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

// recvSeq drains sub up to the first view with at least seq turns played.
func recvSeq(t *testing.T, sub *Sub, seq int) *View {
	t.Helper()
	for {
		if v := recv(t, sub); v.Seq >= seq {
			return v
		}
	}
}

// create starts a game with creator as White.
func create(t *testing.T, h *Hub, creator string) *Game {
	t.Helper()
	g, err := h.Create(creator, false)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	return g
}

// join opens a stream for guest and fails the test if the game has stopped.
func join(t *testing.T, g *Game, guest string) *Sub {
	t.Helper()
	sub, err := g.Join(guest)
	if err != nil {
		t.Fatalf("join %s: %v", guest, err)
	}
	return sub
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
	g := create(t, NewHub(odd{}, nil), "alice")
	a := join(t, g, "alice")
	if v := recv(t, a); v.You != "white" || v.Status != Waiting || len(v.Legal) != 0 {
		t.Fatalf("creator: you=%s status=%s legal=%d", v.You, v.Status, len(v.Legal))
	}
	b := join(t, g, "bob")
	if v := recv(t, b); v.You != "black" || v.Status != Playing || len(v.Legal) != 0 {
		t.Fatalf("second guest: you=%s status=%s legal=%d", v.You, v.Status, len(v.Legal))
	}
	if v := recv(t, a); v.Status != Playing || len(v.Legal) != 33 {
		t.Fatalf("white after black joins: status=%s legal=%d, want playing and 33", v.Status, len(v.Legal))
	}
	c := join(t, g, "carol")
	if v := recv(t, c); v.You != "spectator" || len(v.Legal) != 0 {
		t.Fatalf("third guest: you=%s legal=%d", v.You, len(v.Legal))
	}
	a2 := join(t, g, "alice") // a second tab, or a reconnect
	if v := recv(t, a2); v.You != "white" {
		t.Fatalf("reconnect: you=%s, want white", v.You)
	}
}

func TestMoveBroadcastsToEveryone(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	a, b, c := join(t, g, "alice"), join(t, g, "bob"), join(t, g, "carol")
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
		if !reflect.DeepEqual(v.Log, []LogEntry{{SAN: "e4", Color: "white", Piece: "wP", Dice: "d8 1"}}) {
			t.Errorf("%s: log %+v", name, v.Log)
		}
		// Black: 19 normal moves (a7a5 is blocked by the Mamdani) + 13 Mamdani moves.
		wantLegal := map[string]int{"alice": 0, "bob": 32, "carol": 0}[name]
		if len(v.Legal) != wantLegal {
			t.Errorf("%s: %d legal moves, want %d", name, len(v.Legal), wantLegal)
		}
	}
}

func TestMoveErrors(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrWaiting) {
		t.Errorf("before black joins: %v", err)
	}
	join(t, g, "bob")
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
	g := create(t, NewHub(&script{rolls: []int{2, 4, 4}}, nil), "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	recv(t, a)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	v := recv(t, a)
	if !slices.Equal(v.Potholes, []Pothole{{Sq: "d4", By: "white", Left: 3}}) {
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
		t.Errorf("log %+v", v.Log[0])
	}
}

func TestPotholesCountDownOldestFirst(t *testing.T) {
	// White opens d4, Black opens h3, then White's move counts d4 down.
	g := create(t, NewHub(&script{rolls: []int{2, 4, 4, 4, 8, 3}}, nil), "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	recv(t, a)
	for i, m := range []struct{ guest, uci string }{{"alice", "e2e4"}, {"bob", "e7e5"}, {"alice", "g1f3"}} {
		if err := g.Move(m.guest, mv(t, m.uci), i); err != nil {
			t.Fatal(err)
		}
		recv(t, a)
	}
	v := recvView(t, g, "alice")
	want := []Pothole{{Sq: "d4", By: "white", Left: 2}, {Sq: "h3", By: "black", Left: 3}}
	if !slices.Equal(v.Potholes, want) {
		t.Errorf("potholes %v, want %v", v.Potholes, want)
	}
	if l := g.live.Load(); !slices.Equal(l.Potholes, want) {
		t.Errorf("live potholes %v, want %v", l.Potholes, want)
	}
}

func TestCheckmateEndsTheGame(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	plays(t, g, "f2f3", "e7e5", "g2g4", "d8h4")
	v := recvSeq(t, a, 4)
	if v.Status != Over || v.Result == nil || v.Result.Winner != "black" || v.Result.Reason != rules.Checkmate {
		t.Fatalf("status %s result %+v", v.Status, v.Result)
	}
	if err := g.Move("alice", mv(t, "a2a3"), 4); !errors.Is(err, ErrGameOver) {
		t.Errorf("move after mate: %v", err)
	}
	if err := g.Resign("alice"); !errors.Is(err, ErrGameOver) {
		t.Errorf("resign after checkmate: %v, want ErrGameOver", err)
	}
}

func TestLeaveStopsUpdates(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	a := join(t, g, "alice")
	recv(t, a)
	g.Leave(a)
	join(t, g, "bob") // would broadcast to alice if she were still subscribed
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

func TestPanicRetiresTheGame(t *testing.T) {
	h := NewHub(&panicky{}, nil)
	g := create(t, h, "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrInternal) {
		t.Fatalf("got %v, want ErrInternal", err)
	}
	if _, ok := h.Get(g.Code()); ok {
		t.Error("the hub still lists a game a panic retired")
	}
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrGone) {
		t.Errorf("move after the panic: got %v, want ErrGone", err)
	}
	if _, err := g.Join("carol"); !errors.Is(err, ErrGone) {
		t.Errorf("join after the panic: got %v, want ErrGone", err)
	}
	waitClosed(t, a)
}

// waitClosed drains sub and fails the test unless its channel closes.
func waitClosed(t *testing.T, sub *Sub) {
	t.Helper()
	for {
		select {
		case _, ok := <-sub.C:
			if !ok {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("stream still open 1s after the game stopped")
		}
	}
}

func TestResign(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	if err := g.Resign("alice"); !errors.Is(err, ErrWaiting) {
		t.Errorf("resign before black joins: %v", err)
	}
	b := join(t, g, "bob")
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
	g := create(t, NewHub(&script{rolls: []int{
		2, 7, 8, // e4: g8 is hit; the Mamdani on a5 has no line to it, so the knight falls
		2, 4, 2, 5, // e5: d2 is hit; a5-b4-c3-d2 is clear, so White rolls to save: 5 saves it
		2, 2, 4, // Nf3: b4 is next to the Mamdani, so the new pothole is repaired at once
		1, // d6
		1, // Nxe5
	}}, nil), "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	recv(t, a)
	plays(t, g, "e2e4", "e7e5", "g1f3", "d7d6", "f3e5")
	v := recvSeq(t, a, 5)
	if !slices.Equal(v.Lost.White, []string{}) || !slices.Equal(v.Lost.Black, []string{"bN"}) {
		t.Errorf("lost %+v, want white none and black [bN]", v.Lost)
	}
	if !slices.Equal(v.Taken.White, []string{"bP"}) || !slices.Equal(v.Taken.Black, []string{}) {
		t.Errorf("taken %+v, want white [bP] and black none", v.Taken)
	}
	if want := (StatsJSON{SavingRolls: 1, Saved: 1, Repaired: 1}); v.Stats != want {
		t.Errorf("stats %+v, want %+v", v.Stats, want)
	}
	for i, want := range []LogEntry{
		{SAN: "e4", Color: "white", Piece: "wP", Dice: "d8 2 → g8 · bN falls", Opened: "g8", Fell: []string{"bN"}},
		{SAN: "e5", Color: "black", Piece: "bP", Dice: "d8 2 → d2 · save 5 ✓"}, // saved: no hole,
		{SAN: "Nf3", Color: "white", Piece: "wN", Dice: "d8 2 → b4 repaired", Repaired: true},
		{SAN: "d6", Color: "black", Piece: "bP", Dice: "d8 1"},
		{SAN: "Nxe5", Color: "white", Piece: "wN", Dice: "d8 1"},
	} {
		if !reflect.DeepEqual(v.Log[i], want) {
			t.Errorf("log[%d] %+v, want %+v", i, v.Log[i], want)
		}
	}
}

func TestFinishedGameIsEvictedAfterAQuietDay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{}, nil)
		g := create(t, h, "alice")
		b := join(t, g, "bob") // still watching: a finished game goes anyway
		if err := g.Resign("alice"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Fatal("finished game still listed after a quiet day")
		}
		waitClosed(t, b)
	})
}

func TestUnwatchedGameIsEvictedAfterAQuietDay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{}, nil)
		g := create(t, h, "alice") // nobody ever opens the link
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Fatal("unwatched game still listed after a quiet day")
		}
		if _, err := g.Join("alice"); !errors.Is(err, ErrGone) {
			t.Errorf("join after eviction: got %v, want ErrGone", err)
		}
	})
}

// A game in progress always ends by itself now (a clock or a first-move
// deadline is always counting), so the one game that can stay unfinished
// for days is one still waiting for Black.
func TestWatchedWaitingGameIsKept(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{}, nil)
		g := create(t, h, "alice")
		a := join(t, g, "alice")
		time.Sleep(3 * DefaultIdle)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); !ok {
			t.Fatal("a waiting game with White watching was evicted")
		}
		// Once White leaves it is unwatched, and goes after another quiet
		// day. (synctest also needs the game's goroutine gone by the end.)
		g.Leave(a)
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Fatal("game still listed a quiet day after everyone left")
		}
	})
}

func TestStreamsInOneRoleShareAView(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	c1, c2 := join(t, g, "carol"), join(t, g, "dave")
	recv(t, a)
	recv(t, c1)
	recv(t, c2)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	v1, v2 := recv(t, c1), recv(t, c2)
	if v1 != v2 {
		t.Error("two spectators got separately built views")
	}
	if v1.You != "spectator" || len(v1.Legal) != 0 {
		t.Errorf("shared spectator view: you=%s legal=%d", v1.You, len(v1.Legal))
	}
	if va := recv(t, a); va == v1 || va.You != "white" {
		t.Errorf("white shares the spectators' view: you=%s", va.You)
	}
}

func TestViewIsTheCallersRole(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	join(t, g, "bob")
	for guest, want := range map[string]string{"alice": "white", "bob": "black", "carol": "spectator"} {
		v, err := g.View(guest)
		if err != nil || v.You != want {
			t.Errorf("%s: view %+v err %v, want you=%s", guest, v, err, want)
		}
	}
}

func TestIllegalMoveChangesNothing(t *testing.T) {
	g := create(t, NewHub(odd{}, nil), "alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	recv(t, a)
	if err := g.Move("alice", mv(t, "e2e5"), 0); !errors.Is(err, ErrIllegalMove) {
		t.Fatalf("got %v, want ErrIllegalMove", err)
	}
	select {
	case v := <-a.C:
		t.Fatalf("an illegal move broadcast a view: seq %d", v.Seq)
	case <-time.After(50 * time.Millisecond):
	}
	v, err := g.View("alice")
	if err != nil || v.Seq != 0 || len(v.Log) != 0 || v.Board[12] != "wP" {
		t.Errorf("after an illegal move: seq %d log %v e2 %q err %v", v.Seq, v.Log, v.Board[12], err)
	}
}

func TestStreamsInOneRoleShareOneEncoding(t *testing.T) {
	// synctest stops the clock, so the view built later for comparison has
	// the same clock.now as the one sent.
	synctest.Test(t, func(t *testing.T) {
		g := create(t, NewHub(odd{}, nil), "alice")
		join(t, g, "bob")
		c1, c2 := join(t, g, "carol"), join(t, g, "dave")
		recv(t, c1)
		recv(t, c2)
		if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
			t.Fatal(err)
		}
		e1, e2 := recv(t, c1).JSON(), recv(t, c2).JSON()
		if len(e1) == 0 || &e1[0] != &e2[0] {
			t.Error("each spectator's view was encoded separately")
		}
		want, err := json.Marshal(recvView(t, g, "carol"))
		if err != nil || string(e1) != string(want) {
			t.Errorf("encoded view differs from json.Marshal:\n got %s\nwant %s", e1, want)
		}
		finish(t, g) // synctest needs the game's goroutine gone by the end
	})
}

func TestACrowdJoiningSharesOneView(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{}, nil)
		g := playing(t, h, "alice", "bob")
		first := recv(t, join(t, g, "carol"))
		if again := recv(t, join(t, g, "dave")); again != first {
			t.Error("two spectators joining at once got separately built views")
		}
		// A view carries the server's time (clock.now), so it is reused
		// only briefly.
		time.Sleep(viewReuse + time.Millisecond)
		later := recv(t, join(t, g, "erin"))
		if later == first || later.Clock.Now <= first.Clock.Now {
			t.Errorf("a later joiner got a stale view: now %d, first %d", later.Clock.Now, first.Clock.Now)
		}
		// A change is never hidden by a reused view.
		if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
			t.Fatal(err)
		}
		if v := recv(t, join(t, g, "frank")); v.Seq != 1 {
			t.Errorf("joined after a move: seq %d, want 1", v.Seq)
		}
		finish(t, g)
	})
}

func TestSharedViewsAreFreedWhenEveryoneLeaves(t *testing.T) {
	g := playing(t, NewHub(odd{}, nil), "alice", "bob")
	sub := join(t, g, "carol")
	recv(t, sub)
	g.Leave(sub)
	var kept bool
	g.do(func() {
		for _, b := range g.views {
			kept = kept || b.v != nil
		}
	})
	if kept {
		t.Error("a game nobody watches still holds encoded views")
	}
}

// recvView is guest's current view, built fresh (not the shared one).
func recvView(t *testing.T, g *Game, guest string) *View {
	t.Helper()
	v, err := g.View(guest)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
