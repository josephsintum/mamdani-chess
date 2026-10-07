package game

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"mamdani-chess/rules"
)

// seated starts a game with alice as White and bob as Black, both watching.
func seated(t *testing.T, h *Hub) (*Game, *Sub, *Sub) {
	t.Helper()
	g := create(t, h, "alice")
	a, b := join(t, g, "alice"), join(t, g, "bob")
	return g, a, b
}

// play makes a move that must succeed.
func play(t *testing.T, g *Game, guest, uci string, seq int) {
	t.Helper()
	if err := g.Move(guest, mv(t, uci), seq); err != nil {
		t.Fatalf("%s %s: %v", guest, uci, err)
	}
}

// plays makes moves in turn from the start, alice as White and bob as
// Black. Each must succeed.
func plays(t *testing.T, g *Game, ucis ...string) {
	t.Helper()
	for i, uci := range ucis {
		play(t, g, [2]string{"alice", "bob"}[i%2], uci, i)
	}
}

// openings plays both first moves, so White's clock starts ResolveDelay later.
func openings(t *testing.T, g *Game) {
	t.Helper()
	plays(t, g, "e2e4", "e7e5")
}

// finish ends g if it is still on and lets it be evicted, so the synctest
// bubble's goroutines are gone when the test returns.
func finish(t *testing.T, g *Game) {
	t.Helper()
	g.Resign("alice") // fails harmlessly if the game is already over
	time.Sleep(DefaultIdle + time.Minute)
	synctest.Wait()
}

func TestWhiteMissesTheFirstMove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		time.Sleep(FirstMoveTime - time.Second)
		synctest.Wait()
		if v := recvView(t, g, "bob"); v.Result != nil {
			t.Fatalf("aborted early: %+v", v.Result)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		v := recvView(t, g, "bob")
		if v.Result == nil || v.Result.Reason != Aborted || v.Result.Winner != "" || v.Result.Draw {
			t.Fatalf("result %+v, want aborted with no winner", v.Result)
		}
		if v.Clock.WhiteMS != InitialTime.Milliseconds() {
			t.Errorf("an aborted game keeps its clocks: white %d ms", v.Clock.WhiteMS)
		}
		finish(t, g)
	})
}

func TestBlackMissesTheFirstMove(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		play(t, g, "alice", "e2e4", 0)
		v := recvView(t, g, "bob")
		if want := time.Now().Add(ResolveDelay + FirstMoveTime).UnixMilli(); v.Clock.FirstMoveDeadline != want {
			t.Fatalf("first-move deadline %d, want %d (after the dice pause)", v.Clock.FirstMoveDeadline, want)
		}
		time.Sleep(ResolveDelay + FirstMoveTime)
		synctest.Wait()
		if v := recvView(t, g, "alice"); v.Result == nil || v.Result.Reason != Aborted {
			t.Fatalf("result %+v, want aborted", v.Result)
		}
		finish(t, g)
	})
}

func TestClocksStartAfterBothFirstMoves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		time.Sleep(30 * time.Second) // thinking before the first move is free
		play(t, g, "alice", "e2e4", 0)
		if v := recvView(t, g, "alice"); v.Clock.Running != "" {
			t.Fatalf("clock running after White's first move: %+v", v.Clock)
		}
		time.Sleep(30 * time.Second)
		play(t, g, "bob", "e7e5", 1)
		v := recvView(t, g, "alice")
		want := ClockJSON{
			WhiteMS: InitialTime.Milliseconds(), BlackMS: InitialTime.Milliseconds(),
			Running: "white", Since: time.Now().Add(ResolveDelay).UnixMilli(), Now: time.Now().UnixMilli(),
		}
		if v.Clock != want {
			t.Fatalf("clock %+v, want %+v", v.Clock, want)
		}
		finish(t, g)
	})
}

func TestTimeUsedAndIncrement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		openings(t, g)
		time.Sleep(ResolveDelay + 10*time.Second) // the dice pause is free
		play(t, g, "alice", "g1f3", 2)
		v := recvView(t, g, "bob")
		if want := (InitialTime - 10*time.Second + Increment).Milliseconds(); v.Clock.WhiteMS != want {
			t.Errorf("white %d ms, want %d", v.Clock.WhiteMS, want)
		}
		if v.Clock.Running != "black" || v.Clock.BlackMS != InitialTime.Milliseconds() {
			t.Errorf("clock %+v, want black running with full time", v.Clock)
		}
		finish(t, g)
	})
}

func TestFlagFallLosesOnTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		openings(t, g)
		time.Sleep(ResolveDelay + InitialTime - time.Millisecond)
		synctest.Wait()
		if v := recvView(t, g, "bob"); v.Result != nil {
			t.Fatalf("flag fell early: %+v", v.Result)
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		v := recvView(t, g, "bob")
		if v.Result == nil || v.Result.Reason != Timeout || v.Result.Winner != "black" {
			t.Fatalf("result %+v, want black wins on time", v.Result)
		}
		if v.Clock.WhiteMS != 0 || v.Clock.Running != "" {
			t.Errorf("clock %+v, want white at 0 and stopped", v.Clock)
		}
		if err := g.Move("alice", mv(t, "g1f3"), 2); !errors.Is(err, ErrGameOver) {
			t.Errorf("move after the flag: %v, want ErrGameOver", err)
		}
		finish(t, g)
	})
}

