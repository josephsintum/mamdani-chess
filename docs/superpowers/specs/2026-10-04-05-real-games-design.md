# Milestone 05: Real games — Design

Date: 2026-10-04
Status: Built (plan: [2026-10-04-05-real-games](../plans/2026-10-04-05-real-games.md))
Builds on: [the main design spec](2026-10-01-mamdani-chess-design.md) and [the roadmap](../plans/2026-10-03-00-roadmap.md), row 05.

## Goal

Turn the playable friend game into a real one: 10+5 clocks, games that survive a server restart or a Railway deploy, and rematch. It also adds a log line per game event and a check status that names the side in check.

**Done when:** a server restart mid-game loses nothing (both tabs carry on without a reload, the clocks resume), and flag falls end games.

A guiding concern: a player on a patchy connection (on the subway, say) shouldn't be punished for a short dropout.

## Decisions

| Topic | Decision | Why |
| --- | --- | --- |
| Automatic losses | Only the clock. No disconnect forfeit, no claim button. | The running clock already handles a player who walks away on their own turn (at most about 10 minutes of waiting), and a dropout on the opponent's turn costs nothing. A forfeit timer would only hurt players with patchy connections. Chess.com auto-forfeits after 10% × (base + 40 × increment), clamped to 30 s–3 min (80 s at 10+5); Lichess offers a manual "Claim victory" after about 40 s. A Lichess-style claim can be added later if walk-aways annoy players. |
| Clock start | After each side's first move. White has 60 s from Black joining to make the first move; Black has 60 s from White's first move. Missing either cancels the game: result `aborted`, no winner. | As on Lichess. A player still sharing the link doesn't lose time, and a game nobody starts doesn't sit forever. |
| What is stored | Each game's seats and result, and each turn's move, dice and clocks. Not the board. | `rules` already replays a game exactly from its moves and recorded dice. |
| When it's written | Each move as it happens, in the game goroutine, before the `state` broadcast. | Everything a player saw is saved, so a restored game always matches it. Cheap at our scale (see Storage). |
| Query code | Plain `database/sql`. No sqlc, no sqlx. | About eight queries. Reconsider at milestone 08 if reporting queries multiply. |
| Restore | At startup: every unfinished game, plus games that ended in the last 24 h. | With clocks, every started game ends within a bounded time, so there's no need to load games lazily. Recent finished games keep their result card across a restart. |
| Downtime | Not charged. A restored game's clock restarts from its stored remaining time. | A deploy isn't either player's fault. |
| Rematch URL | A new code, so a new URL. The old URL keeps showing the finished game. | One game, one row, one link. |

## Clocks

Each game is in one of four clock phases:

| Phase | Clocks | Timer in the loop |
| --- | --- | --- |
| Waiting for Black | none | none |
| First moves (before White's and Black's first moves) | stopped | first-move deadline, 60 s |
| Playing | the side to move's clock runs | flag time |
| Over | stopped | none |

- **Start:** both clocks start at 10:00. When Black joins, White's first-move deadline starts. When White's first move lands, Black's deadline starts 2 s later, after the dice pause. When Black's first move lands, White's clock starts 2 s later, and the game is in the Playing phase.
- **The dice pause:** after every move, the next player's clock starts 2 s (`ResolveDelay`) after the move lands, so the dice animation doesn't cost them time. The browser shows "Paused for dice" meanwhile.
- **Increment:** +5 s is added to the mover after each move made while their clock was running, so not after the two first moves: play starts at 10:00 each.
- **Flag fall:** when the side to move's clock reaches zero, that side loses on `timeout`. It's a draw (`timeout_vs_insufficient`) if the other side can't mate (`rules.Position.CannotMate`, already written for this). The server's timer ends the game even if nobody is watching.
- **Time used** is measured on the server from when the clock started (after the pause) to when the move request arrives. No lag compensation: at 10+5, a few hundred milliseconds don't matter.

### In the game loop

`game.loop` (`game/game.go`) gets one more `select` case: a single `*time.Timer` for whichever deadline is next (the first-move deadline or the flag time). Every call that changes the phase or the side to move (join, move, resign, end) recomputes the deadline and resets the timer. When it fires, the loop checks the deadline against the clock again (the timer can be stale) and ends the game if it has really passed. The game's state stays owned by one goroutine, so no locks are needed.

