package store

import (
	"strconv"
	"time"
)

// Count is one labelled count, a row of a list or a histogram's column.
type Count struct {
	Label string
	N     int
}

// KindDay is one day's games created by kind, plus practice games started.
type KindDay struct {
	Day                     string
	Quick, Friend, Practice int
}

// Heat is games that got going by weekday (Monday first) and two-hour
// block of their start.
type Heat [7][12]int

// Summary is what the Games, Quick match, The road and Health sections
// show for a range of days, summed from the daily counters. Playtests are
// never counted.
type Summary struct {
	Games, GotGoing int // games created; of those, ended with a result
	Days            []KindDay
	// Ends: Checkmate, Mate by a roll, Resignation, Out of time, Draw,
	// Aborted, Nobody came, in that order, zeros dropped.
	Ends []Count
	// Lengths: moves by both sides in games that got going, every bucket kept.
	Lengths []Count
	OnClock int // a side ran out of time and lost
	// WinnerLeft: the winner's clock at the end; LeftUnder1m of them had
	// under a minute.
	WinnerLeft  []Count
	LeftUnder1m int
	Heat        Heat
	// Friend: links made (rematches aren't links), how many someone joined
	// and finished, the rematches of friend games, and how long the joined
	// ones took (JoinedIn5m within five minutes).
	Friend struct {
		Links, Joined, Finished, Rematches int
		Join                               []Count
		JoinedIn5m                         int
	}
	Quick struct {
		Searches, Matched, GaveUp, Alone int
		Waits, GaveUpAfter               []Count
		GaveUpIn30s                      int
	}
	Road struct {
		Games, Opened, Fell, SavingRolls, Saved, Repaired, Reset, MateByRoll, ClosedRounds, ClosedCap, NoFall int
		OpenHist                                                                                              [6]int
		// Decisive games: the winner lost fewer, more or as many pieces to the road as the loser.
		LessWon, MoreWon, Same int
	}
	Reconnects, Restarts, Errors int
}

// Summarize sums the days' counters (from Counts) into a Summary.
// fromDay <= day < toDay are the range's days, each listed in Days (an
// empty fromDay starts at the first day with games or practice).
func Summarize(days []DayCounts, fromDay, toDay string) Summary {
	var out Summary
	sum := map[string]int{}
	byDay := map[string]map[string]int{}
	start := fromDay
	for _, d := range days {
		for k, n := range d.N {
			sum[k] += n
		}
		byDay[d.Day] = d.N
		wd := weekday(d.Day)
		for b := range 12 {
			out.Heat[wd][b] += d.N["heat."+strconv.Itoa(b)]
		}
		if start == "" && d.N["games.quick"]+d.N["games.friend"]+d.N["event.practice"] > 0 {
			start = d.Day
		}
	}
	if start != "" {
		for _, day := range daysFrom(start, toDay) {
			n := byDay[day]
			out.Days = append(out.Days, KindDay{Day: day, Quick: n["games.quick"], Friend: n["games.friend"], Practice: n["event.practice"]})
		}
	}

	out.Games = sum["games.quick"] + sum["games.friend"]
	out.GotGoing = sum["got_going"]
	for _, e := range []struct{ key, label string }{
		{"checkmate", "Checkmate"}, {"roll", "Mate by a roll"}, {"resignation", "Resignation"}, {"timeout", "Out of time"},
		{"draw", "Draw"}, {"aborted", "Aborted"}, {"expired", "Nobody came"},
	} {
		if n := sum["end."+e.key]; n > 0 {
			out.Ends = append(out.Ends, Count{e.label, n})
		}
	}
	out.Lengths = histogram(sum, "len.", movesBuckets)
	out.OnClock = sum["clock.decided"]
	out.WinnerLeft = histogram(sum, "left.", leftBuckets)
	out.LeftUnder1m = sum["left.lt30s"] + sum["left.30s-1m"]

	f := &out.Friend
	f.Links, f.Joined, f.Finished, f.Rematches = sum["friend.links"], sum["friend.joined"], sum["friend.finished"], sum["friend.rematches"]
	f.Join = histogram(sum, "join.", joinBuckets)
	f.JoinedIn5m = sum["join.lt1m"] + sum["join.1-5m"]

	q := &out.Quick
	q.Searches, q.Matched, q.GaveUp, q.Alone = sum["search"], sum["search.matched"], sum["search.gaveup"], sum["search.alone"]
	q.Waits = histogram(sum, "wait.", waitBuckets)
	q.GaveUpAfter = histogram(sum, "gaveup.", waitBuckets)
	q.GaveUpIn30s = sum["gaveup.lt10s"] + sum["gaveup.10-30s"]

	r := &out.Road
	r.Games, r.Opened, r.Fell, r.SavingRolls, r.Saved = sum["road.games"], sum["road.opened"], sum["road.fell"], sum["road.saving_rolls"], sum["road.saved"]
	r.Repaired, r.Reset, r.MateByRoll = sum["road.repaired"], sum["road.reset"], sum["road.mate_by_roll"]
	r.ClosedRounds, r.ClosedCap, r.NoFall = sum["road.closed_rounds"], sum["road.closed_cap"], sum["road.no_fall"]
	for i := range r.OpenHist {
		r.OpenHist[i] = sum["open."+strconv.Itoa(i)]
	}
	r.LessWon, r.MoreWon, r.Same = sum["decided.less"], sum["decided.more"], sum["decided.same"]

	out.Reconnects, out.Restarts, out.Errors = sum["event.reconnect"], sum["event.restart"], sum["errors"]
	return out
}

// histogram lists every bucket of a histogram with its count.
func histogram(sum map[string]int, prefix string, bs []bucket) []Count {
	out := make([]Count, len(bs))
	for i, b := range bs {
		out[i] = Count{b.label, sum[prefix+b.key]}
	}
	return out
}

// daysFrom lists the days from <= day < to (2006-01-02, calendar days).
func daysFrom(from, to string) []string {
	t, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return nil
	}
	var out []string
	for d := from; d < to; d = t.Format(time.DateOnly) {
		out = append(out, d)
		t = t.AddDate(0, 0, 1)
	}
	return out
}

// weekday is a day's row in Heat: Monday 0 to Sunday 6.
func weekday(day string) int {
	t, err := time.Parse(time.DateOnly, day)
	if err != nil {
		return 0
	}
	return (int(t.Weekday()) + 6) % 7
}
