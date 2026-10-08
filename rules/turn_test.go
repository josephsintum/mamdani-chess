package rules

import (
	"errors"
	"slices"
	"testing"
)

func TestOddRollNoPothole(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(3))
	if want := []EventKind{Moved, RolledPothole}; !slices.Equal(kinds(ev), want) {
		t.Errorf("events %v, want %v", kinds(ev), want)
	}
	if len(open(p)) != 0 || p.Turn != Black {
		t.Errorf("potholes %v turn %v", open(p), p.Turn)
	}
}

func TestIllegalMovesRejected(t *testing.T) {
	// What a buggy or hostile client might send.
	for _, uci := range []string{
		"e2e5",  // too far
		"e7e5",  // opponent's piece
		"e2e4q", // promotion that isn't one
		"a5a7",  // the Mamdani capturing
		"e3e4",  // empty square
	} {
		if _, _, err := Apply(StartPosition(), mv(t, uci), dice()); !errors.Is(err, ErrIllegalMove) {
			t.Errorf("%s: got %v, want ErrIllegalMove", uci, err)
		}
	}
	p := setup(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare)
	if _, _, err := Apply(p, mv(t, "a7a8"), dice()); !errors.Is(err, ErrIllegalMove) {
		t.Errorf("promotion without a piece: got %v, want ErrIllegalMove", err)
	}
}

func TestPotholeClosesAfterRollersThirdMove(t *testing.T) {
	// Even roll, then file 4 rank 4: d4.
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 4, 4))
	if want := []EventKind{Moved, RolledPothole, Target, PotholeOpened}; !slices.Equal(kinds(ev), want) {
		t.Fatalf("events %v, want %v", kinds(ev), want)
	}
	if e, _ := find(ev, PotholeOpened); e.Color != White {
		t.Errorf("opened by %v, want White", e.Color)
	}
	// Only White's moves count it down; Black's leave it alone.
	for i, step := range []struct {
		uci  string
		left int8
	}{
		{"g8f6", 3}, {"g1f3", 2}, {"b8c6", 2}, {"f1c4", 1}, {"c6b8", 1},
	} {
		p, ev = apply(t, p, step.uci, dice(1))
		if got := open(p); len(got) != 1 || got[0] != (Hole{Sq: D4, By: White, Left: step.left, Seq: 1}) {
			t.Fatalf("after move %d (%s): holes %v, want d4 with %d left", i+1, step.uci, got, step.left)
		}
		if _, ok := find(ev, PotholeClosed); ok {
			t.Fatalf("%s closed a hole: %v", step.uci, ev)
		}
	}
	p, ev = apply(t, p, "b1c3", dice(1))
	if want := []EventKind{Moved, PotholeClosed, RolledPothole}; !slices.Equal(kinds(ev), want) {
		t.Fatalf("events %v, want %v", kinds(ev), want)
	}
	if e := ev[1]; e.Square != D4 || e.Color != White || len(open(p)) != 0 {
		t.Errorf("d4 should close after White's third move: %+v, open %v", e, open(p))
	}
}

func TestBothPlayersHolesOpenAtOnce(t *testing.T) {
	p, _ := apply(t, StartPosition(), "e2e4", dice(2, 4, 4)) // White: d4
	p, _ = apply(t, p, "e7e5", dice(4, 8, 3))                // Black: h3
	want := []Hole{{Sq: D4, By: White, Left: 3, Seq: 1}, {Sq: H3, By: Black, Left: 3, Seq: 2}}
	if got := open(p); !slices.Equal(got, want) {
		t.Errorf("holes %v, want %v", got, want)
	}
}

// fiveHoles is the start position with HoleCap holes open, none next to
// the Mamdani or on the a5-d2 diagonal, and none on its last round. The
// oldest, on d5, sits in a middle slot, so the cap must go by Seq.
func fiveHoles() Position {
	p := StartPosition()
	p.Potholes = [HoleCap]Hole{
		{Sq: F4, By: White, Left: 2, Seq: 2},
		{Sq: G5, By: Black, Left: 3, Seq: 5},
		{Sq: D5, By: Black, Left: 2, Seq: 1},
		{Sq: H4, By: White, Left: 3, Seq: 4},
		{Sq: C6, By: Black, Left: 2, Seq: 3},
	}
	return p
}

