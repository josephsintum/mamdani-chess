package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// freshTries is how many names a new or re-rolled name draws before it
// settles for one another guest already has.
const freshTries = 5

// GuestName returns the guest's name, or "" if they have none yet. It never
// creates one: looking around doesn't need a name.
func (s *Store) GuestName(ctx context.Context, id string) (string, error) {
	var name string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM guests WHERE id = ?`, id).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return name, err
}

// EnsureGuest returns the guest's name, giving them a fresh one from draw
// if they have none yet. Every path that needs a name (creating a game,
// taking a seat, quick match) goes through it.
func (s *Store) EnsureGuest(ctx context.Context, id string, draw func() string) (string, error) {
	if name, err := s.GuestName(ctx, id); err != nil || name != "" {
		return name, err
	}
	name, err := s.fresh(ctx, draw, "")
	if err != nil {
		return "", err
	}
	// Another request for the same guest may have got there first: keep
	// whichever name was saved.
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO guests (id, name, created_at) VALUES (?, ?, ?) ON CONFLICT (id) DO NOTHING`,
		id, name, time.Now().UnixMilli()); err != nil {
		return "", err
	}
	return s.GuestName(ctx, id)
}

// RerollGuest gives the guest a fresh name, different from their current
// one, and returns it. A guest without a name gets their first.
func (s *Store) RerollGuest(ctx context.Context, id string, draw func() string) (string, error) {
	old, err := s.GuestName(ctx, id)
	if err != nil {
		return "", err
	}
	name, err := s.fresh(ctx, draw, old)
	if err != nil {
		return "", err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO guests (id, name, created_at) VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET name = excluded.name`,
		id, name, time.Now().UnixMilli())
	return name, err
}

// fresh draws a name other than not, preferring one no guest has: it
// keeps the first unused draw, or after freshTries the last draw. Names
// don't have to be unique, so two guests named at once may still match.
func (s *Store) fresh(ctx context.Context, draw func() string, not string) (string, error) {
	var name string
	for tries := 0; tries < freshTries || name == not; tries++ {
		if name = draw(); name == not {
			continue
		}
		var taken bool
		if err := s.db.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM guests WHERE name = ?)`, name).Scan(&taken); err != nil {
			return "", err
		}
		if !taken {
			return name, nil
		}
	}
	return name, nil
}
