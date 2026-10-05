package rules

// play makes move m for the side to move, then runs the countdown and
// repair steps and passes the turn. It assumes m is pseudo-legal. Events are
// appended to *ev; pass nil when only the resulting position matters, and
// nothing is recorded or allocated.
func (p *Position) play(m Move, ev *[]Event) {
	mover := p.Turn
	if m.From == p.Mamdani {
		emit(ev, Event{Kind: Moved, Move: m, Piece: MamdaniPiece, Color: mover})
		p.Mamdani = m.To
		p.EP = NoSquare
		p.Halfmove++ // Mamdani moves never reset the 50-move count
	} else {
		p.movePiece(m, ev)
	}
	if mover == Black {
		p.Fullmove++
	}
	// Count down: each of the mover's own holes loses a round, and any
	// with none left closes. The other player's holes wait for their moves.
	for i := range p.Potholes {
		h := &p.Potholes[i]
		if h.Sq == NoSquare || h.By != mover {
			continue
		}
		h.Left--
		if h.Left == 0 {
			p.closeHole(i, ev)
		}
	}
	p.repair(ev)
	p.Turn = mover.Other()
}

// emit records e if events are being kept.
func emit(ev *[]Event, e Event) {
	if ev != nil {
		*ev = append(*ev, e)
	}
}

func (p *Position) movePiece(m Move, ev *[]Event) {
	pc := p.Board[m.From]
	c := pc.Color()
	emit(ev, Event{Kind: Moved, Move: m, Piece: pc, Color: c})

	capSq := m.To
	if pc.Kind() == Pawn && m.To == p.EP && p.Board[m.To] == NoPiece {
		capSq = m.To.Offset(0, -pawnDir(c))
	}
	captured := p.take(capSq)
	if captured != NoPiece {
		emit(ev, Event{Kind: Captured, Square: capSq, Piece: captured})
	}

	p.take(m.From)
	if m.Promo != NoKind {
		p.put(m.To, NewPiece(c, m.Promo))
	} else {
		p.put(m.To, pc)
	}

	if pc.Kind() == King && abs(m.To.File()-m.From.File()) == 2 {
		rookFrom, rookTo := m.From.Offset(3, 0), m.From.Offset(1, 0)
		if m.To.File() < m.From.File() {
			rookFrom, rookTo = m.From.Offset(-4, 0), m.From.Offset(-1, 0)
		}
		p.put(rookTo, p.take(rookFrom))
	}

	p.Castling &^= rightsLost(m.From) | rightsLost(m.To)

	p.EP = NoSquare
	if pc.Kind() == Pawn && abs(m.To.Rank()-m.From.Rank()) == 2 {
		p.EP = m.From.Offset(0, pawnDir(c))
	}

	if pc.Kind() == Pawn || captured != NoPiece {
		p.Halfmove = 0
	} else {
		p.Halfmove++
	}
}

// rightsLost returns the castling rights that disappear when a piece moves
// from, or is captured or falls on, s.
func rightsLost(s Square) Castling {
	switch s {
	case E1:
		return WhiteKingside | WhiteQueenside
	case H1:
		return WhiteKingside
	case A1:
		return WhiteQueenside
	case E8:
		return BlackKingside | BlackQueenside
	case H8:
		return BlackKingside
	case A8:
		return BlackQueenside
	}
	return 0
}

// repair removes every open pothole next to the Mamdani.
func (p *Position) repair(ev *[]Event) {
	if p.Mamdani == NoSquare {
		return
	}
	for i := range p.Potholes {
		if s := p.Potholes[i].Sq; s != NoSquare && adjacent(s, p.Mamdani) {
			p.Potholes[i].Sq = NoSquare
			emit(ev, Event{Kind: Repaired, Square: s})
		}
	}
}
