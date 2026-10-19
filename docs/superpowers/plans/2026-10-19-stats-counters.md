# Stats counters Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `/stats` reads daily counters written with the data, so a page load never scans games, turns or searches, and nothing recomputes history while people play.

**Architecture:** A `stats_days` table holds an amount per day (in `STATS_TZ`) and key. Each store write that the page counts (creating, joining, ending and expiring games; the game-stats backfill; searches; events; browser errors) adds its keys in the transaction it already has, through pure counting functions. The same functions rebuild the table from the raw rows at startup when the counters' version or zone changed; startup now finishes the stats work before restoring games. The page sums one read of counters into its sections; medians become histograms with a threshold line.

**Tech Stack:** Go 1.27, SQLite through `modernc.org/sqlite` v1.60.1 (WAL, one connection), `html/template`. No new dependencies.

**Spec:** `docs/superpowers/specs/2026-10-19-stats-counters-design.md` (builds on `docs/superpowers/specs/2026-10-08-stats-design.md`).

**Branch:** `stats-counters`, stacked on `stats-games` (PR #25). The worktree is `/Users/josephsintum/code/mamdani-chess/.claude/worktrees/stats-games`; run every command from it.

**How this plan was made:** the code was built in a scratch copy, tested, then replayed task by task on a fresh copy of `stats-games`, with the whole Go suite passing after each task. The diffs below are those replays. Each task's diffs apply on top of the task before (`git apply` from the repo root, or edit by hand).

## Global Constraints

- The whole page stays private: `GET /stats` behind Basic Auth with `STATS_PASSWORD`, as now.
- Counters at write time, inside the same transaction as the write they count. A page load only sums counters; it never reads `games`, `turns`, `game_stats` or `searches`.
- Daily buckets, `day` as `YYYY-MM-DD` in `STATS_TZ`; a game's keys go on the day it was created.
- No medians: each becomes a histogram with one share line at a threshold.
- The counting functions are pure: no I/O, no error returns; an unknown result goes to `end.other`.
- Playtests write no counters.
- A guarded update that changes no row counts nothing and returns nil, unless the game was never saved (that stays an error).
- One multi-row upsert per write; zero amounts are never written.
- Startup order: migrations, prune browser errors (at a `STATS_TZ` midnight), `ExpireWaiting`, the `game_stats` backfill, the rebuild if needed, the `restart` event, then `LoadForRestore` and `hub.Restore`, then listen.
- A rebuild of 100,000 games must take under 3 seconds on Railway (it measured 0.46 s on an M4 Pro).
- Repo rules (CLAUDE.md): no Claude attribution in commits; `git add <paths>` only, never `-A`; don't touch `go.mod` or `names/`; stop test servers by PID, never `pkill -f server`.

## Review Focus

- **A game ended or joined twice** (a retried save, a race, a restored game): the second call counts nothing and returns nil. Pinned in Task 2 (`TestCountedGameWrites`: repeated `EndGame` and `SeatBlack`), and an unknown game still errors (`TestEndGameFailsForAnUnknownGame`, `TestSeatBlackFailsForAnUnknownGame`).
- **Days around a clock change, and games that end or are joined after midnight:** each counts on its creation day in `STATS_TZ`. Pinned in Task 2 (`TestGameCountsOnItsCreationDay`, and `TestLiveCountingEqualsRebuild` across New York's Nov 1 change) and Task 4 (`TestDaysFrom`).
- **A deploy over a long history:** the rebuild streams and finishes before any game is restored. Pinned in Task 3 (`TestStartReadiesStatsBeforeRestoring`) and measured in Task 4 (`BenchmarkRebuild`).
- **A flood of error reports** (no per-IP limits yet): past 200 a day the messages stop being stored and the count goes on. Pinned in Task 2 (`TestBrowserErrorsCappedPerDay`).
- **`STATS_TZ` changed between deploys, or a rebuild that fails:** a zone change rebuilds; a failed rebuild keeps the old counters and the page says so. Pinned in Task 1 (`TestEnsureCountersRebuildsOnlyWhenNeeded`, `TestFailedRebuildMarksCountersStale`) and Task 4 (`TestStatsSaysWhenCountersAreStale`).

## Files

| File | Responsibility |
| --- | --- |
| `store/counts.go` (new) | The counters version; `counts` (day, key → amount); `gameFacts`; the pure counting functions `countCreated`, `countJoined`, `countEnded`, `countSearch`, `countEvent`, `countError`; `endKey`, `isFinished` (moved from `gamesection.go`); the histogram buckets. |
| `store/counters.go` (new) | Writing (`addCounts`) and reading (`Counts`) the counters; `PruneErrors`; `EnsureCounters` and the streaming rebuild; `CountersStale`, `Zone`. |
| `store/summary.go` (new) | `Summary` and `Summarize`: the page's sections from summed counters. Replaces `store/gamesection.go`. |
| `store/store.go` | The store's zone (`OpenIn`) and stale flag. |
| `store/migrate.go` | Migration 8 gains `stats_days` and `stats_state`. |
| `store/games.go` | `CreateGame`, `SeatBlack`, `EndGame`, `ExpireWaiting` count; guarded updates with `RETURNING`; `noChange`, `lastClock`. |
| `store/gamestats.go` | `AddGameStats` moves a game's counters; `AddSearch`, `AddEvent` count. |
| `store/errors.go` | `AddBrowserError` counts and caps raw rows; `LatestError`. |
| `cmd/server/main.go`, `backfill.go` | `STATS_TZ` first; the new startup order; `prepareStats`. |
| `server/stats.go`, `stats.html`, `server.go` | The page from counters; new histograms; the stale line. |

## Interfaces (all in package `store` unless named)

- `func OpenIn(path string, zone *time.Location) (*Store, error)`; `Open(path)` is `OpenIn(path, time.UTC)`.
- `func (s *Store) Zone() *time.Location`; `func (s *Store) CountersStale() bool`.
- `type DayCounts struct { Day string; N map[string]int }`; `func (s *Store) Counts(ctx, fromDay, toDay string) ([]DayCounts, error)` (empty `fromDay`: no lower bound).
- `type RebuildReport struct { Days, Rows int; Took time.Duration }`; `func (s *Store) EnsureCounters(ctx, now time.Time) (RebuildReport, error)`.
- `func (s *Store) PruneErrors(ctx, now time.Time) (int64, error)`; `func (s *Store) LatestError(ctx, from, to time.Time) (string, error)`.
- `type Summary struct {…}` and `func Summarize(days []DayCounts, fromDay, toDay string) Summary`, with `KindDay`, `Heat`, `Count` as before.
- Package `main`: `func start(ctx, st *store.Store, hub *game.Hub, now time.Time) error` (replaces `restore`); `func prepareStats(ctx, st *store.Store, now time.Time)` (replaces `startStats`).

---

### Task 1: The counter table, the counting functions and the rebuild

**Files:**
- Create: `store/counts.go`, `store/counters.go`, `store/counts_test.go`, `store/counters_test.go`
- Modify: `store/store.go`, `store/migrate.go` (migration 8), `store/gamesection.go` (`isFinished` moves out)

**Interfaces:**
- Consumes: `GameStats`, `Search`, `parseHist`, `execer` (existing in `store`).
- Produces: `counts`, `gameFacts`, `countCreated/countJoined/countEnded/countSearch/countEvent/countError`, `addCounts`, `dayOf`, `winnerClock`, `OpenIn`, `Zone`, `Counts`, `DayCounts`, `PruneErrors`, `EnsureCounters`, `RebuildReport`, `CountersStale`, the bucket tables (`movesBuckets`, `waitBuckets`, `joinBuckets`, `leftBuckets`) and `bucket`.

- [ ] **Step 1: Write the failing tests**

Create `store/counters_test.go`:

```go
package store

import (
	"path/filepath"
	"reflect"
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

func TestEnsureCountersRebuildsOnlyWhenNeeded(t *testing.T) {
	ctx := t.Context()
	s, path := openTempIn(t, newYork)
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, newYork)
	created := time.Date(2026, 10, 3, 23, 30, 0, 0, newYork)
	must(t, s.CreateGame(ctx, Game{Code: "GAME01", White: "w", Black: "b", Kind: "quick", CreatedAt: created}))
	// An errors count from before the cutoff day has no rows left to recount, so it stays.
	must1(s.db.ExecContext(ctx, `INSERT INTO stats_days (day, key, n) VALUES ('2024-01-01', 'errors', 7), ('2024-01-01', 'games.quick', 3)`))

	rep, err := s.EnsureCounters(ctx, now) // a new database: stats_state is empty
	must(t, err)
	if rep.Rows != 1 || s.CountersStale() {
		t.Fatalf("first start: %+v, stale %v", rep, s.CountersStale())
	}
	want := map[string]int{"2026-10-03 games.quick": 1, "2024-01-01 errors": 7}
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
	if rep, err = s.EnsureCounters(ctx, now); err != nil || rep.Rows != 1 {
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
```

Create `store/counts_test.go`:

```go
package store

import (
	"reflect"
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
	roll := &GameStats{MateByRoll: true}
	for _, c := range []struct {
		f    gameFacts
		want string
	}{
		{gameFacts{Result: "checkmate"}, "checkmate"},
		{gameFacts{Result: "checkmate", Stats: roll}, "roll"},
		{gameFacts{Result: "timeout"}, "timeout"},
		{gameFacts{Result: "timeout_vs_insufficient"}, "draw"}, // a flag fall that can't lose is a draw
		{gameFacts{Result: "fifty_moves"}, "draw"},
		{gameFacts{Result: "expired"}, "expired"},
		{gameFacts{Result: "something new"}, "other"},
	} {
		if got := endKey(c.f); got != c.want {
			t.Errorf("endKey(%q) = %q, want %q", c.f.Result, got, c.want)
		}
	}
}

// Zero amounts are never kept, and minus gives what turns one count into
// another.
func TestCountsDropZeros(t *testing.T) {
	c := counts{}
	c.add("2026-10-01", "road.fell", 0)
	c.add("2026-10-01", "games.quick", 1)
	c.add("2026-10-01", "games.quick", -1)
	if len(c) != 0 {
		t.Fatalf("kept %v", c)
	}
	before, after := counts{}, counts{}
	before.add("d", "end.checkmate", 1)
	after.add("d", "end.roll", 1)
	after.add("d", "road.games", 1)
	want := counts{{"d", "end.checkmate"}: -1, {"d", "end.roll"}: 1, {"d", "road.games"}: 1}
	if got := after.minus(before); !reflect.DeepEqual(got, want) {
		t.Fatalf("minus = %v, want %v", got, want)
	}
}

// A game's ending counts the road only when its stats are known, and the
// winner's clock only when there is a winner and a turn.
func TestCountEnded(t *testing.T) {
	created := time.Date(2026, 10, 5, 21, 0, 0, 0, time.UTC)
	c := counts{}
	countEnded(c, time.UTC, gameFacts{Created: created, Kind: "quick", Result: "resignation", Winner: "white"})
	want := counts{{"2026-10-05", "end.resignation"}: 1, {"2026-10-05", "got_going"}: 1, {"2026-10-05", "heat.10"}: 1}
	if !reflect.DeepEqual(c, want) {
		t.Fatalf("no stats, no clock: %v", c)
	}
	c = counts{}
	countEnded(c, time.UTC, gameFacts{Created: created, Kind: "friend", Result: "timeout", Winner: "black", HasClock: true, WinnerMS: 20_000,
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
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -short ./store -run "TestBucketEdges|TestEndKey|TestCountsDropZeros|TestCountEnded|TestEnsureCounters|TestFailedRebuild"`
Expected: FAIL: build errors (`undefined: lengthBucket`, `undefined: OpenIn`, …).

- [ ] **Step 3: Write the code**

Create `store/counters.go`:

```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// upsertBatch is how many counter rows one statement writes (3 variables
// each, well under SQLite's limit).
const upsertBatch = 300

// addCounts adds c to the daily counters: one multi-row upsert, in the
// caller's transaction.
func addCounts(ctx context.Context, db execer, c counts) error {
	rows := make([]dayKey, 0, len(c))
	for k := range c {
		rows = append(rows, k)
	}
	// A fixed order keeps the statement's page writes in key order.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].day != rows[j].day {
			return rows[i].day < rows[j].day
		}
		return rows[i].key < rows[j].key
	})
	for len(rows) > 0 {
		n := min(len(rows), upsertBatch)
		args := make([]any, 0, 3*n)
		for _, k := range rows[:n] {
			args = append(args, k.day, k.key, c[k])
		}
		q := `INSERT INTO stats_days (day, key, n) VALUES ` + strings.TrimSuffix(strings.Repeat("(?, ?, ?), ", n), ", ") +
			` ON CONFLICT (day, key) DO UPDATE SET n = n + excluded.n`
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			return err
		}
		rows = rows[n:]
	}
	return nil
}

// DayCounts is one day's counters, key → amount.
type DayCounts struct {
	Day string // 2006-01-02, in the store's zone
	N   map[string]int
}

// Counts returns the counters for the days with fromDay <= day < toDay,
// in day order; an empty fromDay has no lower bound. Days with no counters
// are missing.
func (s *Store) Counts(ctx context.Context, fromDay, toDay string) ([]DayCounts, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT day, key, n FROM stats_days WHERE day >= ? AND day < ? ORDER BY day, key`, fromDay, toDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayCounts
	for rows.Next() {
		var day, key string
		var n int
		if err := rows.Scan(&day, &key, &n); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Day != day {
			out = append(out, DayCounts{Day: day, N: map[string]int{}})
		}
		out[len(out)-1].N[key] = n
	}
	return out, rows.Err()
}

// CountersStale reports that the counters were written under another
// version or zone and the rebuild that should have fixed them failed.
func (s *Store) CountersStale() bool { return s.stale.Load() }

// Zone is the zone the counters cut days in.
func (s *Store) Zone() *time.Location { return s.zone }

// errorsKeep is how long browser errors' raw rows are kept: about thirteen
// months. Their daily counts are kept for good.
const errorsKeep = 400

// errorCutoff is the first day whose browser errors are all still kept:
// a midnight in the store's zone, so no day is ever half pruned.
func (s *Store) errorCutoff(now time.Time) time.Time {
	y, m, d := now.In(s.zone).AddDate(0, 0, -errorsKeep).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, s.zone)
}

