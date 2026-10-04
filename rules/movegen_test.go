package rules

import "testing"

func TestSlidersStopAtPotholes(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", NoSquare, D1, NoSquare)
	for _, uci := range []string{"a1b1", "a1c1", "a1a8"} {
		if !legal(t, p, uci) {
			t.Errorf("%s should be legal", uci)
		}
	}
	if legal(t, p, "a1d1") {
		t.Error("rook landed on a pothole")
	}
}

func TestKnightsJumpPotholesButCannotLand(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/1N2K3 w - - 0 1", NoSquare, C3, C2)
	if !legal(t, p, "b1d2") || !legal(t, p, "b1a3") {
		t.Error("knight should jump over the pothole on c2")
	}
	if legal(t, p, "b1c3") {
		t.Error("knight landed on a pothole")
	}
}

func TestPawnsAndPotholes(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, NoSquare, E3)
	if legal(t, p, "e2e3") || legal(t, p, "e2e4") {
		t.Error("pawn moved into or across a pothole on e3")
	}
	p = setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, NoSquare, E4)
	if !legal(t, p, "e2e3") || legal(t, p, "e2e4") {
		t.Error("pothole on e4 should block only the double step")
	}
}

func TestPotholeBlocksCheck(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/4K2r w - - 0 1", NoSquare, NoSquare, F1)
	if p.InCheck(White) {
		t.Error("a pothole on f1 should block the rook's check")
	}
}

func TestCastlingAndPotholes(t *testing.T) {
	const fen = "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1"
	p := setup(t, fen, NoSquare, NoSquare, NoSquare)
	if !legal(t, p, "e1g1") || !legal(t, p, "e1c1") {
		t.Fatal("both castles should be legal")
	}
	p = setup(t, fen, NoSquare, NoSquare, B1)
	if legal(t, p, "e1c1") || !legal(t, p, "e1g1") {
		t.Error("a pothole on b1 should block only queenside castling")
	}
	p = setup(t, fen, NoSquare, NoSquare, F1)
	if legal(t, p, "e1g1") {
		t.Error("a pothole on f1 should block kingside castling")
	}
	p = setup(t, fen, B1, NoSquare, NoSquare)
	if legal(t, p, "e1c1") {
		t.Error("the Mamdani on b1 should block queenside castling")
	}
}

func TestPotholeCancelsEnPassant(t *testing.T) {
	const fen = "4k3/8/8/3Pp3/8/8/8/4K3 w - e6 0 1"
	p := setup(t, fen, NoSquare, NoSquare, NoSquare)
	if !legal(t, p, "d5e6") {
		t.Fatal("en passant should be legal")
	}
	p = setup(t, fen, NoSquare, NoSquare, E6)
	if legal(t, p, "d5e6") {
		t.Error("a pothole on e6 should cancel en passant")
	}
}

func TestMamdaniMovesLikeAQueenWithoutCapturing(t *testing.T) {
	p := StartPosition()
	// 20 normal moves + 13 Mamdani moves from a5: a6, a4, a3, b5-h5, b6, b4, c3.
	if got := len(p.LegalMoves()); got != 33 {
		t.Errorf("start position has %d legal moves, want 33", got)
	}
	for _, uci := range []string{"a5a7", "a5a2", "a5c7", "a5d2"} {
		if legal(t, p, uci) {
			t.Errorf("Mamdani captured with %s", uci)
		}
	}
}

func TestMamdaniCanGoStraightBack(t *testing.T) {
	p, _ := apply(t, StartPosition(), "a5b5", dice(1))
	if !legal(t, p, "b5a5") {
		t.Error("Black should be able to move the Mamdani straight back")
	}
}

func TestMamdaniCannotUnpinOwnKing(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/r3K3 w - - 0 1", C1, NoSquare, NoSquare)
	if !legal(t, p, "c1b1") || !legal(t, p, "c1d1") {
		t.Error("the Mamdani may slide along the pin line")
	}
	if legal(t, p, "c1c2") {
		t.Error("moving the Mamdani off the line would leave White in check")
	}
}

func TestMoveIllegalIfOwnPotholeClosingExposesKing(t *testing.T) {
	const fen = "k3r3/8/8/8/8/8/8/4K3 w - - 0 1"
	p := setup(t, fen, NoSquare, E4, NoSquare)
	if got := len(p.LegalMoves()); got != 4 || legal(t, p, "e1e2") {
		t.Errorf("White's own pothole closes after this move, so only king moves off the e-file are legal; got %v", p.LegalMoves())
	}
	p = setup(t, fen, NoSquare, NoSquare, E4)
	if !legal(t, p, "e1e2") {
		t.Error("Black's pothole stays open, so e1e2 is safe")
	}
}

func TestMoveIllegalIfRepairExposesKing(t *testing.T) {
	p := setup(t, "k3r3/8/8/8/8/8/8/4K3 w - - 0 1", H3, NoSquare, E4)
	if legal(t, p, "h3f3") || legal(t, p, "h3f5") {
		t.Error("moving the Mamdani next to e4 repairs it and exposes the king")
	}
	if !legal(t, p, "h3g3") {
		t.Error("h3g3 leaves the pothole alone")
	}
}
