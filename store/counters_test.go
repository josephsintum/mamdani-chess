package store

import (
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"reflect"
	"strconv"
	"testing"
	"time"
)

func openTempIn(t *testing.T, zone *time.Location) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := OpenIn(path, zone)
	if err != nil {
		t.Fatalf("OpenIn: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// counters reads every counter as "day key" → n, leaving out zeros (an
// amount moved away by a later write).
func counters(t *testing.T, s *Store) map[string]int {
	t.Helper()
	rows, err := s.db.QueryContext(t.Context(), `SELECT day, key, n FROM stats_days`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var day, key string
		var n int
		if err := rows.Scan(&day, &key, &n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			out[day+" "+key] = n
		}
	}
	return out
}

// keysOn prefixes each key with day, for comparing with counters.
func keysOn(day string, kv map[string]int) map[string]int {
	out := map[string]int{}
	for k, n := range kv {
		out[day+" "+k] = n
	}
	return out
}

func TestCountedGameWrites(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	day := dayOf(t0, time.UTC)
	heat := "heat." + strconv.Itoa(t0.UTC().Hour()/2)

	must(t, s.CreateGame(ctx, Game{Code: "LINK01", White: "w", CreatedAt: t0}))                                  // a friend link
	must(t, s.CreateGame(ctx, Game{Code: "QUICK1", White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))       // quick match
	must(t, s.CreateGame(ctx, Game{Code: "REMAT1", White: "b", Black: "w", RematchOf: "LINK01", CreatedAt: t0})) // a rematch of a friend game
	must(t, s.CreateGame(ctx, Game{Code: "PLAYT1", White: "w", CreatedAt: t0, Playtest: true}))                  // a playtest: counts nothing
	want := map[string]int{"games.friend": 2, "games.quick": 1, "friend.links": 1, "friend.rematches": 1}
	if got := counters(t, s); !reflect.DeepEqual(got, keysOn(day, want)) {
		t.Fatalf("after creating: %v", got)
	}

	must(t, s.SeatBlack(ctx, "LINK01", "b", "", t0.Add(3*time.Minute)))
	must(t, s.SeatBlack(ctx, "LINK01", "c", "", t0.Add(4*time.Minute))) // taken: no change, no error
	must(t, s.SeatBlack(ctx, "PLAYT1", "b", "", t0.Add(time.Minute)))
	want["friend.joined"], want["join.1-5m"] = 1, 1
	if got := counters(t, s); !reflect.DeepEqual(got, keysOn(day, want)) {
		t.Fatalf("after joining: %v", got)
	}

	// A resignation has no final turn: the winner's clock is the last saved one.
	must(t, s.AddTurn(ctx, "LINK01", Turn{Ply: 0, Move: "e2e4", WhiteMS: 590_000, BlackMS: 600_000, At: t0.Add(4 * time.Minute)}))
	must(t, s.AddTurn(ctx, "LINK01", Turn{Ply: 1, Move: "e7e5", WhiteMS: 590_000, BlackMS: 45_000, At: t0.Add(5 * time.Minute)}))
	st := &GameStats{Moves: 2, Opened: 1, WhiteLost: 1, OpenHist: [6]int{1, 1}}
	end := Result{EndedAt: t0.Add(6 * time.Minute), Reason: "resignation", Winner: "black"}
	must(t, s.EndGame(ctx, "LINK01", end, nil, st))
	must(t, s.EndGame(ctx, "LINK01", end, nil, st)) // already ended: no change, no error
	for k, n := range map[string]int{"end.resignation": 1, "got_going": 1, heat: 1, "len.1-10": 1, "left.30s-1m": 1, "friend.finished": 1,
		"road.games": 1, "road.opened": 1, "road.no_fall": 1, "open.0": 1, "open.1": 1, "decided.less": 1} {
		want[k] += n
	}
	// A flag fall against a side that can't mate is a draw: no clock.decided, no winner's clock.
	must(t, s.EndGame(ctx, "QUICK1", Result{EndedAt: t0.Add(time.Hour), Reason: "timeout_vs_insufficient"}, nil, &GameStats{Moves: 90, Fell: 2}))
	for k, n := range map[string]int{"end.draw": 1, "got_going": 1, heat: 1, "len.81+": 1, "road.games": 1, "road.fell": 2} {
		want[k] += n
	}
	must(t, s.EndGame(ctx, "REMAT1", Result{EndedAt: t0.Add(time.Hour), Reason: "aborted"}, nil, nil))
	must(t, s.EndGame(ctx, "PLAYT1", Result{EndedAt: t0.Add(time.Hour), Reason: "aborted"}, nil, nil))
	want["end.aborted"]++
	// A link nobody opened expires; a playtest's expires uncounted.
	must(t, s.CreateGame(ctx, Game{Code: "WAIT01", White: "w", CreatedAt: t0}))
	must(t, s.CreateGame(ctx, Game{Code: "WAIT02", White: "w", CreatedAt: t0, Playtest: true}))
	must1(s.ExpireWaiting(ctx, t0.Add(time.Hour), t0.Add(25*time.Hour)))
	want["games.friend"]++
	want["friend.links"]++
	want["end.expired"]++
	if got := counters(t, s); !reflect.DeepEqual(got, keysOn(day, want)) {
		t.Fatalf("after ending:\n got %v\nwant %v", got, keysOn(day, want))
	}
}

// The backfill moves a game from "ended, road unknown" to what its stats
// say, and a second backfill of the same game counts nothing.
func TestAddGameStatsMovesTheCounters(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	day := dayOf(t0, time.UTC)
	must(t, s.CreateGame(ctx, Game{Code: "OLD001", White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))
	must(t, s.EndGame(ctx, "OLD001", Result{EndedAt: t0.Add(time.Hour), Reason: "checkmate", Winner: "white"}, nil, nil))
	st := GameStats{Moves: 31, Opened: 4, Fell: 1, BlackLost: 1, MateByRoll: true}
	must(t, s.AddGameStats(ctx, "OLD001", st))
	must(t, s.AddGameStats(ctx, "OLD001", st))
	want := keysOn(day, map[string]int{"games.quick": 1, "end.roll": 1, "got_going": 1, "heat." + strconv.Itoa(t0.UTC().Hour()/2): 1, "len.31-40": 1,
		"road.games": 1, "road.opened": 4, "road.fell": 1, "road.mate_by_roll": 1, "decided.less": 1})
	if got := counters(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("\n got %v\nwant %v", got, want)
	}
}

func TestSearchesAndEventsCount(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	day := dayOf(t0, time.UTC)
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(12 * time.Second), Matched: true, Others: 1}))
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(25 * time.Second)}))
	must(t, s.AddEvent(ctx, "practice", t0))
	must(t, s.AddEvent(ctx, "restart", t0))
	want := keysOn(day, map[string]int{"search": 2, "search.matched": 1, "wait.10-30s": 1, "search.gaveup": 1, "gaveup.10-30s": 1,
		"search.alone": 1, "event.practice": 1, "event.restart": 1})
	if got := counters(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("\n got %v\nwant %v", got, want)
	}
}