// PruneErrors deletes the browser errors from before the cutoff day and
// returns how many went.
func (s *Store) PruneErrors(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM browser_errors WHERE at < ?`, s.errorCutoff(now).UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RebuildReport is what a rebuild did, for the log.
type RebuildReport struct {
	Days, Rows int
	Took       time.Duration
}

// EnsureCounters rebuilds the counters when they were written under
// another version or zone, or never (a new database, or the first start
// with counters). It returns nil and a zero report when nothing was
// needed. On an error the old counters stay and CountersStale reports it.
// Run it before anything else uses the store: it holds the one connection.
func (s *Store) EnsureCounters(ctx context.Context, now time.Time) (RebuildReport, error) {
	var version int
	var zone string
	err := s.db.QueryRowContext(ctx, `SELECT version, zone FROM stats_state WHERE id = 1`).Scan(&version, &zone)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.stale.Store(true)
		return RebuildReport{}, err
	}
	if err == nil && version == countersVersion && zone == s.zone.String() {
		s.stale.Store(false)
		return RebuildReport{}, nil
	}
	start := time.Now()
	rep, err := s.rebuild(ctx, now)
	if err != nil {
		s.stale.Store(true)
		return RebuildReport{}, fmt.Errorf("rebuild counters: %w", err)
	}
	s.stale.Store(false)
	rep.Took = time.Since(start)
	return rep, nil
}

// rebuild recounts every raw row through the counting functions: each
// table is read once, summed in memory, and written in one transaction.
// Errors' counts before the cutoff day are kept, since their rows are gone.
func (s *Store) rebuild(ctx context.Context, now time.Time) (RebuildReport, error) {
	c := counts{}
	if err := s.recountGames(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("games: %w", err)
	}
	if err := s.recountSearches(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("searches: %w", err)
	}
	if err := s.recountEvents(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("events: %w", err)
	}
	cutoff := s.errorCutoff(now)
	if err := s.recountErrors(ctx, c, cutoff); err != nil {
		return RebuildReport{}, fmt.Errorf("errors: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RebuildReport{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM stats_days WHERE NOT (key = 'errors' AND day < ?)`, dayOf(cutoff, s.zone)); err != nil {
		return RebuildReport{}, err
	}
	if err := addCounts(ctx, tx, c); err != nil {
		return RebuildReport{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stats_state (id, version, zone) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET version = excluded.version, zone = excluded.zone`, countersVersion, s.zone.String()); err != nil {
		return RebuildReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return RebuildReport{}, err
	}
	days := map[string]bool{}
	for k := range c {
		days[k.day] = true
	}
	return RebuildReport{Days: len(days), Rows: len(c)}, nil
}

// recountGames streams every game with its stats row and its last turn's
// clocks (two primary-key seeks), never its turns.
func (s *Store) recountGames(ctx context.Context, c counts) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.created_at, g.kind, g.rematch_of IS NOT NULL, g.black IS NOT NULL, g.joined_at, g.result, g.winner,
		       (SELECT white_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       (SELECT black_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       s.moves, s.opened, s.reset, s.closed_rounds, s.closed_cap, s.repaired, s.fell, s.saving_rolls, s.saved, s.white_lost, s.black_lost, s.mate_by_roll, s.open_hist
		FROM games g LEFT JOIN game_stats s ON s.game = g.code
		WHERE g.playtest = 0`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f gameFacts
		var created int64
		var joined, whiteMS, blackMS sql.NullInt64
		var result, winner, hist sql.NullString
		var st [11]sql.NullInt64
		var mate sql.NullBool
		if err := rows.Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &joined, &result, &winner, &whiteMS, &blackMS,
			&st[0], &st[1], &st[2], &st[3], &st[4], &st[5], &st[6], &st[7], &st[8], &st[9], &st[10], &mate, &hist); err != nil {
			return err
		}
		f.Created, f.Result, f.Winner = time.UnixMilli(created), result.String, winner.String
		if joined.Valid {
			f.Joined = time.UnixMilli(joined.Int64)
		}
		f.WinnerMS, f.HasClock = winnerClock(f.Winner, whiteMS, blackMS)
		if st[0].Valid {
			h, err := parseHist(hist.String)
			if err != nil {
				return err
			}
			n := func(i int) int { return int(st[i].Int64) }
			f.Stats = &GameStats{Moves: n(0), Opened: n(1), Reset: n(2), ClosedRounds: n(3), ClosedCap: n(4), Repaired: n(5),
				Fell: n(6), SavingRolls: n(7), Saved: n(8), WhiteLost: n(9), BlackLost: n(10), MateByRoll: mate.Bool, OpenHist: h}
		}
		countCreated(c, s.zone, f)
		countJoined(c, s.zone, f)
		// Retired games were ended by a migration, which counts nothing.
		if f.Result != "" && f.Result != "retired" {
			countEnded(c, s.zone, f)
		}
	}
	return rows.Err()
}

// winnerClock is the winner's clock from the last turn's two clocks.
func winnerClock(winner string, whiteMS, blackMS sql.NullInt64) (int64, bool) {
	switch {
	case winner == "white" && whiteMS.Valid:
		return whiteMS.Int64, true
	case winner == "black" && blackMS.Valid:
		return blackMS.Int64, true
	}
	return 0, false
}

func (s *Store) recountSearches(ctx context.Context, c counts) error {
	rows, err := s.db.QueryContext(ctx, `SELECT started_at, ended_at, matched, others FROM searches`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sr Search
		var started, ended int64
		if err := rows.Scan(&started, &ended, &sr.Matched, &sr.Others); err != nil {
			return err
		}
		sr.StartedAt, sr.EndedAt = time.UnixMilli(started), time.UnixMilli(ended)
		countSearch(c, s.zone, sr)
	}
	return rows.Err()
}

func (s *Store) recountEvents(ctx context.Context, c counts) error {
	rows, err := s.db.QueryContext(ctx, `SELECT at, kind FROM events`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var at int64
		var kind string
		if err := rows.Scan(&at, &kind); err != nil {
			return err
		}
		countEvent(c, s.zone, kind, time.UnixMilli(at))
	}
	return rows.Err()
}

// recountErrors counts the browser errors from the cutoff day on; the
// counts before it are kept as they are.
func (s *Store) recountErrors(ctx context.Context, c counts, cutoff time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT at FROM browser_errors WHERE at >= ?`, cutoff.UnixMilli())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var at int64
		if err := rows.Scan(&at); err != nil {
			return err
		}
		countError(c, s.zone, time.UnixMilli(at))
	}
	return rows.Err()
}
```

Create `store/counts.go`:

```go
package store

import (
	"strconv"
	"time"
)

// countersVersion is what the stats_days keys mean. Bump it whenever a
// key's meaning changes (a new bucket edge, a new rule for what counts):
// the next start rebuilds every counter from the raw rows.
const countersVersion = 1

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

// dayOf is t's day in loc, as stats_days stores it.
func dayOf(t time.Time, loc *time.Location) string {
	return t.In(loc).Format("2006-01-02")
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
	day := dayOf(f.Created, loc)
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
	day := dayOf(f.Created, loc)
	c.add(day, "friend.joined", 1)
	if !f.Joined.IsZero() {
		c.add(day, "join."+joinBucket(f.Joined.Sub(f.Created)), 1)
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
	day := dayOf(f.Created, loc)
	c.add(day, "end."+endKey(f), 1)
	got := isFinished(f.Result)
	if got {
		c.add(day, "got_going", 1)
		c.add(day, "heat."+strconv.Itoa(f.Created.In(loc).Hour()/2), 1)
		if f.Stats != nil {
			c.add(day, "len."+lengthBucket(f.Stats.Moves), 1)
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
	day := dayOf(sr.StartedAt, loc)
	c.add(day, "search", 1)
	if sr.Others == 0 {
		c.add(day, "search.alone", 1)
	}
	waited := waitBucket(sr.EndedAt.Sub(sr.StartedAt))
	if sr.Matched {
		c.add(day, "search.matched", 1)
		c.add(day, "wait."+waited, 1)
	} else {
		c.add(day, "search.gaveup", 1)
		c.add(day, "gaveup."+waited, 1)
	}
}

// countEvent counts a practice game, a reopened stream or a restart.
func countEvent(c counts, loc *time.Location, kind string, at time.Time) {
	c.add(dayOf(at, loc), "event."+kind, 1)
}

// countError counts a browser error.
func countError(c counts, loc *time.Location, at time.Time) {
	c.add(dayOf(at, loc), "errors", 1)
}

func isFinished(reason string) bool {
	switch reason {
	case "checkmate", "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout", "timeout_vs_insufficient", "resignation":
		return true
	}
	return false
}

// bucket is one bar of a histogram: its key suffix, its label on the page
// and the largest value it holds.
type bucket struct {
	key, label string
	upTo       int64 // moves, or milliseconds; the last bucket has no limit
}

const noLimit = int64(^uint64(0) >> 1)

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
```

Modify `store/gamesection.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/gamesection.go
+++ b/store/gamesection.go
@@ -63,14 +63,6 @@
 	winnerMS             int64     // the winner's clock after the last turn
 	hasClock             bool
 	stats                *GameStats
-}
-
-func isFinished(reason string) bool {
-	switch reason {
-	case "checkmate", "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout", "timeout_vs_insufficient", "resignation":
-		return true
-	}
-	return false
 }
 
 func msTime(n sql.NullInt64) time.Time {
```

Modify `store/migrate.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/migrate.go
+++ b/store/migrate.go
@@ -78,7 +78,9 @@
 	// game that got going, written with its result; open_hist counts the
 	// turns that ended with 0..5 holes open. searches logs quick-match
 	// searches (nothing about who searched). events counts practice
-	// games, reopened game streams and restarts.
+	// games, reopened game streams and restarts. stats_days holds the
+	// daily counters the stats page reads (day in STATS_TZ, a key, an
+	// amount); stats_state says which counters version and zone wrote them.
 	`ALTER TABLE games ADD COLUMN kind TEXT NOT NULL DEFAULT 'friend';
 	 ALTER TABLE games ADD COLUMN joined_at INTEGER;
 	 ALTER TABLE games ADD COLUMN playtest INTEGER NOT NULL DEFAULT 0;
@@ -110,7 +112,18 @@
 		at   INTEGER NOT NULL,
 		kind TEXT NOT NULL
 	 );
-	 CREATE INDEX events_at ON events(at);`,
+	 CREATE INDEX events_at ON events(at);
+	 CREATE TABLE stats_days (
+		day TEXT NOT NULL,
+		key TEXT NOT NULL,
+		n   INTEGER NOT NULL,
+		PRIMARY KEY (day, key)
+	 ) WITHOUT ROWID;
+	 CREATE TABLE stats_state (
+		id      INTEGER PRIMARY KEY CHECK (id = 1),
+		version INTEGER NOT NULL,
+		zone    TEXT NOT NULL
+	 );`,
 }
 
 func (s *Store) migrate(ctx context.Context) error {
```

Modify `store/store.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/store.go
+++ b/store/store.go
@@ -7,6 +7,8 @@
 	"fmt"
 	"os"
 	"path/filepath"
+	"sync/atomic"
+	"time"
 
 	_ "modernc.org/sqlite"
 )
@@ -14,10 +16,19 @@
 // Store wraps the SQLite database.
 type Store struct {
 	db *sql.DB
+	// zone cuts the stats counters' days (STATS_TZ).
+	zone *time.Location
+	// stale is set when the counters are from another version or zone and
+	// rebuilding them failed.
+	stale atomic.Bool
 }
 
