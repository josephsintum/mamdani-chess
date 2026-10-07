package rules

import "math/bits"

// Bitboard is a set of squares, one bit per square: bit 0 is a1, bit 63 is
// h8. Pieces, the Mamdani and potholes all stop sliders the same way, so
// move generation works on one "blocked" set and the tables here never need
// to know which is which.
type Bitboard uint64

// bit is the set holding just s. s must be on the board.
func bit(s Square) Bitboard { return 1 << uint(s) }

// Has reports whether s is in the set. NoSquare never is.
func (b Bitboard) Has(s Square) bool { return s != NoSquare && b&bit(s) != 0 }

// First returns the lowest square in the set, or NoSquare if it is empty.
func (b Bitboard) First() Square {
	if b == 0 {
		return NoSquare
	}
	return Square(bits.TrailingZeros64(uint64(b)))
}

// Last returns the highest square in the set, or NoSquare if it is empty.
func (b Bitboard) Last() Square {
	if b == 0 {
		return NoSquare
	}
	return Square(63 - bits.LeadingZeros64(uint64(b)))
}

// Count returns how many squares are in the set.
func (b Bitboard) Count() int { return bits.OnesCount64(uint64(b)) }

// pop removes and returns the lowest square. b must not be empty.
func (b *Bitboard) pop() Square {
	s := b.First()
	*b &= *b - 1
	return s
}

// The eight queen directions. The first four run towards higher squares,
// so the nearest blocker on them is the lowest set bit; the last four run
// the other way.
var dirSteps = [8][2]int{{0, 1}, {1, 0}, {1, 1}, {-1, 1}, {0, -1}, {-1, 0}, {-1, -1}, {1, -1}}

var (
	rookDirIdx   = [4]int{0, 1, 4, 5}
	bishopDirIdx = [4]int{2, 3, 6, 7}
)

var knightJumps = [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}

// Attack tables, filled once at startup.
var (
	rays          [8][64]Bitboard // squares beyond each square in each direction
	knightAttacks [64]Bitboard
	kingAttacks   [64]Bitboard
	pawnAttacks   [2][64]Bitboard // by the pawn's color
)

func init() {
	for s := range Square(64) {
		for d, step := range dirSteps {
			for t := s.Offset(step[0], step[1]); t != NoSquare; t = t.Offset(step[0], step[1]) {
				rays[d][s] |= bit(t)
			}
		}
		knightAttacks[s] = leaper(s, knightJumps)
		kingAttacks[s] = leaper(s, dirSteps[:])
		pawnAttacks[White][s] = leaper(s, [][2]int{{-1, 1}, {1, 1}})
		pawnAttacks[Black][s] = leaper(s, [][2]int{{-1, -1}, {1, -1}})
	}
}

func leaper(s Square, steps [][2]int) Bitboard {
	var b Bitboard
	for _, st := range steps {
		if t := s.Offset(st[0], st[1]); t != NoSquare {
			b |= bit(t)
		}
	}
	return b
}

// rayAttack returns the squares a slider on s reaches in direction d: every
// empty square up to and including the first blocked one.
func rayAttack(d int, s Square, blocked Bitboard) Bitboard {
	ray := rays[d][s]
	hits := ray & blocked
	if hits == 0 {
		return ray
	}
	first := hits.First()
	if d >= 4 {
		first = hits.Last()
	}
	return ray ^ rays[d][first]
}

func rookAttacks(s Square, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range rookDirIdx {
		b |= rayAttack(d, s, blocked)
	}
	return b
}

func bishopAttacks(s Square, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range bishopDirIdx {
		b |= rayAttack(d, s, blocked)
	}
	return b
}

func queenAttacks(s Square, blocked Bitboard) Bitboard {
	return rookAttacks(s, blocked) | bishopAttacks(s, blocked)
}
