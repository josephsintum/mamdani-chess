package rules

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"
)

// naiveSlide walks one square at a time, the way the mailbox engine did:
// the reference the attack tables must match.
func naiveSlide(s Square, dirs []int, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range dirs {
		step := dirSteps[d]
		for t := s.Offset(step[0], step[1]); t != NoSquare; t = t.Offset(step[0], step[1]) {
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
			if got, want := rookAttacks(s, blocked), naiveSlide(s, rookDirIdx[:], blocked); got != want {
				t.Fatalf("rook on %v, blocked %x: got %x want %x", s, uint64(blocked), uint64(got), uint64(want))
			}
			if got, want := bishopAttacks(s, blocked), naiveSlide(s, bishopDirIdx[:], blocked); got != want {
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
		return errors.New("bitboards out of step with the board")
	}
	return nil
}
