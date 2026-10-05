package rules

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// naiveSlide walks one square at a time, the way the mailbox engine did:
// the reference the attack tables must match.
func naiveSlide(s Square, dirs [][2]int, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range dirs {
		for t := s.Offset(d[0], d[1]); t != NoSquare; t = t.Offset(d[0], d[1]) {
			b |= bit(t)
			if blocked.Has(t) {
				break
			}
		}
	}
	return b
}

func TestSliderAttacksMatchANaiveWalk(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 400 {
		// Sparse to dense blocker patterns.
		blocked := Bitboard(r.Uint64())
		for range i % 4 {
			blocked &= Bitboard(r.Uint64())
		}
		for s := range Square(64) {
			if got, want := rookAttacks(s, blocked), naiveSlide(s, rookDirs, blocked); got != want {
				t.Fatalf("rook on %v, blocked %x: got %x want %x", s, uint64(blocked), uint64(got), uint64(want))
			}
			if got, want := bishopAttacks(s, blocked), naiveSlide(s, bishopDirs, blocked); got != want {
				t.Fatalf("bishop on %v, blocked %x: got %x want %x", s, uint64(blocked), uint64(got), uint64(want))
			}
		}
	}
}

func squares(b Bitboard) []Square {
	var out []Square
	for b != 0 {
		out = append(out, b.pop())
	}
	return out
}

func TestLeaperTables(t *testing.T) {
	for _, c := range []struct {
		name string
		got  Bitboard
		want []Square
	}{
		{"knight a1", knightAttacks[A1], []Square{C2, B3}},
		{"king a1", kingAttacks[A1], []Square{B1, A2, B2}},
		{"white pawn e4", pawnAttacks[White][E4], []Square{D5, F5}},
		{"black pawn e4", pawnAttacks[Black][E4], []Square{D3, F3}},
	} {
		if got := squares(c.got); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestBetween(t *testing.T) {
	if b, ok := between(A5, D2); !ok || fmt.Sprint(squares(b)) != fmt.Sprint([]Square{C3, B4}) {
		t.Errorf("a5-d2: %v %v", squares(b), ok)
	}
	if b, ok := between(A1, A2); !ok || b != 0 {
		t.Errorf("a1-a2 should be aligned with nothing between: %v %v", squares(b), ok)
	}
	for _, pair := range [][2]Square{{A1, B3}, {A1, A1}} {
		if _, ok := between(pair[0], pair[1]); ok {
			t.Errorf("%v-%v should not be aligned", pair[0], pair[1])
		}
	}
}

// checkBitboards fails if p's bitboards disagree with its Board.
func checkBitboards(p *Position) error {
	var want Position
	for s, pc := range p.Board {
		if pc != NoPiece {
			want.byColor[pc.Color()] |= bit(Square(s))
			want.byKind[pc.Kind()] |= bit(Square(s))
		}
	}
	if want.byColor != p.byColor || want.byKind != p.byKind {
		return fmt.Errorf("bitboards out of step with the board")
	}
	return nil
}