Tests use `testing/synctest` (Go 1.27) with the real `time` package, instead of the main spec's `Clock` interface.

### What the browser gets

`state` gains three objects, in the camelCase the rest of the view uses:

```json
"clock": {"whiteMs": 594000, "blackMs": 600000, "running": "black", "since": 1791234567890, "now": 1791234566100},
"online": {"white": true, "black": false},
"rematch": {}
```

- `running`: the color whose clock is running; left out when no clock runs.
- `since`: the server time (unix ms) when the running clock starts or started. It can be in the future during the dice pause.
- `now`: the server's time when it built this state. The browser keeps the offset from its own clock and counts down locally. That way it doesn't matter if the phone's clock is wrong.
- `firstMoveDeadline`: unix ms, only during the First moves phase.
- `online`: which players have a stream open (see Disconnects).
- `rematch`: `offer` (the color that offered), `declined`, or `code` once accepted (see Rematch).

Every change goes out as a `state`, so a reconnecting tab gets all of it at once; there are no separate `presence` or `rematch` events in this milestone. The browser's animator keeps a dice roll playing through a `state` with the same `seq` (a player going offline mid-roll, say).

## Disconnects

- **Your own connection drops:** the "Reconnecting…" banner shows (already built). EventSource reconnects and gets a fresh `state`.
- **Your opponent's connection drops:** the opponent's player bar shows "Disconnected", for information only. There's no countdown. `state.online` says which players have a stream open; the game broadcasts when a player's last stream closes or their first one opens.
- **The server restarts:** EventSource reconnects by itself after a network error, but an error status (a proxy's 502 while the server is down) closes it for good. The page then asks `GET /api/games/:code` (the caller's view, without taking a seat): a 404 means the game is gone; anything else, it opens a new stream after 2 s.
- **A move sent while offline:** the board shows it at once (instant move). If the POST fails with a network error, the browser re-sends it with the same `seq` when the stream reconnects. If the server already has the move, the `seq` check answers 409 with the current `state`, which already includes it, so the move is never played twice. If the game changed in the meantime (the clock ran out, say), the 409's `state` replaces the guess. This is the case where it matters: you move just as the train enters a tunnel.

## Storage

### Schema (migration 3, appended to `store/migrate.go`)

```sql
CREATE TABLE games (
  code        TEXT PRIMARY KEY,
  white       TEXT NOT NULL,             -- guest ID: SHA-256 of the guest cookie (black too)
  black       TEXT,                      -- NULL until Black joins
  created_at  INTEGER NOT NULL,          -- unix ms
  ended_at    INTEGER,                   -- NULL while unfinished
  result      TEXT,                      -- rules.Reason, or resignation, timeout, aborted, expired
  winner      TEXT,                      -- 'white' | 'black' | NULL (draw, aborted, unfinished)
  rematch_of  TEXT REFERENCES games(code)
);
CREATE INDEX games_ended_at ON games(ended_at);  -- unfinished (NULL) and recently ended

CREATE TABLE turns (
  game      TEXT NOT NULL REFERENCES games(code),
  ply       INTEGER NOT NULL,            -- 0-based; equals the move's seq
  move      TEXT NOT NULL,               -- UCI: rules.Move.String / rules.ParseMove
  dice      TEXT NOT NULL,               -- every d8 that turn, in order: "3,6,2"; "" for none
  white_ms  INTEGER NOT NULL,            -- clocks after the turn, increment included
  black_ms  INTEGER NOT NULL,
  at        INTEGER NOT NULL,            -- unix ms when the move landed
  PRIMARY KEY (game, ply)
);
```

Reactions, spectators, presence and rematch offers aren't stored. The game log, lost pieces and stats are rebuilt by replaying the turns.

### Writes

| When | Query |
| --- | --- |
| Game created | `INSERT INTO games` |
| Black joins | `UPDATE games SET black` |
| Each move | `INSERT INTO turns` |
| Game ends (any reason) | `UPDATE games SET ended_at, result, winner` |
| Rematch accepted | `INSERT INTO games` with `black` and `rematch_of` set |

Nothing else is written: no clock ticks, no presence.

