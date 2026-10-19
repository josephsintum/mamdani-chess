package store

import (
	"testing"
	"time"
)

func TestBucketEdges(t *testing.T) {
	for _, c := range []struct {
		got, want string
	}{
		{lengthBucket(10), "1-10"}, {lengthBucket(11), "11-20"}, {lengthBucket(80), "61-80"}, {lengthBucket(81), "81+"},
		{waitBucket(9999 * time.Millisecond), "lt10s"}, {waitBucket(10 * time.Second), "10-30s"}, {waitBucket(2 * time.Minute), "2m+"},
		{joinBucket(59 * time.Second), "lt1m"}, {joinBucket(5 * time.Minute), "5-30m"}, {joinBucket(3 * time.Hour), "2h+"},
		{leftBucket(29 * time.Second), "lt30s"}, {leftBucket(30 * time.Second), "30s-1m"}, {leftBucket(5 * time.Minute), "5m+"},
	} {
		if c.got != c.want {
			t.Errorf("bucket %q, want %q", c.got, c.want)
		}
	}
}

func TestEndKey(t *testing.T) {
	// A mate by a roll, a flag fall that can't lose and an expired link are
	// in TestAddGameStatsMovesTheCounters and TestCountedGameWrites.
	for _, c := range []struct {
		f    gameFacts
		want string
	}{
		{gameFacts{Result: "checkmate"}, "checkmate"},
		{gameFacts{Result: "fifty_moves"}, "draw"},
		{gameFacts{Result: "something new"}, "other"},
	} {
		if got := endKey(c.f); got != c.want {
			t.Errorf("endKey(%q) = %q, want %q", c.f.Result, got, c.want)
		}
	}
}

// A win on time counts as decided on the clock, with the winner's clock
// left. (A game without a winner is in TestCountedGameWrites, one without
// stats in TestAddGameStatsMovesTheCounters.)
func TestCountEnded(t *testing.T) {
	created := time.Date(2026, 10, 5, 21, 0, 0, 0, time.UTC)
	c := counts{}
	countEnded(c, time.UTC, gameFacts{Created: created, Kind: "friend", Result: "timeout", Winner: "black", Moves: 44, HasClock: true, WinnerMS: 20_000,
		Stats: &GameStats{Moves: 44, Fell: 1, WhiteLost: 1, BlackLost: 0, OpenHist: [6]int{0, 3}}})
	for _, k := range []string{"end.timeout", "clock.decided", "left.lt30s", "len.41-50", "friend.finished", "road.games", "decided.less"} {
		if c[dayKey{"2026-10-05", k}] != 1 {
			t.Errorf("%s = %d, want 1 (all: %v)", k, c[dayKey{"2026-10-05", k}], c)
		}
	}
	if c[dayKey{"2026-10-05", "open.1"}] != 3 || c[dayKey{"2026-10-05", "road.no_fall"}] != 0 {
		t.Errorf("open holes and falls: %v", c)
	}
}
