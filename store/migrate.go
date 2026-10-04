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
