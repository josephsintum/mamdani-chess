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
// time and stands for one more error.
const errorRowsPerDay = 200

// eventRowsPerDay caps the raw event rows kept a day for each kind, the
// same way.
const eventRowsPerDay = 1000

// AddBrowserError counts an error and keeps its message.
func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
	c := counts{}
	countError(c, s.zone, e.At, 1)
	return s.insertFolded(ctx, c, DayOf(e.At, s.zone), "errors", errorRowsPerDay,
		`INSERT INTO browser_errors (at, path, message) VALUES (?, ?, ?)`,
		`UPDATE browser_errors SET at = ?, path = ?, message = ?, n = n + 1
		 WHERE rowid = (SELECT rowid FROM browser_errors ORDER BY rowid DESC LIMIT 1)`,
		e.At.UnixMilli(), e.Path, e.Message)
}

// insertFolded adds a row with args and its counts, in one transaction.
// Once the day's key has counted limit, fold runs instead with the same
// args, folding the report into the newest row.
func (s *Store) insertFolded(ctx context.Context, c counts, day, key string, limit int, insert, fold string, args ...any) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var today int
	err = tx.QueryRowContext(ctx, `SELECT n FROM stats_days WHERE day = ? AND key = ?`, day, key).Scan(&today)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	query := insert
	if today >= limit {
		query = fold
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
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
