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
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}
	return nil
}

// Version returns the applied schema version.
func (s *Store) Version(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	return v, err
}
