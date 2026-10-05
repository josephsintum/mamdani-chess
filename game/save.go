package game

import (
	"context"
	"time"

	"mamdani-chess/store"
)

// Store saves games as they happen. *store.Store implements it.
type Store interface {
	CreateGame(ctx context.Context, g store.Game) error
	SeatBlack(ctx context.Context, code, guest string) error
	AddTurn(ctx context.Context, code string, t store.Turn) error
	EndGame(ctx context.Context, code string, r store.Result, final *store.Turn) error
}

// nopStore saves nothing: a hub without a store keeps games in memory only.
type nopStore struct{}

func (nopStore) CreateGame(context.Context, store.Game) error                     { return nil }
func (nopStore) SeatBlack(context.Context, string, string) error                  { return nil }
func (nopStore) AddTurn(context.Context, string, store.Turn) error                { return nil }
func (nopStore) EndGame(context.Context, string, store.Result, *store.Turn) error { return nil }

// save runs one store call. A failure retires the game: what players see
// must never get ahead of what is saved (see the loop).
func (g *Game) save(what string, f func(ctx context.Context) error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
