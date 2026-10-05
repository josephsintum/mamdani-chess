package game

import (
	"time"

	"mamdani-chess/rules"
)

// The one time control: 10+5. Clocks start after each side's first move.
const (
	InitialTime = 10 * time.Minute
	Increment   = 5 * time.Second
	// ResolveDelay is the pause after every move while the dice play out:
	// the next player's clock (or first-move deadline) starts after it.
	ResolveDelay = 2 * time.Second
	// FirstMoveTime is how long each side has for its first move before the
	// game is aborted.
	FirstMoveTime = 60 * time.Second
	// RestoreGrace is how long a restored game waits before the clock runs
	// again, so both players can reconnect after a restart.
	RestoreGrace = 10 * time.Second
)

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
)

// noWinner reports whether a finished game with reason r has no winner
// and isn't a draw either.
func noWinner(r rules.Reason) bool { return r == Aborted || r == Expired }

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
	g.last = nil
	g.end(now, r)
}

// ClockJSON is the clocks on the wire. The browser counts down locally:
// the running side has its remaining time minus (its now − since), where
// its now is shifted by the difference between Now and its own clock.
type ClockJSON struct {
	WhiteMS int64  `json:"whiteMs"`
	BlackMS int64  `json:"blackMs"`
	Running string `json:"running,omitempty"` // "white", "black" or "" when stopped
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
