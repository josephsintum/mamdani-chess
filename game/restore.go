package game

import (
	"context"
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
		if sg.Result != nil && sg.Result.Reason == string(Unrestorable) {
			continue // already found not to replay
		}
		if err := h.restore(sg, now); err != nil {
			slog.Error("game not restored", "code", sg.Code, "err", err)
			if sg.Result == nil { // end it, so later startups don't retry it
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				r := store.Result{EndedAt: now, Reason: string(Unrestorable)}
				if err := h.store.EndGame(ctx, sg.Code, r, nil); err != nil {
					slog.Error("could not end unrestorable game", "code", sg.Code, "err", err)
				}
				cancel()
			}
			continue
		}
		n++
	}
	// An accepted rematch isn't saved on the old game, only as rematch_of on
	// the new one: point each restored old game at its rematch again.
	for _, sg := range saved {
		if sg.RematchOf == "" {
			continue
		}
		if _, ok := h.Get(sg.Code); !ok {
			continue // the rematch itself wasn't restored
		}
		if old, ok := h.Get(sg.RematchOf); ok {
			old.do(func() { old.rematch = rematch{code: sg.Code} })
		}
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
		switch rules.Reason(r.Reason) {
		case Timeout, TimeoutVsInsufficient:
			g.clock.remaining[g.g.Pos.Turn] = 0 // the side to move ran out
			g.last = nil
		case Resignation, Aborted, Expired:
			g.last = nil // no move ended it
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
	slog.Info("game restored", "code", sg.Code, "moves", len(sg.Turns), "phase", g.status())
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
