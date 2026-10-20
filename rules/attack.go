package rules

// attacked reports whether any piece of color by attacks s, with sliders
// stopped by blocked: the position's blocked set, which callers testing
// several squares work out once.
func (p *Position) attacked(s Square, by Color, blocked Bitboard) bool {
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
	return k != NoSquare && p.attacked(k, c.Other(), p.blocked())
}
