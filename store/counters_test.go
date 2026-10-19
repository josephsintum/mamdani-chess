package store

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func openTempIn(t *testing.T, zone *time.Location) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := OpenIn(path, zone)
	if err != nil {
		t.Fatalf("OpenIn: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// counters reads every counter as "day key" → n, leaving out zeros (an
// amount moved away by a later write).
func counters(t *testing.T, s *Store) map[string]int {
	t.Helper()
	rows, err := s.db.QueryContext(t.Context(), `SELECT day, key, n FROM stats_days`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var day, key string
		var n int
		if err := rows.Scan(&day, &key, &n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			out[day+" "+key] = n
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

func TestEnsureCountersRebuildsOnlyWhenNeeded(t *testing.T) {
	ctx := t.Context()
	s, path := openTempIn(t, newYork)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, newYork)
	created := time.Date(2026, 10, 3, 23, 30, 0, 0, newYork)
	must(t, s.CreateGame(ctx, Game{Code: "GAME01", White: "w", Black: "b", Kind: "quick", CreatedAt: created}))
	// An errors count from before the cutoff day has no rows left to recount, so it stays.
	must1(s.db.ExecContext(ctx, `INSERT INTO stats_days (day, key, n) VALUES ('2024-01-01', 'errors', 7), ('2024-01-01', 'games.quick', 3)`))

	rep, err := s.EnsureCounters(ctx, now) // a new database: stats_state is empty
	must(t, err)
	if rep.Rows != 1 || s.CountersStale() {
		t.Fatalf("first start: %+v, stale %v", rep, s.CountersStale())
	}
	want := map[string]int{"2026-10-03 games.quick": 1, "2024-01-01 errors": 7}
	if got := counters(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("after the first rebuild %v, want %v", got, want)
	}
	if rep, err = s.EnsureCounters(ctx, now); err != nil || rep != (RebuildReport{}) {
		t.Fatalf("second start rebuilt: %+v, %v", rep, err)
	}
	s.Close()

	// Another STATS_TZ cuts the days differently: rebuild.
	s, err = OpenIn(path, time.UTC)
	must(t, err)
	defer s.Close()
	if rep, err = s.EnsureCounters(ctx, now); err != nil || rep.Rows != 1 {
		t.Fatalf("zone change: %+v, %v", rep, err)
	}
	if got := counters(t, s); got["2026-10-04 games.quick"] != 1 {
		t.Fatalf("after the zone change %v", got)
	}
}

// A rebuild that fails keeps the old counters and marks them stale.
func TestFailedRebuildMarksCountersStale(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.CreateGame(ctx, Game{Code: "GAME01", White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))
	must1(s.db.ExecContext(ctx, `INSERT INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
		VALUES ('GAME01', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 'not a histogram')`))
	before := counters(t, s)
	if _, err := s.EnsureCounters(ctx, t0); err == nil || !s.CountersStale() {
		t.Fatalf("err %v, stale %v; want an error and stale", err, s.CountersStale())
	}
	if after := counters(t, s); !reflect.DeepEqual(after, before) {
		t.Fatalf("a failed rebuild changed the counters: %v → %v", before, after)
	}
}

func mergeKeys(a, b map[string]int) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
