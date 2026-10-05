package game

import (
	"fmt"
	"log/slog"
	"time"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// Restore rebuilds saved games after a restart and starts them. A game
// that fails to replay is logged and skipped. It returns how many games
// it restored.
func (h *Hub) Restore(saved []store.SavedGame) int {
	now := time.Now()
	n := 0
	for _, sg := range saved {
		if err := h.restore(sg, now); err != nil {
			slog.Error("game not restored", "code", sg.Code, "err", err)
			continue
		}
		n++
	}
	return n
}

func (h *Hub) restore(sg store.SavedGame, now time.Time) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.games[sg.Code] != nil {
		return fmt.Errorf("already running")
	}
	g := newGame(h, sg.Code, [2]string{sg.White, sg.Black}, nil)
	for _, t := range sg.Turns {
		if err := g.replay(t); err != nil {
			return fmt.Errorf("ply %d (%s): %w", t.Ply, t.Move, err)
		}
	}
	if r := sg.Result; r != nil {
		g.g.Result = savedRules(r)
		if r.Reason != string(rules.Checkmate) && !g.g.Result.Draw {
			g.last = nil // resigned or out of time: no move ended it
		}
	}
	// Downtime isn't charged: the side to move's clock starts again after
	// a grace period to reconnect, and a first-move deadline starts over.
	if g.running() {
		g.startCounting(now.Add(RestoreGrace))
	} else {
		g.startCounting(now)
	}
	h.games[sg.Code] = g
	g.onExit = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.games, sg.Code)
	}
	slog.Info("game restored", "code", sg.Code, "moves", len(sg.Turns), "status", g.status())
	go g.loop()
	return nil
}

// replay plays one saved turn with its recorded dice and sets the clocks
// it ended with.
func (g *Game) replay(t store.Turn) (err error) {
	m, err := rules.ParseMove(t.Move)
	if err != nil {
		return err
	}
	if t.Ply != len(g.g.Turns) {
		return fmt.Errorf("expected ply %d", len(g.g.Turns))
	}
	script := &rules.ScriptedDice{Rolls: t.Dice}
	defer func() {
		if r := recover(); r != nil { // the recorded dice ran out
			err = fmt.Errorf("%v", r)
		}
	}()
	if err := g.apply(m, script); err != nil {
		return err
	}
	if script.Left() != 0 {
		return fmt.Errorf("%d unused dice", script.Left())
	}
	g.clock.remaining = [2]time.Duration{
		time.Duration(t.WhiteMS) * time.Millisecond,
		time.Duration(t.BlackMS) * time.Millisecond,
	}
	return nil
}

// savedRules turns a saved result back into a rules.Result.
func savedRules(r *store.Result) rules.Result {
	res := rules.Result{Over: true, Reason: rules.Reason(r.Reason)}
	switch r.Winner {
	case "white":
		res.Winner = rules.White
	case "black":
		res.Winner = rules.Black
	default:
		res.Draw = !noWinner(res.Reason)
	}
	return res
}
