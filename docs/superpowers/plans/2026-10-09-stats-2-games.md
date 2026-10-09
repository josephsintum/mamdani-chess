# Stats 2: Games, Quick Match and The Road Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The `/stats` page gains its Games, Quick match and The road sections, and Health gains reconnects and restarts, as the stats spec describes: every game records what the road did to it when it ends (old games are backfilled once), every quick-match search is logged, and the page reads it all.

**Architecture:**
- **`store`:** migration 8 adds `games.kind` and `games.joined_at`, a `game_stats` row per finished game, a `searches` table and an `events` table (practice games, reconnects, restarts). `store/gamestats.go` writes them; `store/gamesection.go` reads everything the three sections show, aggregated in Go (small data, zone-aware days).
- **`game`:** a `road` accumulator tallies each turn's events (opened, reset, closed by rounds or the cap, repaired, fell, saving rolls, pieces each side lost, open holes after the turn); it is rebuilt by restore's replay, so it survives a restart. `end()` hands it to `EndGame` in the same transaction as the result. `game.StatsOf` replays a saved game's turns for the backfill. Games carry a kind (`friend` or `quick`; a rematch inherits).
- **`server`:** the quick-match stream logs each search; `createPractice` and a reconnecting game stream log events; `/stats` renders the new sections.
- **`cmd/server`:** logs a restart event and backfills `game_stats` at startup.
- **Frontend:** the game page marks a reconnecting stream with `?again=1`. Nothing else changes.

**Tech Stack:** Go 1.27, `modernc.org/sqlite` through `database/sql`, `html/template`, SvelteKit 3 / Svelte 5.57, Vitest, Playwright (playtest).

**Spec:** [docs/superpowers/specs/2026-10-08-stats-design.md](../specs/2026-10-08-stats-design.md), sections "The page" and "Where each number comes from". Read it first; this plan argues from it. The design: the canvas https://claude.ai/artifact/Rzwy4PSQuJoKNZ629PYogL (Full site tab), Games / Quick match / The road cards. Plan 1 (visits, the page's frame) is [2026-10-08-stats-1-visits.md](2026-10-08-stats-1-visits.md); this plan builds on its branch.

**How this plan was made:** from a close reading of the code on branch `stats-visits` (the signatures and line numbers below were checked there), not from a scratch build. Every task ends by running its tests; a step that doesn't compile shows up at once.

## Global Constraints

- **Migrations are append only:** add migration 8; never edit 1–7.
- **No new dependencies.** Don't edit `go.mod` or `names/`.
- **Nothing identifying is stored:** `searches.guest` is the 12-hex visitor key (`visitorOf(guestID)`), never the full guest ID. No IP, no game code in any new text column.
- **Game stats are written in `EndGame`'s transaction** (`store/games.go`), so a game never has a result without its stats or the reverse. Practice games are never saved and get no row. Games ended by SQL (`ExpireWaiting`, migrations) get no row and aren't "got going".
- **Kinds:** `games.kind` is `friend` (the default; every game saved before this plan) or `quick`; a rematch keeps the kind of the game it follows.
- **"Got going"** means the game ended with a result in the finished reasons: `checkmate, stalemate, fifty_moves, repetition, insufficient_material, timeout, timeout_vs_insufficient, resignation` (`finishedReasons` in `store/stats.go`). "Decisive" means it also has a winner.
- **Mate by a roll** is `checkmate` whose final turn's events include `rolled_pothole` (the move itself didn't mate; the dice did). It is counted apart from checkmate in "How games end" and in the road tiles.
- **JSON on the wire doesn't change** except the game stream's optional `?again=1` query (the server ignores anything else). `go test ./cmd/wiregen` must still pass without regenerating anything.
- **Frontend conventions:** `#lib/...` imports with the extension, `$app/env` not `$app/environment`, no `$effect`, run `npx @sveltejs/mcp svelte-autofixer` on every component you change.
- **Commits:** stage only the files you changed (`git add <paths>`, never `git add -A`), and add no Claude attribution (no `Co-Authored-By`, no "Generated with Claude Code").
- **Branch:** `stats-games`, branched from `main` with `stats-visits` merged in. Don't push to `main`.

## Review Focus

1. **A restart mid-game must not lose or double the road stats.** Restore replays every turn through `apply` → `tally`, which is the only place the accumulator changes. Pinned by `TestRoadStatsSurviveRestore` (Task 2).
2. **The backfill must be idempotent and never touch a game that has a row.** Pinned by `TestBackfillSkipsGamesWithStats` (Task 3): run twice, rows unchanged.
3. **A search that is cancelled, one that is matched, and one cut by a shutdown must all be logged with the right `matched` flag.** Pinned by `TestSearchLogged` (Task 3).
4. **Median and histogram arithmetic over an empty range must not panic or divide by zero.** Pinned by `TestGameSectionEmpty` (Task 4) and the template render against an empty database in `TestStatsNeedsPassword` (unchanged, Task 5).
5. **The page's new sections must never show a game code.** Nothing in the new store reads returns one; pinned by `TestGameSectionHasNoCodes` (Task 4), which seeds a game and greps every string in the result.

## File map

| File | Task | Responsibility |
| --- | --- | --- |
| `store/migrate.go` | 1 | Migration 8 |
| `store/games.go`, `store/gamestats.go`, `store/gamestats_test.go` | 1 | `Game.Kind`, `SeatBlack` with a time, `GameStats`, `EndGame` with stats, `AddGameStats`, `GamesWithoutStats`, `AddSearch`, `AddEvent` |
| `game/road.go`, `game/road_test.go` | 2 | The `road` accumulator, `StatsOf` |
| `game/game.go`, `game/save.go`, `game/hub.go`, `game/rematch.go`, `game/save_test.go` | 2 | Tally into `road`, pass stats to `EndGame`, kinds, `joined_at` |
| `server/matchmaking.go`, `server/games.go`, `server/server.go`, `server/matchmaking_test.go`, `server/games_test.go` | 3 | Search log, practice and reconnect events, `Server.Keyed` |
| `cmd/server/main.go`, `cmd/server/backfill.go`, `cmd/server/backfill_test.go` | 3 | Restart event, the backfill |
| `web/src/routes/game/[code]/+page.svelte` | 3 | `?again=1` on a reconnect |
| `store/gamesection.go`, `store/gamesection_test.go` | 4 | `GameSection`: everything the three sections and Health show |
| `server/stats.go`, `server/stats.html`, `server/stats_test.go` | 5 | The sections rendered |
| `CLAUDE.md`, the spec, the roadmap | 5 | Docs |

---

### Task 1: The tables and the writes

**Files:**
- Modify: `store/migrate.go` (append migration 8 after migration 7), `store/games.go` (`Game.Kind`, `CreateGame`, `SeatBlack`, `EndGame`, `LoadForRestore`)
- Create: `store/gamestats.go`
- Test: `store/gamestats_test.go`, and the existing `store/games_test.go` where signatures change

**Interfaces:**
- Consumes: `execOne`, `nullable`, `addTurn`, `rulesVersion`, `finishedReasons` (store package); `openTemp`, `must`, `t0` (tests).
- Produces:
  - `Game.Kind string` (`"friend"` or `"quick"`; `CreateGame` writes it, `""` is saved as `friend`).
  - `func (s *Store) SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error` (sets `joined_at`).
  - `type GameStats struct { Moves, Opened, Reset, ClosedRounds, ClosedCap, Repaired, Fell, SavingRolls, Saved, WhiteLost, BlackLost int; MateByRoll bool; OpenHist [6]int }` (`OpenHist[k]` = turns that ended with k holes open).
  - `func (s *Store) EndGame(ctx context.Context, code string, r Result, final *Turn, st *GameStats) error` (a nil `st` writes no row).
  - `func (s *Store) AddGameStats(ctx context.Context, code string, st GameStats) error` (the backfill; `INSERT OR IGNORE`).
  - `func (s *Store) GamesWithoutStats(ctx context.Context) ([]SavedGame, error)`: games under the current rules that got going and have no `game_stats` row, with their turns and results.
  - `type Search struct { StartedAt, EndedAt time.Time; Guest string; Matched bool; Others int }`; `func (s *Store) AddSearch(ctx context.Context, sr Search) error`.
  - `func (s *Store) AddEvent(ctx context.Context, kind string, at time.Time) error` (kinds `practice`, `reconnect`, `restart`).
  - `SavedGame.Kind` and `SavedGame.JoinedAt time.Time` (zero when unknown) come through `LoadForRestore` and `GamesWithoutStats`.

- [ ] **Step 1: Write the failing tests**

`store/gamestats_test.go`:

```go
package store

import (
	"testing"
	"time"
)

func TestGameStatsSavedWithTheResult(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.CreateGame(ctx, Game{Code: "G00001", White: "w", WhiteName: "w", Kind: "quick", CreatedAt: t0}))
	must(t, s.SeatBlack(ctx, "G00001", "b", "b", t0.Add(90*time.Second)))
	must(t, s.AddTurn(ctx, "G00001", Turn{Ply: 0, Move: "e2e4", At: t0.Add(2 * time.Minute)}))
	st := GameStats{Moves: 2, Opened: 1, Fell: 1, BlackLost: 1, SavingRolls: 1, OpenHist: [6]int{1, 1}}
	must(t, s.EndGame(ctx, "G00001", Result{EndedAt: t0.Add(3 * time.Minute), Reason: "checkmate", Winner: "white"},
		&Turn{Ply: 1, Move: "e7e5", At: t0.Add(3 * time.Minute)}, &st))

	var kind string
	var joined int64
	var moves, fell, blackLost int
	var hist string
	must(t, s.db.QueryRowContext(ctx, `SELECT kind, joined_at FROM games WHERE code = 'G00001'`).Scan(&kind, &joined))
	must(t, s.db.QueryRowContext(ctx, `SELECT moves, fell, black_lost, open_hist FROM game_stats WHERE game = 'G00001'`).Scan(&moves, &fell, &blackLost, &hist))
	if kind != "quick" || joined != t0.Add(90*time.Second).UnixMilli() || moves != 2 || fell != 1 || blackLost != 1 || hist != "1,1,0,0,0,0" {
		t.Fatalf("saved %q %d %d %d %d %q", kind, joined, moves, fell, blackLost, hist)
	}
	// A default kind, and a game ended without stats, both fine.
	must(t, s.CreateGame(ctx, Game{Code: "G00002", White: "w", WhiteName: "w", CreatedAt: t0}))
	must(t, s.EndGame(ctx, "G00002", Result{EndedAt: t0.Add(time.Hour), Reason: "expired"}, nil, nil))
	var n int
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_stats`).Scan(&n))
	must(t, s.db.QueryRowContext(ctx, `SELECT kind FROM games WHERE code = 'G00002'`).Scan(&kind))
	if n != 1 || kind != "friend" {
		t.Fatalf("rows %d, kind %q", n, kind)
	}
}

