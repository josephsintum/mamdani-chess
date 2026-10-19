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

func TestDaysFrom(t *testing.T) {
	for _, c := range []struct {
		from, to string
		want     []string
	}{
		{"2026-10-01", "2026-10-04", []string{"2026-10-01", "2026-10-02", "2026-10-03"}},
		// Calendar days: the autumn clock change is still one row a day.
		{"2026-10-31", "2026-11-03", []string{"2026-10-31", "2026-11-01", "2026-11-02"}},
		{"2026-12-31", "2027-01-02", []string{"2026-12-31", "2027-01-01"}},
		{"2026-10-04", "2026-10-04", nil},
	} {
		if got := daysFrom(c.from, c.to); !reflect.DeepEqual(got, c.want) {
			t.Errorf("daysFrom(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestSummarize(t *testing.T) {
	days := []DayCounts{
		{Day: "2026-10-05", N: map[string]int{ // a Monday
			"games.quick": 2, "games.friend": 1, "event.practice": 1, "got_going": 2, "heat.10": 2,
			"end.checkmate": 1, "end.roll": 1, "end.aborted": 1, "len.11-20": 2, "left.lt30s": 1, "left.5m+": 1, "clock.decided": 1,
			"friend.links": 1, "friend.joined": 1, "join.1-5m": 1, "friend.finished": 1,
			"search": 3, "search.matched": 2, "wait.lt10s": 2, "search.gaveup": 1, "gaveup.30-60s": 1, "search.alone": 1,
			"road.games": 2, "road.opened": 9, "road.fell": 3, "road.no_fall": 1, "open.0": 4, "open.5": 1, "decided.less": 1, "decided.same": 1,
		}},
		{Day: "2026-10-07", N: map[string]int{"games.friend": 1, "end.expired": 1, "friend.links": 1, "event.restart": 1, "event.reconnect": 2, "errors": 3}},
	}
	got := Summarize(days, "2026-10-05", "2026-10-08")
	if want := []KindDay{{"2026-10-05", 2, 1, 1}, {"2026-10-06", 0, 0, 0}, {"2026-10-07", 0, 1, 0}}; !reflect.DeepEqual(got.Days, want) {
		t.Errorf("days %v, want %v", got.Days, want)
	}
	if got.Heat[0][10] != 2 {
		t.Errorf("heat: Monday 8–10 pm block = %d, want 2", got.Heat[0][10])
	}
	if want := []Count{{"Checkmate", 1}, {"Mate by a roll", 1}, {"Aborted", 1}, {"Nobody came", 1}}; !reflect.DeepEqual(got.Ends, want) {
		t.Errorf("ends %v", got.Ends)
	}
	checks := []struct {
		name      string
		got, want int
	}{
		{"games", got.Games, 4}, {"got going", got.GotGoing, 2}, {"on the clock", got.OnClock, 1},
		{"11–20 moves", got.Lengths[1].N, 2}, {"under 1 min left", got.LeftUnder1m, 1},
		{"links", got.Friend.Links, 2}, {"joined", got.Friend.Joined, 1}, {"joined within 5 min", got.Friend.JoinedIn5m, 1}, {"finished", got.Friend.Finished, 1},
		{"searches", got.Quick.Searches, 3}, {"waits under 10 s", got.Quick.Waits[0].N, 2}, {"gave up 30–60 s", got.Quick.GaveUpAfter[2].N, 1},
		{"gave up within 30 s", got.Quick.GaveUpIn30s, 0}, {"alone", got.Quick.Alone, 1},
		{"road games", got.Road.Games, 2}, {"opened", got.Road.Opened, 9}, {"no fall", got.Road.NoFall, 1}, {"open 5", got.Road.OpenHist[5], 1},
		{"less won", got.Road.LessWon, 1}, {"same", got.Road.Same, 1},
		{"reconnects", got.Reconnects, 2}, {"restarts", got.Restarts, 1}, {"errors", got.Errors, 3},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	// Every histogram keeps every bucket, quiet ones at zero.
	if len(got.Lengths) != 8 || len(got.WinnerLeft) != 5 || len(got.Friend.Join) != 5 || len(got.Quick.Waits) != 5 || len(got.Quick.GaveUpAfter) != 5 {
		t.Errorf("bucket counts %d %d %d %d %d", len(got.Lengths), len(got.WinnerLeft), len(got.Friend.Join), len(got.Quick.Waits), len(got.Quick.GaveUpAfter))
	}
	// All time starts at the first day with games or practice.
	all := Summarize(append([]DayCounts{{Day: "2026-09-01", N: map[string]int{"errors": 1}}}, days...), "", "2026-10-08")
	if len(all.Days) != 3 || all.Days[0].Day != "2026-10-05" {
		t.Errorf("all time days %v", all.Days)
	}
	if empty := Summarize(nil, "", "2026-10-08"); empty.Days != nil {
		t.Errorf("all time with nothing: %v", empty.Days)
	}
}
