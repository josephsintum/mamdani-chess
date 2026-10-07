package game

import (
	"time"

	"mamdani-chess/rules"
)

// The one time control: 10+5. Clocks start after each side's first move.
const (
	InitialTime = 10 * time.Minute
	Increment   = 5 * time.Second
	// ResolveDelay is the shortest pause after a move while the dice play
	// out: the next player's clock (or first-move deadline) starts after it.
	// A longer roll gets a longer pause (see pauseFor).
	ResolveDelay = 2 * time.Second
	// MoveTime is how long the browser shows the move before the dice roll.
	// It and each step's time (playTime) come from the browser's table,
	// web/src/lib/dice-timing.json; TestDiceTimingMatchesTheBrowser keeps
	// them the same.
	MoveTime = 550 * time.Millisecond
	// PauseMargin covers the state's trip to the browser on top of the
	// animation itself.
	PauseMargin = 500 * time.Millisecond
	// FirstMoveTime is how long each side has for its first move before the
	// game is aborted.
	FirstMoveTime = 60 * time.Second
	// RestoreGrace is how long a restored game waits before the clock runs
	// again, so both players can reconnect after a restart.
	RestoreGrace = 10 * time.Second
)

// pauseFor is how long the next clock waits after a turn with events ev:
// as long as the browser takes to play it (the move, then every step from
// the pothole roll on, each for its own time), plus PauseMargin, and never
// less than ResolveDelay. So a long roll (a re-roll, a saving roll, a fall)
// costs the next player no clock time.
func pauseFor(ev []rules.Event) time.Duration {
	for i, e := range ev {
		if e.Kind == rules.RolledPothole {
			total := MoveTime + PauseMargin
			for _, step := range ev[i:] {
				total += playTime(step)
			}
			return max(ResolveDelay, total)
		}
	}
	return ResolveDelay
}

// playTime is how long the browser plays one dice step: the throw, the
// file and rank dice with the scan, a fall, a hole cracking open.
func playTime(e rules.Event) time.Duration {
	ms := 550
	switch e.Kind {
	case rules.RolledPothole:
		ms = 600 // even: the pothole roll, then the file and rank dice follow
		if e.Roll%2 == 1 {
			ms = 700 // odd: the throw, and the pill saying nothing happens
		}
	case rules.Target:
		ms = 1250 // the file and rank dice, out of sync, with the scan
	case rules.Reroll:
		ms = 500 // the target blinks twice
	case rules.SavingRoll:
		ms = 750
	case rules.Fell:
		ms = 600
	case rules.PotholeClosed:
		ms = 400
	}
	return time.Duration(ms) * time.Millisecond
}

// Result reasons the game package adds to the rules package's.
const (
	Timeout rules.Reason = "timeout"
	// TimeoutVsInsufficient is a draw: the flag fell, but the other side
	// could never have mated.
	TimeoutVsInsufficient rules.Reason = "timeout_vs_insufficient"
	// Aborted means a side missed its first move. Nobody wins.
	Aborted rules.Reason = "aborted"
	// Expired means nobody ever took Black's seat. Nobody wins.
	Expired rules.Reason = "expired"
	// Retired means the game was saved under rules since replaced, and
	// the store ended it rather than replay it. Nobody wins. Retired games
	// are never loaded, so no view ever shows this.
	Retired rules.Reason = "retired"
)

// noWinner reports whether a finished game with reason r has no winner
// and isn't a draw either.
func noWinner(r rules.Reason) bool { return r == Aborted || r == Expired || r == Retired }

// winnerName is "white" or "black", or "" for a draw, a game nobody won,
// or a game still on.
func winnerName(r rules.Result) string {
	if !r.Over || r.Draw || noWinner(r.Reason) {
		return ""
	}
	return colorName(r.Winner)
}

// clock is a game's time: what each side has left, and when the side to
// move's time (or first-move deadline) starts counting.
type clock struct {
	remaining [2]time.Duration
	// since is when the side to move started counting; it can be in the
	// future during the dice pause. Zero while waiting for Black.
	since time.Time
	// deadline is when the side to move runs out (of clock or of time for a
	// first move). Zero when nothing is counting: waiting, or game over.
	deadline time.Time
}

func newClock() clock {
	return clock{remaining: [2]time.Duration{InitialTime, InitialTime}}
}

// running reports whether a side's clock is counting down: both sides have
// made their first move and the game is on.
func (g *Game) running() bool {
	return g.status() == Playing && len(g.g.Turns) >= 2
}

// startCounting starts the side to move's time from since, and sets the
// deadline that goes with it.
func (g *Game) startCounting(since time.Time) {
	g.clock.since = since
	switch {
	case g.status() != Playing:
		g.clock.deadline = time.Time{}
	case len(g.g.Turns) < 2:
		g.clock.deadline = since.Add(FirstMoveTime)
	default:
		g.clock.deadline = since.Add(g.clock.remaining[g.g.Pos.Turn])
	}
}

// charge takes the time c used on this turn from c's clock. Time before
// since (the dice pause) is free.
func (g *Game) charge(c rules.Color, now time.Time) {
	if used := now.Sub(g.clock.since); used > 0 {
		g.clock.remaining[c] -= used
	}
}

// expired reports whether the side to move's deadline has passed at now.
func (g *Game) expired(now time.Time) bool {
	return !g.clock.deadline.IsZero() && !now.Before(g.clock.deadline)
}

// flag ends a game whose deadline has passed: aborted before both first
// moves, otherwise a loss on time, or a draw if the other side can't mate.
func (g *Game) flag(now time.Time) {
	side := g.g.Pos.Turn
	r := rules.Result{Over: true, Winner: side.Other(), Reason: Timeout}
	switch {
	case len(g.g.Turns) < 2:
		r = rules.Result{Over: true, Reason: Aborted}
	case g.g.Pos.CannotMate(side.Other()):
		r = rules.Result{Over: true, Draw: true, Reason: TimeoutVsInsufficient}
	}
	if r.Reason != Aborted {
		g.clock.remaining[side] = 0
	}
	g.end(now, r, nil)
}

// ClockJSON is the clocks on the wire. The browser counts down locally:
// the running side has its remaining time minus (its now − since), where
// its now is shifted by the difference between Now and its own clock.
type ClockJSON struct {
	WhiteMS int64  `json:"whiteMs"`
	BlackMS int64  `json:"blackMs"`
	Running string `json:"running,omitempty" ts:"Color"` // "white", "black" or "" when stopped
	// Since is when the running clock starts or started, in unix ms. It can
	// be after Now during the dice pause.
	Since int64 `json:"since,omitempty"`
	// Now is the server's time when the view was built, in unix ms.
	Now int64 `json:"now"`
	// FirstMoveDeadline is when the side to move must make its first move
	// by, in unix ms; 0 outside the first moves.
	FirstMoveDeadline int64 `json:"firstMoveDeadline,omitempty"`
}

func (g *Game) clockJSON(now time.Time) ClockJSON {
	c := ClockJSON{
		WhiteMS: g.clock.remaining[rules.White].Milliseconds(),
		BlackMS: g.clock.remaining[rules.Black].Milliseconds(),
		Now:     now.UnixMilli(),
	}
	switch {
	case g.running():
		c.Running, c.Since = colorName(g.g.Pos.Turn), g.clock.since.UnixMilli()
	case g.status() == Playing:
		c.FirstMoveDeadline = g.clock.deadline.UnixMilli()
	}
	return c
}
