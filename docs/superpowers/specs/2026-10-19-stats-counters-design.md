# Stats counters: daily counts written with the data

**Date:** 2026-10-19
**Status:** Built on branch `stats-counters` ([plan](../plans/2026-10-19-stats-counters.md)), stacked on PR #25.
**Builds on:** [the stats spec](2026-10-08-stats-design.md) (what the page shows and why). This replaces that spec's 24-hour cache (plan 3), which is no longer needed.

## Why

`/stats` computes every number from the raw tables on each load: every game in the range is read with its turns and its `game_stats` row, and every quick-match search. The cost grows with the number of games, and the database has one connection that moves also wait on. A cache only hides that: when it expires, or on a refresh, the whole range is scanned again, possibly on a busy evening.

Time-series databases avoid this by storing pre-aggregated buckets and reading those instead of raw events: TimescaleDB's continuous aggregates keep results per time bucket and recompute only buckets whose data changed, and Prometheus keeps histogram bucket counts so that quantiles can be computed after summing. This design does the same with counters written at the moment the data is written.

## Decisions

- **The whole page stays private:** `GET /stats` behind Basic Auth with `STATS_PASSWORD`, as now. A public page was considered and dropped.
- **Counters at write time.** Every write that the page counts also adds to a daily counter, inside the same transaction. A page load only sums counters; it never reads `games`, `turns`, `game_stats` or `searches`. (Considered: rolling up finished days nightly with a live today, as TimescaleDB does; rejected for counters, which have no rollup job and no late-data handling.)
- **Daily buckets.** The page shows nothing finer than a day except the weekday × two-hour grid, which a day carries as twelve keys. Hourly buckets would multiply rows read by up to 24 for no visible gain; if a sub-day view is ever wanted, an hourly table for the last few days can sit beside this one.
- **No medians.** Medians can't be summed across days. Each becomes a histogram with one share line at a threshold that matches a decision (below). Every number on the page is then an exact sum. The median game length in minutes goes; the moves histogram stays.
- **Raw rows remain the source of truth.** Counters are derived and can always be rebuilt from `games`, `turns`, `game_stats`, `searches`, `events` and `browser_errors`.

## Measured cost

Benchmarked with the repo's driver (modernc sqlite v1.60.1), the same DSN (WAL, `synchronous(NORMAL)`) and one connection, on an M4 Pro; expect Railway to be 3–5× slower:

| | |
| --- | --- |
| A game-end transaction today | 16 µs |
| … plus 25 single-row counter upserts | 146 µs |
| … plus the same 25 rows as one multi-row upsert | 64 µs |
| Reading 90 days × 83 keys | 2.4 ms |
| Reading 400 days × 83 keys ("All" after a year) | 7.4 ms |

Writes stay well under a millisecond on a slow vCPU. Reads grow with days, not games.

## Storage

Migration 8 (not yet released) gains:

```sql
CREATE TABLE stats_days (
	day TEXT NOT NULL,     -- 2026-10-19, in STATS_TZ
	key TEXT NOT NULL,     -- what is counted: games.quick, end.checkmate, len.11-20, ...
	n   INTEGER NOT NULL,
	PRIMARY KEY (day, key)
) WITHOUT ROWID;
CREATE TABLE stats_state (
	id      INTEGER PRIMARY KEY CHECK (id = 1),  -- one row
	version INTEGER NOT NULL,  -- the counters' definition the rows were written under
	zone    TEXT NOT NULL      -- the STATS_TZ that cut the days
);
```

- `day` is text in `YYYY-MM-DD`, so ranges compare as strings. A day's keys (about 83 at most) sit together on one or two leaf pages.
- A new number is a new key, with no migration. A change to what an existing key means bumps the code's counters version, which rebuilds the table (below).
- `searches` and `events` are no longer pruned (a few dozen bytes a row), so a rebuild can recount them.
- `browser_errors` is still pruned, now at a midnight in `STATS_TZ`: rows before the start of the day 400 days ago go. A rebuild gives each day the larger of its `errors` counter and its recounted rows: errors past the daily cap, and errors whose rows were pruned, have no rows to recount, so their counts are never lost (found in the final review, 2026-10-19).

