# Launch Scaling Plan

**Goal:** keep the game smooth if the launch goes viral and traffic arrives in spikes. This is a list of investigations and actions agreed item by item on 2026-10-05, from an audit of the server, store, SSE and frontend networking. Each action that changes behaviour gets its own short design pass (spec) before it is built; the rest can be built straight from here.

**Measured where:** on an M-series Mac, with throwaway programs against the real packages (not kept in the repo; the first investigation turns them into `tools/loadgen` modes). Railway's CPUs are slower, so treat the numbers as best cases.

## Principles agreed

- **One process, by design.** Games live in one server's memory and SQLite sits on one volume. Scaling is vertical only, and every deploy pauses every game. Not changing.
- **Memory holds only live games:** a game stays in memory while it can still change or someone is watching it. Everything else is read from SQLite on request.
- **Writes come from explicit actions,** never as a side effect of loading a page: a name is issued by "Start as guest", a seat is taken by "Join", never by opening a stream.

## What is not a bottleneck (measured)

- **JSON encoding:** a full game view encodes in 2–26 µs (move 1 to move 90). Views are encoded once per role, not per stream.
- **Data size:** a spectator's view is 0.7 KB at the start and about 10 KB after 180 plies (the move log grows). A game watched by 1,000 people sends about 6 MB per move.
- **Moves:** milestone 05 measured about 12,000 moves/s with every turn saved (move POST p99 0.86 ms).
- **Broadcasting a move:** 2.9 ms to 20,000 viewers.
- **Static files:** the build is under 1 MB, precompressed, immutable assets cached for a year.
- **HTTP/2:** production serves `h2` to browsers (checked with curl), so the 6-connections-per-host limit doesn't apply. This answers the roadmap's "Launch check: HTTP/2" curl step.

## Investigations (do first: several actions depend on their numbers)