func TestGamesWithoutStatsAndBackfillRow(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	// Three games: one finished without stats (needs a backfill), one with
	// stats, one that never got going.
	for _, g := range []struct{ code, reason string }{{"G00001", "resignation"}, {"G00002", "checkmate"}, {"G00003", "aborted"}} {
		must(t, s.CreateGame(ctx, Game{Code: g.code, White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: t0}))
		must(t, s.AddTurn(ctx, g.code, Turn{Ply: 0, Move: "e2e4", Dice: []int{1}, At: t0}))
		must(t, s.EndGame(ctx, g.code, Result{EndedAt: t0.Add(time.Minute), Reason: g.reason, Winner: "white"}, nil, nil))
	}
	must(t, s.AddGameStats(ctx, "G00002", GameStats{Moves: 1}))
	todo, err := s.GamesWithoutStats(ctx)
	must(t, err)
	if len(todo) != 1 || todo[0].Code != "G00001" || len(todo[0].Turns) != 1 || todo[0].Result == nil || todo[0].Result.Reason != "resignation" {
		t.Fatalf("to backfill: %+v", todo)
	}
	must(t, s.AddGameStats(ctx, "G00001", GameStats{Moves: 1}))
	must(t, s.AddGameStats(ctx, "G00001", GameStats{Moves: 99})) // a second write is ignored
	var moves int
	must(t, s.db.QueryRowContext(ctx, `SELECT moves FROM game_stats WHERE game = 'G00001'`).Scan(&moves))
	if moves != 1 {
		t.Fatalf("moves = %d after a repeat, want 1", moves)
	}
	todo, err = s.GamesWithoutStats(ctx)
	must(t, err)
	if len(todo) != 0 {
		t.Fatalf("still to backfill: %d", len(todo))
	}
}

func TestSearchesAndEvents(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(19 * time.Second), Guest: "ann000000000", Matched: true, Others: 1}))
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(65 * time.Second), Guest: "bob000000000", Matched: false, Others: 0}))
	must(t, s.AddEvent(ctx, "practice", t0))
	must(t, s.AddEvent(ctx, "restart", t0.Add(time.Hour)))
	var matched, others, events int
	must(t, s.db.QueryRowContext(ctx, `SELECT SUM(matched), SUM(others) FROM searches`).Scan(&matched, &others))
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE kind IN ('practice', 'restart')`).Scan(&events))
	if matched != 1 || others != 1 || events != 2 {
		t.Fatalf("matched %d others %d events %d", matched, others, events)
	}
}
```

In `store/games_test.go`, every `SeatBlack(ctx, code, guest, name)` call gains a time argument (`t0`), and every `EndGame(ctx, code, r, final)` call gains a trailing `nil`. `store/stats_test.go`'s `TestFunnelStats` does the same.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./store -run 'TestGameStats|TestGamesWithout|TestSearches' -count=1`
Expected: FAIL to compile (`unknown field Kind`).

- [ ] **Step 3: Implement**

Append to `migrations` in `store/migrate.go`:

```go
	// 8: game stats (stats plan 2). kind is friend or quick (a rematch keeps
	// its game's kind; everything saved before is friend). joined_at is when
	// Black sat down. game_stats is one row per game that got going, written
	// with its result; open_hist counts the turns that ended with 0..5
	// holes open. searches logs quick-match searches (guest is the 12-hex
	// visitor key). events counts practice games, reconnects and restarts.
	`ALTER TABLE games ADD COLUMN kind TEXT NOT NULL DEFAULT 'friend';
	 ALTER TABLE games ADD COLUMN joined_at INTEGER;
	 UPDATE games SET joined_at = created_at WHERE black IS NOT NULL AND rematch_of IS NOT NULL;
	 CREATE TABLE game_stats (
		game          TEXT PRIMARY KEY REFERENCES games(code),
		moves         INTEGER NOT NULL,
		opened        INTEGER NOT NULL,
		reset         INTEGER NOT NULL,
		closed_rounds INTEGER NOT NULL,
		closed_cap    INTEGER NOT NULL,
		repaired      INTEGER NOT NULL,
		fell          INTEGER NOT NULL,
		saving_rolls  INTEGER NOT NULL,
		saved         INTEGER NOT NULL,
		white_lost    INTEGER NOT NULL,
		black_lost    INTEGER NOT NULL,
		mate_by_roll  INTEGER NOT NULL,
		open_hist     TEXT NOT NULL
	 );
	 CREATE TABLE searches (
		started_at INTEGER NOT NULL,
		ended_at   INTEGER NOT NULL,
		guest      TEXT NOT NULL,
		matched    INTEGER NOT NULL,
		others     INTEGER NOT NULL
	 );
	 CREATE INDEX searches_started_at ON searches(started_at);
	 CREATE TABLE events (
		at   INTEGER NOT NULL,
		kind TEXT NOT NULL
	 );
	 CREATE INDEX events_at ON events(at);`,
```

In `store/games.go`:

- `Game` gains, after `RematchOf`:

```go
	// Kind is "friend" (a link) or "quick" (quick match); a rematch keeps
	// the kind of the game it follows. "" saves as friend.
	Kind string
```

- `SavedGame` gains `JoinedAt time.Time // when Black sat down; zero when unknown`.
- `CreateGame`'s INSERT becomes:

```go
	kind := g.Kind
	if kind == "" {
		kind = "friend"
	}
	var joined any
	if g.Black != "" {
		joined = g.CreatedAt.UnixMilli() // both seated from the start: quick match or a rematch
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO games (code, white, black, white_name, black_name, created_at, rematch_of, rules, kind, joined_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.Code, g.White, nullable(g.Black), nullable(g.WhiteName), nullable(g.BlackName), g.CreatedAt.UnixMilli(), nullable(g.RematchOf), rulesVersion, kind, joined)
```

- `SeatBlack`:

```go
// SeatBlack records the guest who took Black's seat, their name, and when.
func (s *Store) SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error {
	return execOne(ctx, s.db, `UPDATE games SET black = ?, black_name = ?, joined_at = ? WHERE code = ?`, guest, nullable(name), now.UnixMilli(), code)
}
```

- `EndGame` takes `st *GameStats` and, inside the transaction after the UPDATE:

```go
	if st != nil {
		if err := addGameStats(ctx, tx, code, *st); err != nil {
			return err
		}
	}
```

- `LoadForRestore`'s SELECT adds `kind, joined_at` (scan `kind` into `g.Kind` and `joined_at` into a `sql.NullInt64`, set `g.JoinedAt = time.UnixMilli(v)` when valid). Factor the row scan into `scanSavedGame(rows) (SavedGame, error)` so `GamesWithoutStats` reuses it.

`store/gamestats.go`:

```go
package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GameStats is what the road did to one game, counted from its dice:
// potholes opened, reset by a roll landing on them, closed by their rounds
// or by the cap, repaired by the Mamdani; pieces that fell, saving rolls
// and saves, pieces each side lost to a pothole; whether the dice
// delivered the mate; and OpenHist[k], the turns that ended with k holes
// open.
type GameStats struct {
	Moves, Opened, Reset, ClosedRounds, ClosedCap, Repaired int
	Fell, SavingRolls, Saved, WhiteLost, BlackLost         int
	MateByRoll                                             bool
	OpenHist                                               [6]int
}

// Search is one quick-match search: who looked (the 12-hex visitor key),
// for how long, whether it found an opponent, and how many others were
// looking when it started.
type Search struct {
	StartedAt, EndedAt time.Time
	Guest              string
	Matched            bool
	Others             int
}

// AddGameStats writes a game's stats on their own (the backfill). A game
// that has a row keeps it.
func (s *Store) AddGameStats(ctx context.Context, code string, st GameStats) error {
	return addGameStats(ctx, s.db, code, st)
}

func addGameStats(ctx context.Context, db execer, code string, st GameStats) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		code, st.Moves, st.Opened, st.Reset, st.ClosedRounds, st.ClosedCap, st.Repaired, st.Fell, st.SavingRolls, st.Saved, st.WhiteLost, st.BlackLost, st.MateByRoll, formatHist(st.OpenHist))
	return err
}

// GamesWithoutStats returns every game under the current rules that got
// going and has no stats row yet, with its turns, oldest first.
func (s *Store) GamesWithoutStats(ctx context.Context) ([]SavedGame, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT code, white, black, white_name, black_name, created_at, rematch_of, ended_at, result, winner, kind, joined_at
		 FROM games WHERE rules = ? AND result IN `+finishedReasons+` AND code NOT IN (SELECT game FROM game_stats) ORDER BY created_at`,
		rulesVersion)
	if err != nil {
		return nil, err
	}
	var games []SavedGame
	for rows.Next() {
		g, err := scanSavedGame(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		games = append(games, g)
	}
	rows.Close() // one connection: close before the next query
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range games {
		if games[i].Turns, err = s.turns(ctx, games[i].Code); err != nil {
			return nil, err
		}
	}
	return games, nil
}

func (s *Store) AddSearch(ctx context.Context, sr Search) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO searches (started_at, ended_at, guest, matched, others) VALUES (?, ?, ?, ?, ?)`,
		sr.StartedAt.UnixMilli(), sr.EndedAt.UnixMilli(), sr.Guest, sr.Matched, sr.Others)
	return err
}

// AddEvent counts one event of kind ("practice", "reconnect", "restart").
func (s *Store) AddEvent(ctx context.Context, kind string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO events (at, kind) VALUES (?, ?)`, at.UnixMilli(), kind)
	return err
}

