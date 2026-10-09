package store

import (
	"context"
	"fmt"
)

// migrations run in order; each index is a schema version. Append only:
// never edit or reorder a migration that has shipped.
var migrations = []string{
	// 1: walking-skeleton honk counter (dropped by the game-server plan).
	`CREATE TABLE honks (id INTEGER PRIMARY KEY CHECK (id = 1), count INTEGER NOT NULL);
	 INSERT INTO honks (id, count) VALUES (1, 0);`,
	// 2: the honk demo is gone.
	`DROP TABLE honks;`,
	// 3: saved games (milestone 05). A game is rebuilt by replaying its turns.
	`CREATE TABLE games (
		code       TEXT PRIMARY KEY,
		white      TEXT NOT NULL,
		black      TEXT,
		created_at INTEGER NOT NULL,
		ended_at   INTEGER,
		result     TEXT,
		winner     TEXT,
		rematch_of TEXT REFERENCES games(code)
	 );
	 CREATE INDEX games_ended_at ON games(ended_at);
	 CREATE TABLE turns (
		game     TEXT NOT NULL REFERENCES games(code),
		ply      INTEGER NOT NULL,
		move     TEXT NOT NULL,
		dice     TEXT NOT NULL,
		white_ms INTEGER NOT NULL,
		black_ms INTEGER NOT NULL,
		at       INTEGER NOT NULL,
		PRIMARY KEY (game, ply)
	 );`,
	// 4: guest names (milestone 06a). A guest gets a row when they first
	// play; each seat keeps the name its player had when they sat down.
	// changes and changes_since count name changes in the current 24-hour
	// window; offers holds the names on offer, comma-separated, until one
	// is chosen.
	`CREATE TABLE guests (
		id            TEXT PRIMARY KEY,
		name          TEXT NOT NULL,
		created_at    INTEGER NOT NULL,
		changes       INTEGER NOT NULL DEFAULT 0,
		changes_since INTEGER,
		offers        TEXT
	 );
	 CREATE INDEX guests_name ON guests(name);
	 ALTER TABLE games ADD COLUMN white_name TEXT;
	 ALTER TABLE games ADD COLUMN black_name TEXT;`,
	// 5: longer potholes (milestone 06c). rules says which rules a game's
	// turns replay under; games saved before are rules 1. Rather than keep
	// the old engine to replay them, unfinished ones end as 'retired', and
	// only games under the current rules are ever loaded.
	`ALTER TABLE games ADD COLUMN rules INTEGER NOT NULL DEFAULT 1;
	 UPDATE games SET ended_at = CAST(strftime('%s','now') AS INTEGER) * 1000, result = 'retired'
	  WHERE ended_at IS NULL;`,
	// 6: a roll onto an open pothole resets it instead of rolling again.
	// Rules 2 turns that re-rolled off a hole don't replay, so unfinished
	// rules 2 games end as 'retired', like migration 5.
	`UPDATE games SET ended_at = CAST(strftime('%s','now') AS INTEGER) * 1000, result = 'retired'
	  WHERE ended_at IS NULL AND rules = 2;`,
	// 7: visits and browser errors (stats plan 1). visitor is the first 12
	// hex characters of the guest ID; path is a route, never a game code;
	// referrer is a host. Both tables are pruned after 400 days.
	`CREATE TABLE visits (
		at       INTEGER NOT NULL,
		visitor  TEXT NOT NULL,
		path     TEXT NOT NULL,
		referrer TEXT NOT NULL DEFAULT '',
		country  TEXT NOT NULL DEFAULT '',
		city     TEXT NOT NULL DEFAULT '',
		device   TEXT NOT NULL DEFAULT '',
		os       TEXT NOT NULL DEFAULT '',
		browser  TEXT NOT NULL DEFAULT ''
	 );
	 CREATE INDEX visits_at ON visits(at);
	 CREATE INDEX visits_visitor ON visits(visitor, at);
	 CREATE TABLE browser_errors (
		at      INTEGER NOT NULL,
		visitor TEXT NOT NULL,
		path    TEXT NOT NULL,
		message TEXT NOT NULL,
		browser TEXT NOT NULL DEFAULT ''
	 );
	 CREATE INDEX browser_errors_at ON browser_errors(at);`,
	// 8: game stats (stats plan 2). kind is friend or quick (a rematch keeps
	// its game's kind; everything saved before is friend). joined_at is when
	// Black sat down. playtest marks a game a playtest started (the stats
	// leave it out; a rematch keeps the mark). game_stats is one row per
	// game that got going, written with its result; open_hist counts the
	// turns that ended with 0..5 holes open. searches logs quick-match
	// searches (guest is the 12-hex visitor key). events counts practice
	// games, reopened game streams and restarts.
	`ALTER TABLE games ADD COLUMN kind TEXT NOT NULL DEFAULT 'friend';
	 ALTER TABLE games ADD COLUMN joined_at INTEGER;
	 ALTER TABLE games ADD COLUMN playtest INTEGER NOT NULL DEFAULT 0;
	 UPDATE games SET joined_at = created_at WHERE black IS NOT NULL AND rematch_of IS NOT NULL;
	 CREATE TABLE game_stats (
		game          TEXT PRIMARY KEY REFERENCES games(code),
		moves         INTEGER NOT NULL,
		opened        INTEGER NOT NULL,
		reset         INTEGER NOT NULL,
		closed_rounds INTEGER NOT NULL,
		closed_cap    INTEGER NOT NULL,
		repaired      INTEGER NOT NULL,
		fell          INTEGER NOT NULL,
		saving_rolls  INTEGER NOT NULL,
		saved         INTEGER NOT NULL,
		white_lost    INTEGER NOT NULL,
		black_lost    INTEGER NOT NULL,
		mate_by_roll  INTEGER NOT NULL,
		open_hist     TEXT NOT NULL
	 );
	 CREATE TABLE searches (
		started_at INTEGER NOT NULL,
		ended_at   INTEGER NOT NULL,
		guest      TEXT NOT NULL,
		matched    INTEGER NOT NULL,
		others     INTEGER NOT NULL
	 );
	 CREATE INDEX searches_started_at ON searches(started_at);
	 CREATE TABLE events (
		at   INTEGER NOT NULL,
		kind TEXT NOT NULL
	 );
	 CREATE INDEX events_at ON events(at);`,
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	version, err := s.Version(ctx)
	if err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		if err := s.apply(ctx, i+1, migrations[i]); err != nil {
			return err
		}
	}
	return nil
}

// apply runs migration version and records it, in one transaction.
func (s *Store) apply(ctx context.Context, version int, migration string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, migration); err != nil {
		return fmt.Errorf("migration %d: %w", version, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_version (version) VALUES (?)`, version); err != nil {
		return fmt.Errorf("record migration %d: %w", version, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", version, err)
	}
	return nil
}

// Version returns the applied schema version.
func (s *Store) Version(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	return v, err
}