| # | Investigation | Decides |
| --- | --- | --- |
| I1 | **A separate Railway environment** (same image, its own volume) for every load test below, so test games never reach production data. | Where I2–I6 run |
| I2 | **`tools/loadgen` modes** for: home page viewers on the live stream (item 2); a crowd on one game (item 7); thousands of abandoned and finished games, then a restart (items 4, 5); a deploy with N open streams (item 9). Fold in the throwaway benchmarks from this audit. | Before/after numbers for every action |
| I3 | **Write latency on Railway's volume:** move POST p50/p99 and the worst stall (WAL checkpoints) under load. | Whether a batched writer is needed (the 05 spec's fallback) |
| I4 | **Restore time at 50,000 games** on Railway's CPU against the 30 s health check. | Whether to load all turns in one query (only if tight) |
| I5 | **Railway limits:** the service's memory cap, the container's open-file limit, and whether the proxy cuts long-lived requests after some time. | `GOMEMLIMIT`, the viewer ceiling, stream reconnects |
| I6 | **How long a deploy keeps the game away** (Railway can't keep two containers on one volume; one report says mounting takes about a minute), with `tools/sse-check.sh` during a deploy. | Whether to warn players before deploys |
| I7 | **Size of the `names/` pool** against the expected number of guests. | Whether names start clashing |
| I8 | Optional: **per-stream memory profile** (47 KB each). Only if I5's memory cap is low. | Whether to slim streams |

## Items

### 1. One process holding every game: accepted

By design (see Principles). No action. Don't deploy during a spike.

### 2. The home page: live games, pushed

Today each home tab polls `/api/games` and `/api/me` every 10 s. Both scan every game under the hub's one lock (`game/live.go:68`, `:90`), and `/api/me` reads SQLite twice. Measured: 63 µs per poll at 1,000 games, 612 µs at 10,000, 4.5 ms at 50,000. Decided: people landing should see games moving.

- **Action 2A: a live games stream** `GET /api/live` replaces the poll. One goroutine rebuilds the top 12 **at most once a second** when something changed, encodes it once, and sends the same bytes to every home viewer (newest wins, as game streams do). A viewer gets the full list on connect, then only the games that changed (`last` lets the boards glide the move) and the new order when a game enters or leaves the 12. The stream closes when the tab is hidden and reopens when it's visible.
- **Action 2B: `/api/me` from memory, mostly:** a guest → active game map replaces the `Hub.Active` scan; name and changes are one primary-key read (two today), no cache. Fetched on load, when the tab comes back, and every 10 s.

### 3. SQLite: one connection, and names issued separately

Once 2B is in, SQLite is almost only written to. One connection is right for a write queue, so no read pool. Name issuing moves off the game paths entirely.

- **Action 3A (design pass): "Start as guest".** Any action that needs a name (play a friend, quick match, join a game) first asks the visitor to start as a guest, which issues and saves the name and lets them change it as today; then they carry on to what they clicked. Create, join and match require a name and never issue one.
- **Action 3B (same design pass): join or watch.** Opening a waiting game's link asks everyone except its creator "Join the game or watch?" The first to pick Join takes Black's seat (`POST /api/games/{code}/join`); a second at the same moment is told the seat was taken and watches. Once the seat is taken, nobody is asked. Join without a name issues it and seats them in one tap; the name can be changed for later games. Opening a stream never seats anyone.
- Investigations: I3, I7.

### 4. Restart time

Measured: 5,000 games of 40 moves restore in 0.75 s (0.21 s loading, 0.54 s replaying). Not a launch risk at thousands of games a day, and 5B shrinks restore to unfinished games only.

- **Action 4A:** the per-game `game restored` line (`game/restore.go:76`) goes to DEBUG; the INFO summary gains how long restore took.
- Investigation: I4.

### 5. Games in memory

Measured: a waiting game is 7.4 KB, a finished 60-ply game 43.6 KB. Finished games stay 24 h after their last request, so they are the real cost (10,000 = 426 MB). Half of a finished game is the repetition table (`rules/game.go:66`): a copy of every position reached, about 270 bytes each.

- **Action 5A (rules engine): shrink the repetition table.** Clear it after an irreversible move (when `Halfmove` resets: pawn move, capture, fall) and drop it when the game ends. No rule changes; the full random-game suite must stay green.
- **Action 5B: memory holds only live games.** Over and nobody watching: dropped at once. Any later request loads it from SQLite and replays it (about 100 µs). Concurrent first requests share one load (singleflight); a request racing the drop gets the loaded game, never "not found"; the rematch link comes from `rematch_of` (add an index). Restore loads only unfinished games.
- **Action 5C: waiting games expire 30 minutes after the creator's last tab closes** (24 h today), never while the creator is on the page.
- **Action 5D: one waiting game per guest.** Play a friend again takes you back to the game you're waiting in.
- **Per-IP limits** on "Start as guest" and creating games join the roadmap's "Launch hardening: limits per IP" (milestone 07), which already covers trusting `X-Forwarded-For` only from Railway's proxy.

### 6. Quick match

`pair()` creates the game, with its database writes, while holding the queue's lock (`match/match.go:113`), so a slow disk would freeze joins, cancels and the `looking` count. Since pairing happens as soon as two people are waiting, the line never holds more than one person.

- **Action 6A: a matcher goroutine with one waiting slot** replaces the locked list: joins and leaves arrive on channels, a second arrival is paired at once, and a separate goroutine creates games for matched pairs, so the matcher never waits on the database. `looking` (0 or 1) is an atomic. Keep `match_test.go`'s behaviours: two tabs share one place, a closed tab leaves, a failed create puts the guest back.
- **Action 6B (UX): after about 30 s with no match,** offer "Play a friend instead" and a share link to `/play` (the phone's share sheet, or Copy link), so whoever taps it lands in quick match and is paired with you.

### 7. A crowd opening one game

Measured: joins cost more as the crowd grows (quadratic in total). 1,000 spectators join in 58 ms, 5,000 in 0.8 s, 20,000 in 12.6 s, and leaving costs the same. Joins run on the game's goroutine, so a player's move waits behind them while their clock runs. The home page lists games most watched first, which funnels viewers onto popular games.

- **Action 7A: keep counts as you go:** a guest → open streams count, updated on join and leave, replaces the scans in `watching()` (`game/live.go:55`) and `connected()`.
- **Action 7B: encode each role's view once per change** and reuse it for joins, instead of building one per spectator (`game/game.go:272`).
- Re-run the crowd benchmark afterwards (I2).

### 8. Logs

Railway keeps at most 500 log lines/s per replica and drops the rest. Every request logs INFO today, static files included (about 30 per first visit), so about 17 new visitors a second hit the limit. Kept light.

- **Action 8A:** successful requests log at DEBUG; errors (5xx) and slow requests (over 1 s, streams excepted) stay visible. Game events stay at INFO.
- Optional, later: a once-a-minute `stats` summary line (requests by route and status, open streams by kind, games in memory by state, move p50/p99, goroutines, heap).

### 9. Open streams and reconnects

Measured with the real server: about 47 KB per open stream (10,000 streams = 495 MB). Memory, not CPU, sets the viewer ceiling. Every deploy drops every stream, and the game page retries on a fixed 2 s timer, so everyone comes back in the same 2 s.

- **Action 9A: jittered backoff on every reconnect** (game page, home stream, quick match): first retry after a random 1–3 s, then 2–6 s, never longer: a tab must be back within the 10 s a restored game waits before the side to move's clock starts (`RestoreGrace`).
- **Action 9B: set `GOMEMLIMIT`** to about 85% of the container's memory (from I5).
- Investigations: I5, I6, I8.

## Suggested order

1. **Measure:** I1 and I2, then a baseline run of I3–I6.
2. **Server fixes with no behaviour change:** 7A, 7B, 5A, 4A, 8A, 9A, 9B. Re-run I2.
   - **Done 2026-10-05** (branch `launch-scaling-server`), except 9B, which waits for I5's memory cap. Measured after: 20,000 spectators join one game in 39 ms (12.6 s before) and leave in 34 ms (12.8 s); a finished 60-ply game is 26.5 KB (43.6 KB). 9A also makes each stream start with a random `retry:` of 1 to 3 s, so the browsers' own reconnects after a deploy are spread too. Checked with the playtest against a production build: 3 link games and 2 quick-match games in Chromium, 1 in WebKit, and 1 game across a server restart, which played all 40 plies (the script flags the browser's connection-refused messages while the server was down).
3. **Design pass, then build:** 3A + 3B (start as guest, join or watch), 5B–5D (dead games from SQLite, waiting expiry, one waiting game), 6A + 6B (matcher, waiting screen), 2A + 2B (live home page).
4. **With milestone 07:** per-IP limits, then a final I2 run on Railway.
