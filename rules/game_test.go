package rules

import (
	"errors"
	"testing"
)

func play(t *testing.T, g *Game, ucis ...string) {
	t.Helper()
	for _, uci := range ucis {
		if _, err := g.Play(mv(t, uci), dice(1)); err != nil {
			t.Fatalf("%s: %v", uci, err)
		}
	}
}

func TestMamdaniCanBlockMate(t *testing.T) {
	const fen = "7k/8/8/8/8/8/6PP/r6K w - - 0 1" // back-rank check from a1
	if g := NewGameFrom(setup(t, fen, D4)); g.Result.Over {
		t.Error("Md1 blocks the check, so it is not mate")
	}
	g := NewGameFrom(setup(t, fen, NoSquare))
	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
		t.Errorf("without the Mamdani it is mate: %+v", g.Result)
	}
}

func TestStalemateCountsMamdaniMoves(t *testing.T) {
	const fen = "k7/8/1Q6/8/8/8/8/7K b - - 0 1"
	if g := NewGameFrom(setup(t, fen, H4)); g.Result.Over {
		t.Error("Black can still move the Mamdani, so it is not stalemate")
	}
	g := NewGameFrom(setup(t, fen, NoSquare))
	if g.Result.Reason != Stalemate || !g.Result.Draw {
		t.Errorf("want stalemate, got %+v", g.Result)
	}
}

func TestThreefoldRepetition(t *testing.T) {
	g := NewGame()
	play(t, g, "g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1")
	if g.Result.Over {
		t.Fatal("only two repetitions so far")
	}
	play(t, g, "f6g8")
	if g.Result.Reason != Repetition {
		t.Errorf("want repetition, got %+v", g.Result)
	}
}

func TestRepetitionForgetsPositionsThatCantComeBack(t *testing.T) {
	g := NewGame()
	play(t, g, "g1f3", "g8f6", "f3g1", "f6g8")
	if len(g.seen) != 4 {
		t.Fatalf("seen %d positions, want 4", len(g.seen))
	}
	// A pawn move can't be undone: no earlier position can repeat.
	play(t, g, "e2e3")
	if len(g.seen) != 1 {
		t.Errorf("after a pawn move seen holds %d positions, want 1", len(g.seen))
	}
	// Repetition still counts the positions since then.
	play(t, g, "g8f6", "g1f3", "f6g8", "f3g1", "g8f6", "g1f3", "f6g8")
	if g.Result.Over {
		t.Fatal("only two repetitions so far")
	}
	play(t, g, "f3g1")
	if g.Result.Reason != Repetition {
		t.Errorf("want repetition, got %+v", g.Result)
	}
	if g.seen != nil {
		t.Error("a finished game should drop its repetition table")
	}
}

func TestRepetitionKeyCountsRoundsLeft(t *testing.T) {
	const fen = "4k3/8/8/8/8/8/8/4K3 w - - 0 1"
	base := setup(t, fen, NoSquare, hole(C4, White, 2), hole(F5, Black, 3))
	// The same holes in other slots, opened in another order: same key.
	if same := setup(t, fen, NoSquare, hole(F5, Black, 3), hole(C4, White, 2)); same.Key() != base.Key() {
		t.Error("slot order and Seq changed the key")
	}
	for name, p := range map[string]Position{
		"rounds left": setup(t, fen, NoSquare, hole(C4, White, 1), hole(F5, Black, 3)),
		"roller":      setup(t, fen, NoSquare, hole(C4, Black, 2), hole(F5, Black, 3)),
		"square":      setup(t, fen, NoSquare, hole(C3, White, 2), hole(F5, Black, 3)),
	} {
		if p.Key() == base.Key() {
			t.Errorf("a different %s gave the same key", name)
		}
	}
}

func TestFiftyMoveRule(t *testing.T) {
	p := StartPosition()
	p.Halfmove = 99
	g := NewGameFrom(p)
	play(t, g, "a5b5") // a Mamdani move counts toward the 50
	if g.Result.Reason != FiftyMoves {
		t.Errorf("want fifty-move draw, got %+v", g.Result)
	}
}

func TestInsufficientMaterial(t *testing.T) {
	cases := []struct {
		fen     string
		mamdani Square
		over    bool
	}{
		{"4k3/8/8/8/8/8/8/4K3 w - - 0 1", A5, true},
		{"4k3/8/8/8/8/8/8/3NK3 w - - 0 1", NoSquare, true},
		{"4k3/8/8/8/8/8/8/3NK3 w - - 0 1", A5, false}, // the Mamdani can help a knight mate
		{"4k3/8/8/8/8/8/8/3RK3 w - - 0 1", NoSquare, false},
	}
	for _, c := range cases {
		g := NewGameFrom(setup(t, c.fen, c.mamdani))
		if over := g.Result.Reason == InsufficientMaterial; over != c.over {
			t.Errorf("%s mamdani %v: insufficient=%v, want %v", c.fen, c.mamdani, over, c.over)
		}
	}
}

// Replay checks the recorded dice: a turn must use them exactly.
func TestReplayRejectsWrongDice(t *testing.T) {
	for name, rolls := range map[string][]int{
		"missing": {2},       // an even roll needs two placement dice
		"unused":  {1, 4, 4}, // an odd roll is the turn's only die
	} {
		turns := []Turn{{Move: Move{From: E2, To: E4}, Dice: rolls}}
		if _, err := Replay(StartPosition(), turns); err == nil {
			t.Errorf("%s dice: replay should fail", name)
		}
	}
}

func TestBoardMateIsNotUndoneByDice(t *testing.T) {
	// With an even roll the dice could drop the h4 queen and break the mate.
	// A checkmating move ends the game before any pothole roll.
	g := NewGame()
	play(t, g, "f2f3", "e7e5", "g2g4")
	d := dice(2, 1, 3, 8, 4)
	if _, err := g.Play(mv(t, "d8h4"), d); err != nil {
		t.Fatal(err)
	}
	if want := (Result{Over: true, Winner: Black, Reason: Checkmate}); g.Result != want {
		t.Fatalf("result %+v, want %+v", g.Result, want)
	}
	if d.Left() != 5 || len(g.Turns[len(g.Turns)-1].Dice) != 0 {
		t.Errorf("no dice should be rolled after mate; %d left, recorded %v", d.Left(), g.Turns[len(g.Turns)-1].Dice)
	}
	if _, err := g.Play(mv(t, "a2a3"), dice(1)); !errors.Is(err, ErrGameOver) {
		t.Errorf("got %v, want ErrGameOver", err)
	}
}

func TestCheckHeldOffOnlyByOwnLastRoundPotholeIsMate(t *testing.T) {
	// White's own hole on e4 blocks the e8 rook. On its last round, every
	// white move closes it, and nothing can block the e-file or step off
	// it: that is checkmate, not stalemate.
	const fen = "k3r3/8/8/8/8/8/3P1P2/3RKR2 w - - 0 1"
	g := NewGameFrom(setup(t, fen, NoSquare, hole(E4, White, 1)))
	if want := (Result{Over: true, Winner: Black, Reason: Checkmate}); g.Result != want {
		t.Errorf("result %+v, want Black wins by checkmate", g.Result)
	}
	// With two rounds left it outlasts White's next move: Ke2 is safe.
	g = NewGameFrom(setup(t, fen, NoSquare, hole(E4, White, 2)))
	if g.Result.Over || !legal(t, g.Pos, "e1e2") {
		t.Errorf("result %+v, want the game on with Ke2 legal", g.Result)
	}
}
