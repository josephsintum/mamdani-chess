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
	st          store.GameStats // all but MateByRoll, which stats works out
	rolled      bool            // the latest turn rolled the pothole die
	mamdaniFell bool
}

// add counts one turn's events; pos is the position after them. A
// pothole_closed before the turn's roll is a hole that ran its rounds, one
// after it is the cap closing the oldest.
func (r *road) add(ev []rules.Event, pos *rules.Position) {
	r.st.Moves++
	rolled := false
	for _, e := range ev {
		switch e.Kind {
		case rules.RolledPothole:
			rolled = true
		case rules.PotholeOpened:
			r.st.Opened++
		case rules.PotholeReset:
			r.st.Reset++
		case rules.PotholeClosed:
			if rolled {
				r.st.ClosedCap++
			} else {
				r.st.ClosedRounds++
			}
		case rules.Repaired:
			r.st.Repaired++
		case rules.Fell:
			r.st.Fell++
			switch {
			case e.Piece == rules.MamdaniPiece: // nobody's loss
				r.mamdaniFell = true
			case e.Piece.Color() == rules.White:
				r.st.WhiteLost++
			default:
				r.st.BlackLost++
			}
		case rules.SavingRoll:
			r.st.SavingRolls++
			if e.Saved {
				r.st.Saved++
			}
		}
	}
	r.st.OpenHist[min(openHoles(pos), len(r.st.OpenHist)-1)]++
	r.rolled = rolled
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

// stats is the total as the store keeps it, for a game that ended for
// reason: a checkmate whose final turn rolled the dice is a mate by a roll.
func (r *road) stats(reason rules.Reason) store.GameStats {
	st := r.st
	st.MateByRoll = reason == rules.Checkmate && r.rolled
	return st
}

// view is the part of the stats a game's view shows.
func (r *road) view() StatsJSON {
	return StatsJSON{
		SavingRolls: r.st.SavingRolls,
		Saved:       r.st.Saved,
		Repaired:    r.st.Repaired,
		MamdaniFell: r.mamdaniFell,
	}
}

// StatsOf replays a saved game's turns from the start with their recorded
// dice and returns its stats: the backfill for games that ended before
// stats were kept.
func StatsOf(turns []store.Turn, result store.Result) (store.GameStats, error) {
	g := rules.NewGame()
	var r road
	for _, t := range turns {
		m, err := rules.ParseMove(t.Move)
		if err != nil {
			return store.GameStats{}, fmt.Errorf("ply %d: %w", t.Ply, err)
		}
		ev, err := g.PlayRecorded(m, t.Dice)
		if err != nil {
			return store.GameStats{}, fmt.Errorf("ply %d (%s): %w", t.Ply, t.Move, err)
		}
		r.add(ev, &g.Pos)
	}
	return r.stats(rules.Reason(result.Reason)), nil
}