func TestEnsureCountersRebuildsOnlyWhenNeeded(t *testing.T) {
	ctx := t.Context()
	s, path := openTempIn(t, newYork)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, newYork)
	// A game counts on the day it was created, in the store's zone, however
	// late it ends: 23:30 in New York is 03:30 UTC on Oct 4.
	created := time.Date(2026, 10, 3, 23, 30, 0, 0, newYork)
	must(t, s.CreateGame(ctx, Game{Code: "GAME01", White: "w", Black: "b", Kind: "quick", CreatedAt: created}))
	must(t, s.EndGame(ctx, "GAME01", Result{EndedAt: created.Add(90 * time.Minute), Reason: "aborted"}, nil, nil))
	// An errors count from before the cutoff day has no rows left to recount, so it stays.
	must1(s.db.ExecContext(ctx, `INSERT INTO stats_days (day, key, n) VALUES ('2024-01-01', 'errors', 7), ('2024-01-01', 'games.quick', 3)`))

	rep, err := s.EnsureCounters(ctx, now) // a new database: stats_state is empty
	must(t, err)
	if rep.Rows != 3 || s.CountersStale() { // the game's two, and the kept 2024 errors count
		t.Fatalf("first start: %+v, stale %v", rep, s.CountersStale())
	}
	want := map[string]int{"2026-10-03 games.quick": 1, "2026-10-03 end.aborted": 1, "2024-01-01 errors": 7}
	if got := counters(t, s); !reflect.DeepEqual(got, want) {
		t.Fatalf("after the first rebuild %v, want %v", got, want)
	}
	if rep, err = s.EnsureCounters(ctx, now); err != nil || rep != (RebuildReport{}) {
		t.Fatalf("second start rebuilt: %+v, %v", rep, err)
	}
	s.Close()

	// Another STATS_TZ cuts the days differently: rebuild.
	s, err = OpenIn(path, time.UTC)
	must(t, err)
	defer s.Close()
	if rep, err = s.EnsureCounters(ctx, now); err != nil || rep.Rows != 3 {
		t.Fatalf("zone change: %+v, %v", rep, err)
	}
	if got := counters(t, s); got["2026-10-04 games.quick"] != 1 {
		t.Fatalf("after the zone change %v", got)
	}
}

