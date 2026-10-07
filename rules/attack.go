package rules

// Attacked reports whether any piece of color by attacks s. Potholes and the
// Mamdani block sliders exactly like pieces do.
func (p *Position) Attacked(s Square, by Color) bool {
	return p.attackedWith(s, by, p.blocked())
}

// attackedWith is Attacked with the blocked set already worked out, for
// callers that test several squares in one position.
func (p *Position) attackedWith(s Square, by Color, blocked Bitboard) bool {
	them := p.byColor[by]
	straight := p.byKind[Rook] | p.byKind[Queen]
	diagonal := p.byKind[Bishop] | p.byKind[Queen]
	// A pawn of color by attacks s from the squares a pawn of the other
	// color on s would attack.
	return pawnAttacks[by.Other()][s]&them&p.byKind[Pawn] != 0 ||
		knightAttacks[s]&them&p.byKind[Knight] != 0 ||
		kingAttacks[s]&them&p.byKind[King] != 0 ||
		rookAttacks(s, blocked)&them&straight != 0 ||
		bishopAttacks(s, blocked)&them&diagonal != 0
}

// InCheck reports whether c's king is attacked.
func (p *Position) InCheck(c Color) bool {
	k := p.King(c)
	return k != NoSquare && p.attackedWith(k, c.Other(), p.blocked())
}
