# Stats counters: daily counts written with the data

**Date:** 2026-10-19
**Status:** Agreed in brainstorming; awaiting spec review.
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
	version INTEGER NOT NULL,  -- the counters' definition the rows were written under
	zone    TEXT NOT NULL      -- the STATS_TZ that cut the days
);
```

- A write is `INSERT INTO stats_days (day, key, n) VALUES (?, ?, ?) ON CONFLICT (day, key) DO UPDATE SET n = n + excluded.n`.
- `day` is text in `YYYY-MM-DD`, so ranges compare as strings.
- A new number is a new key, with no migration. A change to what an existing key means bumps the code's counters version, which rebuilds the table (below).
- `searches` and `events` are no longer pruned (a few dozen bytes a row), so a rebuild can recount every family. `browser_errors` is still pruned after 400 days (its messages are the bulky part); a rebuild keeps the `errors` counts for days before the oldest error row it has.

## What counts what

The `store` package is the only writer, so counting lives there, in each method's existing transaction. The game code and the HTTP handlers don't change.

A game's keys go on **the day it was created**, in `STATS_TZ`, whenever they become known. A game created at 23:00 that ends at 01:00 counts its ending on the day it was created, so "of the games created that day, how did they end" stays consistent, and a late fact (a friend link joined the next morning, a waiting game expiring a day later) simply updates its creation day.

| Write | Day | Keys |
| --- | --- | --- |
| `CreateGame` | the game's | `games.quick` or `games.friend`; `friend.links` (a friend game that isn't a rematch) or `friend.rematches` (a rematch of a friend game) |
| `SeatBlack`, on a friend link that isn't a rematch | the game's | `friend.joined`; `join.<bucket>`: `lt1m`, `1-5m`, `5-30m`, `30m-2h`, `2h+` |
| `EndGame` | the game's | `end.<result>`: `checkmate`, `roll` (mate by a roll), `resignation`, `timeout`, `draw` (any drawn result), `aborted`, `expired`. If the game got going (ended with a finished result): `got_going`; `len.<bucket>` (moves by both sides: `1-10`, `11-20`, `21-30`, `31-40`, `41-50`, `51-60`, `61-80`, `81+`); `heat.<block>` (0–11, the two-hour block of `created_at`); `clock.decided` (lost on time); `left.<bucket>` for the winner's clock (`lt30s`, `30s-1m`, `1-2m`, `2-5m`, `5m+`); `friend.finished` (a friend link that isn't a rematch). With its `game_stats`: `road.games`, `road.opened`, `road.reset`, `road.closed_rounds`, `road.closed_cap`, `road.repaired`, `road.fell`, `road.no_fall`, `road.saving_rolls`, `road.saved`, `road.mate_by_roll`, `open.0`–`open.5` (added by each game's open-holes counts), `decided.less` / `decided.more` / `decided.same` (decisive games: the winner lost fewer, more or as many pieces to the road) |
| `ExpireWaiting` (startup, bulk) | each game's | `end.expired`, in the same statement's transaction, grouped by day |
| `AddGameStats` (the startup backfill) | the game's | the `road.*`, `open.*` and `decided.*` keys, through the same function as `EndGame` |
| `AddSearch` | the search's start | `search`; `search.matched` or `search.gaveup`; `search.alone` (nobody else was looking); `wait.<bucket>` when matched, `gaveup.<bucket>` when not (`lt10s`, `10-30s`, `30-60s`, `1-2m`, `2m+`) |
| `AddEvent` | its own | `event.practice`, `event.reconnect`, `event.restart` |
| `AddBrowserError` | its own | `errors` |

- **Playtests write no counters.** A game marked `playtest` counts nothing at any step, and a playtest's searches, events and errors are never written (as now).
- **Only what changed counts.** `EndGame` updates `WHERE code = ? AND ended_at IS NULL` and `SeatBlack` updates `WHERE code = ? AND black IS NULL`; each counts only when its update changed a row. Both lack that guard today and gain it. `ExpireWaiting` already has it. `AddGameStats` counts only when its `INSERT OR IGNORE` inserted.
- **One counting function per kind of row** (a game, a search, an event, an error) turns the row into its keys. The live writes and the rebuild both call it, so they can't count differently.
- **Cost.** A game end touches about 25 counter rows inside the transaction it already commits: still one commit. A search, event or error becomes a short transaction with its raw insert and 1–4 upserts. Moves are unchanged: a move writes its turn and nothing else.

## Reading the page

`GameSection` and its scans are removed. A page load runs four queries, all on the primary key:

1. `SELECT key, SUM(n) FROM stats_days WHERE day >= ? AND day < ? GROUP BY key` for the range (no lower bound for "All").
2. The same for the range before, for the change chips (skipped for "All").
3. `SELECT day, key, n FROM stats_days WHERE day >= ? AND day < ? AND key IN ('games.quick', 'games.friend', 'event.practice')` for the games-per-day chart; days without rows are drawn as zero.
4. The newest `browser_errors` row in the range, for the "latest" line.

The sections are built from the summed keys. What changes on the page:

- **Game length:** the moves histogram; "Decided on the clock" stays; the median line goes. A new small histogram, **Winner's clock left**, with the line "under 1 min left: N%" (`lt30s` + `30s-1m` over all `left.*`).
- **Friend links:** joined, finished and rematches as now; the median time to join becomes a histogram with "joined within 5 min: N%" (`lt1m` + `1-5m` over `friend.joined`).
- **Quick match:** the wait histogram as now; "gave up after" becomes a histogram with "gave up within 30 s: N%" (`lt10s` + `10-30s` over `search.gaveup`); found an opponent and searched when nobody else was, as now. The median wait line goes.
- **When people play:** the same grid, from `heat.*` per day, the weekday taken from each day.

A 90-day page reads at most about 90 × 60 counter rows, however many games were played. The page is always current: no cache, no Refresh button; reloading is refreshing. Stats queries keep a 2-second timeout as a backstop.

## Rebuild

- The code holds a counters version constant. At startup, after the `game_stats` backfill and before the server accepts requests, the server compares it and `STATS_TZ` with `stats_state`. If either differs (or `stats_state` is empty, as on the first deploy), it deletes the counters (all of them, except `errors` on days before the oldest `browser_errors` row) and recounts every raw row through the counting functions, in one transaction, then writes the new version and zone. It logs `stats rebuilt` at INFO with the days written and the time taken.
- The first deploy fills the table from the existing games this way.
- A rebuild runs only on a deploy that changes the counters (which pauses games anyway), never while serving.
- If it fails, it's logged at ERROR, the transaction rolls back, and the server starts with the counters it had; the next start tries again. While `stats_state.version` is behind the code's, the page shows one line: "These counts are from an older version: a rebuild failed, see the logs."

## Failures

- A counter write that fails fails the write it belongs to, as a failed `game_stats` insert does today; the game's existing save-error handling applies. In exchange, the counters never disagree with the raw rows.
- SQLite has one writer, so upserts are serialized and no increment is lost; reads don't block writes (WAL).

## Testing

1. **Each write writes exactly its keys:** table tests for every store write, including a playtest game (none), a repeated `EndGame` or `SeatBlack` (none the second time), and a game ending after midnight (its creation day).
2. **Live counting equals a rebuild:** a few hundred random games, searches, events and errors written through the store's normal methods across many days, including a daylight-saving change, games ending the next day, friend links joined the next day, aborted and expired games, rematches and playtests. The `stats_days` rows after live counting must equal the rows after a rebuild.
3. **The page from counters:** seeded counters render every section; each threshold line equals its bucket sum; the chips compare the right days; "All" has no chips.
4. **Size:** a benchmark of a page load over 20,000 synthetic games across 400 days, reported in the PR, to show it stays flat as games grow.
5. **As before:** `go test -race`, the contract checks (`/stats` is 401 without the password), svelte-check, Vitest, the build, the browser pass at 1440 and 390 px, and a playtest whose games write no counters.

## Out of scope

- A public page, per-person numbers, visit tracking (removed 2026-10-19).
- Hourly buckets and downsampling.
- Charts over time beyond games per day.