// A rebuild that fails keeps the old counters and marks them stale.
func TestFailedRebuildMarksCountersStale(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.CreateGame(ctx, Game{Code: "GAME01", White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))
	must1(s.db.ExecContext(ctx, `INSERT INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
		VALUES ('GAME01', 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 'not a histogram')`))
	before := counters(t, s)
	if _, err := s.EnsureCounters(ctx, t0); err == nil || !s.CountersStale() {
		t.Fatalf("err %v, stale %v; want an error and stale", err, s.CountersStale())
	}
	if after := counters(t, s); !reflect.DeepEqual(after, before) {
		t.Fatalf("a failed rebuild changed the counters: %v → %v", before, after)
	}
}

// Counting as the data is written must give exactly the counters a
// rebuild from the raw rows gives: across a daylight-saving change, games
// that end or are joined the next day, aborted, expired and retired games,
// rematches, playtests, backfilled stats, searches, events and errors.
func TestLiveCountingEqualsRebuild(t *testing.T) {
	ctx := t.Context()
	s, _ := openTempIn(t, newYork)
	rng := rand.New(rand.NewPCG(1, 2))
	start := time.Date(2026, 10, 28, 0, 0, 0, 0, newYork) // New York leaves daylight saving on Nov 1
	at := func() time.Time { return start.Add(time.Duration(rng.Int64N(int64(10 * 24 * time.Hour)))) }
	results := []string{"checkmate", "resignation", "timeout", "stalemate", "timeout_vs_insufficient", "aborted", "repetition"}
	var friends []string
	for i := range 300 {
		code := fmt.Sprintf("G%05d", i)
		g := Game{Code: code, White: "w", CreatedAt: at(), Playtest: rng.IntN(10) == 0}
		switch rng.IntN(3) {
		case 0:
			g.Kind, g.Black = "quick", "b"
		case 1:
			if len(friends) > 0 {
				g.RematchOf, g.Black = friends[rng.IntN(len(friends))], "b"
			}
		}
		must(t, s.CreateGame(ctx, g))
		if g.Kind != "quick" && g.RematchOf == "" {
			friends = append(friends, code)
		}
		if g.Black == "" {
			if rng.IntN(4) == 0 {
				continue // left waiting: expired below
			}
			must(t, s.SeatBlack(ctx, code, "b", "", g.CreatedAt.Add(time.Duration(rng.Int64N(int64(30*time.Hour))))))
		}
		turns := rng.IntN(4)
		for ply := range turns {
			must(t, s.AddTurn(ctx, code, Turn{Ply: ply, Move: "e2e4", WhiteMS: rng.Int64N(600_000), BlackMS: rng.Int64N(600_000), At: g.CreatedAt}))
		}
		r := Result{EndedAt: g.CreatedAt.Add(time.Duration(rng.Int64N(int64(5 * time.Hour)))), Reason: results[rng.IntN(len(results))]}
		if r.Reason != "aborted" && r.Reason != "stalemate" && r.Reason != "repetition" && r.Reason != "timeout_vs_insufficient" {
			r.Winner = []string{"white", "black"}[rng.IntN(2)]
		}
		var st *GameStats
		if isFinished(r.Reason) {
			st = &GameStats{Moves: rng.IntN(120), Opened: rng.IntN(20), Fell: rng.IntN(5), WhiteLost: rng.IntN(3), BlackLost: rng.IntN(3),
				SavingRolls: rng.IntN(4), MateByRoll: rng.IntN(5) == 0, OpenHist: [6]int{rng.IntN(9), rng.IntN(9), rng.IntN(9), 0, rng.IntN(3), 1}}
		}
		backfilled := st != nil && rng.IntN(5) == 0 // ended before stats were kept
		if backfilled {
			must(t, s.EndGame(ctx, code, r, nil, nil))
			must(t, s.AddGameStats(ctx, code, *st))
		} else {
			must(t, s.EndGame(ctx, code, r, nil, st))
		}
	}
	_, err := s.ExpireWaiting(ctx, start.AddDate(0, 0, 11), start.AddDate(0, 0, 12))
	must(t, err)
	// A game a migration retired was saved before the counters: it counts
	// nowhere, neither made nor ended.
	must1(s.db.ExecContext(ctx, `INSERT INTO games (code, white, black, kind, created_at, ended_at, result)
		VALUES ('RETIRE', 'w', 'b', 'quick', ?, ?, 'retired')`, at().UnixMilli(), at().UnixMilli()))
	// Times are saved in whole milliseconds, so a wait or a join a fraction
	// of a millisecond short of a bucket's edge must land where the saved
	// times put it.
	edge := start.Add(800 * time.Microsecond)
	must(t, s.AddSearch(ctx, Search{StartedAt: edge, EndedAt: edge.Add(10*time.Second - 500*time.Microsecond), Matched: true}))
	must(t, s.CreateGame(ctx, Game{Code: "EDGE01", White: "w", CreatedAt: edge}))
	must(t, s.SeatBlack(ctx, "EDGE01", "b", "", edge.Add(time.Minute-500*time.Microsecond)))
	for range 100 {
		begin := at()
		must(t, s.AddSearch(ctx, Search{StartedAt: begin, EndedAt: begin.Add(time.Duration(rng.Int64N(int64(3 * time.Minute)))), Matched: rng.IntN(2) == 0, Others: rng.IntN(3)}))
	}
	for range 50 {
		must(t, s.AddEvent(ctx, []string{"practice", "reconnect", "restart"}[rng.IntN(3)], at()))
		must(t, s.AddBrowserError(ctx, BrowserError{At: at(), Message: "x"}))
	}
	live := counters(t, s)
	if len(live) < 500 {
		t.Fatalf("only %d counters: the scenario counts too little to compare", len(live))
	}

	if _, err := s.rebuild(ctx, start.AddDate(0, 0, 12)); err != nil {
		t.Fatal(err)
	}
	if rebuilt := counters(t, s); !reflect.DeepEqual(live, rebuilt) {
		for k := range mergeKeys(live, rebuilt) {
			if live[k] != rebuilt[k] {
				t.Errorf("%s: live %d, rebuilt %d", k, live[k], rebuilt[k])
			}
		}
	}
}

func mergeKeys(a, b map[string]int) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		out[k] = true
	}
	for k := range b {
		out[k] = true
	}
	return out
}
