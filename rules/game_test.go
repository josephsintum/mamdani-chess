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

func TestFoolsMate(t *testing.T) {
	g := NewGame()
	play(t, g, "f2f3", "e7e5", "g2g4", "d8h4")
	if want := (Result{Over: true, Winner: Black, Reason: Checkmate}); g.Result != want {
		t.Errorf("result %+v, want %+v", g.Result, want)
	}
	if _, err := g.Play(mv(t, "a2a3"), dice(1)); !errors.Is(err, ErrGameOver) {
		t.Errorf("got %v, want ErrGameOver", err)
	}
}

func TestMamdaniCanBlockMate(t *testing.T) {
	const fen = "7k/8/8/8/8/8/6PP/r6K w - - 0 1" // back-rank check from a1
	if g := NewGameFrom(setup(t, fen, D4, NoSquare, NoSquare)); g.Result.Over {
		t.Error("Md1 blocks the check, so it is not mate")
	}
	g := NewGameFrom(setup(t, fen, NoSquare, NoSquare, NoSquare))
	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
		t.Errorf("without the Mamdani it is mate: %+v", g.Result)
	}
}

func TestStalemateCountsMamdaniMoves(t *testing.T) {
	const fen = "k7/8/1Q6/8/8/8/8/7K b - - 0 1"
	if g := NewGameFrom(setup(t, fen, H4, NoSquare, NoSquare)); g.Result.Over {
		t.Error("Black can still move the Mamdani, so it is not stalemate")
	}
	g := NewGameFrom(setup(t, fen, NoSquare, NoSquare, NoSquare))
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
		g := NewGameFrom(setup(t, c.fen, c.mamdani, NoSquare, NoSquare))
		if over := g.Result.Reason == InsufficientMaterial; over != c.over {
			t.Errorf("%s mamdani %v: insufficient=%v, want %v", c.fen, c.mamdani, over, c.over)
		}
	}
}

func TestReplay(t *testing.T) {
	g := NewGame()
	for _, step := range []struct {
		uci   string
		rolls []int
	}{
		{"e2e4", []int{2, 4, 4}},
		{"e7e5", []int{2, 4, 2, 6}},
		{"a5b5", []int{1}},
	} {
		if _, err := g.Play(mv(t, step.uci), dice(step.rolls...)); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Replay(StartPosition(), g.Turns)
	if err != nil {
		t.Fatal(err)
	}
	if r.Pos != g.Pos || r.Result != g.Result {
		t.Error("replay did not rebuild the same game")
	}
	short := []Turn{{Move: g.Turns[0].Move, Dice: []int{2}}}
	if _, err := Replay(StartPosition(), short); err == nil {
		t.Error("replay with missing dice should fail")
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
	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
		t.Fatalf("result %+v, want Black wins by checkmate", g.Result)
	}
	if d.Left() != 5 || len(g.Turns[len(g.Turns)-1].Dice) != 0 {
		t.Errorf("no dice should be rolled after mate; %d left, recorded %v", d.Left(), g.Turns[len(g.Turns)-1].Dice)
	}
}

func TestCheckHeldOffOnlyByOwnPotholeIsMate(t *testing.T) {
	// White's own hole on e4 blocks the e8 rook. Every white move closes it,
	// and nothing can block the e-file or step off it: that is checkmate,
	// not stalemate.
	p := setup(t, "k3r3/8/8/8/8/8/3P1P2/3RKR2 w - - 0 1", NoSquare, E4, NoSquare)
	g := NewGameFrom(p)
	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
		t.Errorf("result %+v, want Black wins by checkmate", g.Result)
	}
}