func TestCapClosesOldest(t *testing.T) {
	p, ev := apply(t, fiveHoles(), "e2e3", dice(2, 4, 4)) // d4
	want := []EventKind{Moved, RolledPothole, Target, PotholeClosed, PotholeOpened}
	if !slices.Equal(kinds(ev), want) {
		t.Fatalf("events %v, want %v", kinds(ev), want)
	}
	if e := ev[3]; e.Square != D5 || e.Color != Black {
		t.Errorf("closed %+v, want Black's d5", e)
	}
	if e := ev[4]; e.Square != D4 || e.Color != White {
		t.Errorf("opened %+v, want White's d4", e)
	}
	if got, want := openSquares(p), []Square{F4, C6, H4, G5, D4}; !slices.Equal(got, want) {
		t.Errorf("holes %v, want %v", got, want)
	}
	if got := open(p)[4]; got.Left != HoleRounds || got.By != White {
		t.Errorf("new hole %+v, want White's with %d rounds", got, HoleRounds)
	}
}

func TestNothingOpensSoCapClosesNothing(t *testing.T) {
	for name, rolls := range map[string][]int{
		"saved":    {2, 4, 2, 5}, // the d2 pawn, saved
		"repaired": {2, 2, 4},    // b4, next to the Mamdani
	} {
		before := fiveHoles()
		p, ev := apply(t, before, "e2e4", dice(rolls...))
		if _, ok := find(ev, PotholeClosed); ok {
			t.Errorf("%s: a hole closed: %v", name, ev)
		}
		if got, want := openSquares(p), openSquares(before); !slices.Equal(got, want) {
			t.Errorf("%s: holes %v, want %v", name, got, want)
		}
	}
}

func TestRerollIfCapCloseExposesRoller(t *testing.T) {
	// Black's oldest hole on e4 shields the white king from the e8 rook.
	// Any hole that opens now pushes it out, so only a target the Mamdani
	// repairs at once (a1, next to b2) leaves White safe.
	const fen = "k3r3/8/8/8/8/8/7P/4K3 w - - 0 1"
	holes := []Hole{hole(E4, Black, 2), hole(B6, White, 3), hole(C6, Black, 3), hole(G6, White, 2), hole(H6, Black, 2)}
	p := setup(t, fen, B2, holes...)
	p, ev := apply(t, p, "h2h3", dice(2, 8, 1, 1, 1)) // h1, then a1
	if e, ok := find(ev, Reroll); !ok || e.Square != H1 || e.Reason != ReasonExposes {
		t.Fatalf("want an exposes re-roll on h1: %v", ev)
	}
	if e, ok := find(ev, Repaired); !ok || e.Square != A1 || !p.IsPothole(E4) {
		t.Errorf("a1 should be repaired at once and e4 stay open: %v", ev)
	}
	// With room under the cap, h1 opens and e4 stays.
	p = setup(t, fen, B2, holes[:4]...)
	p, ev = apply(t, p, "h2h3", dice(2, 8, 1))
	if e, ok := find(ev, PotholeOpened); !ok || e.Square != H1 || !p.IsPothole(E4) {
		t.Errorf("h1 should open with four holes open: %v", ev)
	}
}

func TestRerollKing(t *testing.T) {
	_, ev := apply(t, StartPosition(), "e2e4", dice(2, 5, 1, 4, 4)) // e1, then d4
	if e, ok := find(ev, Reroll); !ok || e.Square != E1 || e.Reason != ReasonKing {
		t.Errorf("want a king re-roll on e1: %v", ev)
	}
	if e, _ := find(ev, PotholeOpened); e.Square != D4 {
		t.Errorf("pothole should open on d4: %v", ev)
	}
}

