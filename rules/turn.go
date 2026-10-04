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

// Apply is completed in Task 3; for now it only plays the move.
func Apply(p Position, m Move, dice Dice) (Position, []Event, error) {
	if !slices.Contains(p.LegalMoves(), m) {
		return p, nil, ErrIllegalMove
	}
	ev := p.play(m, nil)
	ev = append(ev, Event{Kind: RolledPothole, Roll: dice.D8()})
	return p, ev, nil
}
