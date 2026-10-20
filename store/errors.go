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
// it, an error folds into the newest row: that row takes its message and
// time and stands for one more error. A rebuild in another STATS_TZ puts a
// folded row's errors all on its newest one's day; the totals hold.
const errorRowsPerDay = 200

// eventRowsPerDay caps the raw event rows kept a day for each kind, the
// same way.
const eventRowsPerDay = 1000

// AddBrowserError counts an error and keeps its message.
func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		countError(c, s.zone, e.At, 1)
		return insertFolded(ctx, tx, DayOf(e.At, s.zone), "errors", errorRowsPerDay,
			`INSERT INTO browser_errors (at, path, message) VALUES (?, ?, ?)`,
			`UPDATE browser_errors SET at = ?, path = ?, message = ?, n = n + 1
			 WHERE rowid = (SELECT MAX(rowid) FROM browser_errors)`,
			e.At.UnixMilli(), e.Path, e.Message)
	})
}

// insertFolded runs insert with args, or, once the day's key has counted
// limit, fold with the same args, folding the report into the newest row.
func insertFolded(ctx context.Context, tx *sql.Tx, day, key string, limit int, insert, fold string, args ...any) error {
	var today int
	err := tx.QueryRowContext(ctx, `SELECT n FROM stats_days WHERE day = ? AND key = ?`, day, key).Scan(&today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	query := insert
	if today >= limit {
		query = fold
	}
	_, err = tx.ExecContext(ctx, query, args...)
	return err
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
