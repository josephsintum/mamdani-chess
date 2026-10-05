package rules

import "testing"

func TestSquares(t *testing.T) {
	for _, c := range []struct {
		name       string
		sq         Square
		file, rank int
	}{
		{"a1", A1, 0, 0}, {"h1", H1, 7, 0}, {"e4", E4, 4, 3}, {"h8", H8, 7, 7},
	} {
		s, err := ParseSquare(c.name)
		if err != nil || s != c.sq || s.File() != c.file || s.Rank() != c.rank || s.String() != c.name {
			t.Errorf("%s: got %v (%d,%d) err %v", c.name, s, s.File(), s.Rank(), err)
		}
	}
	for _, bad := range []string{"", "i1", "a9", "a10", "A1"} {
		if _, err := ParseSquare(bad); err == nil {
			t.Errorf("ParseSquare(%q) should fail", bad)
		}
	}
	if H1.Offset(1, 0) != NoSquare || A1.Offset(0, -1) != NoSquare || E4.Offset(-1, 1) != D5 {
		t.Error("Offset is wrong at the edges or in the middle")
	}
}

func TestPieces(t *testing.T) {
	p := NewPiece(Black, Knight)
	if p.Color() != Black || p.Kind() != Knight || NewPiece(White, Pawn).Color() != White {
		t.Error("piece packing is wrong")
	}
	if White.Other() != Black || Black.Other() != White {
		t.Error("Other is wrong")
	}
}

func TestStartPosition(t *testing.T) {
	p := StartPosition()
	if p.Board[E1] != NewPiece(White, King) || p.Board[D8] != NewPiece(Black, Queen) || p.Board[E4] != NoPiece {
		t.Error("pieces are misplaced")
	}
	if p.Mamdani != A5 || p.Potholes != noHoles() {
		t.Errorf("mamdani %v potholes %v", p.Mamdani, p.Potholes)
	}
	if p.Turn != White || p.Castling != WhiteKingside|WhiteQueenside|BlackKingside|BlackQueenside || p.EP != NoSquare || p.Fullmove != 1 {
		t.Errorf("state %+v", p)
	}
}

func TestParseFEN(t *testing.T) {
	p, err := ParseFEN("4k3/8/8/3Pp3/8/8/8/4K3 b - e6 3 40")
	if err != nil {
		t.Fatal(err)
	}
	if p.Turn != Black || p.EP != E6 || p.Halfmove != 3 || p.Fullmove != 40 || p.Castling != 0 || p.Mamdani != NoSquare {
		t.Errorf("state %+v", p)
	}
	for _, bad := range []string{
		"",
		"8/8/8/8/8/8/8 w - - 0 1",                 // 7 ranks
		"9/8/8/8/8/8/8/8 w - - 0 1",               // too wide
		"8/8/8/8/8/8/8/8 x - - 0 1",               // side to move
		"8/8/8/8/8/8/8/8 w X - 0 1",               // castling
		"8/8/8/8/8/8/8/8 w - z9 0 1",              // en passant
		"8/8/8/8/8/8/8/8 w - - zero 1",            // clocks
		"rnbqkbnr/ppppxppp/8/8/8/8/8/8 w - - 0 1", // bad piece
	} {
		if _, err := ParseFEN(bad); err == nil {
			t.Errorf("ParseFEN(%q) should fail", bad)
		}
	}
}