- **Order:** a move is applied with `rules`, its turn row is written, and only then is `state` broadcast. A move that ends the game writes the turn and the result in one transaction.
- **If a write fails** (disk full, say): log it at ERROR, answer the request 500, and stop the game the same way a panic does (`g.stop()`). Nobody has seen the unsaved move, and the next startup restores the game from what was saved. This should never happen in practice; the goal is that the database and what players saw can't disagree.
- **Settings:** add `_pragma=synchronous(NORMAL)` to the DSN in `store/store.go` (WAL is already on). A commit then doesn't wait for fsync: tens of microseconds. A committed row survives a process crash or a Railway restart; only a power cut or an OS crash can lose the last few commits, and the database still isn't corrupted. Keep `SetMaxOpenConns(1)`: it already lines writers up, so SQLite's one-writer limit never shows up as `SQLITE_BUSY`.
- **Game codes:** `Hub.Create` checks codes only against games in memory. It now also relies on `INSERT INTO games` failing on a clash with a saved game, and draws a new code if it does.

### Load

About 60–100 bytes per turn, so roughly 12 KB for a long game, and 12 MB for 1,000 games. With the 2 s dice pause, a game writes at most one row every 2–3 s: about 40 writes/s with 100 games at once, about 400/s with 1,000, far below what SQLite can do. Live play reads nothing from the database; reads happen only at startup.

### The `store` API

Methods on `store.Store`, each one plain SQL:

- `CreateGame(ctx, Game) error`: `store.ErrCodeTaken` on a code clash.
- `SeatBlack(ctx, code, guest) error`
- `AddTurn(ctx, code, Turn) error`
- `EndGame(ctx, code, Result, final *Turn) error`: saves `final` (the move that ended the game, if any) and the result in one transaction.
- `LoadForRestore(ctx, endedAfter) ([]SavedGame, error)`: unfinished games and those that ended after `endedAfter`, each with its turns in order.
- `ExpireWaiting(ctx, cutoff, now) (int64, error)`: ends games still waiting for Black that were created before `cutoff`, with result `expired`.

A game still waiting for Black that is evicted from memory after its quiet day is also ended as `expired`.

The `game` package depends on a small interface holding these methods, so its tests can use a real store on a temporary file.

## Restore at startup

In `cmd/server/main.go`, after `store.Open` and before the server starts listening:

1. `ExpireWaiting(now − 24 h)`.
2. `LoadForRestore(now)`. For each game, replay its turns one by one through `rules.Game.Play` with a scripted `Dice` that returns the recorded rolls. `rules.Replay` isn't enough here, because it returns no events, and the log, lost pieces and stats are built from events by the existing `tally` and log code in `game/game.go`. If a replay fails (it shouldn't: the rules are deterministic), log it at ERROR and skip that game.
3. Put each game in the hub with its seats, and start its loop. The clocks come from the last turn's `white_ms` and `black_ms`. Restart the phase where it was:
   - Playing: the side to move's clock starts 10 s after the restore (`RestoreGrace`), so the players have time to reconnect. Only finished turns are saved, so the time used on the turn in progress when the server stopped is given back too.
   - First moves: the deadline restarts at a full 60 s.
   - Over: no timer. The game is evicted after its quiet day as usual.
4. Log one INFO line per restored game, and one summary line.

Open tabs don't need a reload: EventSource retries on its own, the game code is the same, and the first `state` brings the clocks.

Railway stops the old process before starting the new one (the volume allows one at a time), so two servers never write to the same file.

## Rematch

- **API:** `POST /api/games/:code/rematch` with `{}` to offer or accept, or `{"decline": true}`. A spectator gets 403; before the game is over, 409.
- **Offer:** the opponent's `state.rematch` becomes `{offer: "white"|"black"}` and they see "Your opponent wants a rematch." with **Accept** and **Decline**. Offering when the opponent's offer is already waiting counts as accepting it.
- **How long an offer lasts:** until it's declined, or until the offering player's last stream for that game closes. It isn't stored, so a restart also clears it.
- **On accept:** the game asks the hub for a new game with both seats filled, colors swapped, and `rematch_of` set. Then everyone's `state.rematch` gets `{code}`. The players' pages go to `/game/<code>`; spectators get a **Watch rematch** link instead. The new game starts in the First moves phase.
- **Declined:** `state.rematch` becomes `{declined: true}`; the offerer sees "Rematch declined." and can ask again.
- The hub creates the new game from inside the old game's goroutine. That's safe: `Hub.Create` only takes the hub's mutex, and never calls into an existing game.