// formatHist writes [6]int as "9,14,19,22,18,18"; parseHist reads it back.
func formatHist(h [6]int) string {
	parts := make([]string, len(h))
	for i, n := range h {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func parseHist(s string) ([6]int, error) {
	var h [6]int
	parts := strings.Split(s, ",")
	if len(parts) != len(h) {
		return h, fmt.Errorf("bad open_hist %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return h, fmt.Errorf("bad open_hist %q", s)
		}
		h[i] = n
	}
	return h, nil
}
```

`scanSavedGame` in `store/games.go` (the body `LoadForRestore` had, plus the two new columns):

```go
// scanSavedGame reads one row of the games table's restore columns.
func scanSavedGame(rows *sql.Rows) (SavedGame, error) {
	var g SavedGame
	var black, whiteName, blackName, rematchOf, result, winner sql.NullString
	var created int64
	var ended, joined sql.NullInt64
	if err := rows.Scan(&g.Code, &g.White, &black, &whiteName, &blackName, &created, &rematchOf, &ended, &result, &winner, &g.Kind, &joined); err != nil {
		return g, err
	}
	g.Black, g.RematchOf, g.CreatedAt = black.String, rematchOf.String, time.UnixMilli(created)
	g.WhiteName, g.BlackName = whiteName.String, blackName.String
	if joined.Valid {
		g.JoinedAt = time.UnixMilli(joined.Int64)
	}
	if ended.Valid {
		g.Result = &Result{EndedAt: time.UnixMilli(ended.Int64), Reason: result.String, Winner: winner.String}
	}
	return g, nil
}
```

`go build ./...` now fails in `game` (the interface changed); that is Task 2. Keep the store compiling and its tests green: `go test ./store`.

- [ ] **Step 4: Run the tests**

Run: `go test ./store -count=1 && go vet ./store`
Expected: `ok` (including `TestMigrationsAreIdempotent` at version 8).

- [ ] **Step 5: Commit**

```bash
git add store/migrate.go store/games.go store/gamestats.go store/gamestats_test.go store/games_test.go store/stats_test.go
git commit -m "store: game kinds, when Black joined, a stats row per game, searches and events"
```

---

### Task 2: The road accumulator in the game

**Files:**
- Create: `game/road.go`, `game/road_test.go`
- Modify: `game/game.go` (a `road` field, `tally`, `end`, `Join`), `game/save.go` (`Store`, `nopStore`), `game/hub.go` (`Create`, `CreatePair`, `newGame` copies the kind), `game/rematch.go` (the rematch inherits the kind), `game/save_test.go` (the `failing` fake)
- Test: `game/road_test.go`; existing game tests keep passing

**Interfaces:**
- Consumes: `store.GameStats`, the new `EndGame`/`SeatBlack` signatures (Task 1); `rules.Event` kinds (`rules/event.go`), `rules.Position.Potholes [HoleCap]Hole` with `Sq == rules.NoSquare` for a free slot, `rules.MamdaniPiece`, `rules.Checkmate`.
- Produces:
  - `type road struct` (unexported) with `func (r *road) add(ev []rules.Event, pos *rules.Position)` and `func (r *road) stats(result rules.Result, last []rules.Event, lost [2][]string) store.GameStats`.
  - `func StatsOf(turns []store.Turn, result store.Result) (store.GameStats, error)`: replays a saved game from the start position with its recorded dice and returns its stats (the backfill).
  - `Game.kind string`, set from `store.Game.Kind`.

**How the counting works** (the event order within a turn is in `rules/event.go`): a `PotholeClosed` before the turn's `RolledPothole` is a hole that ran its rounds; one after it is the cap closing the oldest. `Repaired` is either kind of repair. `Fell` has no color: the piece's owner is `e.Piece.Color()` unless it is the Mamdani. After the events, count the open holes in the position for `OpenHist`.

- [ ] **Step 1: Write the failing tests**

`game/road_test.go`:

```go
package game

import (
	"testing"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// A countdown close comes before the roll; a cap close comes after it.
func TestRoadCountsClosesBySource(t *testing.T) {
	var r road
	var pos rules.Position
	r.add([]rules.Event{
		{Kind: rules.Moved}, {Kind: rules.PotholeClosed, Square: 1}, {Kind: rules.Repaired, Square: 2},
		{Kind: rules.RolledPothole, Roll: 4}, {Kind: rules.Target, Square: 3}, {Kind: rules.Reroll, Square: 3},
		{Kind: rules.Target, Square: 4}, {Kind: rules.SavingRoll, Square: 4, Piece: rules.Piece(rules.Pawn) | rules.Piece(rules.Black)<<3, Roll: 2},
		{Kind: rules.Fell, Square: 4, Piece: rules.Piece(rules.Pawn) | rules.Piece(rules.Black)<<3},
		{Kind: rules.PotholeClosed, Square: 5}, {Kind: rules.PotholeOpened, Square: 4},
	}, &pos)
	r.add([]rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 2}, {Kind: rules.Target, Square: 7}, {Kind: rules.PotholeReset, Square: 7}}, &pos)
	r.add([]rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 2}, {Kind: rules.Target, Square: 8}, {Kind: rules.Repaired, Square: 8}}, &pos)
	r.add([]rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 6}, {Kind: rules.Target, Square: 9}, {Kind: rules.Fell, Square: 9, Piece: rules.MamdaniPiece}, {Kind: rules.PotholeOpened, Square: 9}}, &pos)
	got := r.stats(rules.Result{}, nil, [2][]string{nil, {"p"}})
	want := store.GameStats{Moves: 4, Opened: 2, Reset: 1, ClosedRounds: 1, ClosedCap: 1, Repaired: 2, Fell: 2, SavingRolls: 1, BlackLost: 1, OpenHist: [6]int{4}}
	if got != want {
		t.Fatalf("stats\n got %+v\nwant %+v", got, want)
	}
}

func TestRoadMateByRoll(t *testing.T) {
	var r road
	byMove := r.stats(rules.Result{Over: true, Reason: rules.Checkmate}, []rules.Event{{Kind: rules.Moved}}, [2][]string{})
	byRoll := r.stats(rules.Result{Over: true, Reason: rules.Checkmate}, []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole}, {Kind: rules.Target}, {Kind: rules.PotholeOpened}}, [2][]string{})
	resigned := r.stats(rules.Result{Over: true, Reason: Resignation}, []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole}}, [2][]string{})
	if byMove.MateByRoll || !byRoll.MateByRoll || resigned.MateByRoll {
		t.Fatalf("byMove %v byRoll %v resigned %v", byMove.MateByRoll, byRoll.MateByRoll, resigned.MateByRoll)
	}
}

