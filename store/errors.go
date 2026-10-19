package store

import (
	"context"
	"database/sql"
	"time"
)

// BrowserError is an uncaught error a page reported: nothing about who
// saw it. Path is a route, never a game code.
type BrowserError struct {
	At            time.Time
	Path, Message string
}

func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO browser_errors (at, path, message) VALUES (?, ?, ?)`,
		e.At.UnixMilli(), e.Path, e.Message)
	return err
}

// ErrorStats is what the Health section shows of the browser errors in a
// range: how many, and the latest message.
type ErrorStats struct {
	N      int
	Latest string
}

// BrowserErrors counts the errors with from <= at < to.
func (s *Store) BrowserErrors(ctx context.Context, from, to time.Time) (ErrorStats, error) {
	var st ErrorStats
	var latest sql.NullString
	a, b := from.UnixMilli(), to.UnixMilli()
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), (SELECT message FROM browser_errors WHERE at >= ? AND at < ? ORDER BY at DESC LIMIT 1) FROM browser_errors WHERE at >= ? AND at < ?`,
		a, b, a, b).Scan(&st.N, &latest)
	st.Latest = latest.String
	return st, err
}

// PruneStats deletes browser errors, quick-match searches and events from
// before the cutoff and returns how many rows went.
func (s *Store) PruneStats(ctx context.Context, before time.Time) (int64, error) {
	var total int64
	for _, q := range []string{
		`DELETE FROM browser_errors WHERE at < ?`,
		`DELETE FROM searches WHERE started_at < ?`,
		`DELETE FROM events WHERE at < ?`,
	} {
		res, err := s.db.ExecContext(ctx, q, before.UnixMilli())
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