## How a write counts

The `store` package is the only writer, so counting lives there, in each method's existing transaction. The game code and the HTTP handlers don't change.

- **One statement per write.** The counting function returns the write's keys and amounts; zero amounts are dropped (many `road.*` values are 0, so a game end writes about 12 rows, not 25); the rest go in one multi-row upsert: `INSERT INTO stats_days (day, key, n) VALUES (?, ?, ?), (?, ?, ?), … ON CONFLICT (day, key) DO UPDATE SET n = n + excluded.n`.
- **The store reads what it counts from the row it updates, not from the caller.** `EndGame` updates `WHERE code = ? AND ended_at IS NULL RETURNING created_at, kind, rematch_of, joined_at, playtest`; the winner's clock is the last turn's, by one primary-key seek (`SELECT white_ms, black_ms FROM turns WHERE game = ? ORDER BY ply DESC LIMIT 1`, after the final turn is written). Moves come from the `GameStats` the game passes in. Turns are never counted or scanned. `SeatBlack` updates `WHERE code = ? AND black IS NULL RETURNING created_at, kind, rematch_of, playtest`. `ExpireWaiting` updates with `RETURNING created_at, playtest` and groups the days in Go (SQLite can't apply an IANA zone). `CreateGame` has the row it inserts. `AddGameStats` (the backfill) reads the game's `created_at`, `kind`, `rematch_of`, `result`, `winner` and `playtest` by primary key.
- **Only what changed counts, and a no-op is not an error.** A guarded update that changes no row (a repeated `EndGame` or `SeatBlack`) counts nothing and returns nil; one for a game that was never saved still returns an error, as before (a primary-key check that runs only when no row changed). Today `execOne` returns an error when no row changed, which would retire the game; these two calls stop using it, and `execOne` goes. `AddGameStats` counts only when its `INSERT OR IGNORE` inserted, and moves the game's counters from "ended, road unknown" to what the stats say.
- **Counting can't fail on its own.** The counting functions are pure: no I/O, no error returns, every input mapped to a key (an unknown result goes to `end.other`, which the page doesn't show). The only way a counted write can fail is the SQL failing, which would fail the write anyway. So counting adds no new way for a game's save to fail (a failed save retires the game).
- **Searches, events and errors add no commit.** `AddSearch`, `AddEvent` and `AddBrowserError` are already one transaction each; they become a two-statement transaction (the raw insert and one upsert). With WAL and `synchronous(NORMAL)` a commit doesn't fsync.
- **Error spam stays cheap.** Until milestone 07's per-IP limits, `AddBrowserError` reads today's `errors` counter (a primary-key lookup) and stops storing the raw message after 200 a day, while still counting.
- **Moves are unchanged:** a move writes its turn and nothing else.

### What counts what

A game's keys go on **the day it was created**, in `STATS_TZ`, whenever they become known. A game created at 23:00 that ends at 01:00 counts its ending on the day it was created, so "of the games created that day, how did they end" stays consistent, and a late fact (a friend link joined the next morning, a waiting game expiring a day later) updates its creation day.

