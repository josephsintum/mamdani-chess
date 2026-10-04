package rules

var (
	rookDirs    = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	bishopDirs  = [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	queenDirs   = append(append([][2]int{}, rookDirs...), bishopDirs...)
	knightJumps = [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}
)

// Attacked reports whether any piece of color by attacks s. Potholes and the
// Mamdani block sliders exactly like pieces do.
func (p *Position) Attacked(s Square, by Color) bool {
	// A pawn of color by attacks s from one rank behind s (from by's side).
	back := -1
	if by == Black {
		back = 1
	}
	for _, df := range []int{-1, 1} {
		if t := s.Offset(df, back); t != NoSquare && p.Board[t] == NewPiece(by, Pawn) {
			return true
		}
	}
	for _, j := range knightJumps {
		if t := s.Offset(j[0], j[1]); t != NoSquare && p.Board[t] == NewPiece(by, Knight) {
			return true
		}
	}
	for _, d := range queenDirs {
		if t := s.Offset(d[0], d[1]); t != NoSquare && p.Board[t] == NewPiece(by, King) {
			return true
		}
	}
	return p.slidingAttack(s, by, rookDirs, Rook) || p.slidingAttack(s, by, bishopDirs, Bishop)
}

func (p *Position) slidingAttack(s Square, by Color, dirs [][2]int, k Kind) bool {
	for _, d := range dirs {
		t := s.Offset(d[0], d[1])
		for t != NoSquare && !p.Blocked(t) {
			t = t.Offset(d[0], d[1])
		}
		if t == NoSquare {
			continue
		}
		if pc := p.Board[t]; pc == NewPiece(by, k) || pc == NewPiece(by, Queen) {
			return true
		}
	}
	return false
}

// InCheck reports whether c's king is attacked.
func (p *Position) InCheck(c Color) bool {
	k := p.King(c)
	return k != NoSquare && p.Attacked(k, c.Other())
}
