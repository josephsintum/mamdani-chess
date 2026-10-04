package rules

import (
	"errors"
	"slices"
)

// Dice rolls eight-sided dice. D8 returns 1..8.
type Dice interface {
	D8() int
}

// ErrIllegalMove is returned by Apply for a move not in LegalMoves.
var ErrIllegalMove = errors.New("illegal move")

// maxRerolls caps placement re-rolls; past it no pothole opens this turn.
const maxRerolls = 64

// Apply plays one full turn: move, close, repair, pothole roll, placement
// and resolution. It returns the new position and what happened, in order.
// p is not modified.
func Apply(p Position, m Move, dice Dice) (Position, []Event, error) {
	if !slices.Contains(p.LegalMoves(), m) {
		return p, nil, ErrIllegalMove
	}
	mover := p.Turn
	ev := p.play(m, nil)
	ev = p.rollPothole(mover, dice, ev)
	return p, ev, nil
}

func (p *Position) rollPothole(mover Color, dice Dice, ev []Event) []Event {
	r := dice.D8()
	ev = append(ev, Event{Kind: RolledPothole, Roll: r, Color: mover})
	if r%2 == 1 {
		return ev
	}
	for range maxRerolls + 1 {
		file, rank := dice.D8(), dice.D8()
		s := Square((rank-1)*8 + file - 1)
		ev = append(ev, Event{Kind: Target, Square: s})
		if reason := p.rerollReason(s, mover); reason != "" {
			ev = append(ev, Event{Kind: Reroll, Square: s, Reason: reason})
			continue
		}
		return p.resolve(s, mover, dice, ev)
	}
	return append(ev, Event{Kind: NoPothole})
}

// rerollReason returns why the placement dice must be rolled again for s,
// or "" if s is a valid target.
func (p *Position) rerollReason(s Square, mover Color) RerollReason {
	if p.Board[s].Kind() == King {
		return ReasonKing
	}
	if p.IsPothole(s) {
		return ReasonPothole
	}
	if p.nextToMamdani(s) {
		return "" // repaired the moment it opens; nothing changes
	}
	// Would the fall leave the roller in check once the hole is gone?
	// While open, the hole blocks the same lines the piece did, so the test
	// is made without it: the roller must not inherit an unanswerable check
	// when their own pothole closes after their next move.
	gone := *p
	gone.remove(s)
	if gone.InCheck(mover) {
		return ReasonExposes
	}
	// Would the outcome checkmate the next player? A roll never wins.
	opened := gone
	opened.Potholes[mover] = s
	if opened.InCheck(opened.Turn) && len(opened.LegalMoves()) == 0 {
		return ReasonCheckmate
	}
	return ""
}

func (p *Position) nextToMamdani(s Square) bool {
	return p.Mamdani != NoSquare && adjacent(s, p.Mamdani)
}

// remove takes whatever stands on s off the board.
func (p *Position) remove(s Square) {
	if s == p.Mamdani {
		p.Mamdani = NoSquare
		return
	}
	p.Board[s] = NoPiece
}

func (p *Position) resolve(s Square, mover Color, dice Dice, ev []Event) []Event {
	if p.nextToMamdani(s) {
		return append(ev, Event{Kind: Repaired, Square: s})
	}
	switch {
	case s == p.Mamdani:
		// The Mamdani always gets a saving roll, made by the roller.
		r := dice.D8()
		saved := r%2 == 1
		ev = append(ev, Event{Kind: SavingRoll, Square: s, Piece: MamdaniPiece, Roll: r, Saved: saved, Color: mover})
		if saved {
			return ev
		}
		ev = append(ev, Event{Kind: Fell, Square: s, Piece: MamdaniPiece})
		p.Mamdani = NoSquare
	case p.Board[s] != NoPiece:
		pc := p.Board[s]
		if p.mamdaniReaches(s) {
			r := dice.D8()
			saved := r%2 == 1
			ev = append(ev, Event{Kind: SavingRoll, Square: s, Piece: pc, Roll: r, Saved: saved, Color: pc.Color()})
			if saved {
				return ev
			}
		}
		ev = append(ev, Event{Kind: Fell, Square: s, Piece: pc})
		p.Board[s] = NoPiece
		p.Halfmove = 0
		p.Castling &^= rightsLost(s)
	}
	p.Potholes[mover] = s
	return append(ev, Event{Kind: PotholeOpened, Square: s, Color: mover})
}

// mamdaniReaches reports whether the Mamdani has a clear queen line to s:
// same rank, file or diagonal, nothing in between. That is all a saving
// roll needs.
func (p *Position) mamdaniReaches(s Square) bool {
	m := p.Mamdani
	if m == NoSquare || m == s {
		return false
	}
	df, dr := s.File()-m.File(), s.Rank()-m.Rank()
	if df != 0 && dr != 0 && abs(df) != abs(dr) {
		return false
	}
	for t := m.Offset(sign(df), sign(dr)); t != s; t = t.Offset(sign(df), sign(dr)) {
		if p.Blocked(t) {
			return false
		}
	}
	return true
}
