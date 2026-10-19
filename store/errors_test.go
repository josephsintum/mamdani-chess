package store

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBrowserErrorsCountedAndLatest(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0, Path: "/game", Message: "TypeError: x"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(2 * time.Hour), Path: "/", Message: "TypeError: y"}))
	if got := counters(t, s); got[DayOf(t0, time.UTC)+" errors"] != 2 {
		t.Fatalf("counters %v, want 2 errors", got)
	}
	latest, err := s.LatestError(ctx, t0, t0.Add(3*time.Hour))
	must(t, err)
	if latest != "TypeError: y" {
		t.Fatalf("latest %q", latest)
	}
	if latest, err = s.LatestError(ctx, t0.Add(3*time.Hour), t0.Add(4*time.Hour)); err != nil || latest != "" {
		t.Fatalf("a quiet range: %q, %v", latest, err)
	}
}

// Past errorRowsPerDay a day, an error folds into the day's newest row:
// it takes the new message and time and stands for one more error. So the
// rows stay capped, the latest message is the newest, and a rebuild
// recounts every error.
func TestErrorsPastTheCapFoldIntoTheNewestRow(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	for i := range errorRowsPerDay + 5 {
		must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(time.Duration(i) * time.Second), Path: "/", Message: fmt.Sprintf("m%d", i)}))
	}
	var rows int
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM browser_errors`).Scan(&rows))
	if got := counters(t, s)[DayOf(t0, time.UTC)+" errors"]; rows != errorRowsPerDay || got != errorRowsPerDay+5 {
		t.Fatalf("%d rows and %d counted, want %d and %d", rows, got, errorRowsPerDay, errorRowsPerDay+5)
	}
	if latest, err := s.LatestError(ctx, t0, t0.Add(time.Hour)); err != nil || latest != fmt.Sprintf("m%d", errorRowsPerDay+4) {
		t.Fatalf("latest %q, %v; want the newest message", latest, err)
	}
	if _, err := s.rebuild(ctx, t0); err != nil {
		t.Fatal(err)
	}
	if got := counters(t, s)[DayOf(t0, time.UTC)+" errors"]; got != errorRowsPerDay+5 {
		t.Fatalf("after a rebuild %d errors, want %d", got, errorRowsPerDay+5)
	}
}

// A new STATS_TZ cuts the days differently. The rebuild recounts every
// error from the rows, so a day over the cap neither loses errors nor
// counts them twice.
func TestErrorCountsSurviveAZoneChange(t *testing.T) {
	ctx := t.Context()
	s, path := openTempIn(t, newYork)
	morning := time.Date(2026, 10, 3, 10, 0, 0, 0, newYork) // Oct 3 in UTC too
	night := time.Date(2026, 10, 3, 23, 0, 0, 0, newYork)   // Oct 4 in UTC
	for i := range errorRowsPerDay + 5 {
		at := morning
		if i >= errorRowsPerDay/2 {
			at = night
		}
		must(t, s.AddBrowserError(ctx, BrowserError{At: at.Add(time.Duration(i) * time.Second), Message: "x"}))
	}
	s.Close()
	s, err := OpenIn(path, time.UTC)
	must(t, err)
	defer s.Close()
	must1(s.EnsureCounters(ctx, night))
	total := 0
	for k, n := range counters(t, s) {
		if strings.HasSuffix(k, " errors") {
			total += n
		}
	}
	if total != errorRowsPerDay+5 {
		t.Fatalf("%d errors after the zone change, want %d: %v", total, errorRowsPerDay+5, counters(t, s))
	}
}

// Pruning cuts errors, searches and events at a midnight in the store's
// zone, 400 days back. A rebuild keeps the counts of the days pruned.
func TestPruneAtMidnightKeepsTheOldCounts(t *testing.T) {
	ctx := t.Context()
	s, _ := openTempIn(t, newYork)
	now := time.Date(2026, 10, 19, 15, 0, 0, 0, newYork)
	cutoff := time.Date(2025, 9, 14, 0, 0, 0, 0, newYork) // 400 days before Oct 19, 2026
	old := cutoff.Add(-time.Minute)
	must(t, s.AddBrowserError(ctx, BrowserError{At: old, Message: "old"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: cutoff, Message: "kept"}))
	must(t, s.AddSearch(ctx, Search{StartedAt: old, EndedAt: old.Add(time.Second)}))
	must(t, s.AddEvent(ctx, "reconnect", old))
	before := counters(t, s)
	n, err := s.Prune(ctx, now)
	must(t, err)
	if n != 3 {
		t.Fatalf("pruned %d, want 3", n)
	}
	if latest, _ := s.LatestError(ctx, time.Time{}, now); latest != "kept" {
		t.Fatalf("left %q", latest)
	}
	if _, err := s.rebuild(ctx, now); err != nil {
		t.Fatal(err)
	}
	if after := counters(t, s); !reflect.DeepEqual(after, before) {
		t.Fatalf("a rebuild after pruning:\n got %v\nwant %v", after, before)
	}
}
