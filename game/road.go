package game

import (
	"fmt"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// road tallies what the potholes did over a game. The game adds every
// turn's events as they are played (restore replays them, so a restart
// rebuilds it), and hands the total to the store with the result.
type road struct {
	moves, opened, reset, closedRounds, closedCap, repaired int
	fell, savingRolls, saved                                int
	openHist                                                [6]int // turns that ended with k holes open
}

// add counts one turn's events; pos is the position after them. A
// pothole_closed before the turn's roll is a hole that ran its rounds, one
// after it is the cap closing the oldest.
func (r *road) add(ev []rules.Event, pos *rules.Position) {
	r.moves++
	rolled := false
	for _, e := range ev {
		switch e.Kind {
		case rules.RolledPothole:
			rolled = true
		case rules.PotholeOpened:
			r.opened++
		case rules.PotholeReset:
			r.reset++
		case rules.PotholeClosed:
			if rolled {
				r.closedCap++
			} else {
				r.closedRounds++
			}
		case rules.Repaired:
			r.repaired++
		case rules.Fell:
			r.fell++
		case rules.SavingRoll:
			r.savingRolls++
			if e.Saved {
				r.saved++
			}
		}
	}
	r.openHist[min(openHoles(pos), len(r.openHist)-1)]++
}

// openHoles counts the position's open potholes.
func openHoles(pos *rules.Position) int {
	n := 0
	for _, h := range pos.Potholes {
		if h.Sq != rules.NoSquare {
			n++
		}
	}
	return n
}

// stats is the total as the store keeps it. last is the final turn's
// events (a checkmate whose turn rolled the dice is a mate by a roll) and
// lost the pieces each side lost to potholes.
func (r *road) stats(result rules.Result, last []rules.Event, lost [2][]string) store.GameStats {
	st := store.GameStats{
		Moves: r.moves, Opened: r.opened, Reset: r.reset, ClosedRounds: r.closedRounds, ClosedCap: r.closedCap, Repaired: r.repaired,
		Fell: r.fell, SavingRolls: r.savingRolls, Saved: r.saved,
		WhiteLost: len(lost[rules.White]), BlackLost: len(lost[rules.Black]),
		OpenHist: r.openHist,
	}
	if result.Reason == rules.Checkmate {
		for _, e := range last {
			if e.Kind == rules.RolledPothole {
				st.MateByRoll = true
			}
		}
	}
	return st
}

// StatsOf replays a saved game's turns from the start with their recorded
// dice and returns its stats: the backfill for games that ended before
// stats were kept.
func StatsOf(turns []store.Turn, result store.Result) (store.GameStats, error) {
	g := rules.NewGame()
	var r road
	var lost [2][]string
	var last []rules.Event
	for _, t := range turns {
		m, err := rules.ParseMove(t.Move)
		if err != nil {
			return store.GameStats{}, fmt.Errorf("ply %d: %w", t.Ply, err)
		}
		ev, err := playScripted(g, m, t.Dice)
		if err != nil {
			return store.GameStats{}, fmt.Errorf("ply %d (%s): %w", t.Ply, t.Move, err)
		}
		for _, e := range ev {
			if e.Kind == rules.Fell && e.Piece != rules.MamdaniPiece {
				c := e.Piece.Color()
				lost[c] = append(lost[c], pieceCode(e.Piece))
			}
		}
		r.add(ev, &g.Pos)
		last = ev
	}
	res := savedRules(&result)
	if rules.Reason(result.Reason) != rules.Checkmate {
		last = nil
	}
	return r.stats(res, last, lost), nil
}

// playScripted plays m with the recorded dice, turning the dice running
// out (a panic in ScriptedDice) into an error.
func playScripted(g *rules.Game, m rules.Move, dice []int) (ev []rules.Event, err error) {
	script := &rules.ScriptedDice{Rolls: dice}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	ev, err = g.Play(m, script)
	if err == nil && script.Left() != 0 {
		err = fmt.Errorf("%d unused dice", script.Left())
	}
	return ev, err
}
