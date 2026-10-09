package store

import (
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

	var n int
	var city string
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*), MAX(city) FROM visits`).Scan(&n, &city))
	if n != 2 || city != "New York" {
		t.Fatalf("visits: %d rows, city %q", n, city)
	}
	removed, err := s.PruneVisits(ctx, t0.Add(time.Hour))
	must(t, err)
	if removed != 2 { // the first visit and the error
		t.Fatalf("pruned %d rows, want 2", removed)
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
