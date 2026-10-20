// Package store persists games, guests and the stats in SQLite.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
	// zone cuts the stats counters' days (STATS_TZ).
	zone *time.Location
	// stale is set when the counters are from another version or zone and
	// rebuilding them failed.
	stale atomic.Bool
}

// Open opens (creating if needed) the database at path and applies
// migrations. Its stats counters cut days in UTC.
func Open(path string) (*Store, error) { return OpenIn(path, time.UTC) }

// OpenIn is Open with the stats counters' days cut in zone (STATS_TZ).
func OpenIn(path string, zone *time.Location) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer; a single connection avoids SQLITE_BUSY.
	// Every statement shares it, so rows must be closed before the next
	// statement (eachRow does).
	db.SetMaxOpenConns(1)
	s := &Store{db: db, zone: zone}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// inTx runs fn in one transaction, committed when fn returns nil.
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// queryer is a *sql.DB or a *sql.Tx.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// eachRow runs q and calls row for each row it returns, closing the rows
// before it returns.
func eachRow(ctx context.Context, db queryer, q string, args []any, row func(*sql.Rows) error) error {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := row(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
