package store

import (
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := t.Context()
	s, path := openTemp(t)
	s.Close()

	s, err := Open(path) // second open must not re-run migrations
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	v, err := s.Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != len(migrations) {
		t.Fatalf("version = %d, want %d", v, len(migrations))
	}
	var rows int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_version`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != len(migrations) {
		t.Fatalf("schema_version has %d rows, want %d", rows, len(migrations))
	}
}