// StatsOf replays a saved game: the same turns give the same stats as a
// game played live, and a restored game carries them on.
func TestRoadStatsSurviveRestore(t *testing.T) {
	h := NewHub(&rules.ScriptedDice{Rolls: []int{2, 4, 4, 1, 1, 2, 4, 4, 3, 1}}, nil) // e4 opens d4, e5 opens d4 again? no: scripted per turn below
	_ = h
	// Play two turns through the rules with fixed dice, both live and by replay.
	turns := []store.Turn{
		{Ply: 0, Move: "e2e4", Dice: []int{2, 4, 4}}, // even: a pothole opens on d4
		{Ply: 1, Move: "e7e5", Dice: []int{1}},       // odd: nothing
		{Ply: 2, Move: "g1f3", Dice: []int{2, 5, 5}}, // even: e5 — Black's pawn sits there: a fall (saving roll only with a Mamdani line)
	}
	live := &road{}
	g := rules.NewGame()
	for _, t2 := range turns {
		m, _ := rules.ParseMove(t2.Move)
		ev, err := g.Play(m, &rules.ScriptedDice{Rolls: t2.Dice})
		if err != nil {
			t.Fatal(err)
		}
		live.add(ev, &g.Pos)
	}
	want := live.stats(rules.Result{Over: true, Reason: Resignation, Winner: rules.White}, nil, [2][]string{nil, {"p"}})
	got, err := StatsOf(turns, store.Result{Reason: "resignation", Winner: "white"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StatsOf\n got %+v\nwant %+v", got, want)
	}
	if got.Moves != 3 || got.Opened < 1 || got.OpenHist[0]+got.OpenHist[1]+got.OpenHist[2] != 3 {
		t.Fatalf("stats look wrong: %+v", got)
	}
	// Restore builds the same accumulator.
	hub := NewHub(odd{}, nil)
	sg := store.SavedGame{Game: store.Game{Code: "ABCDEF", White: "w", Black: "b"}, Turns: turns}
	if n := hub.Restore([]store.SavedGame{sg}); n != 1 {
		t.Fatalf("restored %d", n)
	}
	rg, _ := hub.Get("ABCDEF")
	var restored store.GameStats
	rg.do(func() { restored = rg.road.stats(rules.Result{Over: true, Reason: Resignation, Winner: rules.White}, nil, rg.lost) })
	if restored != want {
		t.Fatalf("after restore\n got %+v\nwant %+v", restored, want)
	}
}
```

Before relying on the exact dice above, check `rules/turn.go`'s dice order (the pothole d8 first, then file and rank) and the board: the test's comments say what each turn should do. If `e5`'s target doesn't hit the pawn with `5, 5`, pick a square that does (the file die 1–8 is a–h, the rank die 1–8), and adjust the asserted `Fell`. The test's point is that `StatsOf` and the live accumulator agree, and that restore agrees too; keep those three assertions. The `odd{}` dice type exists in `game`'s tests (`game/game_test.go`); if it doesn't, add `type odd struct{}; func (odd) D8() int { return 1 }` to `road_test.go`. The stray `h` at the top of the test is a leftover; delete those two lines.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./game -run 'TestRoad' -count=1`
Expected: FAIL to compile (`undefined: road`).

- [ ] **Step 3: Implement**

`game/road.go`:

```go
package game

import (
	"fmt"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// road tallies what the potholes did over a game. The game adds every
// turn's events as they are played (restore replays them, so a restart
// rebuilds it), and hands the total to the store with the result.
type road struct {
	moves, opened, reset, closedRounds, closedCap, repaired int
	fell, savingRolls, saved                                int
	openHist                                                [6]int // turns that ended with k holes open
}

// add counts one turn's events; pos is the position after them. A
// pothole_closed before the turn's roll is a hole that ran its rounds, one
// after it is the cap closing the oldest.
func (r *road) add(ev []rules.Event, pos *rules.Position) {
	r.moves++
	rolled := false
	for _, e := range ev {
		switch e.Kind {
		case rules.RolledPothole:
			rolled = true
		case rules.PotholeOpened:
			r.opened++
		case rules.PotholeReset:
			r.reset++
		case rules.PotholeClosed:
			if rolled {
				r.closedCap++
			} else {
				r.closedRounds++
			}
		case rules.Repaired:
			r.repaired++
		case rules.Fell:
			r.fell++
		case rules.SavingRoll:
			r.savingRolls++
			if e.Saved {
				r.saved++
			}
		}
	}
	r.openHist[min(openHoles(pos), len(r.openHist)-1)]++
}

// openHoles counts the position's open potholes.
func openHoles(pos *rules.Position) int {
	n := 0
	for _, h := range pos.Potholes {
		if h.Sq != rules.NoSquare {
			n++
		}
	}
	return n
}

// stats is the total as the store keeps it. last is the final turn's
// events (a checkmate whose turn rolled the dice is a mate by a roll) and
// lost the pieces each side lost to potholes.
func (r *road) stats(result rules.Result, last []rules.Event, lost [2][]string) store.GameStats {
	st := store.GameStats{
		Moves: r.moves, Opened: r.opened, Reset: r.reset, ClosedRounds: r.closedRounds, ClosedCap: r.closedCap, Repaired: r.repaired,
		Fell: r.fell, SavingRolls: r.savingRolls, Saved: r.saved,
		WhiteLost: len(lost[rules.White]), BlackLost: len(lost[rules.Black]),
		OpenHist: r.openHist,
	}
	if result.Reason == rules.Checkmate {
		for _, e := range last {
			if e.Kind == rules.RolledPothole {
				st.MateByRoll = true
			}
		}
	}
	return st
}

// StatsOf replays a saved game's turns from the start with their recorded
// dice and returns its stats: the backfill for games that ended before
// stats were kept.
func StatsOf(turns []store.Turn, result store.Result) (store.GameStats, error) {
	g := rules.NewGame()
	var r road
	var lost [2][]string
	var last []rules.Event
	for _, t := range turns {
		m, err := rules.ParseMove(t.Move)
		if err != nil {
			return store.GameStats{}, fmt.Errorf("ply %d: %w", t.Ply, err)
		}
		ev, err := playScripted(g, m, t.Dice)
		if err != nil {
			return store.GameStats{}, fmt.Errorf("ply %d (%s): %w", t.Ply, t.Move, err)
		}
		for _, e := range ev {
			if e.Kind == rules.Fell && e.Piece != rules.MamdaniPiece {
				c := e.Piece.Color()
				lost[c] = append(lost[c], pieceCode(e.Piece))
			}
		}
		r.add(ev, &g.Pos)
		last = ev
	}
	res := savedRules(&result)
	if rules.Reason(result.Reason) != rules.Checkmate {
		last = nil
	}
	return r.stats(res, last, lost), nil
}

// playScripted plays m with the recorded dice, turning the dice running
// out (a panic in ScriptedDice) into an error.
func playScripted(g *rules.Game, m rules.Move, dice []int) (ev []rules.Event, err error) {
	script := &rules.ScriptedDice{Rolls: dice}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	ev, err = g.Play(m, script)
	if err == nil && script.Left() != 0 {
		err = fmt.Errorf("%d unused dice", script.Left())
	}
	return ev, err
}
```

`game/game.go`:
- `Game` gains `road road` (after `stats StatsJSON`) and `kind string` (after `practice bool`); `newGame` sets `kind: sg.Kind`.
- `apply` calls `g.road.add(ev, &g.g.Pos)` after `g.tally(ev)`.
- `end` builds the stats before saving:

```go
	saved := savedResult(now, r)
	var st *store.GameStats
	if !g.practice && isFinished(r.Reason) {
		s := g.road.stats(r, g.last, g.lost)
		st = &s
	}
	g.save("result", func(ctx context.Context) error { return g.store.EndGame(ctx, g.code, saved, final, st) })
```

with, in `game/clock.go` next to `noWinner`:

```go
// isFinished reports a game that got going: it ended on the board or the
// clock, not for want of a second player or a first move.
func isFinished(r rules.Reason) bool {
	switch r {
	case Aborted, Expired, Retired:
		return false
	}
	return true
}
```

- `Join`'s seat save passes the time: `return g.store.SeatBlack(ctx, g.code, guest, name, time.Now())`.

`game/save.go`: the interface and `nopStore` take the new signatures:

```go
	SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error
	EndGame(ctx context.Context, code string, r store.Result, final *store.Turn, st *store.GameStats) error
```

`game/hub.go`: `Create` passes `Kind: "friend"`, `CreatePair` passes `Kind: "quick"`. `game/rematch.go`: the rematch's `store.Game` gets `Kind: g.kind`. `game/save_test.go`: the `failing` fake's `SeatBlack` and `EndGame` take the new parameters. Any other `EndGame(`/`SeatBlack(` call in the game and server tests gains the extra argument (`grep -rn "EndGame(\|SeatBlack(" game server cmd` finds them).

- [ ] **Step 4: Run the tests**

Run: `go build ./... && go test -race -short ./... && go vet ./...`
Expected: every package `ok`. `cmd/server`'s restore test still passes (the restored game's stats are rebuilt by replay).

- [ ] **Step 5: Commit**

```bash
git add game/road.go game/road_test.go game/game.go game/clock.go game/save.go game/hub.go game/rematch.go game/save_test.go
git commit -m "game: count what the road did to each game, and save it with the result"
```

(add any other test file you touched to the `git add`).

---

### Task 3: Searches, events, the backfill and the reconnect mark

**Files:**
- Modify: `server/matchmaking.go` (log each search), `server/games.go` (`createPractice` logs an event; `gameStream` logs a reconnect when `?again=1`), `cmd/server/main.go` (a restart event; call the backfill)
- Create: `cmd/server/backfill.go`, `cmd/server/backfill_test.go`
- Modify: `web/src/routes/game/[code]/+page.svelte` line 238 (`?again=1` on a reconnect)
- Test: `server/matchmaking_test.go` (`TestSearchLogged`), `server/games_test.go` (`TestPracticeAndReconnectEvents`)

**Interfaces:**
- Consumes: `store.AddSearch`, `store.AddEvent`, `store.GamesWithoutStats`, `store.AddGameStats` (Task 1); `game.StatsOf` (Task 2); `visitorOf` (server/visit.go); `s.match.LookingFor(guest)` (match.Queue).
- Produces: `func backfillStats(ctx context.Context, st *store.Store) (done, failed int, err error)` in `cmd/server`.

- [ ] **Step 1: Write the failing tests**

