package store

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

var (
	t0      = time.UnixMilli(1_791_000_000_000)
	newYork = must1(time.LoadLocation("America/New_York"))
)

func must(t testing.TB, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func must1[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// openTemp opens a new database, its counters' days cut in UTC, in a
// directory Open has to create.
func openTemp(t *testing.T) (*Store, string) { return openTempIn(t, time.UTC) }

func openTempIn(t *testing.T, zone *time.Location) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "test.db")
	s, err := OpenIn(path, zone)
	if err != nil {
		t.Fatalf("OpenIn: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// openAt builds a database at schema version v, runs seed against it, and
// opens it with Open, which applies every later migration.
func openAt(t *testing.T, v int, seed ...string) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	must(t, err)
	must1(db.Exec(`CREATE TABLE schema_version (version INTEGER NOT NULL)`))
	for i, m := range migrations[:v] {
		must1(db.Exec(m))
		must1(db.Exec(`INSERT INTO schema_version (version) VALUES (?)`, i+1))
	}
	for _, q := range seed {
		must1(db.Exec(q))
	}
	db.Close()
	s, err := Open(path)
	must(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

// counters reads every counter as "day key" → n, leaving out zeros (an
// amount moved away by a later write).
func counters(t *testing.T, s *Store) map[string]int {
	t.Helper()
	days, err := s.Counts(t.Context(), "", "9999-12-31")
	must(t, err)
	out := map[string]int{}
	for _, d := range days {
		for k, n := range d.N {
			if n != 0 {
				out[d.Day+" "+k] = n
			}
		}
	}
	return out
}

// keysOn prefixes each key with day, for comparing with counters.
func keysOn(day string, kv map[string]int) map[string]int {
	out := map[string]int{}
	for k, n := range kv {
		out[day+" "+k] = n
	}
	return out
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
