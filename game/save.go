package game

import (
	"context"
	"time"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// Store saves games as they happen. *store.Store implements it.
type Store interface {
	CreateGame(ctx context.Context, g store.Game) error
	SeatBlack(ctx context.Context, code, guest, name string) error
	AddTurn(ctx context.Context, code string, t store.Turn) error
	EndGame(ctx context.Context, code string, r store.Result, final *store.Turn) error
	// EnsureGuest returns a guest's name, giving them one from draw if
	// they have none yet.
	EnsureGuest(ctx context.Context, id string, draw func() string) (string, error)
}

// nopStore saves nothing: a hub without a store keeps games in memory only.
type nopStore struct{}

func (nopStore) CreateGame(context.Context, store.Game) error                     { return nil }
func (nopStore) SeatBlack(context.Context, string, string, string) error          { return nil }
func (nopStore) AddTurn(context.Context, string, store.Turn) error                { return nil }
func (nopStore) EndGame(context.Context, string, store.Result, *store.Turn) error { return nil }

// EnsureGuest gives no names: games in memory only are nameless.
func (nopStore) EnsureGuest(context.Context, string, func() string) (string, error) {
	return "", nil
}

// savedTurn is the latest turn as the store keeps it.
func (g *Game) savedTurn(now time.Time) store.Turn {
	n := len(g.g.Turns) - 1
	t := g.g.Turns[n]
	return store.Turn{
		Ply:     n,
		Move:    t.Move.String(),
		Dice:    t.Dice,
		WhiteMS: g.clock.remaining[rules.White].Milliseconds(),
		BlackMS: g.clock.remaining[rules.Black].Milliseconds(),
		At:      now,
	}
}

// savedResult is a result as the store keeps it.
func savedResult(now time.Time, r rules.Result) store.Result {
	return store.Result{EndedAt: now, Reason: string(r.Reason), Winner: winnerName(r)}
}

// storeTimeout bounds each store call.
const storeTimeout = 5 * time.Second

// save runs one store call. A failure retires the game: what players see
// must never get ahead of what is saved (see the loop).
func (g *Game) save(what string, f func(ctx context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	if err := f(ctx); err != nil && g.failed == nil {
		g.failed = &saveError{what: what, err: err}
	}
}

// saveError is a failed save. It matches ErrInternal, so the caller gets a
// 500 and the game is retired.
type saveError struct {
	what string
	err  error
}

func (e *saveError) Error() string { return "save " + e.what + ": " + e.err.Error() }
func (e *saveError) Unwrap() error { return ErrInternal }