In `server/matchmaking_test.go`, add (using the package's `newTestServer`, `newPlayer`, `.queue()` and `matchedCode()` helpers; read them first):

```go
// Every search is logged: matched or not, how long it took, and how many
// others were looking when it started.
func TestSearchLogged(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	// Alice searches alone and gives up.
	sr, _ := alice.queue()
	time.Sleep(20 * time.Millisecond)
	sr.close()
	// Bob and Carol find each other.
	carol := newPlayer(t, ts)
	bs, _ := bob.queue()
	cs, _ := carol.queue()
	if matchedCode(t, bs) == "" || matchedCode(t, cs) == "" {
		t.Fatal("no match")
	}
	var rows []store.Search
	deadline := time.Now().Add(2 * time.Second)
	for len(rows) < 3 && time.Now().Before(deadline) { // the log is written as each stream ends
		time.Sleep(10 * time.Millisecond)
		rows = searches(t, s)
	}
	matched, others := 0, 0
	for _, r := range rows {
		if r.Matched {
			matched++
		}
		others += r.Others
		if len(r.Guest) != 12 || !r.EndedAt.After(r.StartedAt) && !r.EndedAt.Equal(r.StartedAt) {
			t.Errorf("bad row %+v", r)
		}
	}
	if len(rows) != 3 || matched != 2 || others != 1 { // Carol saw Bob looking; Alice and Bob saw nobody
		t.Fatalf("rows %+v", rows)
	}
}
```

with a helper that reads the table through the store (add to `store/gamestats.go` a small `Searches(ctx, from, to time.Time) ([]Search, error)` returning rows ordered by `started_at`; Task 4 reads the same table through `GameSection`, so this helper is also used there):

```go
func searches(t *testing.T, s *Server) []store.Search {
	t.Helper()
	rows, err := s.store.Searches(t.Context(), time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
```

In `server/games_test.go`:

```go
func TestPracticeAndReconnectEvents(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, _ := alice.post("/api/practice", ""); status != http.StatusCreated {
		t.Fatalf("practice: %d", status)
	}
	code := alice.create()
	alice.stream(code).state()
	resp, err := alice.c.Get(ts.URL + "/api/games/" + code + "/stream?again=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	counts := map[string]int{}
	for _, kind := range []string{"practice", "reconnect", "restart"} {
		n, err := s.store.CountEvents(t.Context(), kind, time.Time{}, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		counts[kind] = n
	}
	if counts["practice"] != 1 || counts["reconnect"] != 1 || counts["restart"] != 0 {
		t.Fatalf("events %v", counts)
	}
}
```

(`CountEvents(ctx, kind string, from, to time.Time) (int, error)` is a two-line store query; add it to `store/gamestats.go` with the `Searches` helper, each with a test in `store/gamestats_test.go` that writes two rows and reads one back by range.)

`cmd/server/backfill_test.go`:

```go
package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mamdani-chess/store"
)

func TestBackfillSkipsGamesWithStats(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now()
	// A finished game saved without stats, as games were before this plan.
	if err := st.CreateGame(ctx, store.Game{Code: "G00001", White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for i, tn := range []store.Turn{{Ply: 0, Move: "e2e4", Dice: []int{1}}, {Ply: 1, Move: "e7e5", Dice: []int{1}}} {
		tn.At = now.Add(time.Duration(i) * time.Second)
		if err := st.AddTurn(ctx, "G00001", tn); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.EndGame(ctx, "G00001", store.Result{EndedAt: now.Add(time.Minute), Reason: "resignation", Winner: "white"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	done, failed, err := backfillStats(ctx, st)
	if err != nil || done != 1 || failed != 0 {
		t.Fatalf("first run: %d %d %v", done, failed, err)
	}
	done, failed, err = backfillStats(ctx, st)
	if err != nil || done != 0 || failed != 0 {
		t.Fatalf("second run: %d %d %v", done, failed, err)
	}
	todo, err := st.GamesWithoutStats(ctx)
	if err != nil || len(todo) != 0 {
		t.Fatalf("left: %d %v", len(todo), err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./server -run 'TestSearchLogged|TestPracticeAndReconnect' -count=1; go test ./cmd/server -run TestBackfill -count=1`
Expected: both FAIL to compile (`s.store.Searches undefined`, `undefined: backfillStats`).

- [ ] **Step 3: Implement**

`server/matchmaking.go`: record the start before `Join` and the outcome on every way out:

```go
	started := time.Now()
	others := s.match.LookingFor(guest)
	t := s.match.Join(guest)
	matched := false
	defer func() {
		t.Leave()
		s.logSearch(guest, started, matched, others)
	}()
```

(replace the `defer t.Leave()` line), and in the `case code := <-t.C:` branch set `matched = true` before writing the event. Add:

```go
// logSearch writes one quick-match search. A failure is logged, never
// shown: the search itself has already happened.
func (s *Server) logSearch(guest string, started time.Time, matched bool, others int) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.store.AddSearch(ctx, store.Search{StartedAt: started, EndedAt: time.Now(), Guest: visitorOf(guest), Matched: matched, Others: others}); err != nil {
		s.log.Error("log search", "err", err)
	}
}

// logEvent counts one event (a practice game, a reconnect). Failures are
// logged, never shown.
func (s *Server) logEvent(kind string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.store.AddEvent(ctx, kind, time.Now()); err != nil {
		s.log.Error("log event", "kind", kind, "err", err)
	}
}
```

(the request's context is gone by the time the deferred write runs, hence `context.Background()` with a timeout). `server/games.go`: `createPractice` calls `s.logEvent("practice")` after creating the game; `gameStream` calls `s.logEvent("reconnect")` when `r.URL.Query().Get("again") == "1"`, after `Join` succeeds.

`cmd/server/backfill.go`:

```go
package main

import (
	"context"
	"log/slog"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/store"
)

// backfillStats writes a stats row for every finished game that has none,
// by replaying its saved dice: games that ended before stats were kept.
// Each game is written on its own, so a game that won't replay is skipped
// and logged, and a run cut short leaves the rest for the next start.
func backfillStats(ctx context.Context, st *store.Store) (done, failed int, err error) {
	games, err := st.GamesWithoutStats(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, sg := range games {
		stats, err := game.StatsOf(sg.Turns, *sg.Result)
		if err != nil {
			slog.Warn("game stats not backfilled", "code", sg.Code, "err", err)
			failed++
			continue
		}
		if err := st.AddGameStats(ctx, sg.Code, stats); err != nil {
			return done, failed, err
		}
		done++
	}
	return done, failed, nil
}

// restartEvent counts this start on the stats page's Health section.
func restartEvent(ctx context.Context, st *store.Store) error {
	return st.AddEvent(ctx, "restart", time.Now())
}
```

`cmd/server/main.go`, after `restore(...)`:

```go
	if err := restartEvent(context.Background(), st); err != nil {
		return fmt.Errorf("restart event: %w", err)
	}
	start := time.Now()
	if done, failed, err := backfillStats(context.Background(), st); err != nil {
		return fmt.Errorf("backfill game stats: %w", err)
	} else if done > 0 || failed > 0 {
		slog.Info("game stats backfilled", "games", done, "failed", failed, "duration", time.Since(start))
	}
```

`web/src/routes/game/[code]/+page.svelte` line 238: the page already knows whether it had a view (`view`) when it connects again; mark that:

```ts
			const es = new EventSource(`/api/games/${code}/stream${view ? '?again=1' : ''}`);
```

Run `npx @sveltejs/mcp svelte-autofixer "web/src/routes/game/[code]/+page.svelte"` and apply what it says (the page's pre-existing "use SvelteSet" note is intended; leave it).

- [ ] **Step 4: Run the tests**

Run: `go test -race -short ./... && go vet ./... && pnpm --dir web check && pnpm --dir web test && pnpm --dir web build`
Expected: every package `ok`; `0 errors`; the build succeeds. Then, with the Go server and the dev server running on spare ports, `pnpm --dir web playtest --games 1` passes (a stream's `?again=1` must not break the stream).

- [ ] **Step 5: Commit**

```bash
git add server/matchmaking.go server/games.go server/matchmaking_test.go server/games_test.go store/gamestats.go store/gamestats_test.go cmd/server/backfill.go cmd/server/backfill_test.go cmd/server/main.go "web/src/routes/game/[code]/+page.svelte"
git commit -m "server: log quick-match searches, practice games, reconnects and restarts; backfill game stats at startup"
```

---

### Task 4: Reading the sections

**Files:**
- Create: `store/gamesection.go`
- Test: `store/gamesection_test.go`

**Interfaces:**
- Consumes: the tables of Task 1, `finishedReasons`, `Count`, `top`, `parseHist`; `day()`/`newYork` and `must` from the store tests.
- Produces:

```go
type KindDay struct { Day string; Quick, Friend, Practice int }
type Heat [7][12]int // [Mon..Sun][0-2h, 2-4h, ... 22-24h] games started, in loc
type GameSection struct {
	Games, GotGoing int        // games created in the range; of those, ended with a result
	Days   []KindDay           // every day of the range (zero-filled; "all" from the first game)
	Ends   []Count             // Checkmate, Mate by a roll, Resignation, Out of time, Draw, Aborted, Nobody came, in that order, zeros dropped
	Lengths []Count            // moves by both sides, buckets 1–10 … 81+ (zeros kept, so the chart has every bar)
	MedianMoves, MedianMinutes int
	OnClock int                // ended on time (timeout or timeout_vs_insufficient)
	WinnerClockLeft string     // median of the winner's clock at the end, "3:48"; "" without decisive games
	Heat Heat
	Friend struct { Links, Joined, Finished, Rematches int; MedianJoin string } // kind = friend
	Quick struct { Searches, Matched, GaveUp, Alone int; Waits []Count; MedianWait, MedianGaveUp string }
	Road struct {
		Games, Opened, Fell, SavingRolls, Saved, Repaired, Reset, MateByRoll, ClosedRounds, ClosedCap, NoFall int
		OpenHist [6]int
		LessWon, MoreWon, Same int // decisive games: the winner lost fewer / more / as many pieces to the road as the loser
	}
	Reconnects, Restarts int
}
func (s *Store) GameSection(ctx context.Context, from, to time.Time, loc *time.Location) (GameSection, error)
```

Durations are formatted `m:ss` (`"1:40"`); `MedianMinutes` is whole minutes from the first turn's `at` to `ended_at`. Medians of an empty set are 0 / `""`. "Alone" is searches whose `others` was 0. `Waits` buckets: `under 10s`, `10–30s`, `30–60s`, `1–2 min`, `2 min+`, for matched searches only.

- [ ] **Step 1: Write the failing tests**

`store/gamesection_test.go`:

```go
package store

import (
	"strings"
	"testing"
	"time"
)

func seedGames(t *testing.T, s *Store) {
	t.Helper()
	ctx := t.Context()
	game := func(code, kind, white, black string, created time.Time, turns int, reason, winner string, st *GameStats, rematchOf string) {
		t.Helper()
		g := Game{Code: code, Kind: kind, White: white, WhiteName: white, CreatedAt: created, RematchOf: rematchOf}
		if kind == "quick" {
			g.Black, g.BlackName = black, black
		}
		must(t, s.CreateGame(ctx, g))
		if kind == "friend" && black != "" {
			must(t, s.SeatBlack(ctx, code, black, black, created.Add(100*time.Second)))
		}
		for i := 0; i < turns; i++ {
			left := int64(600000 - 10000*i)
			must(t, s.AddTurn(ctx, code, Turn{Ply: i, Move: "e2e4", WhiteMS: left, BlackMS: left - 5000, At: created.Add(time.Duration(i+1) * 20 * time.Second)}))
		}
		if reason != "" {
			must(t, s.EndGame(ctx, code, Result{EndedAt: created.Add(time.Duration(turns+1) * 20 * time.Second), Reason: reason, Winner: winner}, nil, st))
		}
	}
	// Saturday Oct 3, 2026, 8 pm New York: two quick games, one decided by a roll.
	sat := day(3, 20)
	game("Q00001", "quick", "ann", "bob", sat, 30, "checkmate", "white", &GameStats{Moves: 30, Opened: 5, Fell: 2, WhiteLost: 0, BlackLost: 2, SavingRolls: 1, Saved: 1, Repaired: 1, OpenHist: [6]int{5, 10, 10, 5, 0, 0}}, "")
	game("Q00002", "quick", "cat", "dan", sat.Add(time.Hour), 12, "checkmate", "black", &GameStats{Moves: 12, Opened: 3, Fell: 1, WhiteLost: 1, MateByRoll: true, OpenHist: [6]int{2, 5, 5, 0, 0, 0}}, "")
	// Sunday: a friend game resigned (White lost more to the road and still won), its rematch on time, and a link nobody opened.
	sun := day(4, 15)
	game("F00001", "friend", "ann", "eve", sun, 40, "resignation", "white", &GameStats{Moves: 40, Opened: 6, Fell: 3, WhiteLost: 2, BlackLost: 1, Reset: 1, ClosedCap: 1, ClosedRounds: 4, OpenHist: [6]int{10, 10, 10, 5, 3, 2}}, "")
	game("F00002", "quick", "eve", "ann", sun.Add(time.Hour), 20, "timeout", "black", &GameStats{Moves: 20, Opened: 2, OpenHist: [6]int{10, 10, 0, 0, 0, 0}}, "F00001")
	game("F00003", "friend", "fay", "", sun.Add(2*time.Hour), 0, "expired", "", nil, "")
	// An aborted quick game (no first move).
	game("Q00003", "quick", "gus", "hal", sun.Add(3*time.Hour), 0, "aborted", "", nil, "")
	must(t, s.AddSearch(ctx, Search{StartedAt: sat, EndedAt: sat.Add(19 * time.Second), Guest: "ann000000000", Matched: true, Others: 0}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sat, EndedAt: sat.Add(19 * time.Second), Guest: "bob000000000", Matched: true, Others: 1}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sun, EndedAt: sun.Add(65 * time.Second), Guest: "fay000000000", Matched: false, Others: 0}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sun, EndedAt: sun.Add(150 * time.Second), Guest: "gus000000000", Matched: true, Others: 0}))
	must(t, s.AddEvent(ctx, "practice", sat))
	must(t, s.AddEvent(ctx, "practice", sun))
	must(t, s.AddEvent(ctx, "reconnect", sun))
	must(t, s.AddEvent(ctx, "restart", sun))
}

func TestGameSection(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedGames(t, s)
	g, err := s.GameSection(ctx, day(1, 0), day(8, 0), newYork)
	must(t, err)
	check := func(what string, got, want any) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	check("games", g.Games, 6)
	check("got going", g.GotGoing, 4)
	check("days", len(g.Days), 7)
	check("Sat", g.Days[2], KindDay{"2026-10-03", 2, 0, 1})
	check("Sun", g.Days[3], KindDay{"2026-10-04", 2, 2, 1})
	check("ends", g.Ends, []Count{{"Checkmate", 1}, {"Mate by a roll", 1}, {"Resignation", 1}, {"Out of time", 1}, {"Aborted", 1}, {"Nobody came", 1}})
	check("lengths", g.Lengths, []Count{{"1–10", 0}, {"11–20", 2}, {"21–30", 1}, {"31–40", 1}, {"41–50", 0}, {"51–60", 0}, {"61–80", 0}, {"81+", 0}})
	check("median moves", g.MedianMoves, 25) // 12, 20, 30, 40 → (20+30)/2
	check("on clock", g.OnClock, 1)
	check("winner clock", g.WinnerClockLeft != "", true)
	check("heat Sat 8pm", g.Heat[5][10], 2) // Saturday, the 20–22 block
	check("heat Sun 4pm", g.Heat[6][7]+g.Heat[6][8]+g.Heat[6][9], 4)
	check("friend", fmt.Sprint(g.Friend.Links, g.Friend.Joined, g.Friend.Finished, g.Friend.Rematches, g.Friend.MedianJoin), "2 1 1 1 1:40")
	check("quick", fmt.Sprint(g.Quick.Searches, g.Quick.Matched, g.Quick.GaveUp, g.Quick.Alone, g.Quick.MedianWait, g.Quick.MedianGaveUp), "4 3 1 3 0:19 1:05")
	check("waits", g.Quick.Waits, []Count{{"under 10s", 0}, {"10–30s", 2}, {"30–60s", 0}, {"1–2 min", 0}, {"2 min+", 1}})
	check("road", fmt.Sprint(g.Road.Games, g.Road.Opened, g.Road.Fell, g.Road.SavingRolls, g.Road.Saved, g.Road.Repaired, g.Road.Reset, g.Road.MateByRoll, g.Road.ClosedRounds, g.Road.ClosedCap, g.Road.NoFall), "4 16 6 1 1 1 1 1 4 1 1")
	check("open hist", g.Road.OpenHist, [6]int{27, 35, 25, 10, 3, 2})
	// Q00001: white lost 0 < black 2 → less; Q00002: white 1 > black 0, black won → less; F00001: white 2 > black 1, white won → more; F00002: 0 = 0 → same.
	check("decided", fmt.Sprint(g.Road.LessWon, g.Road.MoreWon, g.Road.Same), "2 1 1")
	check("health", fmt.Sprint(g.Reconnects, g.Restarts), "1 1")
}

func TestGameSectionEmpty(t *testing.T) {
	s, _ := openTemp(t)
	g, err := s.GameSection(t.Context(), day(1, 0), day(3, 0), newYork)
	must(t, err)
	if g.Games != 0 || len(g.Days) != 2 || g.MedianMoves != 0 || g.WinnerClockLeft != "" || g.Quick.MedianWait != "" || len(g.Lengths) != 8 {
		t.Fatalf("empty: %+v", g)
	}
	g, err = s.GameSection(t.Context(), time.Time{}, day(3, 0), newYork)
	must(t, err)
	if len(g.Days) != 0 {
		t.Fatalf("all time with no games has days: %v", g.Days)
	}
}

// Nothing the page reads carries a game code.
func TestGameSectionHasNoCodes(t *testing.T) {
	s, _ := openTemp(t)
	seedGames(t, s)
	g, err := s.GameSection(t.Context(), day(1, 0), day(8, 0), newYork)
	must(t, err)
	if text := fmt.Sprintf("%+v", g); strings.Contains(text, "Q00001") || strings.Contains(text, "F00001") {
		t.Fatalf("a code leaked: %s", text)
	}
}
```

Add `"fmt"` to the imports. The seed's expectations are worked out by hand in the comments; if a number comes out differently because of how the seed's turns are timed, fix the seed or the comment, not the query, unless the query is wrong.

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./store -run 'TestGameSection' -count=1`
Expected: FAIL to compile (`undefined: KindDay`).

- [ ] **Step 3: Implement**

`store/gamesection.go`. Read the rows once each and aggregate in Go, as `VisitStats` does. The shape:

```go
package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type KindDay struct {
	Day                   string
	Quick, Friend, Practice int
}

// Heat is games started by weekday (Monday first) and two-hour block.
type Heat [7][12]int

// GameSection is what the Games, Quick match and The road sections and the
// Health tiles show for a range. Zero-valued where there is nothing.
type GameSection struct {
	Games, GotGoing            int
	Days                       []KindDay
	Ends                       []Count
	Lengths                    []Count
	MedianMoves, MedianMinutes int
	OnClock                    int
	WinnerClockLeft            string
	Heat                       Heat
	Friend                     struct {
		Links, Joined, Finished, Rematches int
		MedianJoin                         string
	}
	Quick struct {
		Searches, Matched, GaveUp, Alone int
		Waits                            []Count
		MedianWait, MedianGaveUp         string
	}
	Road struct {
		Games, Opened, Fell, SavingRolls, Saved, Repaired, Reset, MateByRoll, ClosedRounds, ClosedCap, NoFall int
		OpenHist                                                                                         [6]int
		LessWon, MoreWon, Same                                                                           int
	}
	Reconnects, Restarts int
}

// gameRow is one game in the range with what the section needs.
type gameRow struct {
	kind, result, winner string
	created, joined      time.Time
	hasBlack, rematch    bool
	moves                int
	first, ended         time.Time // the first turn and the end; zero when none
	winnerMS             int64     // the winner's clock after the last turn
	stats                *GameStats
}

func (s *Store) GameSection(ctx context.Context, from, to time.Time, loc *time.Location) (GameSection, error) {
	var out GameSection
	a, b := from.UnixMilli(), to.UnixMilli()
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.kind, g.result, g.winner, g.created_at, g.joined_at, g.black IS NOT NULL, g.rematch_of IS NOT NULL, g.ended_at,
		       (SELECT COUNT(*) FROM turns t WHERE t.game = g.code),
		       (SELECT MIN(at) FROM turns t WHERE t.game = g.code),
		       (SELECT white_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       (SELECT black_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       s.moves, s.opened, s.reset, s.closed_rounds, s.closed_cap, s.repaired, s.fell, s.saving_rolls, s.saved, s.white_lost, s.black_lost, s.mate_by_roll, s.open_hist
		FROM games g LEFT JOIN game_stats s ON s.game = g.code
		WHERE g.created_at >= ? AND g.created_at < ? ORDER BY g.created_at`, a, b)
	if err != nil {
		return out, err
	}
	var games []gameRow
	for rows.Next() {
		var r gameRow
		var result, winner sql.NullString
		var joined, ended, first, whiteMS, blackMS sql.NullInt64
		var st struct {
			moves, opened, reset, closedRounds, closedCap, repaired, fell, savingRolls, saved, whiteLost, blackLost, mateByRoll sql.NullInt64
			hist                                                                                                               sql.NullString
		}
		if err := rows.Scan(&r.kind, &result, &winner, &r.created, &joined, &r.hasBlack, &r.rematch, &ended, &r.moves, &first, &whiteMS, &blackMS,
			&st.moves, &st.opened, &st.reset, &st.closedRounds, &st.closedCap, &st.repaired, &st.fell, &st.savingRolls, &st.saved, &st.whiteLost, &st.blackLost, &st.mateByRoll, &st.hist); err != nil {
			rows.Close()
			return out, err
		}
		// ... (fill r from the nullables; created scans as int64 ms: scan into an int64 and convert)
		games = append(games, r)
	}
	rows.Close()
	// then: searches in the range, events in the range (two more queries),
	// then the aggregation below.
	...
}
```

Write the scan with `created` and the other times scanned as `int64`/`sql.NullInt64` and converted with `time.UnixMilli`; `r.stats` is set only when `st.moves.Valid`, with `parseHist` for the histogram. Then aggregate:

- `Games = len(games)`; `GotGoing` = result in the finished set (write `isFinishedReason(result string) bool` from the same list as `finishedReasons`).
- `Days`: the same zero-fill as `VisitStats` (factor its loop into `func dayRange(from, to time.Time, earliest time.Time, loc *time.Location) []string` returning the day keys, and use it from both; "all" starts at the first game's or first practice event's day). Practice counts come from the events query (`kind = 'practice'`).
- `Ends`, in this fixed order, dropping zeros: Checkmate (checkmate without `mate_by_roll`), Mate by a roll, Resignation, Out of time (`timeout`), Draw (`stalemate, fifty_moves, repetition, insufficient_material, timeout_vs_insufficient`), Aborted, Nobody came (`expired`). `retired` games are ignored.
- `Lengths`: for games that got going, bucket `moves` into `1–10, 11–20, 21–30, 31–40, 41–50, 51–60, 61–80, 81+` (keep zeros). `MedianMoves` is the median of those `moves` (the mean of the middle two for an even count, rounded). `MedianMinutes`: median of `ended − first` in whole minutes, games with a first turn only. `OnClock`: `timeout` or `timeout_vs_insufficient`. `WinnerClockLeft`: median of the winner's clock (`white_ms` or `black_ms` of the last turn by `winner`) over decisive games with a turn, as `m:ss`.
- `Heat[weekday][hour/2]++` on `created.In(loc)`, with Monday = 0 (`(int(t.Weekday()) + 6) % 7`).
- `Friend`: `kind == "friend"`: `Links` = count, `Joined` = `hasBlack`, `Finished` = got going, `Rematches` = games in the range with `rematch` (any kind) whose parent... keep it simple: `Rematches` = games in the range that are rematches (`rematch_of` set). `MedianJoin`: median of `joined − created` over friend games with a `joined`, as `m:ss`.
- `Quick` from the `searches` rows in the range (by `started_at`): `Searches`, `Matched`, `GaveUp = Searches − Matched`, `Alone` = `others == 0`; `Waits` buckets over matched searches' `ended − started`: `< 10s, < 30s, < 60s, < 120s, else`; `MedianWait` over matched, `MedianGaveUp` over unmatched, as `m:ss` (`""` when none).
- `Road` over games with stats: sums; `Games` = games with stats; `NoFall` = stats with `Fell == 0`; `OpenHist` summed; decisive games (`winner != ""`): compare `WhiteLost` and `BlackLost` from the winner's side.
- `Reconnects`/`Restarts` from the events query.

Helpers at the bottom of the file: `median(ns []int) int`, `clock(ms int64) string` (`m:ss`, minutes unbounded), `bucket(moves int) string`.

- [ ] **Step 4: Run the tests**

Run: `go test ./store -count=1 && go vet ./store`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add store/gamesection.go store/gamesection_test.go store/stats.go
git commit -m "store: everything the Games, Quick match and The road sections show"
```

---

### Task 5: The sections on the page, and the docs

**Files:**
- Modify: `server/stats.go` (`statsPage` gains `Section store.GameSection` plus the derived chart data), `server/stats.html` (three sections, the Health tiles, the sidebar links), `server/stats_test.go`
- Modify: `CLAUDE.md`, `docs/superpowers/specs/2026-10-08-stats-design.md` (status line), `docs/superpowers/plans/2026-10-03-00-roadmap.md`

**Interfaces:**
- Consumes: `store.GameSection` (Task 4); the template's existing `pct`, `share`, `dict`, `list` and `chip` helpers.
- Produces: the rendered sections; `statsPage.Section`, `statsPage.DayKindMax int` (the busiest day's games, all kinds), `statsPage.HeatMax int`, `statsPage.LengthMax`, `statsPage.WaitMax`, `statsPage.OpenMax int`, `statsPage.OpenPct [6]int` (each open-holes count as a percentage of turns), `statsPage.HeatRows []heatRow{Day string; Cells []heatCell{Level int; Title string}}` (level 0–4 by the cell's share of `HeatMax`: 0, then ≤25%, ≤50%, ≤85%, more).

- [ ] **Step 1: Write the failing test**

Append to `server/stats_test.go`:

```go
func TestStatsShowsTheGames(t *testing.T) {
	s, ts := newTestServer(t)
	s.StatsPassword = "hunter2"
	ctx := t.Context()
	now := time.Now()
	for _, g := range []struct {
		code, kind, reason, winner string
		st                         *store.GameStats
	}{
		{"G00001", "quick", "checkmate", "white", &store.GameStats{Moves: 30, Opened: 5, Fell: 2, BlackLost: 2, OpenHist: [6]int{5, 10, 10, 5, 0, 0}}},
		{"G00002", "friend", "resignation", "black", &store.GameStats{Moves: 12, Opened: 3, Fell: 1, WhiteLost: 1, MateByRoll: false, OpenHist: [6]int{2, 5, 5, 0, 0, 0}}},
	} {
		if err := s.store.CreateGame(ctx, store.Game{Code: g.code, Kind: g.kind, White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: now.Add(-2 * time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if err := s.store.AddTurn(ctx, g.code, store.Turn{Ply: 0, Move: "e2e4", WhiteMS: 500000, BlackMS: 600000, At: now.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if err := s.store.EndGame(ctx, g.code, store.Result{EndedAt: now.Add(-30 * time.Minute), Reason: g.reason, Winner: g.winner}, nil, g.st); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.AddSearch(ctx, store.Search{StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-time.Hour).Add(19 * time.Second), Guest: "ann000000000", Matched: true}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats?days=7", nil)
	req.SetBasicAuth("", "hunter2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	for _, want := range []string{
		`<h2>Games</h2>`, `<h2>Quick match</h2>`, `<h2>The road</h2>`,
		"Checkmate", "Resignation", "Quick match 1", "Friend link 1", // games per day legend with totals
		"How full the road gets", "Did the road decide it?", "Lost less to the road, and won",
		"found an opponent", "0:19", "Reconnects", "Server restarts",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	for _, never := range []string{"G00001", "G00002", "soon", "{{", "ZgotmplZ"} {
		if strings.Contains(body, never) {
			t.Errorf("the page shows %q", never)
		}
	}
}
```

- [ ] **Step 2: Run it to see it fail**

Run: `go test ./server -run TestStatsShowsTheGames -count=1`
Expected: FAIL (the page lacks `<h2>Games</h2>`; `soon` is present).

- [ ] **Step 3: Implement**

`server/stats.go`: in `stats()`, after the funnel, `section, err := s.store.GameSection(r.Context(), from, to, loc)` into `page.Section`, and compute the maxima and the heat rows:

```go
	for _, d := range section.Days {
		page.DayKindMax = max(page.DayKindMax, d.Quick+d.Friend+d.Practice)
	}
	for _, c := range section.Lengths {
		page.LengthMax = max(page.LengthMax, c.N)
	}
	for _, c := range section.Quick.Waits {
		page.WaitMax = max(page.WaitMax, c.N)
	}
	turns := 0
	for _, n := range section.Road.OpenHist {
		turns += n
	}
	for i, n := range section.Road.OpenHist {
		page.OpenPct[i] = share(n, turns)
		page.OpenMax = max(page.OpenMax, page.OpenPct[i])
	}
	page.HeatRows = heatRows(section.Heat)
```

with

```go
type heatRow struct {
	Day   string
	Cells []heatCell
}

type heatCell struct {
	Level int // 0 (none) to 4 (the busiest)
	Title string
}

var heatBlocks = [12]string{"12–2 am", "2–4 am", "4–6 am", "6–8 am", "8–10 am", "10–12 am", "12–2 pm", "2–4 pm", "4–6 pm", "6–8 pm", "8–10 pm", "10–12 pm"}

// heatRows shades each weekday × two-hour block by its share of the
// busiest block: 0 for none, then up to a quarter, half, 85%, and the rest.
func heatRows(h store.Heat) []heatRow {
	top := 0
	for _, row := range h {
		for _, n := range row {
			top = max(top, n)
		}
	}
	days := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var rows []heatRow
	for d, row := range h {
		hr := heatRow{Day: days[d]}
		for b, n := range row {
			level := 0
			switch t := 100 * n / max(top, 1); {
			case n == 0:
			case t <= 25:
				level = 1
			case t <= 50:
				level = 2
			case t <= 85:
				level = 3
			default:
				level = 4
			}
			hr.Cells = append(hr.Cells, heatCell{Level: level, Title: fmt.Sprintf("%s %s: %d games", days[d], heatBlocks[b], n)})
		}
		rows = append(rows, hr)
	}
	return rows
}
```

`server/stats.html`: the sidebar becomes `<a href="#overview">Overview</a><a href="#games">Games</a><a href="#match">Quick match</a><a href="#road">The road</a><a href="#visitors">Visitors</a><a href="#health">Health</a>` (no more "soon"). Between the Overview and Visitors sections add, following the canvas (monotone blues: `--c1` dark for quick match, `--c2` for friend link, `--c3` light for practice; graphite bars for single-series charts; the sequential blues `--s0`…`--s4` for the heat grid, add them to `:root`: `--s0:#eff1f3;--s1:#b6e3ff;--s2:#54aeff;--s3:#0969da;--s4:#0a3069`):

```html
<section id="games">
<div><h2>Games</h2><p class="sub">{{.Section.Games}} games created, {{.Section.GotGoing}} of them got going</p></div>
<div class="card">
<div class="row-top"><div><h3>Games per day</h3><p class="sub">Quick match, friend links and practice</p></div>
<div class="legend"><span><i class="dot" style="background:var(--c1)"></i>Quick match {{.QuickTotal}}</span><span><i class="dot" style="background:var(--c2)"></i>Friend link {{.FriendTotal}}</span><span><i class="dot" style="background:var(--c3)"></i>Practice {{.PracticeTotal}}</span></div></div>
<div class="bars" style="--n:{{len .Section.Days}}">{{$max := .DayKindMax}}{{range .Section.Days}}<div class="col" title="{{.Day}}: {{.Quick}} quick match, {{.Friend}} friend link, {{.Practice}} practice">{{if .Practice}}<i style="height:{{pct .Practice $max}}%;background:var(--c3)"></i>{{end}}{{if .Friend}}<i style="height:{{pct .Friend $max}}%;background:var(--c2)"></i>{{end}}{{if .Quick}}<i style="height:{{pct .Quick $max}}%;background:var(--c1)"></i>{{end}}</div>{{end}}</div>
<div class="axis">{{with .Section.Days}}<span>{{(index . 0).Day}}</span><span>{{(index . (len . | sub1)).Day}}</span>{{end}}</div>
</div>
<div class="grid g2">
{{template "list" dict "Title" "How games end" "Sub" (printf "%d games created in the range" .Section.Games) "Rows" .Section.Ends "Of" .Section.Games}}
<div class="card"><div><h3>Game length</h3><p class="sub">Moves by both sides, games that got going</p></div>
<div class="hist">{{$m := .LengthMax}}{{range .Section.Lengths}}<div class="hcol">{{.N}}<i style="height:{{pct .N $m}}%"></i></div>{{end}}</div>
<div class="hlab">{{range .Section.Lengths}}<span>{{.Label}}</span>{{end}}</div>
<div class="list"><div><span>Median game</span><span class="val">{{.Section.MedianMoves}} moves · {{.Section.MedianMinutes}} min</span></div><div><span>Decided on the clock</span><span class="val">{{.Section.OnClock}} of {{.Section.GotGoing}} · {{share .Section.OnClock .Section.GotGoing}}%</span></div><div><span>Winner's clock left (median)</span><span class="val">{{or .Section.WinnerClockLeft "–"}}</span></div></div>
</div>
</div>
<div class="grid g21">
<div class="card"><div class="row-top"><div><h3>When people play</h3><p class="sub">Games started, by weekday and time</p></div><div class="scale"><span>Fewer</span><i style="background:var(--s0)"></i><i style="background:var(--s1)"></i><i style="background:var(--s2)"></i><i style="background:var(--s3)"></i><i style="background:var(--s4)"></i><span>More</span></div></div>
<div class="heat"><div class="h"><span></span><span>12a</span><span>2a</span><span>4a</span><span>6a</span><span>8a</span><span>10a</span><span>12p</span><span>2p</span><span>4p</span><span>6p</span><span>8p</span><span>10p</span></div>
{{range .HeatRows}}<div><span>{{.Day}}</span>{{range .Cells}}<i class="l{{.Level}}" title="{{.Title}}"></i>{{end}}</div>{{end}}</div>
</div>
<div class="card"><div><h3>Friend links</h3><p class="sub">{{.Section.Friend.Links}} links made</p></div>
<div class="rows">{{$links := .Section.Friend.Links}}{{range .FriendRows}}<div class="row"><div class="row-top"><span>{{.Label}}</span><span class="val">{{.N}} <small>{{share .N $links}}%</small></span></div><div class="track thick"><i style="width:{{pct .N $links}}%"></i></div></div>{{end}}</div>
<div class="list"><div><span>Median time to join</span><span class="val">{{or .Section.Friend.MedianJoin "–"}}</span></div><div><span>Rematches in the range</span><span class="val">{{.Section.Friend.Rematches}}</span></div></div>
</div>
</div>
</section>

<section id="match">
<div><h2>Quick match</h2><p class="sub">{{.Section.Quick.Searches}} searches</p></div>
<div class="grid g21">
<div class="card"><div><h3>How long people waited</h3><p class="sub">Searches that found an opponent</p></div>
<div class="hist">{{$w := .WaitMax}}{{range .Section.Quick.Waits}}<div class="hcol">{{.N}}<i style="height:{{pct .N $w}}%"></i></div>{{end}}</div>
<div class="hlab">{{range .Section.Quick.Waits}}<span>{{.Label}}</span>{{end}}</div></div>
<div class="card"><div><h3>Found an opponent</h3><p class="sub">Out of {{.Section.Quick.Searches}} searches</p></div>
<div class="big"><span class="num">{{share .Section.Quick.Matched .Section.Quick.Searches}}%</span><span class="sub">{{.Section.Quick.Matched}} matched · {{.Section.Quick.GaveUp}} gave up</span></div>
<div class="share"><i style="flex:{{.Section.Quick.Matched}} 1 0;background:var(--c1)"></i><i style="flex:{{.Section.Quick.GaveUp}} 1 0;background:var(--a4)"></i></div>
<div class="list"><div><span>Median wait</span><span class="val">{{or .Section.Quick.MedianWait "–"}}</span></div><div><span>Gave up after (median)</span><span class="val">{{or .Section.Quick.MedianGaveUp "–"}}</span></div><div><span>Searched when nobody else was</span><span class="val">{{.Section.Quick.Alone}} · {{share .Section.Quick.Alone .Section.Quick.Searches}}%</span></div></div>
</div>
</div>
</section>

<section id="road">
<div><h2>The road</h2><p class="sub">Potholes, falls and the Mamdani, from the dice of the {{.Section.Road.Games}} games that got going</p></div>
<div class="grid g4">
<div class="tile"><span>Potholes a game</span><span class="num">{{.PerGame .Section.Road.Opened}}</span><span class="note">{{.Section.Road.Opened}} opened</span></div>
<div class="tile"><span>Falls a game</span><span class="num">{{.PerGame .Section.Road.Fell}}</span><span class="note">{{.Section.Road.Fell}} pieces · {{share .Section.Road.NoFall .Section.Road.Games}}% of games had none</span></div>
<div class="tile"><span>Saving rolls saved</span><span class="num">{{share .Section.Road.Saved .Section.Road.SavingRolls}}%</span><span class="note">{{.Section.Road.Saved}} of {{.Section.Road.SavingRolls}}</span></div>
<div class="tile"><span>Ended by a roll</span><span class="num">{{share .Section.Road.MateByRoll .Section.Road.Games}}%</span><span class="note">{{.Section.Road.MateByRoll}} mates by the dice</span></div>
</div>
<div class="grid g2">
<div class="card"><div><h3>How full the road gets</h3><p class="sub">Potholes open after each turn. Tells us whether 3 rounds and a cap of 5 feel right.</p></div>
<div class="hist">{{$o := .OpenMax}}{{range $i, $p := .OpenPct}}<div class="hcol">{{$p}}%<i style="height:{{pct $p $o}}%"></i></div>{{end}}</div>
<div class="hlab"><span>0</span><span>1</span><span>2</span><span>3</span><span>4</span><span>5 (cap)</span></div>
<div class="list"><div><span>Ran their 3 rounds</span><span class="val">{{.Section.Road.ClosedRounds}}</span></div><div><span>Closed early by the cap</span><span class="val">{{.Section.Road.ClosedCap}}</span></div><div><span>Repaired by the Mamdani</span><span class="val">{{.Section.Road.Repaired}}</span></div><div><span>Reset by a roll landing on them</span><span class="val">{{.Section.Road.Reset}}</span></div></div>
</div>
<div class="card"><div><h3>Did the road decide it?</h3><p class="sub">{{.Decisive}} decisive games: did the winner lose less to potholes than the loser?</p></div>
<div class="share"><i style="flex:{{.Section.Road.LessWon}} 1 0;background:var(--c1)"></i><i style="flex:{{.Section.Road.MoreWon}} 1 0;background:var(--c3)"></i><i style="flex:{{.Section.Road.Same}} 1 0;background:var(--a4)"></i></div>
<div class="rows">{{$d := .Decisive}}{{range .DecidedRows}}<div class="row"><div class="row-top"><span>{{.Label}}</span><span class="val">{{.N}} <small>{{share .N $d}}%</small></span></div><div class="track"><i style="width:{{pct .N $d}}%;background:{{.Color}}"></i></div></div>{{end}}</div>
</div>
</div>
</section>
```

Where the template reads a derived value (`QuickTotal`, `FriendTotal`, `PracticeTotal`, `FriendRows` (Someone joined / Game finished), `Decisive`, `DecidedRows` with a `Color` each, and the method `PerGame(n int) string` returning `n / Road.Games` to one decimal, `"0"` without games), add it to `statsPage` and fill it in `stats()`. The Health section adds two tiles: `Reconnects` ("a live stream dropped and came back") and `Server restarts`. New CSS, next to the existing rules: `.g2{grid-template-columns:repeat(2,minmax(0,1fr))}.g21{grid-template-columns:minmax(0,1.7fr) minmax(0,1fr)}`, `.hist{height:130px;display:flex;align-items:flex-end;gap:6px}.hcol{flex:1;min-width:0;height:100%;display:flex;flex-direction:column;align-items:center;justify-content:flex-end;gap:6px;font:500 12px "IBM Plex Mono",monospace;color:var(--text-2)}.hcol i{display:block;width:100%;background:var(--a);border-radius:4px 4px 0 0}.hlab{display:flex;gap:6px}.hlab span{flex:1;min-width:0;text-align:center;font:500 11px "IBM Plex Mono",monospace;color:var(--text-3)}`, `.list{display:flex;flex-direction:column}.list>div{display:flex;justify-content:space-between;gap:12px;padding:10px 0;border-bottom:1px solid var(--line);font-size:13px}.list>div:last-child{border-bottom:0;padding-bottom:0}`, `.heat{display:flex;flex-direction:column;gap:4px}.heat>div{display:grid;grid-template-columns:34px repeat(12,minmax(0,1fr));gap:4px;align-items:center}.heat span{font-size:12px;color:var(--text-3)}.heat .h span{font:500 11px "IBM Plex Mono",monospace;text-align:center}.heat i{display:block;height:22px;border-radius:3px}.l0{background:var(--s0)}.l1{background:var(--s1)}.l2{background:var(--s2)}.l3{background:var(--s3)}.l4{background:var(--s4)}`, `.scale{display:flex;align-items:center;gap:4px;font-size:12px;color:var(--text-3)}.scale i{width:14px;height:14px;border-radius:4px}`, `.share{display:flex;height:10px;gap:3px}.share i{display:block;border-radius:999px}`, `.big{display:flex;flex-direction:column;gap:4px}`; and in the phone media query `.g2,.g21{grid-template-columns:minmax(0,1fr)}` and `.heat>div{grid-template-columns:28px repeat(12,minmax(0,1fr));gap:3px}`.

The `hcol` bars use `height:{{pct ...}}%` inside a flex column of fixed height; the bar's `i` needs `flex:none`. Check in the browser (Step 5) that the bars stand on the baseline.

- [ ] **Step 4: Run the tests**

Run: `go test ./server -count=1 && go vet ./... && go test -race -short ./...`
Expected: `ok`. `TestStatsNeedsPassword` still renders the empty page (every new value has a zero case).

- [ ] **Step 5: Look at it**

With `pnpm --dir web build` done, run the server on a spare port with `STATS_PASSWORD=x` and its own database; play a short practice game and a quick match between two browsers against it (or `pnpm --dir web playtest --base <url> --match --games 2`, which plays through quick match; the playtest's `X-Playtest` header keeps it out of the visits but not out of the games, which is what we want here). Open `/stats?days=7` at 1280 px and 390 px: the three new sections render with real numbers, the heat grid has a dark cell for this hour, nothing overflows on the phone, and no `{{` or `ZgotmplZ` appears in the source.

- [ ] **Step 6: Docs**

- `CLAUDE.md`: under **Where things stand**, "**Done: stats, plan 2** ([plan](docs/superpowers/plans/2026-10-09-stats-2-games.md)): every game that gets going saves what the road did to it (`game_stats`, written with the result; old games backfilled at startup), games carry a kind (`quick`/`friend`) and when Black joined, quick-match searches are logged (`searches`), practice games, reconnects and restarts are counted (`events`), and `/stats` shows Games, Quick match, The road and the Health tiles." In **Working notes**, extend the Stats note: "`game_stats` is written in `EndGame`'s transaction; `game.StatsOf` replays a saved game for the backfill (idempotent, runs at startup)."
- The spec's status line: "Plan 1 built (PR #22); plan 2: [2026-10-09-stats-2-games.md](../plans/2026-10-09-stats-2-games.md)."
- The roadmap's row 07: "plan 1 done, plan 2 done".

- [ ] **Step 7: Commit**

```bash
git add server/stats.go server/stats.html server/stats_test.go CLAUDE.md docs/superpowers/specs/2026-10-08-stats-design.md docs/superpowers/plans/2026-10-03-00-roadmap.md
git commit -m "server: the Games, Quick match and The road sections of /stats"
```

- [ ] **Step 8: Every check, then hand over**

```bash
go vet ./... && go test -race -short ./...
go test ./rules -count=1
pnpm --dir web check && pnpm --dir web test && pnpm --dir web build
CONTRACT=1 go test ./contract -count=1
```

Then a playtest against a server on a spare port (`--games 2` and `--phone`), and the whole-branch review (superpowers:requesting-code-review), then superpowers:finishing-a-development-branch. **Don't push to `main` without asking.**

## Notes for the executor

- Ports 8080 and 5173 may belong to another session: use spare ports and your own `DB_PATH`; stop a server by its PID.
- The store opens one connection (`SetMaxOpenConns(1)`): close every `rows` before the next query, as `GameSection` does with its three reads in sequence.
- `GameSection`'s SQL uses correlated subqueries on `turns`; with an index on `turns(game, ply)` (the primary key) they are cheap at this scale.
- Expected numbers in the tests are worked out by hand in comments; when one is off, recheck the seed before the query.
