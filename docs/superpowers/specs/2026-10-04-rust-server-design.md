# Rust server (`server_rs/`) — Design

Date: 2026-10-04
Status: Draft for review
Branch: `rust-rewrite`
Ports: the Go server as of `52ed857` (milestone 04). Design spec for the product: [2026-10-01-mamdani-chess-design.md](2026-10-01-mamdani-chess-design.md).

## Goal

A Rust implementation of the existing Go server, written idiomatically and with attention to efficiency, living next to it in `server_rs/`.

This is a **parallel experiment**. The Go server stays the shipped server, the roadmap is unchanged, and CLAUDE.md's "Go server" decision stands. The Rust server follows the Go one; it does not lead.

**Success:**
1. The SvelteKit frontend runs unchanged against the Rust server (`pnpm dev` proxying `/api` to :8080) and plays a full friend game, resign included.
2. Rust ports of the Go tests pass: rules, perft, random games, game, HTTP.
3. A differential test replays thousands of games recorded from the Go server and finds zero byte differences.

## Scope

In: everything the Go server does at milestone 04, which is `rules`, `game`, `server`, `store` and `cmd/server`.

Out:
- `names/`. It is unused until milestone 06; port it then if the experiment continues.
- Milestone 05+ features: clocks, persistence of games, quick match and the rest.
- Dockerfile and Railway changes.
- Any change to Go code, `go.mod`, `names/` or `web/`.

## Compatibility contract

The wire format is **byte-identical** to the Go server's. Concretely:

| Surface | Must match |
| --- | --- |
| Routes | `GET /healthz`, `POST /api/games`, `GET /api/games/{code}/stream`, `POST /api/games/{code}/move`, `POST /api/games/{code}/resign`, `/api/` catch-all 404, `/` static with 405 for non-GET/HEAD. |
| Status codes | 201 create; 204 move/resign; 400 `bad request body` / `bad move`; 403 not a player; 404 `game not found` / `not found`; 409 with the Go error string; 500 `internal error`. |
| API JSON | Same field order, `omitempty` fields omitted, slices `[]` not `null`, `saved:false` emitted, trailing `\n` (Go `json.Encoder`). |
| SSE | Headers `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`, `X-Accel-Buffering: no`. Frames `event: state\ndata: <json>\n\n` with no newline inside the JSON (Go `json.Marshal`). Heartbeat `: ping\n\n` every 15 s. The first state is sent on connect. |
| JSON escaping | Go escapes `<`, `>`, `&`, U+2028 and U+2029 as `<` and so on. The Rust encoder must do the same. No current string contains them; a test pins the behaviour. |
| Cookie | `guest`, 32 hex chars, `Path=/`, `Max-Age=31536000`, `HttpOnly`, `SameSite=Lax`, plus `Secure` when TLS or `X-Forwarded-Proto: https`. Set lazily by API handlers only. A value is accepted only if it is exactly 32 characters. |
| Codes | 6 chars from `ABCDEFGHJKMNPQRSTUVWXYZ23456789`. |
| Error strings | `waiting for an opponent`, `not your turn`, `the game has moved on; reload the position`, `game is over`, `illegal move`. |
| Database | Same pragmas, the same append-only migration list and the same `schema_version` table, so either server can open the other's file. |

Static serving is functionally the same but not byte-for-byte:
- SPA fallback to `index.html`.
- `_app/immutable/` gets `public, max-age=31536000, immutable`; everything else gets `no-cache`.
- Range and If-Modified-Since are not ported.

## Layout

```
server_rs/
  Cargo.toml        workspace: members rules, server; [workspace.lints]; [workspace.dependencies]
  rules/            lib crate: pure rules engine, no I/O, no async
    src/  board.rs position.rs event.rs attack.rs movegen.rs turn.rs game.rs san.rs dice.rs lib.rs
    benches/        criterion: perft, random games
    tests/          perft.rs random.rs parity.rs
  server/           bin crate
    src/  main.rs  game/{mod,actor,hub,view,dice}.rs  http/{mod,games,sse,guest,static_files}.rs  store/{mod,migrate}.rs
    tests/          http.rs parity.rs
  parity/main.go    Go fixture dumper (package main under the root go.mod)
  testdata/         generated parity fixtures (gitignored; a small committed sample)
```