-// Open opens (creating if needed) the database at path and applies migrations.
-func Open(path string) (*Store, error) {
+// Open opens (creating if needed) the database at path and applies
+// migrations. Its stats counters cut days in UTC.
+func Open(path string) (*Store, error) { return OpenIn(path, time.UTC) }
+
+// OpenIn is Open with the stats counters' days cut in zone (STATS_TZ).
+func OpenIn(path string, zone *time.Location) (*Store, error) {
 	if dir := filepath.Dir(path); dir != "." {
 		if err := os.MkdirAll(dir, 0o755); err != nil {
 			return nil, fmt.Errorf("create db dir: %w", err)
@@ -30,7 +41,7 @@
 	}
 	// SQLite allows one writer; a single connection avoids SQLITE_BUSY.
 	db.SetMaxOpenConns(1)
-	s := &Store{db: db}
+	s := &Store{db: db, zone: zone}
 	if err := s.migrate(context.Background()); err != nil {
 		db.Close()
 		return nil, err
```

- [ ] **Step 4: Run the tests, then everything**

Run: `go test -short ./store -run "TestBucketEdges|TestEndKey|TestCountsDropZeros|TestCountEnded|TestEnsureCounters|TestFailedRebuild"`
Expected: PASS.

Run: `go vet ./... && go test -short ./...`
Expected: every package `ok`. Also `gofmt -l store server cmd` prints nothing.

Nothing writes counters yet; `EnsureCounters` already fills them from the raw rows, which the tests use.

- [ ] **Step 5: Commit**

```bash
git add store/counts.go store/counters.go store/counts_test.go store/counters_test.go store/store.go store/migrate.go store/gamesection.go
git commit -m "store: daily stats counters, the counting functions and the rebuild"
```

---

### Task 2: The counted writes

**Files:**
- Modify: `store/games.go`, `store/gamestats.go`, `store/errors.go`
- Modify: `store/counters_test.go`, `store/errors_test.go`, `store/games_test.go`

**Interfaces:**
- Consumes: everything Task 1 produced.
- Produces: counting `CreateGame`, `SeatBlack`, `EndGame`, `ExpireWaiting`, `AddGameStats`, `AddSearch`, `AddEvent`, `AddBrowserError`; `LatestError`; `noChange`, `lastClock`, `querier`, `insertCounted`; `addGameStats` now returns `(bool, error)`; `execOne` is gone. `errors.go` keeps `ErrorStats`, `BrowserErrors` and `PruneStats` for now (the page and startup still call them until Tasks 3 and 4).

- [ ] **Step 1: Write the failing tests**

Modify `store/counters_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/counters_test.go
+++ b/store/counters_test.go
@@ -1,8 +1,11 @@
 package store
 
 import (
+	"fmt"
+	"math/rand/v2"
 	"path/filepath"
 	"reflect"
+	"strconv"
 	"testing"
 	"time"
 )
@@ -50,6 +53,112 @@
 	return out
 }
 
+func TestCountedGameWrites(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTemp(t)
+	day := dayOf(t0, time.UTC)
+	heat := "heat." + strconv.Itoa(t0.UTC().Hour()/2)
+
+	must(t, s.CreateGame(ctx, Game{Code: "LINK01", White: "w", CreatedAt: t0}))                                  // a friend link
+	must(t, s.CreateGame(ctx, Game{Code: "QUICK1", White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))       // quick match
+	must(t, s.CreateGame(ctx, Game{Code: "REMAT1", White: "b", Black: "w", RematchOf: "LINK01", CreatedAt: t0})) // a rematch of a friend game
+	must(t, s.CreateGame(ctx, Game{Code: "PLAYT1", White: "w", CreatedAt: t0, Playtest: true}))                  // a playtest: counts nothing
+	want := map[string]int{"games.friend": 2, "games.quick": 1, "friend.links": 1, "friend.rematches": 1}
+	if got := counters(t, s); !reflect.DeepEqual(got, keysOn(day, want)) {
+		t.Fatalf("after creating: %v", got)
+	}
+
+	must(t, s.SeatBlack(ctx, "LINK01", "b", "", t0.Add(3*time.Minute)))
+	must(t, s.SeatBlack(ctx, "LINK01", "c", "", t0.Add(4*time.Minute))) // taken: no change, no error
+	must(t, s.SeatBlack(ctx, "PLAYT1", "b", "", t0.Add(time.Minute)))
+	want["friend.joined"], want["join.1-5m"] = 1, 1
+	if got := counters(t, s); !reflect.DeepEqual(got, keysOn(day, want)) {
+		t.Fatalf("after joining: %v", got)
+	}
+
+	// A resignation has no final turn: the winner's clock is the last saved one.
+	must(t, s.AddTurn(ctx, "LINK01", Turn{Ply: 0, Move: "e2e4", WhiteMS: 590_000, BlackMS: 600_000, At: t0.Add(4 * time.Minute)}))
+	must(t, s.AddTurn(ctx, "LINK01", Turn{Ply: 1, Move: "e7e5", WhiteMS: 590_000, BlackMS: 45_000, At: t0.Add(5 * time.Minute)}))
+	st := &GameStats{Moves: 2, Opened: 1, WhiteLost: 1, OpenHist: [6]int{1, 1}}
+	end := Result{EndedAt: t0.Add(6 * time.Minute), Reason: "resignation", Winner: "black"}
+	must(t, s.EndGame(ctx, "LINK01", end, nil, st))
+	must(t, s.EndGame(ctx, "LINK01", end, nil, st)) // already ended: no change, no error
+	for k, n := range map[string]int{"end.resignation": 1, "got_going": 1, heat: 1, "len.1-10": 1, "left.30s-1m": 1, "friend.finished": 1,
+		"road.games": 1, "road.opened": 1, "road.no_fall": 1, "open.0": 1, "open.1": 1, "decided.less": 1} {
+		want[k] += n
+	}
+	// A flag fall against a side that can't mate is a draw: no clock.decided, no winner's clock.
+	must(t, s.EndGame(ctx, "QUICK1", Result{EndedAt: t0.Add(time.Hour), Reason: "timeout_vs_insufficient"}, nil, &GameStats{Moves: 90, Fell: 2}))
+	for k, n := range map[string]int{"end.draw": 1, "got_going": 1, heat: 1, "len.81+": 1, "road.games": 1, "road.fell": 2} {
+		want[k] += n
+	}
+	must(t, s.EndGame(ctx, "REMAT1", Result{EndedAt: t0.Add(time.Hour), Reason: "aborted"}, nil, nil))
+	must(t, s.EndGame(ctx, "PLAYT1", Result{EndedAt: t0.Add(time.Hour), Reason: "aborted"}, nil, nil))
+	want["end.aborted"]++
+	if got := counters(t, s); !reflect.DeepEqual(got, keysOn(day, want)) {
+		t.Fatalf("after ending:\n got %v\nwant %v", got, keysOn(day, want))
+	}
+}
+
+// A game counts on the day it was created, in the store's zone, however
+// late it ends.
+func TestGameCountsOnItsCreationDay(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTempIn(t, newYork)
+	created := time.Date(2026, 10, 3, 23, 30, 0, 0, newYork) // 03:30 UTC on Oct 4
+	must(t, s.CreateGame(ctx, Game{Code: "LATE01", White: "w", Black: "b", Kind: "quick", CreatedAt: created}))
+	must(t, s.EndGame(ctx, "LATE01", Result{EndedAt: created.Add(90 * time.Minute), Reason: "aborted"}, nil, nil))
+	want := map[string]int{"2026-10-03 games.quick": 1, "2026-10-03 end.aborted": 1}
+	if got := counters(t, s); !reflect.DeepEqual(got, want) {
+		t.Fatalf("counters %v, want %v", got, want)
+	}
+}
+
+func TestExpireWaitingCounts(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTemp(t)
+	must(t, s.CreateGame(ctx, Game{Code: "WAIT01", White: "w", CreatedAt: t0}))
+	must(t, s.CreateGame(ctx, Game{Code: "WAIT02", White: "w", CreatedAt: t0, Playtest: true}))
+	n, err := s.ExpireWaiting(ctx, t0.Add(time.Hour), t0.Add(25*time.Hour))
+	must(t, err)
+	if got := counters(t, s)[dayOf(t0, time.UTC)+" end.expired"]; n != 2 || got != 1 {
+		t.Fatalf("expired %d, counted %d; want 2 and 1 (the playtest isn't counted)", n, got)
+	}
+}
+
+// The backfill moves a game from "ended, road unknown" to what its stats
+// say, and a second backfill of the same game counts nothing.
+func TestAddGameStatsMovesTheCounters(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTemp(t)
+	day := dayOf(t0, time.UTC)
+	must(t, s.CreateGame(ctx, Game{Code: "OLD001", White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))
+	must(t, s.EndGame(ctx, "OLD001", Result{EndedAt: t0.Add(time.Hour), Reason: "checkmate", Winner: "white"}, nil, nil))
+	st := GameStats{Moves: 31, Opened: 4, Fell: 1, BlackLost: 1, MateByRoll: true}
+	must(t, s.AddGameStats(ctx, "OLD001", st))
+	must(t, s.AddGameStats(ctx, "OLD001", st))
+	want := keysOn(day, map[string]int{"games.quick": 1, "end.roll": 1, "got_going": 1, "heat." + strconv.Itoa(t0.UTC().Hour()/2): 1, "len.31-40": 1,
+		"road.games": 1, "road.opened": 4, "road.fell": 1, "road.mate_by_roll": 1, "decided.less": 1})
+	if got := counters(t, s); !reflect.DeepEqual(got, want) {
+		t.Fatalf("\n got %v\nwant %v", got, want)
+	}
+}
+
+func TestSearchesAndEventsCount(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTemp(t)
+	day := dayOf(t0, time.UTC)
+	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(12 * time.Second), Matched: true, Others: 1}))
+	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(25 * time.Second)}))
+	must(t, s.AddEvent(ctx, "practice", t0))
+	must(t, s.AddEvent(ctx, "restart", t0))
+	want := keysOn(day, map[string]int{"search": 2, "search.matched": 1, "wait.10-30s": 1, "search.gaveup": 1, "gaveup.10-30s": 1,
+		"search.alone": 1, "event.practice": 1, "event.restart": 1})
+	if got := counters(t, s); !reflect.DeepEqual(got, want) {
+		t.Fatalf("\n got %v\nwant %v", got, want)
+	}
+}
+
 func TestEnsureCountersRebuildsOnlyWhenNeeded(t *testing.T) {
 	ctx := t.Context()
 	s, path := openTempIn(t, newYork)
@@ -101,6 +210,90 @@
 	}
 }
 
+// Counting as the data is written must give exactly the counters a
+// rebuild from the raw rows gives: across a daylight-saving change, games
+// that end or are joined the next day, aborted, expired and retired games,
+// rematches, playtests, backfilled stats, searches, events and errors.
+func TestLiveCountingEqualsRebuild(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTempIn(t, newYork)
+	rng := rand.New(rand.NewPCG(1, 2))
+	start := time.Date(2026, 10, 28, 0, 0, 0, 0, newYork) // New York leaves daylight saving on Nov 1
+	at := func() time.Time { return start.Add(time.Duration(rng.Int64N(int64(10 * 24 * time.Hour)))) }
+	results := []string{"checkmate", "resignation", "timeout", "stalemate", "timeout_vs_insufficient", "aborted", "repetition"}
+	var friends []string
+	for i := range 300 {
+		code := fmt.Sprintf("G%05d", i)
+		g := Game{Code: code, White: "w", CreatedAt: at(), Playtest: rng.IntN(10) == 0}
+		switch rng.IntN(3) {
+		case 0:
+			g.Kind, g.Black = "quick", "b"
+		case 1:
+			if len(friends) > 0 {
+				g.RematchOf, g.Black = friends[rng.IntN(len(friends))], "b"
+			}
+		}
+		must(t, s.CreateGame(ctx, g))
+		if g.Kind != "quick" && g.RematchOf == "" {
+			friends = append(friends, code)
+		}
+		if g.Black == "" {
+			if rng.IntN(4) == 0 {
+				continue // left waiting: expired below
+			}
+			must(t, s.SeatBlack(ctx, code, "b", "", g.CreatedAt.Add(time.Duration(rng.Int64N(int64(30*time.Hour))))))
+		}
+		turns := rng.IntN(4)
+		for ply := range turns {
+			must(t, s.AddTurn(ctx, code, Turn{Ply: ply, Move: "e2e4", WhiteMS: rng.Int64N(600_000), BlackMS: rng.Int64N(600_000), At: g.CreatedAt}))
+		}
+		r := Result{EndedAt: g.CreatedAt.Add(time.Duration(rng.Int64N(int64(5 * time.Hour)))), Reason: results[rng.IntN(len(results))]}
+		if r.Reason != "aborted" && r.Reason != "stalemate" && r.Reason != "repetition" && r.Reason != "timeout_vs_insufficient" {
+			r.Winner = []string{"white", "black"}[rng.IntN(2)]
+		}
+		var st *GameStats
+		if isFinished(r.Reason) {
+			st = &GameStats{Moves: rng.IntN(120), Opened: rng.IntN(20), Fell: rng.IntN(5), WhiteLost: rng.IntN(3), BlackLost: rng.IntN(3),
+				SavingRolls: rng.IntN(4), MateByRoll: rng.IntN(5) == 0, OpenHist: [6]int{rng.IntN(9), rng.IntN(9), rng.IntN(9), 0, rng.IntN(3), 1}}
+		}
+		backfilled := st != nil && rng.IntN(5) == 0 // ended before stats were kept
+		if backfilled {
+			must(t, s.EndGame(ctx, code, r, nil, nil))
+			must(t, s.AddGameStats(ctx, code, *st))
+		} else {
+			must(t, s.EndGame(ctx, code, r, nil, st))
+		}
+	}
+	_, err := s.ExpireWaiting(ctx, start.AddDate(0, 0, 11), start.AddDate(0, 0, 12))
+	must(t, err)
+	// A migration retires an unfinished game without counting it.
+	must(t, s.CreateGame(ctx, Game{Code: "RETIRE", White: "w", Black: "b", Kind: "quick", CreatedAt: at()}))
+	must1(s.db.ExecContext(ctx, `UPDATE games SET ended_at = 1, result = 'retired' WHERE code = 'RETIRE'`))
+	for range 100 {
+		begin := at()
+		must(t, s.AddSearch(ctx, Search{StartedAt: begin, EndedAt: begin.Add(time.Duration(rng.Int64N(int64(3 * time.Minute)))), Matched: rng.IntN(2) == 0, Others: rng.IntN(3)}))
+	}
+	for range 50 {
+		must(t, s.AddEvent(ctx, []string{"practice", "reconnect", "restart"}[rng.IntN(3)], at()))
+		must(t, s.AddBrowserError(ctx, BrowserError{At: at(), Message: "x"}))
+	}
+	live := counters(t, s)
+	if len(live) < 500 {
+		t.Fatalf("only %d counters: the scenario counts too little to compare", len(live))
+	}
+
+	if _, err := s.rebuild(ctx, start.AddDate(0, 0, 12)); err != nil {
+		t.Fatal(err)
+	}
+	if rebuilt := counters(t, s); !reflect.DeepEqual(live, rebuilt) {
+		for k := range mergeKeys(live, rebuilt) {
+			if live[k] != rebuilt[k] {
+				t.Errorf("%s: live %d, rebuilt %d", k, live[k], rebuilt[k])
+			}
+		}
+	}
+}
+
 func mergeKeys(a, b map[string]int) map[string]bool {
 	out := map[string]bool{}
 	for k := range a {
```

Modify `store/errors_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/errors_test.go
+++ b/store/errors_test.go
@@ -5,34 +5,53 @@
 	"time"
 )
 
-func TestBrowserErrorsCountAndPrune(t *testing.T) {
+func TestBrowserErrorsCountedAndLatest(t *testing.T) {
 	ctx := t.Context()
 	s, _ := openTemp(t)
 	must(t, s.AddBrowserError(ctx, BrowserError{At: t0, Path: "/game", Message: "TypeError: x"}))
 	must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(2 * time.Hour), Path: "/", Message: "TypeError: y"}))
-	// Searches and events go by the same cutoff.
-	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(2 * time.Hour)}))
-	must(t, s.AddSearch(ctx, Search{StartedAt: t0.Add(2 * time.Hour), EndedAt: t0.Add(3 * time.Hour)}))
-	must(t, s.AddEvent(ctx, "restart", t0))
-	must(t, s.AddEvent(ctx, "restart", t0.Add(2*time.Hour)))
-
-	st, err := s.BrowserErrors(ctx, t0, t0.Add(3*time.Hour))
+	if got := counters(t, s); got[dayOf(t0, time.UTC)+" errors"] != 2 {
+		t.Fatalf("counters %v, want 2 errors", got)
+	}
+	latest, err := s.LatestError(ctx, t0, t0.Add(3*time.Hour))
 	must(t, err)
-	if st.N != 2 || st.Latest != "TypeError: y" {
-		t.Fatalf("errors = %+v, want 2 and the later message", st)
+	if latest != "TypeError: y" {
+		t.Fatalf("latest %q", latest)
 	}
-	if st, err = s.BrowserErrors(ctx, t0.Add(3*time.Hour), t0.Add(4*time.Hour)); err != nil || st != (ErrorStats{}) {
-		t.Fatalf("a quiet range = %+v, %v", st, err)
+	if latest, err = s.LatestError(ctx, t0.Add(3*time.Hour), t0.Add(4*time.Hour)); err != nil || latest != "" {
+		t.Fatalf("a quiet range: %q, %v", latest, err)
 	}
+}
 
-	removed, err := s.PruneStats(ctx, t0.Add(time.Hour))
+// Past errorRowsPerDay a day the messages stop being stored, and the
+// errors are still counted.
+func TestBrowserErrorsCappedPerDay(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTemp(t)
+	for i := range errorRowsPerDay + 5 {
+		must(t, s.AddBrowserError(ctx, BrowserError{At: t0.Add(time.Duration(i) * time.Second), Path: "/", Message: "x"}))
+	}
+	var rows int
+	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM browser_errors`).Scan(&rows))
+	if got := counters(t, s)[dayOf(t0, time.UTC)+" errors"]; rows != errorRowsPerDay || got != errorRowsPerDay+5 {
+		t.Fatalf("%d rows and %d counted, want %d and %d", rows, got, errorRowsPerDay, errorRowsPerDay+5)
+	}
+}
+
+// Pruning cuts at a midnight in the store's zone, 400 days back.
+func TestPruneErrorsAtMidnight(t *testing.T) {
+	ctx := t.Context()
+	s, _ := openTempIn(t, newYork)
+	now := time.Date(2026, 10, 19, 15, 0, 0, 0, newYork)
+	cutoff := time.Date(2025, 9, 14, 0, 0, 0, 0, newYork) // 400 days before Oct 19, 2026
+	must(t, s.AddBrowserError(ctx, BrowserError{At: cutoff.Add(-time.Minute), Message: "old"}))
+	must(t, s.AddBrowserError(ctx, BrowserError{At: cutoff, Message: "kept"}))
+	n, err := s.PruneErrors(ctx, now)
 	must(t, err)
-	if removed != 3 { // the first error, the first search and the first event
-		t.Fatalf("pruned %d rows, want 3", removed)
+	if n != 1 {
+		t.Fatalf("pruned %d, want 1", n)
 	}
-	var errs, searches, events int
-	must(t, s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM browser_errors), (SELECT COUNT(*) FROM searches), (SELECT COUNT(*) FROM events)`).Scan(&errs, &searches, &events))
-	if errs != 1 || searches != 1 || events != 1 {
-		t.Fatalf("%d errors, %d searches and %d events left, want 1 each", errs, searches, events)
+	if latest, _ := s.LatestError(ctx, time.Time{}, now); latest != "kept" {
+		t.Fatalf("left %q", latest)
 	}
 }
