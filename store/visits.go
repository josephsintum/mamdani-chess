package store

import (
	"context"
	"time"
)

// Visit is one page view. Visitor is the first 12 hex characters of the
// guest ID, Path a route (never a game code), Referrer a host, or
// "ref:<tag>" for a ?ref= link. Country, City, Device, OS and Browser are
// "" when unknown.
type Visit struct {
	At                                                          time.Time
	Visitor, Path, Referrer, Country, City, Device, OS, Browser string
}

// BrowserError is an uncaught error a page reported.
type BrowserError struct {
	At                              time.Time
	Visitor, Path, Message, Browser string
}

func (s *Store) AddVisit(ctx context.Context, v Visit) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO visits (at, visitor, path, referrer, country, city, device, os, browser) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.At.UnixMilli(), v.Visitor, v.Path, v.Referrer, v.Country, v.City, v.Device, v.OS, v.Browser)
	return err
}

func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO browser_errors (at, visitor, path, message, browser) VALUES (?, ?, ?, ?, ?)`,
		e.At.UnixMilli(), e.Visitor, e.Path, e.Message, e.Browser)
	return err
}

// Visitors returns the distinct visitor keys seen with from <= at < to,
// sorted.
func (s *Store) Visitors(ctx context.Context, from, to time.Time) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT visitor FROM visits WHERE at >= ? AND at < ? ORDER BY visitor`, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// PruneVisits deletes visits and browser errors from before the cutoff and
// returns how many rows went.
func (s *Store) PruneVisits(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for _, table := range []string{"visits", "browser_errors"} {
		res, err := s.db.ExecContext(ctx, `DELETE FROM `+table+` WHERE at < ?`, before.UnixMilli())
		if err != nil {
			return total, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
