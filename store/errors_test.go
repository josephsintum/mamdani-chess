package store

import (
	"testing"
	"time"
)

func TestBrowserErrorsCountAndPrune(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0, Path: "/game", Message: "TypeError: x"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(2 * time.Hour), Path: "/", Message: "TypeError: y"}))
	// Searches and events go by the same cutoff.
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(2 * time.Hour)}))
	must(t, s.AddSearch(ctx, Search{StartedAt: t0.Add(2 * time.Hour), EndedAt: t0.Add(3 * time.Hour)}))
	must(t, s.AddEvent(ctx, "restart", t0))
	must(t, s.AddEvent(ctx, "restart", t0.Add(2*time.Hour)))

	st, err := s.BrowserErrors(ctx, t0, t0.Add(3*time.Hour))
	must(t, err)
	if st.N != 2 || st.Latest != "TypeError: y" {
		t.Fatalf("errors = %+v, want 2 and the later message", st)
	}
	if st, err = s.BrowserErrors(ctx, t0.Add(3*time.Hour), t0.Add(4*time.Hour)); err != nil || st != (ErrorStats{}) {
		t.Fatalf("a quiet range = %+v, %v", st, err)
	}

	removed, err := s.PruneStats(ctx, t0.Add(time.Hour))
	must(t, err)
	if removed != 3 { // the first error, the first search and the first event
		t.Fatalf("pruned %d rows, want 3", removed)
	}
	var errs, searches, events int
	must(t, s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM browser_errors), (SELECT COUNT(*) FROM searches), (SELECT COUNT(*) FROM events)`).Scan(&errs, &searches, &events))
	if errs != 1 || searches != 1 || events != 1 {
		t.Fatalf("%d errors, %d searches and %d events left, want 1 each", errs, searches, events)
	}
}
