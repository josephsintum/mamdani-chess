package game

import (
	"log/slog"
	"time"

	"mamdani-chess/store"
)

// PracticeIdle is how long a practice game lasts with nobody watching it.
const PracticeIdle = 10 * time.Minute

// CreatePractice starts a practice game: guest plays both sides, with the
// rules engine and dice of a real game but no clock. It lives in memory
// only (a restart ends it), is never listed, and ends the guest's previous
// practice game, so each guest has at most one.
func (h *Hub) CreatePractice(guest string) *Game {
	sg := store.Game{Code: h.reserve(), White: guest, Black: guest, CreatedAt: time.Now()}
	g := newGame(h, sg, h.forget(sg.Code))
	g.practice, g.store, g.idle = true, nopStore{}, PracticeIdle
	g.publish()
	// Finding the old ones and adding this one under one lock means two
	// creates at once (a double click) still leave one.
	h.mu.Lock()
	var old []*Game
	for _, o := range h.games {
		if l := o.live.Load(); l != nil && l.practice && l.seats[0] == guest {
			old = append(old, o)
		}
	}
	delete(h.reserved, sg.Code)
	h.games[sg.Code] = g
	h.mu.Unlock()
	for _, o := range old {
		_ = o.do(func() { o.quit = true }) // ErrGone: it stopped by itself
	}
	slog.Info("practice game created", "code", sg.Code, "guest", guestTag(guest))
	go g.loop()
	return g
}
