package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

// draws returns a draw function that hands out names in order.
func draws(t *testing.T, names ...string) func() string {
	return func() string {
		if len(names) == 0 {
			t.Fatal("drew more names than scripted")
		}
		n := names[0]
		names = names[1:]
		return n
	}
}

func TestGuestNameIsEmptyUntilEnsured(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	if name, err := s.GuestName(ctx, "alice"); err != nil || name != "" {
		t.Fatalf("GuestName before any name = %q, %v", name, err)
	}
	var rows int
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM guests`).Scan(&rows))
	if rows != 0 {
		t.Fatalf("GuestName created %d rows", rows)
	}
}

func TestEnsureGuestCreatesOnceThenKeepsTheName(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	name, err := s.EnsureGuest(ctx, "alice", draws(t, "pigeon-astoria"))
	if err != nil || name != "pigeon-astoria" {
		t.Fatalf("first EnsureGuest = %q, %v", name, err)
	}
	// A second call draws nothing (draws would fail the test) and keeps it.
	name, err = s.EnsureGuest(ctx, "alice", draws(t))
	if err != nil || name != "pigeon-astoria" {
		t.Fatalf("second EnsureGuest = %q, %v", name, err)
	}
	if got, _ := s.GuestName(ctx, "alice"); got != "pigeon-astoria" {
		t.Fatalf("GuestName = %q", got)
	}
}

func TestRerollGuestChangesTheName(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	_, err := s.EnsureGuest(ctx, "alice", draws(t, "pigeon-astoria"))
	must(t, err)
	// Drawing the current name again doesn't count: it draws on.
	name, err := s.RerollGuest(ctx, "alice", draws(t, "pigeon-astoria", "bagel-soho"))
	if err != nil || name != "bagel-soho" {
		t.Fatalf("RerollGuest = %q, %v", name, err)
	}
	if got, _ := s.GuestName(ctx, "alice"); got != "bagel-soho" {
		t.Fatalf("GuestName after reroll = %q", got)
	}
}

func TestRerollGivesANameToAGuestWithout(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	name, err := s.RerollGuest(ctx, "alice", draws(t, "pigeon-astoria"))
	if err != nil || name != "pigeon-astoria" {
		t.Fatalf("RerollGuest = %q, %v", name, err)
	}
}

func TestFreshNamesPreferUnusedOnes(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	_, err := s.EnsureGuest(ctx, "alice", draws(t, "pigeon-astoria"))
	must(t, err)
	name, err := s.EnsureGuest(ctx, "bob", draws(t, "pigeon-astoria", "bagel-soho"))
	if err != nil || name != "bagel-soho" {
		t.Fatalf("bob = %q, %v; want the unused bagel-soho", name, err)
	}
	// Every draw taken: settle for the last one after five tries.
	name, err = s.EnsureGuest(ctx, "carol", draws(t, "pigeon-astoria", "bagel-soho", "pigeon-astoria", "bagel-soho", "pigeon-astoria"))
	if err != nil || name != "pigeon-astoria" {
		t.Fatalf("carol = %q, %v; want the fifth draw", name, err)
	}
}

// A database from milestone 05 (schema version 3, a game without names)
// gains the guests table and name columns, and its games still load.
func TestMigration4KeepsOldGames(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	must(t, err)
	must(t, exec(db, `CREATE TABLE schema_version (version INTEGER NOT NULL)`))
	for i, m := range migrations[:3] {
		must(t, exec(db, m))
		must(t, exec(db, `INSERT INTO schema_version (version) VALUES (?)`, i+1))
	}
	must(t, exec(db, `INSERT INTO games (code, white, black, created_at) VALUES ('OLD123', 'alice', 'bob', ?)`, t0.UnixMilli()))
	db.Close()

	s, err := Open(path)
	must(t, err)
	defer s.Close()
	got, err := s.LoadForRestore(ctx, t0)
	must(t, err)
	want := []SavedGame{{Game: Game{Code: "OLD123", White: "alice", Black: "bob", CreatedAt: t0}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded\n%+v\nwant\n%+v", got, want)
	}
	if _, err := s.EnsureGuest(ctx, "alice", draws(t, "pigeon-astoria")); err != nil {
		t.Fatalf("guests table missing after migration: %v", err)
	}
}

func exec(db *sql.DB, q string, args ...any) error {
	_, err := db.Exec(q, args...)
	return err
}