## Log lines

The guest cookie works as a login (whoever holds it takes that guest's seat), so `guestID()` in `server/guest.go` hands the rest of the server only its SHA-256: seats, the database and logs never hold the cookie. The 128 random bits need no salt. Leaderboards and achievements (milestone 08) key on the same hash. Logs shorten it further to an 8-character `guestTag`.

At INFO, one line per game event, each with `code`:

- `game created` (`white`)
- `black joined` (`black`)
- `game ended` (`result`, `winner`, `moves`)
- `game aborted` (which side missed the first move)
- `rematch` (`from`, `to`)
- `game restored` (`moves`, `phase`)
- `game evicted` (the existing line, which gets the same fields)

## Check status

The status line names the side in check: "Your move — you're in check" for the player in check, "Black to move — Black is in check" for the other player and spectators. In the agent-played games, two agents read "check!" as being about their own king.

## Changes to the main spec

The main design spec changes to match:

- §1 Connection problems: no forfeit countdown.
- §1 Result: the reasons list `aborted` instead of `forfeit`.
- §2 `presence` and Disconnects: no forfeit timer.
- §2 Clocks: they start after the first moves; `testing/synctest` replaces the `Clock` interface.
- §2 Persistence: restore at startup, no charge for downtime.
- §5 `game` tests: drop the disconnect forfeit, add the first-move abort.

## Testing

- **`store`:** round-trips against a temporary file: create, seat, add turns, end, load. `LoadForRestore` returns unfinished and recently finished games but not older ones. A code clash gives `ErrCodeTaken`. `ExpireWaiting` ends only old games still waiting.
- **`game`, with `testing/synctest`:**
  - The first-move abort for each side.
  - Clocks: they don't run during the 2 s pause; the increment is added.
  - Flag fall: a loss, and a draw when the opponent can't mate.
  - A stale timer doesn't end a game that has moved on.
  - Resignation stops the timer.
  - Rematch: offer, accept (colors swapped, `rematch_of` set), decline, the offerer leaving, and crossing offers.
- **Restore:** play some turns against a real store, throw the hub away, restore from the same file, and compare. The position, log, lost pieces, stats, `seq` and clocks must match, and a game in the First moves phase restarts its deadline.
- **Write failure:** a store that fails `AddTurn` makes the move return 500, and the game stops without broadcasting.
- **Server:** HTTP tests for the rematch endpoint (wrong caller, game not over, decline), and the `clock` object in `state`.
- **Web:** Vitest tests for the countdown (`since` in the future, a stopped clock, the first-move deadline, formatting) and for the animator keeping a roll playing through a same-turn update. Re-sending a move after a network error lives in the game page, which has no unit tests, so a browser check covers it (go offline in DevTools, move, go online: the move lands once). Then `pnpm --dir web check` and `pnpm --dir web build`.
- **Whole system:**
  - `pnpm --dir web playtest` in Chromium and WebKit, with clocks on.
  - A manual restart check: play a few moves, kill and restart `go run ./cmd/server`, and confirm both tabs carry on without a reload and the clocks continue.
  - An offline move: turn the network off in DevTools, move, turn it back on, and confirm the move lands once.
- **Write load:** run `tools/loadgen` with a few hundred games at once against a server writing to SQLite, and record the insert latency at p50 and p99 in the implementation plan. A batched writer is the fallback, only if the numbers show a problem. Measured while writing the plan (300 games, 2 spectators each, 60 plies, about 12,000 moves/s, roughly 100 times the real rate): move POST p50 135 µs → 177 µs and p99 585 µs → 858 µs against the in-memory server; every turn saved. No batching needed.

## Out of scope

- A claim-win button (add later if walk-aways annoy players).
- Loading games that finished more than a day ago (game history), and anything using the saved games (milestone 08).
- Lag compensation.
- Names, quick match and the live games list (milestone 06).
