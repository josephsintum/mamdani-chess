package store

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"time"
)

// A guest may change their name NameChanges times in any NameWindow,
// counted from their first change in the window. The name they're given
// when they first play doesn't count.
const (
	NameChanges = 3
	NameWindow  = 24 * time.Hour
	// nameOffers is how many names a guest is offered to choose from.
	nameOffers = 3
	// freshTries is how many names a new name draws before it settles for
	// one another guest already has.
	freshTries = 5
)

var (
	// ErrNoName is returned for a guest who hasn't played yet: there's no
	// name to change.
	ErrNoName = errors.New("you get a name when you first play")
	// ErrNoChanges is returned once a guest has used every change in the
	// current window.
	ErrNoChanges = errors.New("no name changes left today")
	// ErrNotOffered is returned when choosing a name that isn't on offer.
	ErrNotOffered = errors.New("that name isn't on offer")
)

// Allowance is how many name changes a guest has left, and when the window
// ends and they get NameChanges again (zero while none are used).
type Allowance struct {
	Left    int
	ResetAt time.Time
}

// GuestName returns the guest's name, or "" if they have none yet. It never
// creates one: looking around doesn't need a name.
func (s *Store) GuestName(ctx context.Context, id string) (string, error) {
	g, err := s.guest(ctx, id)
	return g.name, err
}

// EnsureGuest returns the guest's name, giving them a fresh one from draw
// if they have none yet. Every path that needs a name (creating a game,
// taking a seat, quick match) goes through it.
func (s *Store) EnsureGuest(ctx context.Context, id string, draw func() string) (string, error) {
	if name, err := s.GuestName(ctx, id); err != nil || name != "" {
		return name, err
	}
	name, err := s.fresh(ctx, draw, nil)
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

// Allowance returns how many name changes the guest has left at now.
func (s *Store) Allowance(ctx context.Context, id string, now time.Time) (Allowance, error) {
	g, err := s.guest(ctx, id)
	return g.allowance(now), err
}

// NameOffers returns the names the guest may choose from. They stay the
// same until one is chosen, so asking again shows nothing new; the first
// ask after a change draws fresh ones, none of them the current name. It
// returns ErrNoName for a guest without a name and ErrNoChanges, with the
// allowance, once the day's changes are used.
func (s *Store) NameOffers(ctx context.Context, id string, draw func() string, now time.Time) ([]string, Allowance, error) {
	g, err := s.guest(ctx, id)
	if err != nil {
		return nil, Allowance{}, err
	}
	left := g.allowance(now)
	switch {
	case g.name == "":
		return nil, left, ErrNoName
	case left.Left == 0:
		return nil, left, ErrNoChanges
	case len(g.offers) > 0:
		return g.offers, left, nil
	}
	avoid := map[string]bool{g.name: true}
	var offers []string
	for len(offers) < nameOffers {
		name, err := s.fresh(ctx, draw, avoid)
		if err != nil {
			return nil, left, err
		}
		avoid[name] = true
		offers = append(offers, name)
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE guests SET offers = ? WHERE id = ?`,
		strings.Join(offers, ","), id); err != nil {
		return nil, left, err
	}
	return offers, left, nil
}

// ChooseName gives the guest one of their offered names, using one of the
// day's changes, and clears the offers. It returns ErrNotOffered for any
// other name and ErrNoChanges once the day's changes are used.
func (s *Store) ChooseName(ctx context.Context, id, name string, now time.Time) (Allowance, error) {
	g, err := s.guest(ctx, id)
	if err != nil {
		return Allowance{}, err
	}
	left := g.allowance(now)
	switch {
	case left.Left == 0:
		return left, ErrNoChanges
	case !slices.Contains(g.offers, name):
		return left, ErrNotOffered
	}
	changes, since := g.changes+1, g.since
	if left.Left == NameChanges { // the first change in a new window
		changes, since = 1, now
	}
	if _, err := s.db.ExecContext(ctx,
		`UPDATE guests SET name = ?, offers = NULL, changes = ?, changes_since = ? WHERE id = ?`,
		name, changes, since.UnixMilli(), id); err != nil {
		return left, err
	}
	return Allowance{Left: NameChanges - changes, ResetAt: since.Add(NameWindow)}, nil
}

// guestRow is a guest as stored; name is "" for a guest without a row.
type guestRow struct {
	name    string
	changes int
	since   time.Time
	offers  []string
}

func (s *Store) guest(ctx context.Context, id string) (guestRow, error) {
	var g guestRow
	var since sql.NullInt64
	var offers sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT name, changes, changes_since, offers FROM guests WHERE id = ?`, id).
		Scan(&g.name, &g.changes, &since, &offers)
	if errors.Is(err, sql.ErrNoRows) {
		return guestRow{}, nil
	}
	if since.Valid {
		g.since = time.UnixMilli(since.Int64)
	}
	if offers.String != "" {
		g.offers = strings.Split(offers.String, ",")
	}
	return g, err
}

// allowance is the changes left at now: the window ends NameWindow after
// its first change, and then the count starts over.
func (g guestRow) allowance(now time.Time) Allowance {
	if g.changes == 0 || g.since.IsZero() || !now.Before(g.since.Add(NameWindow)) {
		return Allowance{Left: NameChanges}
	}
	return Allowance{Left: max(0, NameChanges-g.changes), ResetAt: g.since.Add(NameWindow)}
}

// fresh draws a name not in avoid, preferring one no guest has: it keeps
// the first unused draw, or after freshTries the last draw. Names don't
// have to be unique, so two guests named at once may still match.
func (s *Store) fresh(ctx context.Context, draw func() string, avoid map[string]bool) (string, error) {
	var name string
	for tries := 0; tries < freshTries || avoid[name]; tries++ {
		if name = draw(); avoid[name] {
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
