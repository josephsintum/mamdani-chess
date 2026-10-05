package game

import (
	"fmt"
	"log/slog"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// rematch is a finished game's rematch: an offer waiting for an answer,
// a declined offer, or the new game's code once accepted. It lives in
// memory only; a restart clears it.
type rematch struct {
	offered  bool
	from     rules.Color // who offered, while offered
	declined bool        // the last offer was declined
	code     string      // the rematch's code, once accepted
}

// RematchJSON is the rematch on the wire.
type RematchJSON struct {
	Offer    string `json:"offer,omitempty"` // the color that offered, while waiting
	Declined bool   `json:"declined,omitempty"`
	Code     string `json:"code,omitempty"` // set once accepted: everyone goes there
}

func (g *Game) rematchJSON() RematchJSON {
	r := RematchJSON{Declined: g.rematch.declined, Code: g.rematch.code}
	if g.rematch.offered {
		r.Offer = colorName(g.rematch.from)
	}
	return r
}

// Rematch offers a rematch, or accepts the opponent's offer, or (with
// decline) declines it. Accepting starts a new game with colors swapped;
// its code arrives in everyone's view.
func (g *Game) Rematch(guest string, decline bool) error {
	var err error
	if perr := g.do(func() { err = g.offerRematch(guest, decline) }); perr != nil {
		return perr
	}
	return err
}

func (g *Game) offerRematch(guest string, decline bool) error {
	color, seated := g.seatOf(guest)
	switch {
	case !seated:
		return ErrNotPlayer
	case !g.g.Result.Over:
		return ErrNotOver
	case g.rematch.code != "": // already accepted
		return nil
	}
	theirs := g.rematch.offered && g.rematch.from != color
	switch {
	case decline && theirs:
		g.rematch = rematch{declined: true}
	case decline: // nothing to decline
		return nil
	case theirs: // crossing offers count as accepting
		next, err := g.hub.create(store.Game{
			White:     g.seats[rules.Black],
			Black:     g.seats[rules.White],
			RematchOf: g.code,
		})
		if err != nil {
			return fmt.Errorf("%w: create rematch: %v", ErrInternal, err)
		}
		g.rematch = rematch{code: next.Code()}
		slog.Info("rematch", "from", g.code, "to", next.Code())
	default:
		g.rematch = rematch{offered: true, from: color}
	}
	g.broadcast()
	return nil
}
