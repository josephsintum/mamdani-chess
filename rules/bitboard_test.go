package rules

import (
	"math/rand/v2"
	"testing"
)

// naiveSlide walks one square at a time: the reference the attack tables
// must match.
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
