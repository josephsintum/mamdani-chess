package store

import (
	"reflect"
	"testing"
	"time"
)

var newYork = must1(time.LoadLocation("America/New_York"))

func must1[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// day returns the instant at hour o'clock in New York on day d of October 2026.
func day(d, hour int) time.Time { return time.Date(2026, 10, d, hour, 0, 0, 0, newYork) }

func seedVisits(t *testing.T, s *Store) {
	t.Helper()
	ctx := t.Context()
	v := func(at time.Time, who, path string, more Visit) {
		more.At, more.Visitor, more.Path = at, who, path
		must(t, s.AddVisit(ctx, more))
	}
	// ann: three days, phone from New York, came from Instagram
	v(day(1, 9), "ann000000000", "/", Visit{Referrer: "l.instagram.com", Country: "United States", City: "New York", Device: "phone", OS: "iOS", Browser: "Safari"})
	v(day(1, 9), "ann000000000", "/play", Visit{Device: "phone", OS: "iOS", Browser: "Safari"})
	v(day(2, 20), "ann000000000", "/game", Visit{Device: "phone", OS: "iOS", Browser: "Safari"})
	v(day(3, 20), "ann000000000", "/game", Visit{Device: "phone", OS: "iOS", Browser: "Safari"})
	// bob: one day, desktop from Toronto, an invite link
	v(day(2, 12), "bob000000000", "/game", Visit{Country: "Canada", City: "Toronto", Device: "desktop", OS: "macOS", Browser: "Chrome"})
	// cat: first visit before the range, so she is returning on day 3
	v(day(1, 9).Add(-30*24*time.Hour), "cat000000000", "/", Visit{Device: "desktop", OS: "Windows", Browser: "Firefox"})
	v(day(3, 23).Add(59*time.Minute), "cat000000000", "/rules", Visit{Device: "desktop", OS: "Windows", Browser: "Firefox"})
	must(t, s.AddBrowserError(ctx, BrowserError{At: day(3, 10), Visitor: "ann000000000", Path: "/game", Message: "TypeError: x is undefined", Browser: "Safari"}))
}

func TestVisitStats(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedVisits(t, s)
	st, err := s.VisitStats(ctx, day(1, 0), day(4, 0), newYork)
	must(t, err)
	if st.Visitors != 3 || st.Returning != 1 || st.Views != 6 {
		t.Errorf("visitors %d returning %d views %d, want 3 1 6", st.Visitors, st.Returning, st.Views)
	}
	wantDays := []DayCount{{"2026-10-01", 1, 0}, {"2026-10-02", 1, 1}, {"2026-10-03", 0, 2}}
	if !reflect.DeepEqual(st.Days, wantDays) {
		t.Errorf("days = %v, want %v", st.Days, wantDays)
	}
	check := func(what string, got, want []Count) {
		t.Helper()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	// Ties sort by label, ignoring case.
	check("countries", st.Countries, []Count{{"Canada", 1}, {"United States", 1}, {"Unknown", 1}})
	check("cities", st.Cities, []Count{{"New York", 1}, {"Toronto", 1}, {"Unknown", 1}})
	check("sources", st.Sources, []Count{{"A game invite link", 1}, {"Direct or a chat app", 1}, {"Instagram", 1}})
	check("devices", st.Devices, []Count{{"desktop", 2}, {"phone", 1}})
	check("systems", st.Systems, []Count{{"iOS", 1}, {"macOS", 1}, {"Windows", 1}})
	check("browsers", st.Browsers, []Count{{"Chrome", 1}, {"Firefox", 1}, {"Safari", 1}})
	check("pages", st.Pages, []Count{{"/game", 3}, {"/", 1}, {"/play", 1}, {"/rules", 1}})
	if st.Errors != 1 || st.LatestError != "TypeError: x is undefined" {
		t.Errorf("errors %d %q", st.Errors, st.LatestError)
	}
}

// A visit at 03:00 UTC on Oct 4 is still Oct 3 in New York.
func TestVisitStatsGroupsDaysInZone(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddVisit(ctx, Visit{At: time.Date(2026, 10, 4, 3, 0, 0, 0, time.UTC), Visitor: "ann000000000", Path: "/"}))
	st, err := s.VisitStats(ctx, day(3, 0), day(4, 0), newYork)
	must(t, err)
	if len(st.Days) != 1 || st.Days[0].Day != "2026-10-03" || st.Days[0].New != 1 {
		t.Fatalf("days = %v", st.Days)
	}
}

// Top lists keep six entries and fold the rest into "Other".
func TestVisitStatsFoldsTheLongTail(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	for i, c := range []string{"A", "B", "C", "D", "E", "F", "G", "H"} {
		must(t, s.AddVisit(ctx, Visit{At: day(1, 10), Visitor: string(rune('a'+i)) + "00000000000", Path: "/", Country: c}))
	}
	must(t, s.AddVisit(ctx, Visit{At: day(1, 11), Visitor: "a00000000000", Path: "/", Country: "A"}))
	st, err := s.VisitStats(ctx, day(1, 0), day(2, 0), newYork)
	must(t, err)
	if len(st.Countries) != 7 || st.Countries[0] != (Count{"A", 1}) || st.Countries[6] != (Count{"Other", 2}) {
		t.Fatalf("countries = %v", st.Countries)
	}
}

func TestFunnelStats(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedVisits(t, s)
	// ann (white) and bob (black) played a game to checkmate; ann played a second one
	// that was aborted; dan moved in a game that is still on.
	ann, bob, dan := "ann000000000ffffffffffffffffffffffffffffffffffffffffffffffffffff", "bob000000000ffffffffffffffffffffffffffffffffffffffffffffffffffff", "dan000000000ffffffffffffffffffffffffffffffffffffffffffffffffffff"
	must(t, s.CreateGame(ctx, Game{Code: "G00001", White: ann, WhiteName: "a", CreatedAt: day(2, 12)}))
	must(t, s.SeatBlack(ctx, "G00001", bob, "b"))
	must(t, s.AddTurn(ctx, "G00001", Turn{Ply: 0, Move: "e2e4", At: day(2, 12)}))
	must(t, s.EndGame(ctx, "G00001", Result{EndedAt: day(2, 13), Reason: "checkmate", Winner: "white"}, &Turn{Ply: 1, Move: "e7e5", At: day(2, 13)}))
	must(t, s.CreateGame(ctx, Game{Code: "G00002", White: ann, WhiteName: "a", Black: dan, BlackName: "d", CreatedAt: day(3, 12)}))
	must(t, s.EndGame(ctx, "G00002", Result{EndedAt: day(3, 13), Reason: "aborted"}, nil))
	must(t, s.CreateGame(ctx, Game{Code: "G00003", White: dan, WhiteName: "d", CreatedAt: day(3, 14)}))
	must(t, s.AddTurn(ctx, "G00003", Turn{Ply: 0, Move: "d2d4", At: day(3, 14)}))
	f, err := s.FunnelStats(ctx, day(1, 0), day(4, 0))
	must(t, err)
	want := Funnel{Visited: 3, Opened: 2, Moved: 3, Finished: 2, Again: 0, Games: 3}
	if f != want {
		t.Fatalf("funnel = %+v, want %+v", f, want)
	}
}