There are two crates because the rules engine is where both the effort and the risk are. On its own it compiles fast, carries no async dependencies and can be benchmarked in isolation. The rest is about 600 lines of glue and lives in one crate.

## Rules crate

It is a straight port of `rules/*.go` that keeps every behaviour the Go tests pin (see `docs/superpowers/plans/2026-10-03-02-rules-engine.md`).

**Types:**
- `Color` (`White=0`, `Black=1`) and `Kind` (`Pawn` through `King`, plus `Mamdani`) are `#[repr(u8)]` enums.
- `Piece(u8)` is a newtype with the same `kind | color<<3` packing and an `EMPTY` constant.
- `Square(i8)` holds a1=0 to h8=63, and `Option<Square>` replaces `NoSquare`.
- All of these are `Copy`.

**Position:**
- `[Piece; 64]` mailbox, `mamdani: Option<Square>`, `potholes: [Option<Square>; 2]` indexed by the roller's colour, `turn`, `castling: u8`, `ep: Option<Square>`, `halfmove`, `fullmove`.
- It is `Copy`, about 80 bytes, and passed and returned by value as in Go. Making a move copies the position and never allocates.

**Moves:**
- `Move { from, to, promo: Option<Kind> }`.
- `LegalMoves` returns `ArrayVec<Move, 256>`, so no move generation touches the heap. `arrayvec` is the only runtime dependency of the crate besides `thiserror`.

**Dice:**
- `pub trait Dice { fn d8(&mut self) -> u8; }`.
- `ScriptedDice` and `RecordingDice<D>` are generic wrappers.
- `Apply` and `Game::play` take `&mut impl Dice`, which gives static dispatch.
- Rolls outside 1..=8 return `RulesError::BadDie`.

**Events:**
- `enum Event` has one variant per Go `EventKind`, and each variant carries exactly its own fields: `Moved { from, to, promo }`, `Reroll { sq, reason }`, `SavingRoll { color, roll, saved }` and so on.
- `kind()` returns the Go wire string.
- JSON shaping belongs to the server's view layer, not to `rules`.

