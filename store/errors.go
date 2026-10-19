package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// BrowserError is an uncaught error a page reported: nothing about who
// saw it. Path is a route, never a game code.
type BrowserError struct {
	At            time.Time
	Path, Message string
}

// errorRowsPerDay caps the raw error rows kept a day, so a flood of
// reports (there are no per-IP limits yet) can't fill the database. Past
// it, errors are still counted.
const errorRowsPerDay = 200

// AddBrowserError counts an error and keeps its message, up to
// errorRowsPerDay a day.
func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	day := dayOf(e.At, s.zone)
	var today int
	err = tx.QueryRowContext(ctx, `SELECT n FROM stats_days WHERE day = ? AND key = 'errors'`, day).Scan(&today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if today < errorRowsPerDay {
		if _, err := tx.ExecContext(ctx, `INSERT INTO browser_errors (at, path, message) VALUES (?, ?, ?)`,
			e.At.UnixMilli(), e.Path, e.Message); err != nil {
			return err
		}
	}
	c := counts{}
	countError(c, s.zone, e.At)
	if err := addCounts(ctx, tx, c); err != nil {
		return err
	}
	return tx.Commit()
}

// LatestError is the newest stored error message with from <= at < to,
// or "" for none.
func (s *Store) LatestError(ctx context.Context, from, to time.Time) (string, error) {
	var msg string
	err := s.db.QueryRowContext(ctx, `SELECT message FROM browser_errors WHERE at >= ? AND at < ? ORDER BY at DESC LIMIT 1`,
		from.UnixMilli(), to.UnixMilli()).Scan(&msg)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return msg, err
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
