package rules

import (
	"errors"
	"fmt"
)

// Dice rolls eight-sided dice. D8 returns 1..8.
type Dice interface {
	D8() int
}

// ErrIllegalMove is returned by Apply for a move not in LegalMoves.
var ErrIllegalMove = errors.New("illegal move")

// ErrBadDie is returned by Apply when Dice produces a roll outside 1..8.
var ErrBadDie = errors.New("die roll outside 1..8")

// maxRerolls caps placement re-rolls; past it no pothole opens this turn.
const maxRerolls = 64

// Apply plays one full turn: move, countdown, repair, pothole roll,
// placement and resolution (or the reset of a hole already there). It returns the new position and what happened,
// in order. A move that checkmates ends the game at once: no pothole roll
// follows, so the dice can't undo a mate made on the board. The roll itself
// may leave the next player mated; the game sees that like any other mate.
// p is not modified.
func Apply(p Position, m Move, dice Dice) (Position, []Event, error) {
	if !p.isLegal(m) {
		return p, nil, ErrIllegalMove
	}
	mover := p.Turn
	next := p
	var ev []Event
	next.play(m, &ev)
	if next.Mated() {
		return next, ev, nil
	}
	checked := &checkedDice{dice: dice}
	next.rollPothole(mover, checked, &ev)
	if checked.err != nil {
		return p, nil, checked.err
	}
	return next, ev, nil
}

// checkedDice rejects rolls outside 1..8. After the first bad roll it keeps
// the error and returns 1 (odd), which ends the turn quickly.
type checkedDice struct {
	dice Dice
	err  error
}

func (c *checkedDice) D8() int {
	if c.err != nil {
		return 1
	}
	r := c.dice.D8()
	if r < 1 || r > 8 {
		c.err = fmt.Errorf("%w: got %d", ErrBadDie, r)
		return 1
	}
	return r
}

// Mated reports whether the side to move is checkmated: no legal move, and
// its king attacked once its own holes on their last round are counted as
// closed. Every move closes those, so a check they are only holding off
// can't be escaped either. A hole with rounds to spare outlasts the next
// move, so a check it holds off leaves a stalemate, not a mate.
func (p *Position) Mated() bool {
	return p.threatened() && !p.hasLegalMove()
}

// threatened reports whether the side to move's king is attacked, ignoring
// its own holes that close on its next move.
func (p *Position) threatened() bool {
	q := *p
	for i, h := range q.Potholes {
		if h.Sq != NoSquare && h.By == q.Turn && h.Left == 1 {
			q.Potholes[i].Sq = NoSquare
		}
	}
	return q.InCheck(q.Turn)
}

func (p *Position) rollPothole(mover Color, dice Dice, ev *[]Event) {
	r := dice.D8()
	emit(ev, Event{Kind: RolledPothole, Roll: r, Color: mover})
	if r%2 == 1 {
		return
	}
	for range maxRerolls + 1 {
		file, rank := dice.D8(), dice.D8()
		s := Square((rank-1)*8 + file - 1)
		emit(ev, Event{Kind: Target, Square: s})
		if p.IsPothole(s) {
			p.resetHole(s, mover, ev)
			return
		}
		if reason := p.rerollReason(s, mover); reason != "" {
			emit(ev, Event{Kind: Reroll, Square: s, Reason: reason})
			continue
		}
		p.resolve(s, mover, dice, ev)
		return
	}
	emit(ev, Event{Kind: NoPothole})
}

// rerollReason returns why the placement dice must be rolled again for s,
// or "" if s is a valid target.
func (p *Position) rerollReason(s Square, mover Color) RerollReason {
	if p.Board[s].Kind() == King {
		return ReasonKing
	}
	if p.nextToMamdani(s) {
		return "" // repaired the moment it opens; nothing changes
	}
	// Would the fall leave the roller in check once the hole is gone?
	// While open, the hole blocks the same lines the piece did, so the test
	// is made without it: the roller must not inherit an unanswerable check
	// when their own pothole closes. With HoleCap open, the oldest hole
	// closes as this one opens, and it may be the one shielding the
	// roller's king: then the roller would be in check on the other
	// player's turn, so that is tested too.
	gone := *p
	gone.remove(s)
	if i := gone.capVictim(); i >= 0 {
		gone.Potholes[i].Sq = NoSquare
	}
	if gone.InCheck(mover) {
		return ReasonExposes
	}
	return ""
}

// nextToMamdani reports whether s touches the Mamdani, diagonals included.
func (p *Position) nextToMamdani(s Square) bool {
	return p.Mamdani != NoSquare && kingAttacks[p.Mamdani].Has(s)
}

// remove takes whatever stands on s off the board.
func (p *Position) remove(s Square) {
	if s == p.Mamdani {
		p.Mamdani = NoSquare
		return
	}
	p.take(s)
}

func (p *Position) resolve(s Square, mover Color, dice Dice, ev *[]Event) {
	if p.nextToMamdani(s) {
		emit(ev, Event{Kind: Repaired, Square: s})
		return
	}
	switch {
	case s == p.Mamdani:
		// The Mamdani always gets a saving roll, made by the roller.
		if savingRoll(s, MamdaniPiece, mover, dice, ev) {
			return
		}
		emit(ev, Event{Kind: Fell, Square: s, Piece: MamdaniPiece})
		p.Mamdani = NoSquare
	case p.Board[s] != NoPiece:
		pc := p.Board[s]
		if p.mamdaniReaches(s) && savingRoll(s, pc, pc.Color(), dice, ev) {
			return
		}
		emit(ev, Event{Kind: Fell, Square: s, Piece: pc})
		p.take(s)
		p.Halfmove = 0
		p.Castling &^= rightsLost(s)
	}
	p.openHole(s, mover, ev)
}

// savingRoll rolls a d8 for pc on s, made by c, and reports whether it
// saves the piece: an odd roll does.
func savingRoll(s Square, pc Piece, c Color, dice Dice, ev *[]Event) bool {
	r := dice.D8()
	saved := r%2 == 1
	emit(ev, Event{Kind: SavingRoll, Square: s, Piece: pc, Roll: r, Saved: saved, Color: c})
	return saved
}

// mamdaniReaches reports whether the Mamdani has a clear queen line to s:
// same rank, file or diagonal, nothing in between. That is all a saving
// roll needs. A queen's attacks run up to and including the first blocked
// square on each line, so s is among them exactly when the line is clear.
func (p *Position) mamdaniReaches(s Square) bool {
	return p.Mamdani != NoSquare && queenAttacks(p.Mamdani, p.blocked()).Has(s)
}
