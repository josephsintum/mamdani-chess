package store

import (
	"testing"
	"time"
)

func TestBrowserErrorsCountedAndLatest(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0, Path: "/game", Message: "TypeError: x"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(2 * time.Hour), Path: "/", Message: "TypeError: y"}))
	if got := counters(t, s); got[dayOf(t0, time.UTC)+" errors"] != 2 {
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

// Past errorRowsPerDay a day the messages stop being stored, and the
// errors are still counted. A rebuild keeps them: their messages were
// never stored, so the rows can't recount them.
func TestRebuildKeepsErrorsPastTheCap(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	for i := range errorRowsPerDay + 5 {
		must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(time.Duration(i) * time.Second), Path: "/", Message: "x"}))
	}
	var rows int
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM browser_errors`).Scan(&rows))
	if got := counters(t, s)[dayOf(t0, time.UTC)+" errors"]; rows != errorRowsPerDay || got != errorRowsPerDay+5 {
		t.Fatalf("%d rows and %d counted, want %d and %d", rows, got, errorRowsPerDay, errorRowsPerDay+5)
	}
	if _, err := s.rebuild(ctx, t0); err != nil {
		t.Fatal(err)
	}
	if got := counters(t, s)[dayOf(t0, time.UTC)+" errors"]; got != errorRowsPerDay+5 {
		t.Fatalf("after a rebuild %d errors, want %d", got, errorRowsPerDay+5)
	}
}

// Pruning cuts at a midnight in the store's zone, 400 days back.
func TestPruneErrorsAtMidnight(t *testing.T) {
	ctx := t.Context()
	s, _ := openTempIn(t, newYork)
	now := time.Date(2026, 10, 19, 15, 0, 0, 0, newYork)
	cutoff := time.Date(2025, 9, 14, 0, 0, 0, 0, newYork) // 400 days before Oct 19, 2026
	must(t, s.AddBrowserError(ctx, BrowserError{At: cutoff.Add(-time.Minute), Message: "old"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: cutoff, Message: "kept"}))
	n, err := s.PruneErrors(ctx, now)
	must(t, err)
	if n != 1 {
		t.Fatalf("pruned %d, want 1", n)
	}
	if latest, _ := s.LatestError(ctx, time.Time{}, now); latest != "kept" {
		t.Fatalf("left %q", latest)
	}
}
