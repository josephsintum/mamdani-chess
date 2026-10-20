package store

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

// countersVersion is what the stats_days keys mean. Bump it whenever a
// key's meaning changes (a new bucket edge, a new rule for what counts):
// the next start rebuilds every counter from the raw rows. Searches,
// events and errors are pruned after 400 days, so their counters for older
// days are kept as they were: a new search bucket edge can't re-bucket
// them, and their old keys drop out of the histograms.
const countersVersion = 2 // 2: lengths come from the turns, for every game that got going

// counts is what one write adds to the daily counters: an amount per day
// and key. Zero amounts are never kept, so they are never written.
type counts map[dayKey]int

type dayKey struct{ day, key string }

func (c counts) add(day, key string, n int) {
	if n == 0 {
		return
	}
	k := dayKey{day, key}
	if c[k] += n; c[k] == 0 {
		delete(c, k)
	}
}

// minus returns c − other: what turns other's counts into c's.
func (c counts) minus(other counts) counts {
	out := counts{}
	for k, n := range c {
		out.add(k.day, k.key, n)
	}
	for k, n := range other {
		out.add(k.day, k.key, -n)
	}
	return out
}

// DayOf is t's day in loc, as stats_days stores it.
func DayOf(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(time.DateOnly)
}

// between is how long from a to b as the saved rows tell it: both in whole
// milliseconds, so a live count and a rebuild bucket it the same.
func between(a, b time.Time) time.Duration {
	return time.Duration(b.UnixMilli()-a.UnixMilli()) * time.Millisecond
}

// gameFacts is what counting needs to know about a game. The store reads
// it from the row it writes (or, in a rebuild, from the saved game).
type gameFacts struct {
	Created  time.Time
	Kind     string // "quick" or "friend"
	Rematch  bool
	HasBlack bool
	Joined   time.Time // when Black sat down; zero when unknown
	Result   string    // "" while the game is on
	Winner   string    // "white", "black" or ""
	Moves    int       // turns saved: the last ply + 1
	WinnerMS int64     // the winner's clock after the last turn
	HasClock bool      // WinnerMS is known: there is a winner and a turn
	Stats    *GameStats
}

func (f gameFacts) friendLink() bool { return f.Kind == "friend" && !f.Rematch }

// The counting functions are pure: no I/O and no errors, and every input
// maps to a key, so counting can never be why a game's save fails. The
// live writes and the rebuild call the same ones, so they can't disagree.

// countCreated counts a new game.
func countCreated(c counts, loc *time.Location, f gameFacts) {
	day := DayOf(f.Created, loc)
	if f.Kind == "quick" {
		c.add(day, "games.quick", 1)
	} else {
		c.add(day, "games.friend", 1)
	}
	switch {
	case f.Kind == "friend" && f.Rematch:
		c.add(day, "friend.rematches", 1)
	case f.friendLink():
		c.add(day, "friend.links", 1)
	}
}

// countJoined counts Black taking the seat of a friend link.
func countJoined(c counts, loc *time.Location, f gameFacts) {
	if !f.friendLink() || !f.HasBlack {
		return
	}
	day := DayOf(f.Created, loc)
	c.add(day, "friend.joined", 1)
	if !f.Joined.IsZero() {
		c.add(day, "join."+joinBucket(between(f.Created, f.Joined)), 1)
	}
}

// endKey names how a game ended. A drawn result, including a flag fall
// against a side that couldn't mate, is a draw.
func endKey(f gameFacts) string {
	switch f.Result {
	case "checkmate":
		if f.Stats != nil && f.Stats.MateByRoll {
			return "roll"
		}
		return "checkmate"
	case "resignation", "timeout", "aborted", "expired":
		return f.Result
	case "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout_vs_insufficient":
		return "draw"
	}
	return "other"
}

