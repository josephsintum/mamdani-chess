package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"
	"time"
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

// named gives alice the name pigeon-astoria.
func named(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.EnsureGuest(context.Background(), "alice", draws(t, "pigeon-astoria")); err != nil {
		t.Fatal(err)
	}
}

func TestOffersStayTheSameUntilOneIsChosen(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	named(t, s)
	// The current name is never offered, and offers don't repeat.
	offers, left, err := s.NameOffers(ctx, "alice", draws(t, "pigeon-astoria", "bagel-soho", "bagel-soho", "knish-dumbo", "rook-harlem"), t0)
	if err != nil || !reflect.DeepEqual(offers, []string{"bagel-soho", "knish-dumbo", "rook-harlem"}) || left.Left != 3 {
		t.Fatalf("offers %v, %+v, %v", offers, left, err)
	}
	// Asking again draws nothing (draws would fail the test): the same three.
	again, _, err := s.NameOffers(ctx, "alice", draws(t), t0.Add(time.Hour))
	if err != nil || !reflect.DeepEqual(again, offers) {
		t.Fatalf("second ask %v, %v; want the same offers", again, err)
	}
}

func TestChoosingAnOfferUsesAChange(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	named(t, s)
	_, _, err := s.NameOffers(ctx, "alice", draws(t, "bagel-soho", "knish-dumbo", "rook-harlem"), t0)
	must(t, err)
	left, err := s.ChooseName(ctx, "alice", "knish-dumbo", t0)
	if err != nil || left.Left != 2 || !left.ResetAt.Equal(t0.Add(NameWindow)) {
		t.Fatalf("choose: %+v, %v; want 2 left, reset 24 h after", left, err)
	}
	if got, _ := s.GuestName(ctx, "alice"); got != "knish-dumbo" {
		t.Fatalf("name %q after choosing", got)
	}
	// The used offers are gone: the next ask draws three new ones.
	offers, _, err := s.NameOffers(ctx, "alice", draws(t, "hero-inwood", "lox-fidi", "stoop-chelsea"), t0)
	if err != nil || offers[0] != "hero-inwood" {
		t.Fatalf("offers after choosing %v, %v", offers, err)
	}
}

func TestOnlyAnOfferedNameCanBeChosen(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	named(t, s)
	if _, err := s.ChooseName(ctx, "alice", "bagel-soho", t0); !errors.Is(err, ErrNotOffered) {
		t.Fatalf("choosing with no offers: %v, want ErrNotOffered", err)
	}
	_, _, err := s.NameOffers(ctx, "alice", draws(t, "bagel-soho", "knish-dumbo", "rook-harlem"), t0)
	must(t, err)
	if _, err := s.ChooseName(ctx, "alice", "anything-i-like", t0); !errors.Is(err, ErrNotOffered) {
		t.Fatalf("choosing an unoffered name: %v, want ErrNotOffered", err)
	}
	if got, _ := s.GuestName(ctx, "alice"); got != "pigeon-astoria" {
		t.Fatalf("name changed to %q", got)
	}
}

func TestThreeChangesADay(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	named(t, s)
	change := func(at time.Time, a, b, c string) (Allowance, error) {
		t.Helper()
		if _, _, err := s.NameOffers(ctx, "alice", draws(t, a, b, c), at); err != nil {
			return Allowance{}, err
		}
		return s.ChooseName(ctx, "alice", a, at)
	}
	for i, at := range []time.Time{t0, t0.Add(time.Hour), t0.Add(2 * time.Hour)} {
		left, err := change(at, fmt.Sprintf("a%d-soho", i), fmt.Sprintf("b%d-soho", i), fmt.Sprintf("c%d-soho", i))
		if err != nil || left.Left != 2-i {
			t.Fatalf("change %d: %+v, %v", i+1, left, err)
		}
	}
	// The fourth within 24 hours of the first is refused, offers included.
	_, left, err := s.NameOffers(ctx, "alice", draws(t), t0.Add(23*time.Hour))
	if !errors.Is(err, ErrNoChanges) || left.Left != 0 || !left.ResetAt.Equal(t0.Add(NameWindow)) {
		t.Fatalf("fourth offers: %+v, %v; want ErrNoChanges, reset at 24 h", left, err)
	}
	if left, _ := s.Allowance(ctx, "alice", t0.Add(23*time.Hour)); left.Left != 0 {
		t.Fatalf("allowance at 23 h: %+v", left)
	}
	// 24 hours after the first change, three more.
	if left, _ := s.Allowance(ctx, "alice", t0.Add(NameWindow)); left.Left != 3 || !left.ResetAt.IsZero() {
		t.Fatalf("allowance at 24 h: %+v; want 3 and no reset time", left)
	}
	if left, err := change(t0.Add(NameWindow), "d-soho", "e-soho", "f-soho"); err != nil || left.Left != 2 {
		t.Fatalf("change after the window: %+v, %v", left, err)
	}
}

func TestOffersNeedAName(t *testing.T) {
	s, _ := openTemp(t)
	defer s.Close()
	if _, _, err := s.NameOffers(context.Background(), "bob", draws(t), t0); !errors.Is(err, ErrNoName) {
		t.Fatalf("offers for a guest without a name: %v, want ErrNoName", err)
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
