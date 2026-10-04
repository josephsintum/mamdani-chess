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
	if p.Potholes != [2]Square{NoSquare, NoSquare} || p.Turn != Black {
		t.Errorf("potholes %v turn %v", p.Potholes, p.Turn)
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
	p := setup(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare, NoSquare, NoSquare)
	if _, _, err := Apply(p, mv(t, "a7a8"), dice()); !errors.Is(err, ErrIllegalMove) {
		t.Errorf("promotion without a piece: got %v, want ErrIllegalMove", err)
	}
}

func TestPotholeOpensAndClosesAfterRollersNextMove(t *testing.T) {
	// Even roll, then file 4 rank 4: d4.
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 4, 4))
	if want := []EventKind{Moved, RolledPothole, Target, PotholeOpened}; !slices.Equal(kinds(ev), want) {
		t.Fatalf("events %v, want %v", kinds(ev), want)
	}
	if p.Potholes[White] != D4 {
		t.Fatalf("white pothole %v, want d4", p.Potholes[White])
	}
	p, _ = apply(t, p, "g8f6", dice(1))
	if p.Potholes[White] != D4 {
		t.Fatal("pothole closed before White's next move")
	}
	p, ev = apply(t, p, "g1f3", dice(1))
	if e, ok := find(ev, PotholeClosed); !ok || e.Square != D4 || p.Potholes[White] != NoSquare {
		t.Errorf("pothole should close after White's next move: %v", ev)
	}
}

func TestTwoPotholesAtOnce(t *testing.T) {
	p, _ := apply(t, StartPosition(), "e2e4", dice(2, 4, 4)) // White: d4
	p, _ = apply(t, p, "e7e5", dice(4, 8, 3))                // Black: h3
	if p.Potholes != [2]Square{D4, H3} {
		t.Errorf("potholes %v, want [d4 h3]", p.Potholes)
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

func TestRerollExistingPothole(t *testing.T) {
	p := StartPosition()
	p.Potholes[Black] = D4
	_, ev := apply(t, p, "e2e4", dice(6, 4, 4, 8, 3)) // d4, then h3
	if e, ok := find(ev, Reroll); !ok || e.Reason != ReasonPothole {
		t.Errorf("want a pothole re-roll: %v", ev)
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
	if p.Potholes != [2]Square{NoSquare, NoSquare} {
		t.Errorf("no pothole should stay open: %v", p.Potholes)
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
	if p.Board[G8] != NoPiece || p.Potholes[White] != G8 || p.Halfmove != 0 {
		t.Errorf("board %v pothole %v halfmove %d", p.Board[G8], p.Potholes[White], p.Halfmove)
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
	if p.Board[D2] != NewPiece(White, Pawn) || p.Potholes[White] != NoSquare {
		t.Error("a saved piece stays and no pothole opens")
	}

	p, ev = apply(t, StartPosition(), "e2e4", dice(2, 4, 2, 6))
	if e, _ := find(ev, SavingRoll); e.Saved {
		t.Fatal("even saving roll should fail")
	}
	if p.Board[D2] != NoPiece || p.Potholes[White] != D2 {
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
	if p.Mamdani != NoSquare || p.Potholes[White] != A5 {
		t.Fatalf("mamdani %v pothole %v", p.Mamdani, p.Potholes[White])
	}
	// With the Mamdani gone there are no more saving rolls: a7 falls at once.
	_, ev = apply(t, p, "e7e5", dice(2, 1, 7))
	if _, ok := find(ev, SavingRoll); ok {
		t.Error("no saving rolls after the Mamdani has fallen")
	}
}

func TestRerollIfFallExposesRoller(t *testing.T) {
	// The e2 bishop shields the white king from the e8 rook.
	p := setup(t, "k3r3/8/8/8/8/8/4B2P/4K3 w - - 0 1", NoSquare, NoSquare, NoSquare)
	_, ev := apply(t, p, "h2h3", dice(2, 5, 2, 8, 8)) // e2, then h8
	if e, ok := find(ev, Reroll); !ok || e.Square != E2 || e.Reason != ReasonExposes {
		t.Errorf("want an exposes re-roll on e2: %v", ev)
	}
}

func TestRerollIfResultWouldCheckmate(t *testing.T) {
	// Ra8+ is answered only by the c7 knight (Nxa8 or Ne8). If it fell,
	// Black would be mated by the roll, so the dice go again.
	p := setup(t, "7k/2n3pp/8/8/8/8/8/R5K1 w - - 0 1", NoSquare, NoSquare, NoSquare)
	p, ev := apply(t, p, "a1a8", dice(2, 3, 7, 1, 1)) // c7, then a1
	if e, ok := find(ev, Reroll); !ok || e.Square != C7 || e.Reason != ReasonCheckmate {
		t.Errorf("want a checkmate re-roll on c7: %v", ev)
	}
	if p.Board[C7] != NewPiece(Black, Knight) {
		t.Error("the knight must survive")
	}
}

func TestRepairStepAfterMove(t *testing.T) {
	p := StartPosition()
	p.Potholes[Black] = D4
	p, ev := apply(t, p, "a5c5", dice(1)) // c5 touches d4
	if e, ok := find(ev, Repaired); !ok || e.Square != D4 || p.Potholes[Black] != NoSquare {
		t.Errorf("moving next to d4 should repair it: %v", ev)
	}
}

func TestMamdaniMoveDoesNotResetFiftyMoveCount(t *testing.T) {
	p := StartPosition()
	p.Halfmove = 10
	p, _ = apply(t, p, "a5b5", dice(1))
	if p.Halfmove != 11 {
		t.Errorf("halfmove %d, want 11", p.Halfmove)
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