```

Modify `store/games_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/games_test.go
+++ b/store/games_test.go
@@ -69,6 +69,13 @@
 	}
 }
 
+func TestSeatBlackFailsForAnUnknownGame(t *testing.T) {
+	s, _ := openTemp(t)
+	if err := s.SeatBlack(t.Context(), "NOPE00", "bob", "", t0); err == nil {
+		t.Fatal("seating Black in a game that was never saved should fail")
+	}
+}
+
 func TestAddTurnRejectsADuplicatePly(t *testing.T) {
 	ctx := t.Context()
 	s, _ := openTemp(t)
@@ -128,7 +135,7 @@
 	}
 }
 
-func must(t *testing.T, err error) {
+func must(t testing.TB, err error) {
 	t.Helper()
 	if err != nil {
 		t.Fatal(err)
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -short ./store -run "TestCounted|TestGameCounts|TestExpireWaitingCounts|TestAddGameStatsMoves|TestSearchesAndEventsCount|TestLiveCountingEqualsRebuild|TestBrowserErrors|TestPruneErrors|UnknownGame"`
Expected: FAIL: the package doesn't build (`s.LatestError undefined`). Once it does, the counters stay empty (`after creating: map[]`) and a repeated `EndGame` errors.

- [ ] **Step 3: Write the code**

Modify `store/errors.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/errors.go
+++ b/store/errors.go
@@ -3,6 +3,7 @@
 import (
 	"context"
 	"database/sql"
+	"errors"
 	"time"
 )
 
@@ -13,13 +14,51 @@
 	Path, Message string
 }
 
+// errorRowsPerDay caps the raw error rows kept a day, so a flood of
+// reports (there are no per-IP limits yet) can't fill the database. Past
+// it, errors are still counted.
+const errorRowsPerDay = 200
+
+// AddBrowserError counts an error and keeps its message, up to
+// errorRowsPerDay a day.
 func (s *Store) AddBrowserError(ctx context.Context, e BrowserError) error {
-	_, err := s.db.ExecContext(ctx,
-		`INSERT INTO browser_errors (at, path, message) VALUES (?, ?, ?)`,
-		e.At.UnixMilli(), e.Path, e.Message)
-	return err
+	tx, err := s.db.BeginTx(ctx, nil)
+	if err != nil {
+		return err
+	}
+	defer tx.Rollback()
+	day := dayOf(e.At, s.zone)
+	var today int
+	err = tx.QueryRowContext(ctx, `SELECT n FROM stats_days WHERE day = ? AND key = 'errors'`, day).Scan(&today)
+	if err != nil && !errors.Is(err, sql.ErrNoRows) {
+		return err
+	}
+	if today < errorRowsPerDay {
+		if _, err := tx.ExecContext(ctx, `INSERT INTO browser_errors (at, path, message) VALUES (?, ?, ?)`,
+			e.At.UnixMilli(), e.Path, e.Message); err != nil {
+			return err
+		}
+	}
+	c := counts{}
+	countError(c, s.zone, e.At)
+	if err := addCounts(ctx, tx, c); err != nil {
+		return err
+	}
+	return tx.Commit()
 }
 
+// LatestError is the newest stored error message with from <= at < to,
+// or "" for none.
+func (s *Store) LatestError(ctx context.Context, from, to time.Time) (string, error) {
+	var msg string
+	err := s.db.QueryRowContext(ctx, `SELECT message FROM browser_errors WHERE at >= ? AND at < ? ORDER BY at DESC LIMIT 1`,
+		from.UnixMilli(), to.UnixMilli()).Scan(&msg)
+	if errors.Is(err, sql.ErrNoRows) {
+		return "", nil
+	}
+	return msg, err
+}
+
 // ErrorStats is what the Health section shows of the browser errors in a
 // range: how many, and the latest message.
 type ErrorStats struct {
```

Modify `store/games.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/games.go
+++ b/store/games.go
@@ -68,7 +68,7 @@
 	JoinedAt time.Time // when Black sat down; zero when unknown
 }
 
-// CreateGame saves a new game.
+// CreateGame saves a new game and counts it.
 func (s *Store) CreateGame(ctx context.Context, g Game) error {
 	kind := g.Kind
 	if kind == "" {
@@ -78,19 +78,61 @@
 	if g.Black != "" {
 		joined = g.CreatedAt.UnixMilli() // both seated from the start: quick match or a rematch
 	}
-	_, err := s.db.ExecContext(ctx,
+	tx, err := s.db.BeginTx(ctx, nil)
+	if err != nil {
+		return err
+	}
+	defer tx.Rollback()
+	_, err = tx.ExecContext(ctx,
 		`INSERT INTO games (code, white, black, white_name, black_name, created_at, rematch_of, rules, kind, joined_at, playtest) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
 		g.Code, g.White, nullable(g.Black), nullable(g.WhiteName), nullable(g.BlackName), g.CreatedAt.UnixMilli(), nullable(g.RematchOf), rulesVersion, kind, joined, g.Playtest)
 	var se *sqlite.Error
 	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
 		return ErrCodeTaken
 	}
-	return err
+	if err != nil {
+		return err
+	}
+	if !g.Playtest {
+		c := counts{}
+		countCreated(c, s.zone, gameFacts{Created: g.CreatedAt, Kind: kind, Rematch: g.RematchOf != ""})
+		if err := addCounts(ctx, tx, c); err != nil {
+			return err
+		}
+	}
+	return tx.Commit()
 }
 
-// SeatBlack records the guest who took Black's seat, their name, and when.
+// SeatBlack records the guest who took Black's seat, their name, and when,
+// and counts the join. A seat already taken changes nothing and is no error.
 func (s *Store) SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error {
-	return execOne(ctx, s.db, `UPDATE games SET black = ?, black_name = ?, joined_at = ? WHERE code = ?`, guest, nullable(name), now.UnixMilli(), code)
+	tx, err := s.db.BeginTx(ctx, nil)
+	if err != nil {
+		return err
+	}
+	defer tx.Rollback()
+	var created int64
+	var playtest bool
+	f := gameFacts{HasBlack: true, Joined: now}
+	err = tx.QueryRowContext(ctx,
+		`UPDATE games SET black = ?, black_name = ?, joined_at = ? WHERE code = ? AND black IS NULL
+		 RETURNING created_at, kind, rematch_of IS NOT NULL, playtest`,
+		guest, nullable(name), now.UnixMilli(), code).Scan(&created, &f.Kind, &f.Rematch, &playtest)
+	if errors.Is(err, sql.ErrNoRows) {
+		return noChange(ctx, tx, code)
+	}
+	if err != nil {
+		return err
+	}
+	if !playtest {
+		f.Created = time.UnixMilli(created)
+		c := counts{}
+		countJoined(c, s.zone, f)
+		if err := addCounts(ctx, tx, c); err != nil {
+			return err
+		}
+	}
+	return tx.Commit()
 }
 
 // AddTurn saves one move.
@@ -98,43 +140,125 @@
 	return addTurn(ctx, s.db, code, t)
 }
 
-// EndGame records how a game ended. final, if not nil, is the move that
-// ended it, saved in the same transaction.
+// EndGame records how a game ended and counts it. final, if not nil, is
+// the move that ended it, saved in the same transaction. A game that has
+// already ended changes nothing and is no error.
 func (s *Store) EndGame(ctx context.Context, code string, r Result, final *Turn, st *GameStats) error {
 	tx, err := s.db.BeginTx(ctx, nil)
 	if err != nil {
 		return err
 	}
 	defer tx.Rollback()
+	var created int64
+	var playtest bool
+	f := gameFacts{Result: r.Reason, Winner: r.Winner, Stats: st}
+	err = tx.QueryRowContext(ctx,
+		`UPDATE games SET ended_at = ?, result = ?, winner = ? WHERE code = ? AND ended_at IS NULL
+		 RETURNING created_at, kind, rematch_of IS NOT NULL, black IS NOT NULL, playtest`,
+		r.EndedAt.UnixMilli(), r.Reason, nullable(r.Winner), code).Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &playtest)
+	if errors.Is(err, sql.ErrNoRows) {
+		return noChange(ctx, tx, code)
+	}
+	if err != nil {
+		return err
+	}
 	if final != nil {
 		if err := addTurn(ctx, tx, code, *final); err != nil {
 			return err
 		}
 	}
-	if err := execOne(ctx, tx,
-		`UPDATE games SET ended_at = ?, result = ?, winner = ? WHERE code = ?`,
-		r.EndedAt.UnixMilli(), r.Reason, nullable(r.Winner), code); err != nil {
-		return err
-	}
 	if st != nil {
-		if err := addGameStats(ctx, tx, code, *st); err != nil {
+		if _, err := addGameStats(ctx, tx, code, *st); err != nil {
 			return err
 		}
 	}
+	if !playtest {
+		f.Created = time.UnixMilli(created)
+		if f.WinnerMS, f.HasClock, err = lastClock(ctx, tx, code, r.Winner); err != nil {
+			return err
+		}
+		c := counts{}
+		countEnded(c, s.zone, f)
+		if err := addCounts(ctx, tx, c); err != nil {
+			return err
+		}
+	}
 	return tx.Commit()
 }
 
-// ExpireWaiting ends every game still waiting for Black that was created
-// before cutoff, with result "expired". It returns how many it ended.
+// noChange explains a guarded update that changed no row: nil when the
+// game exists (it had already ended, or its seat was taken); an error when
+// it was never saved.
+func noChange(ctx context.Context, db querier, code string) error {
+	var exists bool
+	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE code = ?)`, code).Scan(&exists); err != nil {
+		return err
+	}
+	if !exists {
+		return fmt.Errorf("no saved game %s", code)
+	}
+	return nil
+}
+
+// lastClock is the winner's clock after a game's last turn: one
+// primary-key seek. It reports false with no winner or no turn.
+func lastClock(ctx context.Context, db querier, code, winner string) (int64, bool, error) {
+	if winner == "" {
+		return 0, false, nil
+	}
+	var white, black sql.NullInt64
+	err := db.QueryRowContext(ctx, `SELECT white_ms, black_ms FROM turns WHERE game = ? ORDER BY ply DESC LIMIT 1`, code).Scan(&white, &black)
+	if errors.Is(err, sql.ErrNoRows) {
+		return 0, false, nil
+	}
+	if err != nil {
+		return 0, false, err
+	}
+	ms, ok := winnerClock(winner, white, black)
+	return ms, ok, nil
+}
+
+// ExpireWaiting ends every game still waiting for Black that was created
+// before cutoff, with result "expired", and counts them. It returns how
+// many it ended.
 func (s *Store) ExpireWaiting(ctx context.Context, cutoff, now time.Time) (int64, error) {
-	res, err := s.db.ExecContext(ctx,
+	tx, err := s.db.BeginTx(ctx, nil)
+	if err != nil {
+		return 0, err
+	}
+	defer tx.Rollback()
+	rows, err := tx.QueryContext(ctx,
 		`UPDATE games SET ended_at = ?, result = 'expired'
-		 WHERE ended_at IS NULL AND black IS NULL AND created_at < ?`,
+		 WHERE ended_at IS NULL AND black IS NULL AND created_at < ?
+		 RETURNING created_at, kind, rematch_of IS NOT NULL, playtest`,
 		now.UnixMilli(), cutoff.UnixMilli())
 	if err != nil {
 		return 0, err
 	}
-	return res.RowsAffected()
+	var n int64
+	c := counts{}
+	for rows.Next() {
+		var created int64
+		var playtest bool
+		f := gameFacts{Result: "expired"}
+		if err := rows.Scan(&created, &f.Kind, &f.Rematch, &playtest); err != nil {
+			rows.Close()
+			return 0, err
+		}
+		n++
+		if !playtest {
+			f.Created = time.UnixMilli(created)
+			countEnded(c, s.zone, f)
+		}
+	}
+	rows.Close() // one connection: close before the next statement
+	if err := rows.Err(); err != nil {
+		return 0, err
+	}
+	if err := addCounts(ctx, tx, c); err != nil {
+		return 0, err
+	}
+	return n, tx.Commit()
 }
 
 // LoadForRestore returns every unfinished game and every game that ended
@@ -221,6 +345,10 @@
 	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
 }
 
+type querier interface {
+	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
+}
+
 func addTurn(ctx context.Context, db execer, code string, t Turn) error {
 	_, err := db.ExecContext(ctx,
 		`INSERT INTO turns (game, ply, move, dice, white_ms, black_ms, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
@@ -228,20 +356,6 @@
 	return err
 }
 
-// execOne runs an UPDATE that must change exactly one row.
-func execOne(ctx context.Context, db execer, query string, args ...any) error {
-	res, err := db.ExecContext(ctx, query, args...)
-	if err != nil {
-		return err
-	}
-	if n, err := res.RowsAffected(); err != nil {
-		return err
-	} else if n != 1 {
-		return fmt.Errorf("updated %d games, want 1", n)
-	}
-	return nil
-}
-
 func nullable(s string) any {
 	if s == "" {
 		return nil
```

Modify `store/gamestats.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/gamestats.go
+++ b/store/gamestats.go
@@ -2,6 +2,8 @@
 
 import (
 	"context"
+	"database/sql"
+	"errors"
 	"fmt"
 	"strconv"
 	"strings"
@@ -30,18 +32,60 @@
 	Others             int
 }
 
-// AddGameStats writes a game's stats on their own (the backfill). A game
-// that has a row keeps it.
+// AddGameStats writes a game's stats on their own (the backfill) and
+// moves its counters from "ended, road unknown" to what the stats say. A
+// game that has a row keeps it and counts nothing.
 func (s *Store) AddGameStats(ctx context.Context, code string, st GameStats) error {
-	return addGameStats(ctx, s.db, code, st)
+	tx, err := s.db.BeginTx(ctx, nil)
+	if err != nil {
+		return err
+	}
+	defer tx.Rollback()
+	added, err := addGameStats(ctx, tx, code, st)
+	if err != nil || !added {
+		return err
+	}
+	var created int64
+	var result, winner sql.NullString
+	var playtest bool
+	f := gameFacts{}
+	err = tx.QueryRowContext(ctx, `SELECT created_at, kind, rematch_of IS NOT NULL, black IS NOT NULL, result, winner, playtest FROM games WHERE code = ?`, code).
+		Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &result, &winner, &playtest)
+	if errors.Is(err, sql.ErrNoRows) {
+		return tx.Commit() // a stats row for no game: nothing to count
+	}
+	if err != nil {
+		return err
+	}
+	if !playtest && result.Valid && result.String != "retired" {
+		f.Created, f.Result, f.Winner = time.UnixMilli(created), result.String, winner.String
+		if f.WinnerMS, f.HasClock, err = lastClock(ctx, tx, code, f.Winner); err != nil {
+			return err
+		}
+		before := counts{}
+		countEnded(before, s.zone, f)
+		f.Stats = &st
+		after := counts{}
+		countEnded(after, s.zone, f)
+		if err := addCounts(ctx, tx, after.minus(before)); err != nil {
+			return err
+		}
+	}
+	return tx.Commit()
 }
 
-func addGameStats(ctx context.Context, db execer, code string, st GameStats) error {
-	_, err := db.ExecContext(ctx,
+// addGameStats writes a stats row unless the game has one, and reports
+// whether it wrote.
+func addGameStats(ctx context.Context, db execer, code string, st GameStats) (bool, error) {
+	res, err := db.ExecContext(ctx,
 		`INSERT OR IGNORE INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
 		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
 		code, st.Moves, st.Opened, st.Reset, st.ClosedRounds, st.ClosedCap, st.Repaired, st.Fell, st.SavingRolls, st.Saved, st.WhiteLost, st.BlackLost, st.MateByRoll, formatHist(st.OpenHist))
-	return err
+	if err != nil {
+		return false, err
+	}
+	n, err := res.RowsAffected()
+	return n == 1, err
 }
 
 // GamesWithoutStats returns every game under the current rules that got
@@ -89,20 +133,40 @@
 	return has, err
 }
 
-// AddSearch logs one quick-match search.
+// AddSearch logs one quick-match search and counts it.
 func (s *Store) AddSearch(ctx context.Context, sr Search) error {
-	_, err := s.db.ExecContext(ctx,
+	c := counts{}
+	countSearch(c, s.zone, sr)
+	return s.insertCounted(ctx, c,
 		`INSERT INTO searches (started_at, ended_at, matched, others) VALUES (?, ?, ?, ?)`,
 		sr.StartedAt.UnixMilli(), sr.EndedAt.UnixMilli(), sr.Matched, sr.Others)
-	return err
 }
 
-// AddEvent counts one event of kind ("practice", "reconnect", "restart").
+// AddEvent logs one event of kind ("practice", "reconnect", "restart") and
+// counts it.
 func (s *Store) AddEvent(ctx context.Context, kind string, at time.Time) error {
-	_, err := s.db.ExecContext(ctx, `INSERT INTO events (at, kind) VALUES (?, ?)`, at.UnixMilli(), kind)
-	return err
+	c := counts{}
+	countEvent(c, s.zone, kind, at)
+	return s.insertCounted(ctx, c, `INSERT INTO events (at, kind) VALUES (?, ?)`, at.UnixMilli(), kind)
 }
 
+// insertCounted runs one raw insert and adds its counts, in one
+// transaction (one commit, as the insert alone was).
+func (s *Store) insertCounted(ctx context.Context, c counts, query string, args ...any) error {
+	tx, err := s.db.BeginTx(ctx, nil)
+	if err != nil {
+		return err
+	}
+	defer tx.Rollback()
+	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
+		return err
+	}
+	if err := addCounts(ctx, tx, c); err != nil {
+		return err
+	}
+	return tx.Commit()
+}
+
 // formatHist writes [6]int as "9,14,19,22,18,18"; parseHist reads it back.
 func formatHist(h [6]int) string {
 	parts := make([]string, len(h))
```

- [ ] **Step 4: Run the tests, then everything**

Run: `go test -short ./store -run "TestCounted|TestGameCounts|TestExpireWaitingCounts|TestAddGameStatsMoves|TestSearchesAndEventsCount|TestLiveCountingEqualsRebuild|TestBrowserErrors|TestPruneErrors|UnknownGame"`
Expected: PASS.

Run: `go vet ./... && go test -race -short ./...`
Expected: every package `ok`. Also `gofmt -l store server cmd` prints nothing.

`TestLiveCountingEqualsRebuild` is the safety net: if it fails, print the differing keys (it already does) and fix the counting, never the test. It was checked by deleting `countJoined` from the rebuild: 27 mismatches.

- [ ] **Step 5: Commit**

```bash
git add store/games.go store/gamestats.go store/errors.go store/counters_test.go store/errors_test.go store/games_test.go
git commit -m "store: count games, searches, events and errors as they are written"
```

---

### Task 3: The startup order

**Files:**
- Modify: `cmd/server/main.go`, `cmd/server/backfill.go`, `store/errors.go` (drop `PruneStats`)
- Modify: `cmd/server/main_test.go`, `cmd/server/backfill_test.go`

**Interfaces:**
- Consumes: `OpenIn`, `PruneErrors`, `ExpireWaiting`, `EnsureCounters`, `AddEvent`.
- Produces: `start(ctx, st, hub, now) error` (replaces `restore`), `prepareStats(ctx, st, now)` (replaces `startStats`). `main` loads `STATS_TZ` before opening the store and still sets `handler.StatsZone = zone` until Task 4.

- [ ] **Step 1: Write the failing tests**

Modify `cmd/server/backfill_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/cmd/server/backfill_test.go
+++ b/cmd/server/backfill_test.go
@@ -12,9 +12,9 @@
 	"mamdani-chess/store"
 )
 
-// With the database gone, the restart count and the backfill log at ERROR
-// and startup goes on.
-func TestStartStatsNeverStopsTheServer(t *testing.T) {
+// With the database gone, the backfill, the counters and the restart count
+// each log at ERROR and startup goes on.
+func TestPrepareStatsNeverStopsTheServer(t *testing.T) {
 	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
 	if err != nil {
 		t.Fatal(err)
@@ -23,9 +23,10 @@
 	var buf bytes.Buffer
 	defer slog.SetDefault(slog.Default())
 	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
-	startStats(context.Background(), st)
+	prepareStats(context.Background(), st, time.Now())
 	out := buf.String()
-	if strings.Count(out, "level=ERROR") != 2 || !strings.Contains(out, "restart not counted") || !strings.Contains(out, "game stats backfill stopped") {
+	if strings.Count(out, "level=ERROR") != 3 || !strings.Contains(out, "restart not counted") || !strings.Contains(out, "game stats backfill stopped") ||
+		!strings.Contains(out, "stats counters not rebuilt") {
 		t.Fatalf("logged %q", out)
 	}
 }
```

Modify `cmd/server/main_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/cmd/server/main_test.go
+++ b/cmd/server/main_test.go
@@ -48,7 +48,7 @@
 	}
 	defer st.Close()
 	h2 := game.NewHub(odd{}, st)
-	if err := restore(ctx, st, h2, time.Now()); err != nil {
+	if err := start(ctx, st, h2, time.Now()); err != nil {
 		t.Fatal(err)
 	}
 	g, ok := h2.Get(playing.Code())
@@ -65,7 +65,7 @@
 	// A day later, the waiting game expires. It ended just now, so it is
 	// still restored, to show that it's over.
 	h3 := game.NewHub(odd{}, st)
-	if err := restore(ctx, st, h3, time.Now().Add(game.DefaultIdle+time.Minute)); err != nil {
+	if err := start(ctx, st, h3, time.Now().Add(game.DefaultIdle+time.Minute)); err != nil {
 		t.Fatal(err)
 	}
 	g3, ok := h3.Get(waiting.Code())
@@ -77,6 +77,49 @@
 	}
 }
 
+// The stats work finishes before any game is restored (each restored
+// game's grace starts then), and a first start fills the counters from the
+// games already saved.
+func TestStartReadiesStatsBeforeRestoring(t *testing.T) {
+	ctx := t.Context()
+	path := filepath.Join(t.TempDir(), "games.db")
+	st, err := store.Open(path)
+	if err != nil {
+		t.Fatal(err)
+	}
+	defer st.Close()
+	h := game.NewHub(odd{}, st)
+	playing, _ := h.Create("alice", false)
+	sub, _ := playing.Join("bob")
+	defer playing.Leave(sub)
+	// A new database has no counters version yet, as on the first start
+	// with counters: the start rebuilds.
+	var buf bytes.Buffer
+	defer slog.SetDefault(slog.Default())
+	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
+	if err := start(ctx, st, game.NewHub(odd{}, st), time.Now()); err != nil {
+		t.Fatal(err)
+	}
+	out := buf.String()
+	rebuilt, restored := strings.Index(out, "stats rebuilt"), strings.Index(out, "games restored")
+	if rebuilt < 0 || restored < 0 || rebuilt > restored {
+		t.Fatalf("want the rebuild logged before the restore:\n%s", out)
+	}
+	days, err := st.Counts(ctx, "", "9999-12-31")
+	if err != nil {
+		t.Fatal(err)
+	}
+	sum := map[string]int{}
+	for _, d := range days {
+		for k, n := range d.N {
+			sum[k] += n
+		}
+	}
+	if sum["games.friend"] != 1 || sum["friend.joined"] != 1 || sum["event.restart"] != 1 {
+		t.Fatalf("counters after the first start: %v", sum)
+	}
+}
+
 func TestNewLoggerJSON(t *testing.T) {
 	var buf bytes.Buffer
 	newLogger("json", &buf).Info("listening", "addr", ":8080")
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -short ./cmd/server`
Expected: FAIL: `undefined: start`, `undefined: prepareStats`.

- [ ] **Step 3: Write the code**

Modify `cmd/server/backfill.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/cmd/server/backfill.go
+++ b/cmd/server/backfill.go
@@ -33,20 +33,24 @@
 	return done, failed, nil
 }
 
-// startStats counts this start on the stats page's Health section and
-// backfills game stats. A failure is logged at ERROR and startup carries
+// prepareStats backfills game stats, rebuilds the counters when their
+// version or zone changed, and counts this start, in that order, before
+// any game is restored. A failure is logged at ERROR and startup carries
 // on: the stats must never keep the game down.
-func startStats(ctx context.Context, st *store.Store) {
-	if err := st.AddEvent(ctx, "restart", time.Now()); err != nil {
-		slog.Error("restart not counted", "err", err)
-	}
-	start := time.Now()
+func prepareStats(ctx context.Context, st *store.Store, now time.Time) {
+	begin := time.Now()
 	done, failed, err := backfillStats(ctx, st)
 	if err != nil {
 		slog.Error("game stats backfill stopped", "games", done, "failed", failed, "err", err)
-		return
+	} else if done > 0 || failed > 0 {
+		slog.Info("game stats backfilled", "games", done, "failed", failed, "duration", time.Since(begin))
 	}
-	if done > 0 || failed > 0 {
-		slog.Info("game stats backfilled", "games", done, "failed", failed, "duration", time.Since(start))
+	if rep, err := st.EnsureCounters(ctx, now); err != nil {
+		slog.Error("stats counters not rebuilt", "err", err)
+	} else if rep.Rows > 0 || rep.Took > 0 {
+		slog.Info("stats rebuilt", "days", rep.Days, "rows", rep.Rows, "duration", rep.Took)
 	}
+	if err := st.AddEvent(ctx, "restart", now); err != nil {
+		slog.Error("restart not counted", "err", err)
+	}
 }
```

Modify `cmd/server/main.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/cmd/server/main.go
+++ b/cmd/server/main.go
@@ -20,11 +20,6 @@
 	"mamdani-chess/web"
 )
 
-// statsKeep is how long browser errors, quick-match searches and events
-// are kept: about thirteen months, so a year can be compared with the year
-// before.
-const statsKeep = 400 * 24 * time.Hour
-
 func main() {
 	if err := run(); err != nil {
 		slog.Error("server stopped", "err", err)
@@ -38,31 +33,29 @@
 	port := envOr("PORT", "8080")
 	dbPath := envOr("DB_PATH", "data/mamdani.db")
 
-	st, err := store.Open(dbPath)
+	// The stats counters cut days in STATS_TZ from the first write on.
+	zoneName := envOr("STATS_TZ", "America/New_York")
+	zone, err := time.LoadLocation(zoneName)
 	if err != nil {
+		return fmt.Errorf("STATS_TZ %q: %w", zoneName, err)
+	}
+	st, err := store.OpenIn(dbPath, zone)
+	if err != nil {
 		return err
 	}
 	defer st.Close()
 
 	hub := game.NewHub(game.CryptoDice{}, st)
-	if err := restore(context.Background(), st, hub, time.Now()); err != nil {
+	if err := start(context.Background(), st, hub, time.Now()); err != nil {
 		return err
 	}
-	startStats(context.Background(), st)
 	handler := server.New(st, hub, web.Assets())
 	// Railway sets this to the deployed commit; locally it's empty ("dev").
 	handler.Version = os.Getenv("RAILWAY_GIT_COMMIT_SHA")
-	// The stats page (spec: docs/superpowers/specs/2026-10-08-stats-design.md).
+	// The stats page (specs: docs/superpowers/specs/2026-10-08-stats-design.md
+	// and 2026-10-19-stats-counters-design.md).
 	handler.StatsPassword = os.Getenv("STATS_PASSWORD")
-	zone := envOr("STATS_TZ", "America/New_York")
-	if handler.StatsZone, err = time.LoadLocation(zone); err != nil {
-		return fmt.Errorf("STATS_TZ %q: %w", zone, err)
-	}
-	if n, err := st.PruneStats(context.Background(), time.Now().Add(-statsKeep)); err != nil {
-		return fmt.Errorf("prune stats: %w", err)
-	} else if n > 0 {
-		slog.Info("old stats pruned", "rows", n)
-	}
+	handler.StatsZone = zone
 	srv := &http.Server{
 		Addr:              ":" + port,
 		Handler:           handler,
@@ -97,15 +90,24 @@
 	return nil
 }
 
-// restore ends games nobody joined within a day, then rebuilds every
-// unfinished game, and every game that ended within the last day, so open
-// tabs carry on after a restart or a deploy.
-func restore(ctx context.Context, st *store.Store, hub *game.Hub, now time.Time) error {
-	start := time.Now()
+// start readies the store before anything can play: it prunes old browser
+// errors, ends games nobody joined within a day, brings the stats up to
+// date, and only then rebuilds every unfinished game, and every game that
+// ended within the last day, so open tabs carry on after a restart or a
+// deploy. Restoring last means no restored game's clock runs while the
+// stats work holds the one database connection.
+func start(ctx context.Context, st *store.Store, hub *game.Hub, now time.Time) error {
+	begin := time.Now()
+	if n, err := st.PruneErrors(ctx, now); err != nil {
+		return fmt.Errorf("prune browser errors: %w", err)
+	} else if n > 0 {
+		slog.Info("old browser errors pruned", "rows", n)
+	}
 	expired, err := st.ExpireWaiting(ctx, now.Add(-game.DefaultIdle), now)
 	if err != nil {
 		return fmt.Errorf("expire waiting games: %w", err)
 	}
+	prepareStats(ctx, st, now)
 	saved, err := st.LoadForRestore(ctx, now.Add(-game.DefaultIdle))
 	if err != nil {
 		return fmt.Errorf("load saved games: %w", err)
@@ -113,7 +115,7 @@
 	n := hub.Restore(saved)
 	// One line for the whole restore (each game logs at DEBUG): after a busy
 	// day, a line per game would pass Railway's 500 lines/s at once.
-	slog.Info("games restored", "restored", n, "failed", len(saved)-n, "expired", expired, "duration", time.Since(start))
+	slog.Info("games restored", "restored", n, "failed", len(saved)-n, "expired", expired, "duration", time.Since(begin))
 	return nil
 }
```

Modify `store/errors.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/errors.go
+++ b/store/errors.go
@@ -77,25 +77,3 @@
 	st.Latest = latest.String
 	return st, err
 }
-
-// PruneStats deletes browser errors, quick-match searches and events from
-// before the cutoff and returns how many rows went.
-func (s *Store) PruneStats(ctx context.Context, before time.Time) (int64, error) {
-	var total int64
-	for _, q := range []string{
-		`DELETE FROM browser_errors WHERE at < ?`,
-		`DELETE FROM searches WHERE started_at < ?`,
-		`DELETE FROM events WHERE at < ?`,
-	} {
-		res, err := s.db.ExecContext(ctx, q, before.UnixMilli())
-		if err != nil {
-			return total, err
-		}
-		n, err := res.RowsAffected()
-		if err != nil {
-			return total, err
-		}
-		total += n
-	}
-	return total, nil
-}
```

- [ ] **Step 4: Run the tests, then everything**

Run: `go test -short ./cmd/server`
Expected: PASS.

Run: `go vet ./... && go test -short ./...`
Expected: every package `ok`. Also `gofmt -l store server cmd` prints nothing.

Searches and events are no longer pruned (the rebuild recounts them); browser errors are pruned at a `STATS_TZ` midnight.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go cmd/server/backfill.go cmd/server/main_test.go cmd/server/backfill_test.go store/errors.go
git commit -m "cmd/server: stats work finishes before games are restored; STATS_TZ is loaded first"
```

---

### Task 4: The page from counters

**Files:**
- Create: `store/summary.go`, `store/counters_bench_test.go`
- Delete: `store/gamesection.go`, `store/gamesection_test.go`
- Modify: `store/stats.go`, `store/stats_test.go`, `store/errors.go` (drop `ErrorStats`, `BrowserErrors`)
- Modify: `server/stats.go`, `server/stats.html`, `server/server.go` (drop `StatsZone`), `cmd/server/main.go` (drop `handler.StatsZone`)
- Modify: `server/stats_test.go`, `server/browsererror_test.go`, `server/games_test.go`

**Interfaces:**
- Consumes: `Counts`, `DayCounts`, `LatestError`, `CountersStale`, `Zone`, the bucket tables.
- Produces: `Summary`, `Summarize`, `daysFrom`, `weekday`, `histogram` (store); `statsPage` gains `LatestError`, `Stale`, `LeftKnown`, `JoinKnown` and loses `Errors`, `LengthMax`, `WaitMax`; the template gains a `hist` block.

- [ ] **Step 1: Write the failing tests**

Modify `server/browsererror_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/server/browsererror_test.go
+++ b/server/browsererror_test.go
@@ -6,6 +6,8 @@
 	"strings"
 	"testing"
 	"time"
+
+	"mamdani-chess/store"
 )
 
 func TestPageOf(t *testing.T) {
@@ -59,15 +61,21 @@
 	}
 }
 
-// errorsSoFar returns the stored browser errors through the same path the
-// page uses.
+// errorsSoFar returns the counted browser errors and the latest stored
+// message, through the same reads the page uses.
 func errorsSoFar(t *testing.T, s *Server) (int, string) {
 	t.Helper()
-	st, err := s.store.BrowserErrors(t.Context(), time.Time{}, time.Now().Add(time.Hour))
+	ctx := t.Context()
+	end := time.Now().Add(48 * time.Hour)
+	days, err := s.store.Counts(ctx, "", end.UTC().Format("2006-01-02"))
 	if err != nil {
 		t.Fatal(err)
 	}
-	return st.N, st.Latest
+	latest, err := s.store.LatestError(ctx, time.Time{}, end)
+	if err != nil {
+		t.Fatal(err)
+	}
+	return store.Summarize(days, "", end.UTC().Format("2006-01-02")).Errors, latest
 }
 
 func TestErrorBodies(t *testing.T) {
```

Modify `server/games_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/server/games_test.go
+++ b/server/games_test.go
@@ -152,12 +152,13 @@
 	if counts := eventCounts(t, s); counts["practice"] != 0 || counts["reconnect"] != 0 {
 		t.Fatalf("a playtest's events were logged: %v", counts)
 	}
-	section, err := s.store.GameSection(t.Context(), time.Time{}, time.Now().Add(time.Hour), time.UTC)
+	// A playtest writes no counters at all.
+	days, err := s.store.Counts(t.Context(), "", "9999-12-31")
 	if err != nil {
 		t.Fatal(err)
 	}
-	if section.Games != 0 || section.Road.Games != 0 {
-		t.Fatalf("the stats count a playtest: %d games (%d on the road)", section.Games, section.Road.Games)
+	if len(days) != 0 {
+		t.Fatalf("the stats count a playtest: %v", days)
 	}
 }
 
@@ -214,10 +215,11 @@
 			if err != nil {
 				t.Fatal(err)
 			}
-			section, err := st.GameSection(t.Context(), time.Time{}, time.Now().Add(time.Hour), time.UTC)
+			days, err := st.Counts(t.Context(), "", "9999-12-31")
 			if err != nil {
 				t.Fatal(err)
 			}
+			section := store.Summarize(days, "", "9999-12-31")
 			if has || len(section.Ends) != 1 || section.Ends[0] != (store.Count{Label: "Aborted", N: 1}) {
 				t.Fatalf("stats row %v, ends %v; want none and one aborted", has, section.Ends)
 			}
```

Modify `server/stats_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/server/stats_test.go
+++ b/server/stats_test.go
@@ -36,6 +36,19 @@
 	}
 }
 
+// The page says when its counters are from an older version.
+func TestStatsSaysWhenCountersAreStale(t *testing.T) {
+	for _, stale := range []bool{false, true} {
+		var b strings.Builder
+		if err := statsTemplate.Execute(&b, statsPage{Stale: stale}); err != nil {
+			t.Fatal(err)
+		}
+		if got := strings.Contains(b.String(), "a rebuild failed, see the logs"); got != stale {
+			t.Errorf("stale %v: the line shows %v", stale, got)
+		}
+	}
+}
+
 func TestStatsNeedsPassword(t *testing.T) {
 	s, ts := newTestServer(t)
 	get := func(user, pass string) (int, http.Header, string) {
@@ -215,7 +228,7 @@
 		`<h2>Games</h2>`, `<h2>Quick match</h2>`, `<h2>The road</h2>`,
 		"Checkmate", "Resignation", "Quick match 3", "Friend link 1", // games per day legend with totals
 		"How full the road gets", "Did the road decide it?", "Lost less to the road, and won",
-		"found an opponent", "0:19", "Streams reopened", "Server restarts",
+		"found an opponent", "Gave up within 30 s", "Under 1 min left", "Joined within 5 min", "Streams reopened", "Server restarts",
 		"from the 2 games with a stats row", "Rematches of friend games", "pieces, the Mamdani included",
 		"Mon 10 am–12 pm: 0 games", "Mon 10 pm–12 am: 0 games",
 	} {
```

Create `store/counters_bench_test.go`:

```go
package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// Run with: go test ./store -run XXX -bench . -benchtime 5x
// The numbers go in the PR; Railway's vCPU is about 3–5× slower.

// BenchmarkEndGame is one counted game end: the update, the final turn,
// the stats row, the clock seek and the counters, in one transaction.
func BenchmarkEndGame(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	ctx := b.Context()
	st := &GameStats{Moves: 40, Opened: 12, Reset: 1, ClosedRounds: 9, ClosedCap: 2, Repaired: 1, Fell: 3, SavingRolls: 2, Saved: 1, WhiteLost: 1, BlackLost: 2, OpenHist: [6]int{4, 8, 10, 9, 6, 3}}
	for i := 0; b.Loop(); i++ {
		code := fmt.Sprintf("B%05d", i)
		b.StopTimer()
		must(b, s.CreateGame(ctx, Game{Code: code, White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))
		b.StartTimer()
		final := &Turn{Ply: 0, Move: "e2e4", WhiteMS: 300_000, BlackMS: 200_000, At: t0}
		must(b, s.EndGame(ctx, code, Result{EndedAt: t0, Reason: "checkmate", Winner: "white"}, final, st))
	}
}

// seedGames writes n finished quick games with stats and two turns each,
// spread over days, straight in SQL (fast enough for 100,000).
func seedGames(b *testing.B, s *Store, n, days int) {
	b.Helper()
	ctx := b.Context()
	start := t0.AddDate(0, 0, -days).UnixMilli()
	step := int64(days) * int64(24*time.Hour/time.Millisecond) / int64(n)
	for _, q := range []string{
		`WITH RECURSIVE i(x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM i WHERE x < ? - 1)
		 INSERT INTO games (code, white, black, created_at, ended_at, result, winner, rules, kind, joined_at)
		 SELECT printf('S%07d', x), 'w', 'b', ? + x * ?, ? + x * ? + 600000, 'checkmate', 'white', 3, 'quick', ? + x * ? FROM i`,
	} {
		if _, err := s.db.ExecContext(ctx, q, n, start, step, start, step, start, step); err != nil {
			b.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO game_stats SELECT code, 40, 12, 1, 9, 2, 1, 3, 2, 1, 1, 2, 0, '4,8,10,9,6,3' FROM games`,
		`INSERT INTO turns SELECT code, 0, 'e2e4', '', 600000, 600000, created_at FROM games`,
		`INSERT INTO turns SELECT code, 1, 'e7e5', '', 590000, 595000, created_at FROM games`,
	} {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRebuild recounts 100,000 games over 400 days. Budget: under 3 s
// on Railway, so under about 0.6–1 s here.
func BenchmarkRebuild(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	seedGames(b, s, 100_000, 400)
	for b.Loop() {
		if _, err := s.rebuild(b.Context(), t0); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCounts reads a 90-day page and its range before (180 days) out
// of 400 days of counters, the way the page does. It doesn't depend on how
// many games there were.
func BenchmarkCounts(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	seedGames(b, s, 20_000, 400)
	if _, err := s.rebuild(b.Context(), t0); err != nil {
		b.Fatal(err)
	}
	to := dayOf(t0.AddDate(0, 0, 1), time.UTC)
	from := dayOf(t0.AddDate(0, 0, -179), time.UTC)
	for b.Loop() {
		days, err := s.Counts(b.Context(), from, to)
		if err != nil {
			b.Fatal(err)
		}
		_ = Summarize(days, from, to)
	}
}
```

Delete `store/gamesection_test.go`:

```bash
git rm -q store/gamesection_test.go
```

Modify `store/stats_test.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/stats_test.go
+++ b/store/stats_test.go
@@ -15,26 +15,72 @@
 	return v
 }
 
-// day returns the instant at hour o'clock in New York on day d of October 2026.
-func day(d, hour int) time.Time { return time.Date(2026, 10, d, hour, 0, 0, 0, newYork) }
-
-func TestDayRange(t *testing.T) {
+func TestDaysFrom(t *testing.T) {
 	for _, c := range []struct {
-		name               string
-		from, to, earliest time.Time
-		want               []string
+		from, to string
+		want     []string
 	}{
-		{"a range", day(1, 0), day(4, 0), time.Time{}, []string{"2026-10-01", "2026-10-02", "2026-10-03"}},
-		// Across the autumn clock change each day is still one row.
-		{"across DST", time.Date(2026, 10, 31, 0, 0, 0, 0, newYork), time.Date(2026, 11, 3, 0, 0, 0, 0, newYork), time.Time{},
-			[]string{"2026-10-31", "2026-11-01", "2026-11-02"}},
-		// All time starts at the earliest row's day, in the stats zone:
-		// 03:00 UTC on Oct 3 is still Oct 2 in New York.
-		{"all time", time.Time{}, day(4, 0), time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC), []string{"2026-10-02", "2026-10-03"}},
-		{"all time, nothing yet", time.Time{}, day(4, 0), time.Time{}, nil},
+		{"2026-10-01", "2026-10-04", []string{"2026-10-01", "2026-10-02", "2026-10-03"}},
+		// Calendar days: the autumn clock change is still one row a day.
+		{"2026-10-31", "2026-11-03", []string{"2026-10-31", "2026-11-01", "2026-11-02"}},
+		{"2026-12-31", "2027-01-02", []string{"2026-12-31", "2027-01-01"}},
+		{"2026-10-04", "2026-10-04", nil},
 	} {
-		if got := dayRange(c.from, c.to, c.earliest, newYork); !reflect.DeepEqual(got, c.want) {
-			t.Errorf("%s: %v, want %v", c.name, got, c.want)
+		if got := daysFrom(c.from, c.to); !reflect.DeepEqual(got, c.want) {
+			t.Errorf("daysFrom(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
 		}
 	}
 }
+
+func TestSummarize(t *testing.T) {
+	days := []DayCounts{
+		{Day: "2026-10-05", N: map[string]int{ // a Monday
+			"games.quick": 2, "games.friend": 1, "event.practice": 1, "got_going": 2, "heat.10": 2,
+			"end.checkmate": 1, "end.roll": 1, "end.aborted": 1, "len.11-20": 2, "left.lt30s": 1, "left.5m+": 1, "clock.decided": 1,
+			"friend.links": 1, "friend.joined": 1, "join.1-5m": 1, "friend.finished": 1,
+			"search": 3, "search.matched": 2, "wait.lt10s": 2, "search.gaveup": 1, "gaveup.30-60s": 1, "search.alone": 1,
+			"road.games": 2, "road.opened": 9, "road.fell": 3, "road.no_fall": 1, "open.0": 4, "open.5": 1, "decided.less": 1, "decided.same": 1,
+		}},
+		{Day: "2026-10-07", N: map[string]int{"games.friend": 1, "end.expired": 1, "friend.links": 1, "event.restart": 1, "event.reconnect": 2, "errors": 3}},
+	}
+	got := Summarize(days, "2026-10-05", "2026-10-08")
+	if want := []KindDay{{"2026-10-05", 2, 1, 1}, {"2026-10-06", 0, 0, 0}, {"2026-10-07", 0, 1, 0}}; !reflect.DeepEqual(got.Days, want) {
+		t.Errorf("days %v, want %v", got.Days, want)
+	}
+	if got.Heat[0][10] != 2 {
+		t.Errorf("heat: Monday 8–10 pm block = %d, want 2", got.Heat[0][10])
+	}
+	if want := []Count{{"Checkmate", 1}, {"Mate by a roll", 1}, {"Aborted", 1}, {"Nobody came", 1}}; !reflect.DeepEqual(got.Ends, want) {
+		t.Errorf("ends %v", got.Ends)
+	}
+	checks := []struct {
+		name      string
+		got, want int
+	}{
+		{"games", got.Games, 4}, {"got going", got.GotGoing, 2}, {"on the clock", got.OnClock, 1},
+		{"11–20 moves", got.Lengths[1].N, 2}, {"under 1 min left", got.LeftUnder1m, 1},
+		{"links", got.Friend.Links, 2}, {"joined", got.Friend.Joined, 1}, {"joined within 5 min", got.Friend.JoinedIn5m, 1}, {"finished", got.Friend.Finished, 1},
+		{"searches", got.Quick.Searches, 3}, {"waits under 10 s", got.Quick.Waits[0].N, 2}, {"gave up 30–60 s", got.Quick.GaveUpAfter[2].N, 1},
+		{"gave up within 30 s", got.Quick.GaveUpIn30s, 0}, {"alone", got.Quick.Alone, 1},
+		{"road games", got.Road.Games, 2}, {"opened", got.Road.Opened, 9}, {"no fall", got.Road.NoFall, 1}, {"open 5", got.Road.OpenHist[5], 1},
+		{"less won", got.Road.LessWon, 1}, {"same", got.Road.Same, 1},
+		{"reconnects", got.Reconnects, 2}, {"restarts", got.Restarts, 1}, {"errors", got.Errors, 3},
+	}
+	for _, c := range checks {
+		if c.got != c.want {
+			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
+		}
+	}
+	// Every histogram keeps every bucket, quiet ones at zero.
+	if len(got.Lengths) != 8 || len(got.WinnerLeft) != 5 || len(got.Friend.Join) != 5 || len(got.Quick.Waits) != 5 || len(got.Quick.GaveUpAfter) != 5 {
+		t.Errorf("bucket counts %d %d %d %d %d", len(got.Lengths), len(got.WinnerLeft), len(got.Friend.Join), len(got.Quick.Waits), len(got.Quick.GaveUpAfter))
+	}
+	// All time starts at the first day with games or practice.
+	all := Summarize(append([]DayCounts{{Day: "2026-09-01", N: map[string]int{"errors": 1}}}, days...), "", "2026-10-08")
+	if len(all.Days) != 3 || all.Days[0].Day != "2026-10-05" {
+		t.Errorf("all time days %v", all.Days)
+	}
+	if empty := Summarize(nil, "", "2026-10-08"); empty.Days != nil {
+		t.Errorf("all time with nothing: %v", empty.Days)
+	}
+}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test -short ./store ./server`
Expected: FAIL: `undefined: Summarize`, `undefined: daysFrom`, and the page test lacks "Gave up within 30 s".

- [ ] **Step 3: Write the code**

Modify `cmd/server/main.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/cmd/server/main.go
+++ b/cmd/server/main.go
@@ -55,7 +55,6 @@
 	// The stats page (specs: docs/superpowers/specs/2026-10-08-stats-design.md
 	// and 2026-10-19-stats-counters-design.md).
 	handler.StatsPassword = os.Getenv("STATS_PASSWORD")
-	handler.StatsZone = zone
 	srv := &http.Server{
 		Addr:              ":" + port,
 		Handler:           handler,
```

Modify `server/server.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/server/server.go
+++ b/server/server.go
@@ -34,8 +34,6 @@
 	// StatsPassword guards GET /stats (Basic Auth, any user name). Empty
 	// means the page is off: the path serves the app shell like any other.
 	StatsPassword string
-	// StatsZone groups the stats page's days; nil means UTC.
-	StatsZone *time.Location
 }
 
 // New returns a Server for the games in hub that serves the frontend from
```

Modify `server/stats.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/server/stats.go
+++ b/server/stats.go
@@ -1,6 +1,7 @@
 package server
 
 import (
+	"context"
 	"crypto/subtle"
 	_ "embed"
 	"fmt"
@@ -145,20 +146,23 @@
 type statsPage struct {
 	Range, Days string
 	Ranges      []statsRangeOption
-	Errors      store.ErrorStats
+	LatestError string
+	// Stale says the counters are from an older version: a rebuild failed.
+	Stale bool
 	// Each KPI against the range of the same length before; empty for
 	// all time. Before names that range.
 	Before                           string
 	Games, GotGoing, Practice, Found change
 
-	Section                                 store.GameSection
-	QuickTotal, FriendTotal, PracticeTotal  int
-	DayKindMax, LengthMax, WaitMax, OpenMax int
-	OpenPct                                 [6]int // each open-holes count as a percentage of turns
-	Decisive                                int
-	FriendRows                              []countRow
-	DecidedRows                             []countRow
-	HeatRows                                []heatRow
+	Section                                store.Summary
+	QuickTotal, FriendTotal, PracticeTotal int
+	DayKindMax, OpenMax                    int
+	OpenPct                                [6]int // each open-holes count as a percentage of turns
+	LeftKnown, JoinKnown                   int    // games with a winner's clock; joins with a known time
+	Decisive                               int
+	FriendRows                             []countRow
+	DecidedRows                            []countRow
+	HeatRows                               []heatRow
 }
 
 // countRow is a labelled count, with the colour of its bar when it has one.
@@ -236,30 +240,41 @@
 		http.Error(w, "stats need the password", http.StatusUnauthorized)
 		return
 	}
-	loc := s.StatsZone
-	if loc == nil {
-		loc = time.UTC
-	}
+	ctx, cancel := context.WithTimeout(r.Context(), statsTimeout)
+	defer cancel()
+	loc := s.store.Zone()
 	days := r.URL.Query().Get("days")
 	from, to, label := statsRange(days, time.Now(), loc)
-	section, err := s.store.GameSection(r.Context(), from, to, loc)
+	fromDay, toDay := "", dayOf(to, loc)
+	readFrom := ""
+	var prevFrom time.Time
+	if !from.IsZero() {
+		fromDay = dayOf(from, loc)
+		prevFrom = rangeBefore(from, to)
+		readFrom = dayOf(prevFrom, loc)
+	}
+	// One read for the range and the range before it.
+	counted, err := s.store.Counts(ctx, readFrom, toDay)
 	if err != nil {
-		s.internalError(w, "game section", err)
+		s.internalError(w, "stats counters", err)
 		return
 	}
-	errs, err := s.store.BrowserErrors(r.Context(), from, to)
+	split := len(counted)
+	for i, d := range counted {
+		if d.Day >= fromDay {
+			split = i
+			break
+		}
+	}
+	section := store.Summarize(counted[split:], fromDay, toDay)
+	latest, err := s.store.LatestError(ctx, from, to)
 	if err != nil {
-		s.internalError(w, "browser errors", err)
+		s.internalError(w, "latest error", err)
 		return
 	}
-	page := statsPage{Range: label, Days: days, Errors: errs}
+	page := statsPage{Range: label, Days: days, LatestError: latest, Stale: s.store.CountersStale()}
 	if !from.IsZero() {
-		prevFrom := rangeBefore(from, to)
-		prev, err := s.store.GameSection(r.Context(), prevFrom, from, loc)
-		if err != nil {
-			s.internalError(w, "game section before", err)
-			return
-		}
+		prev := store.Summarize(counted[:split], readFrom, fromDay)
 		page.Before = "vs " + rangeLabel(prevFrom, from)
 		page.Games = pctChange(section.Games, prev.Games)
 		page.GotGoing = pctChange(section.GotGoing, prev.GotGoing)
@@ -277,11 +292,11 @@
 		page.PracticeTotal += d.Practice
 		page.DayKindMax = max(page.DayKindMax, d.Quick+d.Friend+d.Practice)
 	}
-	for _, c := range section.Lengths {
-		page.LengthMax = max(page.LengthMax, c.N)
+	for _, c := range section.WinnerLeft {
+		page.LeftKnown += c.N
 	}
-	for _, c := range section.Quick.Waits {
-		page.WaitMax = max(page.WaitMax, c.N)
+	for _, c := range section.Friend.Join {
+		page.JoinKnown += c.N
 	}
 	turns := 0
 	for _, n := range section.Road.OpenHist {
@@ -307,8 +322,15 @@
 	}
 }
 
-// practiceGames is how many practice games a section's days started.
-func practiceGames(g store.GameSection) int {
+// statsTimeout bounds the page's reads, so it can never hold the one
+// connection long enough to keep a game's save waiting.
+const statsTimeout = 2 * time.Second
+
+// dayOf is t's day in loc, as the counters store it.
+func dayOf(t time.Time, loc *time.Location) string { return t.In(loc).Format("2006-01-02") }
+
+// practiceGames is how many practice games a summary's days started.
+func practiceGames(g store.Summary) int {
 	n := 0
 	for _, d := range g.Days {
 		n += d.Practice
```

Modify `server/stats.html` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/server/stats.html
+++ b/server/stats.html
@@ -37,6 +37,7 @@
 .g2{grid-template-columns:repeat(2,minmax(0,1fr))}.g21{grid-template-columns:minmax(0,1.7fr) minmax(0,1fr)}
 .chart{display:flex;flex-direction:column;gap:8px}.hist{height:130px;display:flex;align-items:flex-end;gap:6px;border-bottom:1px solid var(--line)}.hcol{flex:1;min-width:0;height:100%;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;gap:6px;font:500 12px "IBM Plex Mono",monospace;color:var(--text-2)}.hcol i{display:block;flex:none;width:100%;background:var(--a);border-radius:4px 4px 0 0}.hlab{display:flex;gap:6px}.hlab span{flex:1;min-width:0;text-align:center;font:500 11px "IBM Plex Mono",monospace;color:var(--text-3)}
 .list{display:flex;flex-direction:column}.list>div{display:flex;justify-content:space-between;gap:12px;padding:10px 0;border-bottom:1px solid var(--line);font-size:13px}.list>div:last-child{border-bottom:0;padding-bottom:0}
+.stale{margin:0;padding:12px 16px;border:1px solid #d4a72c;border-radius:var(--r);background:#fff8c5;color:#4d2d00;font-size:13px}
 .heat{display:flex;flex-direction:column;gap:4px}.heat>div{display:grid;grid-template-columns:34px repeat(12,minmax(0,1fr));gap:4px;align-items:center}.heat span{font-size:12px;color:var(--text-3)}.heat .h span{font:500 11px "IBM Plex Mono",monospace;text-align:center}.heat i{display:block;height:22px;border-radius:3px}.l0{background:var(--s0)}.l1{background:var(--s1)}.l2{background:var(--s2)}.l3{background:var(--s3)}.l4{background:var(--s4)}
 .scale{display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-3)}.scale i{width:14px;height:14px;border-radius:4px}
 .share{display:flex;height:10px;gap:3px}.share i{display:block;border-radius:999px}
@@ -51,6 +52,7 @@
 <div><h1>Stats</h1><p class="sub">{{.Range}}</p></div>
 <div class="seg" role="group" aria-label="Range">{{range .Ranges}}<a href="?days={{.Days}}"{{if .On}} class="on"{{end}}>{{.Label}}</a>{{end}}</div>
 </div>
+{{if .Stale}}<p class="stale">These counts are from an older version: a rebuild failed, see the logs.</p>{{end}}
 
 <section id="overview">
 <h2>Overview</h2>
@@ -73,9 +75,8 @@
 <div class="grid g2">
 {{template "list" dict "Title" "How games end" "Sub" (printf "%d games created in the range" .Section.Games) "Rows" .Section.Ends "Top" (top .Section.Ends) "Of" .Section.Games}}
 <div class="card"><div><h3>Game length</h3><p class="sub">Moves by both sides, games that got going</p></div>
-<div class="chart"><div class="hist">{{$m := .LengthMax}}{{range .Section.Lengths}}<div class="hcol">{{.N}}<i style="height:calc((100% - 24px) * {{pct .N $m}} / 100)"></i></div>{{end}}</div>
-<div class="hlab">{{range .Section.Lengths}}<span>{{.Label}}</span>{{end}}</div></div>
-<div class="list"><div><span>Median game</span><span class="val">{{.Section.MedianMoves}} moves · {{.Section.MedianMinutes}} min</span></div><div><span>Decided on the clock</span><span class="val">{{.Section.OnClock}} of {{.Section.GotGoing}} · {{share .Section.OnClock .Section.GotGoing}}%</span></div><div><span>Winner's clock left (median)</span><span class="val">{{or .Section.WinnerClockLeft "–"}}</span></div></div>
+{{template "hist" .Section.Lengths}}
+<div class="list"><div><span>Decided on the clock</span><span class="val">{{.Section.OnClock}} of {{.Section.GotGoing}} · {{share .Section.OnClock .Section.GotGoing}}%</span></div></div>
 </div>
 </div>
 <div class="grid g21">
@@ -83,23 +84,37 @@
 <div class="heat"><div class="h"><span></span><span>12a</span><span>2a</span><span>4a</span><span>6a</span><span>8a</span><span>10a</span><span>12p</span><span>2p</span><span>4p</span><span>6p</span><span>8p</span><span>10p</span></div>
 {{range .HeatRows}}<div><span>{{.Day}}</span>{{range .Cells}}<i class="l{{.Level}}" title="{{.Title}}"></i>{{end}}</div>{{end}}</div>
 </div>
+<div class="card"><div><h3>Winner's clock left</h3><p class="sub">At the end of games with a winner</p></div>
+{{template "hist" .Section.WinnerLeft}}
+<div class="list"><div><span>Under 1 min left</span><span class="val">{{.Section.LeftUnder1m}} of {{.LeftKnown}} · {{share .Section.LeftUnder1m .LeftKnown}}%</span></div></div>
+</div>
+</div>
+<div class="grid g2">
 <div class="card"><div><h3>Friend links</h3><p class="sub">{{.Section.Friend.Links}} links made</p></div>
 <div class="rows">{{$links := .Section.Friend.Links}}{{range .FriendRows}}<div class="row"><div class="row-top"><span>{{.Label}}</span><span class="val">{{.N}} <small>{{share .N $links}}%</small></span></div><div class="track thick"><i style="width:{{pct .N $links}}%"></i></div></div>{{end}}</div>
-<div class="list"><div><span>Median time to join</span><span class="val">{{or .Section.Friend.MedianJoin "–"}}</span></div><div><span>Rematches of friend games</span><span class="val">{{.Section.Friend.Rematches}}</span></div></div>
+<div class="list"><div><span>Rematches of friend games</span><span class="val">{{.Section.Friend.Rematches}}</span></div></div>
 </div>
+<div class="card"><div><h3>Time to join</h3><p class="sub">From the link being made to a friend sitting down</p></div>
+{{template "hist" .Section.Friend.Join}}
+<div class="list"><div><span>Joined within 5 min</span><span class="val">{{.Section.Friend.JoinedIn5m}} of {{.JoinKnown}} · {{share .Section.Friend.JoinedIn5m .JoinKnown}}%</span></div></div>
 </div>
+</div>
 </section>
 
 <section id="match">
 <div><h2>Quick match</h2><p class="sub">{{.Section.Quick.Searches}} searches</p></div>
-<div class="grid g21">
+<div class="grid g3">
 <div class="card"><div><h3>How long people waited</h3><p class="sub">Searches that found an opponent</p></div>
-<div class="chart"><div class="hist">{{$w := .WaitMax}}{{range .Section.Quick.Waits}}<div class="hcol">{{.N}}<i style="height:calc((100% - 24px) * {{pct .N $w}} / 100)"></i></div>{{end}}</div>
-<div class="hlab">{{range .Section.Quick.Waits}}<span>{{.Label}}</span>{{end}}</div></div></div>
+{{template "hist" .Section.Quick.Waits}}
+</div>
+<div class="card"><div><h3>Gave up after</h3><p class="sub">Searches that ended without one</p></div>
+{{template "hist" .Section.Quick.GaveUpAfter}}
+<div class="list"><div><span>Gave up within 30 s</span><span class="val">{{.Section.Quick.GaveUpIn30s}} of {{.Section.Quick.GaveUp}} · {{share .Section.Quick.GaveUpIn30s .Section.Quick.GaveUp}}%</span></div></div>
+</div>
 <div class="card"><div><h3>Found an opponent</h3><p class="sub">Out of {{.Section.Quick.Searches}} searches</p></div>
 <div class="big"><span class="num">{{share .Section.Quick.Matched .Section.Quick.Searches}}%</span><span class="sub">{{.Section.Quick.Matched}} matched · {{.Section.Quick.GaveUp}} gave up</span></div>
 <div class="share"><i style="flex:{{.Section.Quick.Matched}} 1 0;background:var(--c1)"></i><i style="flex:{{.Section.Quick.GaveUp}} 1 0;background:var(--a4)"></i></div>
-<div class="list"><div><span>Median wait</span><span class="val">{{or .Section.Quick.MedianWait "–"}}</span></div><div><span>Gave up after (median)</span><span class="val">{{or .Section.Quick.MedianGaveUp "–"}}</span></div><div><span>Searched when nobody else was</span><span class="val">{{.Section.Quick.Alone}} · {{share .Section.Quick.Alone .Section.Quick.Searches}}%</span></div></div>
+<div class="list"><div><span>Searched when nobody else was</span><span class="val">{{.Section.Quick.Alone}} · {{share .Section.Quick.Alone .Section.Quick.Searches}}%</span></div></div>
 </div>
 </div>
 </section>
@@ -128,7 +143,7 @@
 <section id="health">
 <h2>Health</h2>
 <div class="grid g3">
-<div class="tile"><span>Browser errors</span><span class="num">{{.Errors.N}}</span><span class="note">{{if .Errors.Latest}}latest: {{.Errors.Latest}}{{else}}none in this range{{end}}</span></div>
+<div class="tile"><span>Browser errors</span><span class="num">{{.Section.Errors}}</span><span class="note">{{if .LatestError}}latest: {{.LatestError}}{{else}}none in this range{{end}}</span></div>
 <div class="tile"><span>Streams reopened</span><span class="num">{{.Section.Reconnects}}</span><span class="note">the page reopened a game stream after an error; the browser's own retries aren't counted</span></div>
 <div class="tile"><span>Server restarts</span><span class="num">{{.Section.Restarts}}</span><span class="note">each one pauses the games in progress</span></div>
 </div>
@@ -141,6 +156,9 @@
 
 {{define "chip"}}{{with .C}}{{if .Text}}<span class="chip {{.Dir}}" title="{{$.Before}}">{{if eq .Dir "up"}}<svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M1.5 8.5 4.5 5.5 6.5 7.5 10.5 3.5M7.5 3.5H10.5V6.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>{{else if eq .Dir "down"}}<svg width="12" height="12" viewBox="0 0 12 12" aria-hidden="true"><path d="M1.5 3.5 4.5 6.5 6.5 4.5 10.5 8.5M7.5 8.5H10.5V5.5" fill="none" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>{{end}}{{.Text}}</span>{{end}}{{end}}{{end}}
 
+{{define "hist"}}<div class="chart"><div class="hist">{{$m := top .}}{{range .}}<div class="hcol">{{.N}}<i style="height:calc((100% - 24px) * {{pct .N $m}} / 100)"></i></div>{{end}}</div>
+<div class="hlab">{{range .}}<span>{{.Label}}</span>{{end}}</div></div>{{end}}
+
 {{define "list"}}<div class="card"><div><h3>{{.Title}}</h3><p class="sub">{{.Sub}}</p></div>
 <div class="rows">{{$of := .Of}}{{range .Rows}}
 <div class="row"><div class="row-top"><span>{{.Label}}</span><span class="val">{{.N}} <small>{{share .N $of}}%</small></span></div><div class="track"><i style="width:{{pct .N $.Top}}%"></i></div></div>{{end}}
```

Modify `store/errors.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/errors.go
+++ b/store/errors.go
@@ -58,22 +58,3 @@
 	}
 	return msg, err
 }
-
-// ErrorStats is what the Health section shows of the browser errors in a
-// range: how many, and the latest message.
-type ErrorStats struct {
-	N      int
-	Latest string
-}
-
-// BrowserErrors counts the errors with from <= at < to.
-func (s *Store) BrowserErrors(ctx context.Context, from, to time.Time) (ErrorStats, error) {
-	var st ErrorStats
-	var latest sql.NullString
-	a, b := from.UnixMilli(), to.UnixMilli()
-	err := s.db.QueryRowContext(ctx,
-		`SELECT COUNT(*), (SELECT message FROM browser_errors WHERE at >= ? AND at < ? ORDER BY at DESC LIMIT 1) FROM browser_errors WHERE at >= ? AND at < ?`,
-		a, b, a, b).Scan(&st.N, &latest)
-	st.Latest = latest.String
-	return st, err
-}
```

Delete `store/gamesection.go`:

```bash
git rm -q store/gamesection.go
```

Modify `store/stats.go` (apply this diff; `git apply` takes it from the repo root):

```diff
--- a/store/stats.go
+++ b/store/stats.go
@@ -1,32 +1,11 @@
 package store
 
-import "time"
-
 // Count is one labelled count, a row of a list or a histogram's column.
 type Count struct {
 	Label string
 	N     int
 }
 
-// dayRange lists the days (2006-01-02, in loc) from from's up to but not
-// including to. A zero from starts at earliest's day instead, and with no
-// earliest either there are no days.
-func dayRange(from, to, earliest time.Time, loc *time.Location) []string {
-	start := from
-	if start.IsZero() {
-		if earliest.IsZero() {
-			return nil
-		}
-		start = earliest
-	}
-	var days []string
-	y, m, d := start.In(loc).Date()
-	for t := time.Date(y, m, d, 0, 0, 0, 0, loc); t.Before(to); t = t.AddDate(0, 0, 1) {
-		days = append(days, t.Format("2006-01-02"))
-	}
-	return days
-}
-
 // finishedReasons are the results of a game that was played to the end
 // (not aborted, expired or retired).
 const finishedReasons = `('checkmate','stalemate','fifty_moves','repetition','insufficient_material','timeout','timeout_vs_insufficient','resignation')`
```

Create `store/summary.go`:

```go
package store

import (
	"strconv"
	"time"
)

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
	for _, d := range days {
		for k, n := range d.N {
			sum[k] += n
		}
	}
	start := fromDay
	if start == "" {
		for _, d := range days {
			if d.N["games.quick"]+d.N["games.friend"]+d.N["event.practice"] > 0 {
				start = d.Day
				break
			}
		}
	}
	byDay := map[string]map[string]int{}
	for _, d := range days {
		byDay[d.Day] = d.N
	}
	if start != "" {
		for _, day := range daysFrom(start, toDay) {
			n := byDay[day]
			out.Days = append(out.Days, KindDay{Day: day, Quick: n["games.quick"], Friend: n["games.friend"], Practice: n["event.practice"]})
		}
	}
	for _, d := range days {
		wd := weekday(d.Day)
		for b := range 12 {
			out.Heat[wd][b] += d.N["heat."+strconv.Itoa(b)]
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
	t, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil
	}
	var out []string
	for d := from; d < to; d = t.Format("2006-01-02") {
		out = append(out, d)
		t = t.AddDate(0, 0, 1)
	}
	return out
}

// weekday is a day's row in Heat: Monday 0 to Sunday 6.
func weekday(day string) int {
	t, err := time.Parse("2006-01-02", day)
	if err != nil {
		return 0
	}
	return (int(t.Weekday()) + 6) % 7
}
```

- [ ] **Step 4: Run the tests, then everything**

Run: `go test -short ./store ./server`
Expected: PASS.

Run: `go vet ./... && go test -race -short ./...`
Expected: every package `ok`. Also `gofmt -l store server cmd` prints nothing.

Then run the benchmarks and keep their numbers for the PR (on an M4 Pro they were: `BenchmarkEndGame` 0.19 ms, `BenchmarkRebuild` 0.46 s, `BenchmarkCounts` 2.5 ms):

```bash
go test ./store -run XXX -bench . -benchtime 3x
```

- [ ] **Step 5: Commit**

```bash
git add store/summary.go store/counters_bench_test.go store/gamesection.go store/gamesection_test.go store/stats.go store/stats_test.go store/errors.go server/stats.go server/stats.html server/server.go cmd/server/main.go server/stats_test.go server/browsererror_test.go server/games_test.go
git commit -m "server: /stats sums daily counters; medians become histograms with a threshold line"
```

---

### Task 5: Docs, end-to-end checks and the PR

**Files:**
- Modify: `CLAUDE.md`, `docs/superpowers/specs/2026-10-19-stats-counters-design.md`, `docs/superpowers/specs/2026-10-08-stats-design.md`

- [ ] **Step 1: Update CLAUDE.md**

In "Where things stand", after the line that starts `- **Done: stats, plan 2**`, add:

```markdown
- **Done: stats counters** ([spec](docs/superpowers/specs/2026-10-19-stats-counters-design.md), [plan](docs/superpowers/plans/2026-10-19-stats-counters.md)): every counted write (creating, joining, ending and expiring games, the game-stats backfill, searches, events, browser errors) adds to daily counters (`stats_days`) in its own transaction; `/stats` sums them and never scans games. Medians became histograms with a threshold line.
```

In "Production logs", replace

```markdown
A start that backfills game stats logs `game stats backfilled` at INFO, and each game that won't replay logs `game stats not backfilled` at WARN.
```

with

```markdown
A start that backfills game stats logs `game stats backfilled` at INFO, and each game that won't replay logs `game stats not backfilled` at WARN. A start that rebuilds the stats counters (a new counters version or `STATS_TZ`) logs `stats rebuilt` with days, rows and duration; a failed rebuild logs `stats counters not rebuilt` at ERROR and the page says its counts are from an older version.
```

In the "Stats" working note, replace

```markdown
`game_stats` is written in `EndGame`'s transaction; `game.StatsOf` replays a saved game for the backfill (idempotent, runs at startup; a failure is logged at ERROR and the server starts anyway).
```

with

```markdown
`game_stats` is written in `EndGame`'s transaction; `game.StatsOf` replays a saved game for the backfill (idempotent, runs at startup; a failure is logged at ERROR and the server starts anyway). The page reads only `stats_days` (one row per day and key, days in `STATS_TZ`, a game counted on the day it was created). The counting functions in `store/counts.go` are shared by the live writes and the startup rebuild; change what a key means only together with `countersVersion`, which rebuilds the table on the next start. Startup prunes browser errors, expires waiting games, backfills, rebuilds if needed and counts the restart before it restores any game.
```

- [ ] **Step 2: Update the specs**

In `docs/superpowers/specs/2026-10-19-stats-counters-design.md`, replace the status line with:

```markdown
**Status:** Built on branch `stats-counters` ([plan](../plans/2026-10-19-stats-counters.md)), stacked on PR #25.
```

and in "How a write counts", replace the bullet that starts `- **Only what changed counts, and a no-op is not an error.**` with:

```markdown
- **Only what changed counts, and a no-op is not an error.** A guarded update that changes no row (a repeated `EndGame` or `SeatBlack`) counts nothing and returns nil; one for a game that was never saved still returns an error, as before (a primary-key check that runs only when no row changed). Today `execOne` returns an error when no row changed, which would retire the game; these two calls stop using it, and `execOne` goes. `AddGameStats` counts only when its `INSERT OR IGNORE` inserted, and moves the game's counters from "ended, road unknown" to what the stats say.
```

and in "Reading the page", replace the line that starts `- **Quick match:**` with:

```markdown
- **Quick match:** three cards: the wait histogram, **Gave up after** (a histogram, with "gave up within 30 s: N%": `lt10s` + `10-30s` over `search.gaveup`), and found an opponent with "searched when nobody else was". The median wait line goes.
```

and replace the line that starts `- **Friend links:**` with:

```markdown
- **Friend links:** joined, finished and rematches as now; the median time to join becomes its own card, **Time to join**, a histogram with "joined within 5 min: N%" (`lt1m` + `1-5m` over the joins with a known time).
```

In `docs/superpowers/specs/2026-10-08-stats-design.md`, replace the status line's end `Plan 1's visit tracking was removed before it shipped (2026-10-19, below).` with `Plan 1's visit tracking was removed before it shipped (2026-10-19, below). The page reads daily counters ([counters spec](2026-10-19-stats-counters-design.md)).`

- [ ] **Step 3: Check everything end to end**

```bash
go vet ./... && go test -race -short ./...
pnpm --dir web build
CONTRACT=1 go test ./contract -count=1
```

Expected: all `ok`; the contract run takes about 16 s.

Then, with a built server on a spare port and a database seeded with made-up games (raw rows only, so the first start rebuilds the counters), open `/stats?days=7` at 1440 and 390 px: every card renders, nothing overflows its card, no sideways scroll, and the numbers match the same seed on `stats-games` (the scan version). Run `pnpm --dir web playtest --base http://localhost:<port> --games 1 --match` and check `SELECT COUNT(*), SUM(n) FROM stats_days` is unchanged afterwards (playtests write no counters). Stop the server by its PID.

- [ ] **Step 4: Commit**

```bash
git add CLAUDE.md docs/superpowers/specs/2026-10-19-stats-counters-design.md docs/superpowers/specs/2026-10-08-stats-design.md docs/superpowers/plans/2026-10-19-stats-counters.md
git commit -m "Docs: the stats counters are built"
```

- [ ] **Step 5: Push and open the PR (only with the user's OK)**

Push `stats-counters` and open a PR against `stats-games` (it moves to `main` once #25 merges), with the benchmark numbers from Task 4 and screenshots of the new cards at 1440 and 390 px on the `pr-screenshots` branch.
