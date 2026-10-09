package store

import (
	"reflect"
	"testing"
	"time"
)

func TestVisitRoundTripAndPrune(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddVisit(ctx, Visit{At: t0, Visitor: "0123456789ab", Path: "/game", Referrer: "l.instagram.com",
		Country: "United States", City: "New York", Device: "phone", OS: "iOS", Browser: "Safari"}))
	must(t, s.AddVisit(ctx, Visit{At: t0.Add(24 * time.Hour), Visitor: "0123456789ab", Path: "/"}))
	must(t, s.AddBrowserError(ctx, BrowserError{At: t0, Visitor: "0123456789ab", Path: "/game", Message: "TypeError: x", Browser: "Safari"}))
	// Searches and events go by the same cutoff.
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(2 * time.Hour), Guest: "0123456789ab"}))
	must(t, s.AddSearch(ctx, Search{StartedAt: t0.Add(2 * time.Hour), EndedAt: t0.Add(3 * time.Hour), Guest: "0123456789ab"}))
	must(t, s.AddEvent(ctx, "restart", t0))
	must(t, s.AddEvent(ctx, "restart", t0.Add(2*time.Hour)))

	var n int
	var city string
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(city) FROM visits`).Scan(&n, &city))
	if n != 2 || city != "New York" {
		t.Fatalf("visits: %d rows, city %q", n, city)
	}
	removed, err := s.PruneVisits(ctx, t0.Add(time.Hour))
	must(t, err)
	if removed != 4 { // the first visit, the error, the first search and the first event
		t.Fatalf("pruned %d rows, want 4", removed)
	}
	var searches, events int
	must(t, s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM searches), (SELECT COUNT(*) FROM events)`).Scan(&searches, &events))
	if searches != 1 || events != 1 {
		t.Fatalf("%d searches and %d events left, want 1 and 1", searches, events)
	}
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM visits`).Scan(&n))
	if n != 1 {
		t.Fatalf("%d visits left, want 1", n)
	}
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM browser_errors`).Scan(&n))
	if n != 0 {
		t.Fatalf("%d errors left, want 0", n)
	}
}

func TestVisitors(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedVisits(t, s)
	got, err := s.Visitors(ctx, day(1, 0), day(4, 0))
	must(t, err)
	if want := []string{"ann000000000", "bob000000000", "cat000000000"}; !reflect.DeepEqual(got, want) {
		t.Errorf("visitors = %v, want %v", got, want)
	}
	got, err = s.Visitors(ctx, day(2, 0), day(3, 0))
	must(t, err)
	if want := []string{"ann000000000", "bob000000000"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Oct 2 = %v, want %v", got, want)
	}
	got, err = s.Visitors(ctx, day(10, 0), day(11, 0))
	must(t, err)
	if len(got) != 0 {
		t.Errorf("a quiet day = %v, want none", got)
	}
}
