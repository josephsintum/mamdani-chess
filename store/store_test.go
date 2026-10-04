package store

import (
	"context"
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
	return s, path
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
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

func TestHonkIncrements(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	for want := int64(1); want <= 3; want++ {
		got, err := s.Honk(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Honk = %d, want %d", got, want)
		}
	}
}

func TestHonksPersistAcrossReopen(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	s.Honk(ctx)
	s.Honk(ctx)
	s.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, err := s.Honks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("Honks after reopen = %d, want 2", n)
	}
}