func TestRollOnPotholeResetsIt(t *testing.T) {
	// Black's d4 is on its last round and older than White's h3. White's
	// roll lands on d4: it becomes White's, with every round to go, and the
	// newest hole, so the cap now closes h3 before it.
	p := StartPosition()
	p.Potholes[0] = Hole{Sq: D4, By: Black, Left: 1, Seq: 1}
	p.Potholes[1] = Hole{Sq: H3, By: White, Left: 2, Seq: 2}
	p, ev := apply(t, p, "e2e4", dice(6, 4, 4))
	if want := []EventKind{Moved, RolledPothole, Target, PotholeReset}; !slices.Equal(kinds(ev), want) {
		t.Fatalf("events %v, want %v", kinds(ev), want)
	}
	if e := ev[3]; e.Square != D4 || e.Color != White || e.Was != Black || e.Left != 1 {
		t.Errorf("reset %+v, want d4 by White, was Black's with 1 left", e)
	}
	want := []Hole{{Sq: H3, By: White, Left: 1, Seq: 2}, {Sq: D4, By: White, Left: HoleRounds, Seq: 3}}
	if got := open(p); !slices.Equal(got, want) {
		t.Errorf("holes %v, want %v", got, want)
	}
}

func TestResetUnderCapClosesNothing(t *testing.T) {
	before := fiveHoles()
	p, ev := apply(t, before, "e2e4", dice(2, 6, 4)) // f4, White's
	if _, ok := find(ev, PotholeClosed); ok {
		t.Errorf("a hole closed: %v", ev)
	}
	if got, want := openSquares(p), []Square{D5, C6, H4, G5, F4}; !slices.Equal(got, want) {
		t.Errorf("holes %v, want %v", got, want)
	}
}

func TestRerollCap(t *testing.T) {
	rolls := []int{2}
	for range maxRerolls + 1 {
		rolls = append(rolls, 5, 1) // e1, the king, every time
	}
	_, ev := apply(t, StartPosition(), "e2e4", dice(rolls...))
	if ev[len(ev)-1].Kind != NoPothole {
		t.Errorf("last event %v, want no_pothole", ev[len(ev)-1].Kind)
	}
}

func TestNextToMamdaniRepairedOnOpening(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 2, 4)) // b4, next to a5
	if e, ok := find(ev, Repaired); !ok || e.Square != B4 {
		t.Errorf("want b4 repaired at once: %v", ev)
	}
	if len(open(p)) != 0 {
		t.Errorf("no pothole should stay open: %v", open(p))
	}
}

func TestPieceWithoutMamdaniLineFalls(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 7, 8)) // g8 knight; no line from a5
	if e, ok := find(ev, Fell); !ok || e.Square != G8 || e.Piece != NewPiece(Black, Knight) {
		t.Fatalf("want the g8 knight to fall: %v", ev)
	}
	if _, ok := find(ev, SavingRoll); ok {
		t.Error("no saving roll without a clear Mamdani line")
	}
	if p.Board[G8] != NoPiece || !slices.Equal(openSquares(p), []Square{G8}) || p.Halfmove != 0 {
		t.Errorf("board %v potholes %v halfmove %d", p.Board[G8], openSquares(p), p.Halfmove)
	}
}

func TestSavingRoll(t *testing.T) {
	// a5-b4-c3-d2 is a clear diagonal, so the d2 pawn gets a saving roll,
	// made by its owner.
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 4, 2, 5))
	e, ok := find(ev, SavingRoll)
	if !ok || !e.Saved || e.Color != White || e.Roll != 5 {
		t.Fatalf("want a successful white saving roll: %v", ev)
	}
	if p.Board[D2] != NewPiece(White, Pawn) || len(open(p)) != 0 {
		t.Error("a saved piece stays and no pothole opens")
	}

	p, ev = apply(t, StartPosition(), "e2e4", dice(2, 4, 2, 6))
	if e, _ := find(ev, SavingRoll); e.Saved {
		t.Fatal("even saving roll should fail")
	}
	if p.Board[D2] != NoPiece || !slices.Equal(openSquares(p), []Square{D2}) {
		t.Error("the pawn falls and the pothole opens")
	}
}