// countEnded counts a game's end, with what the road did to it when its
// stats are known.
func countEnded(c counts, loc *time.Location, f gameFacts) {
	day := DayOf(f.Created, loc)
	c.add(day, "end."+endKey(f), 1)
	got := isFinished(f.Result)
	if got {
		c.add(day, "got_going", 1)
		c.add(day, "heat."+strconv.Itoa(f.Created.In(loc).Hour()/2), 1)
		if f.Moves > 0 { // a game resigned before its first move has no length
			c.add(day, "len."+lengthBucket(f.Moves), 1)
		}
		if f.Result == "timeout" {
			c.add(day, "clock.decided", 1)
		}
		if f.Winner != "" && f.HasClock {
			c.add(day, "left."+leftBucket(time.Duration(f.WinnerMS)*time.Millisecond), 1)
		}
		if f.friendLink() {
			c.add(day, "friend.finished", 1)
		}
	}
	st := f.Stats
	if st == nil {
		return
	}
	c.add(day, "road.games", 1)
	c.add(day, "road.opened", st.Opened)
	c.add(day, "road.reset", st.Reset)
	c.add(day, "road.closed_rounds", st.ClosedRounds)
	c.add(day, "road.closed_cap", st.ClosedCap)
	c.add(day, "road.repaired", st.Repaired)
	c.add(day, "road.fell", st.Fell)
	c.add(day, "road.saving_rolls", st.SavingRolls)
	c.add(day, "road.saved", st.Saved)
	if st.Fell == 0 {
		c.add(day, "road.no_fall", 1)
	}
	if st.MateByRoll {
		c.add(day, "road.mate_by_roll", 1)
	}
	for i, n := range st.OpenHist {
		c.add(day, "open."+strconv.Itoa(i), n)
	}
	if got && f.Winner != "" {
		won, lost := st.WhiteLost, st.BlackLost
		if f.Winner == "black" {
			won, lost = lost, won
		}
		switch {
		case won < lost:
			c.add(day, "decided.less", 1)
		case won > lost:
			c.add(day, "decided.more", 1)
		default:
			c.add(day, "decided.same", 1)
		}
	}
}

// countSearch counts a quick-match search on the day it started.
func countSearch(c counts, loc *time.Location, sr Search) {
	day := DayOf(sr.StartedAt, loc)
	c.add(day, "search", 1)
	if sr.Others == 0 {
		c.add(day, "search.alone", 1)
	}
	waited := waitBucket(between(sr.StartedAt, sr.EndedAt))
	if sr.Matched {
		c.add(day, "search.matched", 1)
		c.add(day, "wait."+waited, 1)
	} else {
		c.add(day, "search.gaveup", 1)
		c.add(day, "gaveup."+waited, 1)
	}
}

// countEvent counts n practice games, reopened streams or restarts (a row
// past the day's cap stands for more than one).
func countEvent(c counts, loc *time.Location, kind string, at time.Time, n int) {
	c.add(DayOf(at, loc), "event."+kind, n)
}

// countError counts n browser errors.
func countError(c counts, loc *time.Location, at time.Time, n int) {
	c.add(DayOf(at, loc), "errors", n)
}

// finished are the results of a game that was played to the end (not
// aborted, expired or retired); finishedReasons is the same list in SQL.
var (
	finished        = []string{"checkmate", "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout", "timeout_vs_insufficient", "resignation"}
	finishedReasons = "('" + strings.Join(finished, "','") + "')"
)

// isFinished reports a result of a game that got going: played to the
// end, not aborted, expired or retired.
func isFinished(reason string) bool { return slices.Contains(finished, reason) }

// bucket is one bar of a histogram: its key suffix, its label on the page
// and the largest value it holds.
type bucket struct {
	key, label string
	upTo       int64 // moves, or milliseconds; the last bucket has no limit
}

const noLimit = math.MaxInt64

var (
	movesBuckets = []bucket{{"1-10", "1–10", 10}, {"11-20", "11–20", 20}, {"21-30", "21–30", 30}, {"31-40", "31–40", 40},
		{"41-50", "41–50", 50}, {"51-60", "51–60", 60}, {"61-80", "61–80", 80}, {"81+", "81+", noLimit}}
	// Waits and gave-up times: a matched search, or how long a search
	// waited before the guest left.
	waitBuckets = []bucket{{"lt10s", "0–10s", 10_000 - 1}, {"10-30s", "10–30s", 30_000 - 1}, {"30-60s", "30–60s", 60_000 - 1},
		{"1-2m", "1–2 min", 120_000 - 1}, {"2m+", "2 min+", noLimit}}
	joinBuckets = []bucket{{"lt1m", "<1m", 60_000 - 1}, {"1-5m", "1–5m", 300_000 - 1}, {"5-30m", "5–30m", 1_800_000 - 1},
		{"30m-2h", "30m–2h", 7_200_000 - 1}, {"2h+", "2h+", noLimit}}
	leftBuckets = []bucket{{"lt30s", "<0:30", 30_000 - 1}, {"30s-1m", "0:30–1", 60_000 - 1}, {"1-2m", "1–2m", 120_000 - 1},
		{"2-5m", "2–5m", 300_000 - 1}, {"5m+", "5m+", noLimit}}
)

func bucketOf(bs []bucket, v int64) string {
	for _, b := range bs {
		if v <= b.upTo {
			return b.key
		}
	}
	return bs[len(bs)-1].key
}

func lengthBucket(moves int) string     { return bucketOf(movesBuckets, int64(moves)) }
func waitBucket(d time.Duration) string { return bucketOf(waitBuckets, d.Milliseconds()) }
func joinBucket(d time.Duration) string { return bucketOf(joinBuckets, d.Milliseconds()) }
func leftBucket(d time.Duration) string { return bucketOf(leftBuckets, d.Milliseconds()) }