func TestFlagFallIsADrawWhenTheWinnerCannotMate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		// White: king and rook. Black: a bare king, so Black can never mate.
		p, err := rules.ParseFEN("4k3/8/8/8/8/8/8/R3K3 w - - 0 1")
		if err != nil {
			t.Fatal(err)
		}
		p.Mamdani = rules.NoSquare
		g.do(func() { g.g = rules.NewGameFrom(p) })
		play(t, g, "alice", "a1a2", 0)
		play(t, g, "bob", "e8d8", 1)
		time.Sleep(ResolveDelay + InitialTime)
		synctest.Wait()
		v := recvView(t, g, "bob")
		if v.Result == nil || v.Result.Reason != TimeoutVsInsufficient || !v.Result.Draw || v.Result.Winner != "" {
			t.Fatalf("result %+v, want a draw by timeout vs insufficient material", v.Result)
		}
		finish(t, g)
	})
}

// The timer can lag behind the clock. A move that arrives after the
// deadline loses on time instead of being played.
func TestAMoveAfterTheDeadlineLosesOnTime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		openings(t, g)
		g.do(func() { g.clock.deadline = time.Now().Add(-time.Millisecond) }) // as if the timer were late
		if err := g.Move("alice", mv(t, "g1f3"), 2); !errors.Is(err, ErrGameOver) {
			t.Fatalf("late move: %v, want ErrGameOver", err)
		}
		if v := recvView(t, g, "bob"); v.Result == nil || v.Result.Reason != Timeout || v.Seq != 2 {
			t.Fatalf("result %+v seq %d, want a timeout before the move", v.Result, v.Seq)
		}
		finish(t, g)
	})
}

// A move replaces the side to move's deadline. Long after White's first
// deadline has passed, the game must still be on.
func TestAMoveReplacesTheDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		openings(t, g)
		time.Sleep(ResolveDelay + 9*time.Minute)
		play(t, g, "alice", "g1f3", 2) // white: 1:05 left
		time.Sleep(ResolveDelay + time.Second)
		play(t, g, "bob", "g8f6", 3)
		time.Sleep(ResolveDelay + time.Minute) // past White's first deadline
		synctest.Wait()
		v := recvView(t, g, "bob")
		if v.Result != nil {
			t.Fatalf("game ended: %+v", v.Result)
		}
		if want := (65 * time.Second).Milliseconds(); v.Clock.WhiteMS != want {
			t.Errorf("white %d ms, want %d", v.Clock.WhiteMS, want)
		}
		time.Sleep(5 * time.Second) // now White's 1:05 is up
		synctest.Wait()
		if v := recvView(t, g, "bob"); v.Result == nil || v.Result.Reason != Timeout {
			t.Fatalf("result %+v, want timeout", v.Result)
		}
		finish(t, g)
	})
}

func TestResignStopsTheClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(odd{}, nil))
		openings(t, g)
		time.Sleep(ResolveDelay + 30*time.Second)
		if err := g.Resign("alice"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(InitialTime)
		synctest.Wait()
		v := recvView(t, g, "bob")
		if v.Result == nil || v.Result.Reason != Resignation {
			t.Fatalf("result %+v, want resignation", v.Result)
		}
		if want := (InitialTime - 30*time.Second).Milliseconds(); v.Clock.WhiteMS != want || v.Clock.Running != "" {
			t.Errorf("clock %+v, want white at %d ms and stopped", v.Clock, want)
		}
		finish(t, g)
	})
}

func TestThePauseCoversTheDiceAnimation(t *testing.T) {
	ev := func(kinds ...rules.EventKind) []rules.Event {
		out := make([]rules.Event, len(kinds))
		for i, k := range kinds {
			out[i] = rules.Event{Kind: k}
			if k == rules.RolledPothole {
				out[i].Roll = 2
			}
		}
		return out
	}
	oddRoll := []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 3}}
	cases := []struct {
		name string
		ev   []rules.Event
		want time.Duration
	}{
		{"checkmate, no roll", ev(rules.Moved), ResolveDelay},
		{"odd roll", oddRoll, ResolveDelay},
		// The move glides, then each step plays in turn, then the margin.
		{"pothole opens", ev(rules.Moved, rules.RolledPothole, rules.Target, rules.PotholeOpened),
			MoveTime + 600*time.Millisecond + 1250*time.Millisecond + 550*time.Millisecond + PauseMargin},
		// The longest kind of turn: a re-roll, a second target, a saving roll
		// that fails, the cap closing the oldest hole, the fall, the new hole.
		{"re-roll, saving roll, cap, fall", ev(rules.Moved, rules.Captured, rules.RolledPothole, rules.Target, rules.Reroll, rules.Target,
			rules.SavingRoll, rules.Fell, rules.PotholeClosed, rules.PotholeOpened),
			MoveTime + (600+1250+500+1250+750+600+400+550)*time.Millisecond + PauseMargin},
	}
	for _, c := range cases {
		if got := pauseFor(c.ev); got != c.want {
			t.Errorf("%s: pause %v, want %v", c.name, got, c.want)
		}
	}
}

// A roll that opens a pothole animates for longer than 2 s (the roll, the
// file and rank dice, the hole), so Black's first-move deadline starts when
// the animation ends.
func TestALongRollDelaysTheNextClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g, _, _ := seated(t, NewHub(&script{rolls: []int{2, 2, 4}}, nil)) // even, then b4: a pothole opens
		play(t, g, "alice", "e2e4", 0)
		v := recvView(t, g, "bob")
		roll := MoveTime + 600*time.Millisecond + 1250*time.Millisecond + 550*time.Millisecond + PauseMargin
		want := time.Now().Add(roll + FirstMoveTime).UnixMilli()
		if v.Clock.FirstMoveDeadline != want {
			t.Fatalf("first-move deadline %d, want %d (after the roll's %v)", v.Clock.FirstMoveDeadline, want, roll)
		}
		finish(t, g)
	})
}