func TestMamdaniFalls(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 1, 5, 3)) // a5, saved
	if e, ok := find(ev, SavingRoll); !ok || !e.Saved || e.Piece != MamdaniPiece || e.Color != White {
		t.Fatalf("the roller makes the Mamdani's saving roll: %v", ev)
	}
	if p.Mamdani != A5 {
		t.Fatal("saved Mamdani stays")
	}

	p, ev = apply(t, StartPosition(), "e2e4", dice(2, 1, 5, 4)) // a5, falls
	if e, ok := find(ev, Fell); !ok || e.Piece != MamdaniPiece {
		t.Fatalf("Mamdani should fall: %v", ev)
	}
	if p.Mamdani != NoSquare || !slices.Equal(openSquares(p), []Square{A5}) {
		t.Fatalf("mamdani %v potholes %v", p.Mamdani, openSquares(p))
	}
	// With the Mamdani gone there are no more saving rolls: a7 falls at once.
	_, ev = apply(t, p, "e7e5", dice(2, 1, 7))
	if _, ok := find(ev, SavingRoll); ok {
		t.Error("no saving rolls after the Mamdani has fallen")
	}
}

func TestRerollIfFallExposesRoller(t *testing.T) {
	// The e2 bishop shields the white king from the e8 rook.
	p := setup(t, "k3r3/8/8/8/8/8/4B2P/4K3 w - - 0 1", NoSquare)
	_, ev := apply(t, p, "h2h3", dice(2, 5, 2, 8, 8)) // e2, then h8
	if e, ok := find(ev, Reroll); !ok || e.Square != E2 || e.Reason != ReasonExposes {
		t.Errorf("want an exposes re-roll on e2: %v", ev)
	}
}

func TestRollMatesOnLastEscapeSquare(t *testing.T) {
	// Nf7+ leaves the h8 king one way out, g8. The roll opens a hole there.
	g := NewGameFrom(setup(t, "7k/6pp/8/6N1/8/8/8/4K3 w - - 0 1", NoSquare))
	ev, err := g.Play(mv(t, "g5f7"), dice(2, 7, 8)) // g8
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := find(ev, PotholeOpened); !ok || e.Square != G8 {
		t.Fatalf("g8 should open: %v", ev)
	}
	if want := (Result{Over: true, Winner: White, Reason: Checkmate}); g.Result != want {
		t.Errorf("result %+v, want White wins by checkmate", g.Result)
	}
}

func TestRollMatesWhenOnlyDefenderFalls(t *testing.T) {
	// Ra8+ is answered only by the c7 knight (Nxa8 or Ne8). It falls.
	g := NewGameFrom(setup(t, "7k/2n3pp/8/8/8/8/8/R5K1 w - - 0 1", NoSquare))
	ev, err := g.Play(mv(t, "a1a8"), dice(2, 3, 7)) // c7
	if err != nil {
		t.Fatal(err)
	}
	if e, ok := find(ev, Fell); !ok || e.Square != C7 {
		t.Fatalf("the c7 knight should fall: %v", ev)
	}
	if want := (Result{Over: true, Winner: White, Reason: Checkmate}); g.Result != want {
		t.Errorf("result %+v, want White wins by checkmate", g.Result)
	}
}

func TestRepairStepAfterMove(t *testing.T) {
	p := StartPosition()
	p.Potholes[0] = hole(D4, Black, 2)
	p, ev := apply(t, p, "a5c5", dice(1)) // c5 touches d4
	if e, ok := find(ev, Repaired); !ok || e.Square != D4 || len(open(p)) != 0 {
		t.Errorf("moving next to d4 should repair it: %v", ev)
	}
}

func TestBadDiceRejected(t *testing.T) {
	for _, rolls := range [][]int{{0}, {9}, {2, 9, 1}, {2, 1, 0}, {2, 4, 2, 10}} {
		p := StartPosition()
		_, _, err := Apply(p, mv(t, "e2e4"), dice(rolls...))
		if !errors.Is(err, ErrBadDie) {
			t.Errorf("rolls %v: got %v, want ErrBadDie", rolls, err)
		}
		if p != StartPosition() {
			t.Errorf("rolls %v: position changed", rolls)
		}
	}
	corrupt := []Turn{{Move: mv(t, "e2e4"), Dice: []int{2, 9, 1}}}
	if _, err := Replay(StartPosition(), corrupt); !errors.Is(err, ErrBadDie) {
		t.Errorf("replay with a 9: got %v, want ErrBadDie", err)
	}
}