| Write | Day | Keys |
| --- | --- | --- |
| `CreateGame` | the game's | `games.quick` or `games.friend`; `friend.links` (a friend game that isn't a rematch) or `friend.rematches` (a rematch of a friend game) |
| `SeatBlack`, on a friend link that isn't a rematch | the game's | `friend.joined`; `join.<bucket>` by `joined_at − created_at`: `lt1m`, `1-5m`, `5-30m`, `30m-2h`, `2h+` |
| `EndGame` | the game's | `end.<result>`: `checkmate`, `roll` (checkmate by a roll), `resignation`, `timeout` (lost on time), `draw` (stalemate, fifty moves, repetition, insufficient material, and `timeout_vs_insufficient`), `aborted`, `expired`, else `end.other`. If the game got going (any of those but `aborted`, `expired`, `other`): `got_going`; `len.<bucket>` (moves by both sides: `1-10`, `11-20`, `21-30`, `31-40`, `41-50`, `51-60`, `61-80`, `81+`); `heat.<block>` (0–11, the two-hour block of `created_at`); `clock.decided` (result `timeout` only); `left.<bucket>` for the winner's clock, only when there is a winner and at least one turn (`lt30s`, `30s-1m`, `1-2m`, `2-5m`, `5m+`); `friend.finished` (a friend link that isn't a rematch). From its `GameStats`: `road.games`, `road.opened`, `road.reset`, `road.closed_rounds`, `road.closed_cap`, `road.repaired`, `road.fell`, `road.no_fall`, `road.saving_rolls`, `road.saved`, `road.mate_by_roll`, `open.0`–`open.5` (each game's open-holes counts added in), `decided.less` / `decided.more` / `decided.same` (decisive games: the winner lost fewer, more or as many pieces to the road) |
| `ExpireWaiting` (startup, bulk) | each game's | `end.expired` |
| `AddGameStats` (the startup backfill) | the game's | the `road.*`, `open.*` and `decided.*` keys, through the same function as `EndGame` |
| `AddSearch` | the search's start | `search`; `search.matched` or `search.gaveup`; `search.alone` (nobody else was looking); `wait.<bucket>` when matched, `gaveup.<bucket>` when not (`lt10s`, `10-30s`, `30-60s`, `1-2m`, `2m+`) |
| `AddEvent` | its own | `event.practice`, `event.reconnect`, `event.restart` |
| `AddBrowserError` | its own | `errors` |

- **Playtests write no counters.** A game marked `playtest` counts nothing at any step (the store sees the mark in each `RETURNING`), and a playtest's searches, events and errors are never written (as now).
- **Retired games** (ended only by migrations, which bypass counting) keep their creation keys and count no ending, live or in a rebuild. Any future migration that changes games bumps the counters version.
- **One counting function per step**, shared by the live writes and the rebuild: `countCreated`, `countJoined` and `countEnded` for a game (the rebuild calls whichever apply to each saved game), plus one each for a search, an event and an error.

## Reading the page

`GameSection` and its scans are removed. A page load is two queries:

1. `SELECT day, key, n FROM stats_days WHERE day >= ? AND day < ?` over the range and the range before it together (`[prevFrom, to)`; no lower bound for "All"). It reads in primary-key order with no sort. Go sums it into the range's totals, the previous range's totals (the change chips), the per-day chart (`games.quick`, `games.friend`, `event.practice`; days without rows are zero) and the heat grid (each day's `heat.*` under its weekday).
2. The newest `browser_errors` row in the range, through the `browser_errors_at` index, for the "latest" line.

Whether the counters are current is known in memory from startup (below), so the page reads nothing else. A 90-day page reads at most 180 × 83 rows, however many games were played. The page is always current: no cache, no Refresh button; reloading is refreshing. Stats queries keep a 2-second timeout as a backstop.

What changes on the page:

- **Game length:** the moves histogram; "Decided on the clock" stays; the median line goes. A new small histogram, **Winner's clock left**, with the line "under 1 min left: N%" (`lt30s` + `30s-1m` over all `left.*`).
- **Friend links:** joined, finished and rematches as now; the median time to join becomes its own card, **Time to join**, a histogram with "joined within 5 min: N%" (`lt1m` + `1-5m` over the joins with a known time).
- **Quick match:** three cards: the wait histogram, **Gave up after** (a histogram, with "gave up within 30 s: N%": `lt10s` + `10-30s` over `search.gaveup`), and found an opponent with "searched when nobody else was". The median wait line goes.
- **When people play:** the same grid, from `heat.*` per day.

## Startup

The store needs `STATS_TZ` before its first write, and the stats work must finish before any game's clock can run. So `main` loads `STATS_TZ` first and hands the zone to the store as it opens it (tests that don't care get UTC). Then, in order, before the server listens:

1. migrations;
2. prune browser errors (at a `STATS_TZ` midnight);
3. `ExpireWaiting` (counts `end.expired`);
4. the `game_stats` backfill (counts each backfilled game's road keys);
5. the rebuild, if needed;
6. the `restart` event;
7. `LoadForRestore` and `hub.Restore` (each restored game's 10-second grace starts here);
8. listen.

Today the backfill and the restart event run after `hub.Restore`, while restored games' graces are already running; this order fixes that too.

## Rebuild

- The code holds a counters version constant. At step 5 the store compares it and the zone with `stats_state`. If either differs, or `stats_state` is empty (the first deploy), it rebuilds; otherwise it does nothing.
- **A rebuild streams, sums in memory, then writes once per key.** One query streams every game with its `game_stats` row (a left join) and its last turn's clocks (a primary-key seek per game); then the searches, the events and the errors, each in one pass. Each row goes through the counting functions into a Go map from (day, key) to a count. Nothing loads a game's turns. Then, in one transaction: delete the counters, insert the map's rows in multi-row batches, and write the version and zone. At 400 days that is about 33,000 rows.
- **Budget:** a rebuild of 100,000 games must take under 3 seconds on Railway, well inside its 30-second health check, and the benchmark (below) checks it. It holds the database only during the final transaction, and nothing else is running yet.
- A rebuild runs only on a deploy that changes the counters version or `STATS_TZ`, never while serving. It logs `stats rebuilt` at INFO with the days and rows written and the time taken.
- If it fails, it's logged at ERROR, the transaction rolls back, the server starts with the counters it had, and the next start tries again. The store remembers that the counters are stale, and the page shows one line: "These counts are from an older version: a rebuild failed, see the logs."

## Concurrency

SQLite has one writer, so upserts are serialized and no increment is lost; there's no contention on a hot row beyond that. Reads don't block writes (WAL).

## Testing

1. **Each write writes exactly its keys:** table tests for every store write, including a playtest game (none), a repeated `EndGame` or `SeatBlack` (none, and no error), a game ending after midnight (its creation day), a resignation and a timeout (no final turn: the clock comes from the last saved turn), `timeout_vs_insufficient` (a draw, not `clock.decided`), and zero amounts (not written).
2. **Live counting equals a rebuild:** a few hundred random games, searches, events and errors written through the store's normal methods across many days, including a daylight-saving change, games ending the next day, friend links joined the next day, aborted, expired and retired games, rematches and playtests. The `stats_days` rows after live counting must equal the rows after a rebuild.
3. **Startup order:** a test that the backfill and the rebuild finish before `hub.Restore`, and that the first start fills the counters from existing games.
4. **The page from counters:** seeded counters render every section; each threshold line equals its bucket sum; the chips compare the right days; "All" has no chips; the stale-counters line shows when the version is behind.
5. **Size:** benchmarks, reported in the PR: a page load over 20,000 synthetic games across 400 days (flat as games grow), a game end's write, and a rebuild of 100,000 games (under the 3-second budget once scaled for Railway).
6. **Error spam:** past 200 errors in a day, the raw rows stop and the counter keeps counting.
7. **As before:** `go test -race`, the contract checks (`/stats` is 401 without the password), svelte-check, Vitest, the build, the browser pass at 1440 and 390 px, and a playtest whose games write no counters.

## Out of scope

- A public page, per-person numbers, visit tracking (removed 2026-10-19).
- Hourly buckets and downsampling.
- Charts over time beyond games per day.
- Per-IP limits (milestone 07); the 200-a-day cap on stored error messages stands in until then.