**Kept exactly:**
- Pseudo-legal generation followed by the legality filter through `play()`, which runs move → close → repair.
- Pothole roll order, reroll reasons and their order, and the 65-attempt cap leading to `no_pothole`.
- Saving-roll targets, and that falls reset the halfmove count and castling.
- Status order: mate/stalemate (with `threatened()` ignoring the mover's own pothole) → 50-move → threefold → `CannotMate`.
- `Key` and repetition counting (`HashMap<Key, u8>`).
- SAN, including `Mb5`, `+` and `#`.
- UCI parse/format, FEN parse, the start position with the Mamdani on a5, and `Replay` failing on unused dice.

**Errors:** `#[derive(thiserror::Error)] enum RulesError { IllegalMove, BadDie, UnusedDice, Fen(..) }`.

**Optimisation policy:** no bitboards up front. There are criterion benches for perft from the start position to depth 4 and for 1,000 seeded random games, plus a Go `testing.B` equivalent in the parity tool so the numbers are comparable. Optimise only where the profile points.

## Server crate

**Runtime:** `tokio` (multi-thread), `axum`, `tower-http` (body limit), `serde`/`serde_json`, `bytes`, `tokio-util` (`CancellationToken`), `tracing`, `rusqlite` (`bundled`), `rust-embed`, `getrandom`, and `anyhow` in `main.rs` only.

### Game actor

This keeps Go's one-goroutine-per-game model, which the roadmap requires so that the milestone 05 timers stay race-free.

- `Hub { games: std::sync::Mutex<HashMap<Code, GameHandle>> }`. The lock is never held across an `.await`. `create` retries until the code is unused, and games are never evicted (as in Go).
- `GameHandle` holds `mpsc::Sender<Cmd>` and is cheap to clone.
- `Cmd` is one of:
  - `Join { guest, reply: oneshot<Subscription> }`
  - `Leave`
  - `Move { guest, mv, seq, reply: oneshot<Result<(), GameError>> }`
  - `Resign { guest, reply }`
- One `tokio::spawn`ed task owns `GameState`, which holds the rules game, seats, last events, log, lost pieces and stats. It runs `while let Some(cmd) = rx.recv().await`.
  - Each command runs inside `catch_unwind(AssertUnwindSafe(..))`. A panic becomes `GameError::Internal` and is logged.
  - The state stays consistent after a panic because `Game::play` only commits on success, the same argument the Go code makes.
- The check order in `move` is: seated, over, waiting, turn, `seq == turns.len()`, legal. Then compute SAN on the pre-move position, play, record, broadcast.
- `GameError` is a `thiserror` enum. Its `Display` strings are the Go error strings, and the HTTP layer maps each variant to a status.

### Fan-out

This is the main efficiency change from Go.

- Go builds and marshals a `View` per subscriber, but the JSON only varies by role: `you`, and `legal` for the player to move.
- The actor keeps three `tokio::sync::watch::Sender<Bytes>`, one each for white, black and spectator. On every broadcast it serialises at most three views, into `Bytes`, and skips roles with no receivers.
- `watch` has exactly the semantics of Go's buffer-of-1-drop-stale `send`: the game never blocks, and a slow reader wakes up to the newest state.
- Every spectator shares one refcounted buffer.
- A `Subscription` is a `watch::Receiver<Bytes>` plus an RAII guard. Dropping the guard sends `Leave`, which replaces Go's explicit `Leave`.
- When a guest takes Black's seat, the actor re-broadcasts, as Go does.

### HTTP

- **Request bodies:** handlers take the body as raw `Bytes` under a 4 KiB `DefaultBodyLimit`, and parse it with `serde_json`. Axum's `Json` extractor is not used, because its rejection bodies would differ from Go's `bad request body`.
- **Guest extractor:** returns the guest id plus an optional `Set-Cookie` header value. Handlers attach the header to their response.
- **`writeJSON` equivalent:** a function that serialises with the Go-compatible formatter (below), appends `\n` and sets `Content-Type: application/json`.
- **SSE:**
  - The body is `Body::from_stream` over a hand-written stream. That stream sends the initial frame, then `select!`s over `watch.changed()`, a heartbeat `tokio::time::interval`, and the server's `CancellationToken`.
  - The heartbeat interval is a field on the app state, so tests can shorten it.
  - axum's `Sse` type is not used, because its comment and frame formatting does not match Go byte for byte.
  - Opening the stream is what claims Black's seat.
- **Go-compatible JSON:** a `serde_json::ser::Formatter` impl overriding `write_char_escape`/string writing to escape `<`, `>`, `&`, U+2028 and U+2029 like Go. View structs use `#[serde(skip_serializing_if = ...)]` where Go uses `omitempty`, and `Option<bool>` for `saved`.
- **Static files:**
  - `rust-embed` over `../../web/build`.
  - With cargo feature `embedweb` the files are compiled in.
  - Without it, debug builds read from disk, and if `index.html` is missing the server returns Go's "Frontend not built" page.
  - The handler uses the same `path.Clean` logic, SPA fallback and cache-header rule as `server/static.go`.
- **Shutdown:**
  - `axum::serve(...).with_graceful_shutdown(..)` on SIGINT or SIGTERM.
  - The `CancellationToken` is cancelled first so that SSE streams end, mirroring `RegisterOnShutdown`. Requests still in flight get 10 s.

### Store

- `rusqlite::Connection`, opened at `DB_PATH` after creating the parent directory.
- The same pragmas as Go: WAL, `busy_timeout=5000`, `foreign_keys=1`.
- `migrate()` runs the same append-only list, `create honks` then `drop honks`, one transaction per step, tracked in `schema_version`.
- `Store` wraps `Mutex<Connection>`, so there is a single connection as with Go's `SetMaxOpenConns(1)`. It is unused by handlers, as in Go. Later callers go through `spawn_blocking`.

### main

- `PORT` defaults to `8080` and `DB_PATH` to `data/mamdani.db`.
- `tracing_subscriber` logs to stdout.
- `anyhow::Result` for startup errors.

## Errors and lints

- Library-style modules (`rules`, `game`, `store`) use `thiserror` enums. `anyhow` appears only in `main.rs`.
- No `unwrap`/`expect` outside tests. Where an invariant genuinely cannot fail, `expect` carries a reason and a `// invariant:` comment.
- `[workspace.lints.clippy]` sets `all = deny` and `pedantic = warn`, with `unwrap_used = deny` and `expect_used = warn`. Tests opt out locally. Any lint that is turned off uses `#[expect(..)]` with a reason.
- The CI command is `cargo fmt --check && cargo clippy --all-targets --locked -- -D warnings && cargo test --locked`.

## Testing

1. **Rules unit tests.**
   - A port of every Go rules test, with the same FENs, Mamdani squares, potholes and dice scripts.
   - Helpers mirror `helpers_test.go`: `setup(fen, mamdani, white_hole, black_hole)`, `dice(&[..])`, and `apply`, which asserts the script was fully used.
   - Test names are descriptive, for example `pothole_on_square_next_to_mamdani_is_repaired`.
2. **Perft.** Start position to depth 4 = 197,281, Kiwipete to depth 3, and positions 3–5, all plain chess. Deep cases are `#[ignore]` in debug and run with `cargo test --release -- --ignored`.
3. **Random games.**
   - 500 seeded games by default and 10,000 under `--ignored`, using `rand_pcg` as a dev-dependency.
   - Invariants are checked after each ply, and replay must give the same position.
   - Seeds will not reproduce Go's games, because the PRNG differs. Cross-checking against Go is the parity test's job.
4. **Parity (differential).**
   - `server_rs/parity/main.go` uses only the exported Go API: `game.NewHub` with a seeded dice source wrapped to record rolls, `Hub.Create`, `Game.Join` for white, black and a spectator, `Game.Move`, and `Sub.C`.
   - It plays N games, choosing moves at random from `View.Legal` with a seeded `math/rand/v2` PCG.
   - For each game it writes one JSONL record: `{code, turns:[{move, seq, dice:[..], views:{white, black, spectator}}]}`. Each view holds the exact `json.Marshal` bytes as a string.
   - The Rust test `rules/tests/parity.rs` checks rules only: replay, SAN, events and position.
   - The Rust test `server/tests/parity.rs` checks the full actor. It drives a game with `ScriptedDice` and the recorded code, and asserts that each role's `Bytes` equal the recorded views.
   - The default run uses a small committed sample of about 50 games. `PARITY_FILE=… cargo test` runs a larger generated file.
5. **Game actor tests.** Ports of `game/game_test.go` and `view_test.go`: seats, reconnect, check order, panic recovery via a panicking dice, and exact event JSON bytes.
6. **HTTP integration.**
   - A port of `server/server_test.go`: a real `TcpListener` on port 0, a `reqwest` client with a cookie jar per player, and a small SSE line reader.
   - It covers the friend game (33 legal moves at the start), move error codes and strings, cookie attributes, the heartbeat (shortened interval), static files and cache headers with a temp dir, shutdown ending streams but not other requests, and resign.

## CI

- A new `rust` job in `.github/workflows/ci.yml` runs `dtolnay/rust-toolchain@stable` with clippy and rustfmt, and `Swatinem/rust-cache`, with working directory `server_rs`.
- It runs the fmt/clippy/test command above, then `go run ./server_rs/parity -n 500` piped into the parity test.
- The existing jobs are untouched.

## Repo hygiene

- New files are confined to `server_rs/**`, this spec and its plan.
- `.gitignore` gains `server_rs/target/` and `server_rs/testdata/*.jsonl`, except the committed sample.
- The only edit to an existing file is adding the CI job.
- Stage specific paths only, and no Claude attribution in commits (CLAUDE.md).

## Risks

- **Subtle rule divergence.** The parity test is the main defence. The random-game invariants are the second.
- **JSON byte differences**, such as escaping, field order, or `null` vs `[]`. They are covered by the parity views and pinned unit tests.
- **Panic catching across `.await`.** Only the synchronous command body is wrapped in `catch_unwind`, never an await point.
- **Embedding `web/build` from the crate.** The path is relative to the manifest (`$CARGO_MANIFEST_DIR/../../web/build`). A missing build only matters with `embedweb` on, and the job that turns it on builds the web app first.
