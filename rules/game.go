package rules

import (
	"errors"
	"fmt"
)

// Reason says why a game ended.
type Reason string

const (
	Checkmate            Reason = "checkmate"
	Stalemate            Reason = "stalemate"
	FiftyMoves           Reason = "fifty_moves"
	Repetition           Reason = "repetition"
	InsufficientMaterial Reason = "insufficient_material"
)

// Result is a game's outcome. The zero value means the game is still on.
type Result struct {
	Over   bool
	Draw   bool
	Winner Color // set when Over && !Draw
	Reason Reason
}

// Turn is one recorded turn: the move and every d8 rolled, in order.
// Replaying turns through Replay rebuilds the game exactly.
type Turn struct {
	Move Move
	Dice []int
}

// Game is a position plus the history needed for repetition and replay.
type Game struct {
	Pos    Position
	Turns  []Turn
	Result Result
	seen   map[Key]int
}

var ErrGameOver = errors.New("game is over")

// NewGame starts from StartPosition.
func NewGame() *Game { return NewGameFrom(StartPosition()) }

// NewGameFrom starts from any position (tests, replays).
func NewGameFrom(p Position) *Game {
	g := &Game{Pos: p, seen: map[Key]int{p.Key(): 1}}
	g.Result = g.status()
	return g
}

// Play applies one turn and updates the result.
func (g *Game) Play(m Move, dice Dice) ([]Event, error) {
	if g.Result.Over {
		return nil, ErrGameOver
	}
	rec := &recordingDice{dice: dice}
	next, ev, err := Apply(g.Pos, m, rec)
	if err != nil {
		return nil, err
	}
	g.Pos = next
	g.Turns = append(g.Turns, Turn{Move: m, Dice: rec.rolls})
	g.seen[next.Key()]++
	g.Result = g.status()
	return ev, nil
}

func (g *Game) status() Result {
	p := &g.Pos
	if !p.hasLegalMove() {
		if p.threatened() {
			return Result{Over: true, Winner: p.Turn.Other(), Reason: Checkmate}
		}
		return Result{Over: true, Draw: true, Reason: Stalemate}
	}
	if p.Halfmove >= 100 {
		return Result{Over: true, Draw: true, Reason: FiftyMoves}
	}
	if g.seen[p.Key()] >= 3 {
		return Result{Over: true, Draw: true, Reason: Repetition}
	}
	if p.CannotMate(White) && p.CannotMate(Black) {
		return Result{Over: true, Draw: true, Reason: InsufficientMaterial}
	}
	return Result{}
}

// CannotMate reports whether c can never deliver checkmate: a bare king,
// or king and one knight or bishop once the Mamdani has fallen. (While the
// Mamdani is on the board it can block escape squares, so a minor piece
// might mate.) The game server also uses this for timeouts.
func (p *Position) CannotMate(c Color) bool {
	if p.byColor[c]&(p.byKind[Pawn]|p.byKind[Rook]|p.byKind[Queen]) != 0 {
		return false
	}
	minors := (p.byColor[c] & (p.byKind[Knight] | p.byKind[Bishop])).Count()
	return minors == 0 || (minors == 1 && p.Mamdani == NoSquare)
}

// Replay rebuilds a game from start by playing turns with their recorded
// dice. It fails if a move is illegal or the dice don't match exactly.
func Replay(start Position, turns []Turn) (*Game, error) {
	g := NewGameFrom(start)
	for i, t := range turns {
		if err := g.replayTurn(t); err != nil {
			return nil, fmt.Errorf("turn %d (%s): %w", i+1, t.Move, err)
		}
	}
	return g, nil
}

func (g *Game) replayTurn(t Turn) (err error) {
	script := &ScriptedDice{Rolls: t.Dice}
	defer func() {
		if r := recover(); r != nil { // the script ran out
			err = fmt.Errorf("%v", r)
		}
	}()
	if _, err := g.Play(t.Move, script); err != nil {
		return err
	}
	if script.Left() != 0 {
		return fmt.Errorf("%d unused dice", script.Left())
	}
	return nil
}

// ScriptedDice returns preset rolls in order. It panics when it runs out:
// in tests that means the script is too short.
type ScriptedDice struct {
	Rolls []int
	next  int
}

func (s *ScriptedDice) D8() int {
	if s.next >= len(s.Rolls) {
		panic("scripted dice ran out")
	}
	r := s.Rolls[s.next]
	s.next++
	return r
}

// Left returns how many rolls have not been used.
func (s *ScriptedDice) Left() int { return len(s.Rolls) - s.next }

type recordingDice struct {
	dice  Dice
	rolls []int
}

func (r *recordingDice) D8() int {
	v := r.dice.D8()
	r.rolls = append(r.rolls, v)
	return v
}
