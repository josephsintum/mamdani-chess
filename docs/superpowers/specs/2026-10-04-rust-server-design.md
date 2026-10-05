# Rust server (`server_rs/`): design

Date: 2026-10-04
Status: Implemented on `rust-rewrite` (first pass). Where the build differed from the draft, this document was updated to match the code.
Branch: `rust-rewrite`
Ports: the Go server as of `52ed857` (milestone 04). The product design spec is [2026-10-01-mamdani-chess-design.md](2026-10-01-mamdani-chess-design.md).

## Goal

This is a Rust implementation of the game server. It lives next to the Go server in `server_rs/`, and is designed the way Rust wants it rather than as a line-by-line port.

This is a **parallel experiment**. The Go server stays the shipped server, the roadmap is unchanged, and CLAUDE.md's "Go server" decision stands.

**What changed from the first draft.** The first draft copied Go byte for byte. This version keeps the *frontend* contract, but it drops Go's quirks and limitations wherever Rust has a better idiom. Where the product spec and the Go code disagree, it follows the product spec.

**Success:**
1. The SvelteKit frontend runs **unchanged** against the Rust server (`pnpm dev` proxying `/api` to :8080) and plays a full friend game, including a resignation.
2. Rust tests cover everything the Go tests cover: rules, perft, random games, game and HTTP.
3. A differential test replays thousands of games recorded from the Go server, and the rules outcome and the game state match on every turn.

## Scope

In scope: everything the Go server does at milestone 04, which is the `rules`, `game`, `server` and `cmd/server` packages. It also adds two things the product spec asks for and the Go server lacks: memory eviction, and a 409 response that carries the game state.

Out of scope:
- `names/`, which is unused until milestone 06.
- The SQLite store. In Go it holds only two migrations that cancel each other out, and nothing calls it. Persistence is a later phase (see the end of this spec).
- Clocks, quick match and everything else from milestone 05 onwards.
- Dockerfile and Railway changes.
- Any change to Go code, `go.mod`, `names/` or `web/`.

## Frontend contract

These are the only things the browser depends on (`web/src/lib/game.ts` and its callers), so they are what must hold.

| Surface | Contract |
| --- | --- |
| Routes | `GET /healthz`, `POST /api/games` → 201 `{code}`, `GET /api/games/{code}/stream` (SSE), `POST /api/games/{code}/move`, `POST /api/games/{code}/resign`. Unknown `/api/*` → 404. Everything else is the SPA. |
| Status codes | 204 for move/resign; 400 for malformed bodies; 403 if the caller is not a player; 404 if the game or route is unknown; 409 for rule or turn conflicts; 500 for internal errors. |
| Error body | `{"error": "<message>"}`. The 409 messages keep the Go strings, because the UI shows them: `waiting for an opponent`, `not your turn`, `the game has moved on; reload the position`, `game is over`, `illegal move`. **New:** a 409 also carries `"state": <View>`, so the client can resync without waiting for the stream (product spec §abuse). |
| SSE | Event name `state`, whose data is one View JSON. The current state is sent on connect, and keep-alive comments every 15 s. |
| View JSON | The same field names, types and meanings as `game/view.go`: `code, status, you, board[64], mamdani, potholes, turn, check, legal, last, log, lost, stats, result, seq`, with event fields as in `EventJSON`. Arrays are always arrays, never `null`. Optional event fields are omitted when unset. |
| Cookie | `guest`, 32 hex characters, `Path=/`, 1-year `Max-Age`, `HttpOnly`, `SameSite=Lax`, and `Secure` behind TLS or `X-Forwarded-Proto: https`. |
| Codes | 6 characters from `ABCDEFGHJKMNPQRSTUVWXYZ23456789`. |

These are **not** promised: byte-level JSON formatting (escaping, trailing newlines, key spacing), the exact SSE comment text, header ordering, and the 400/404 message wording.

## Layout

```
server_rs/
  Cargo.toml        workspace: members rules, server; [workspace.lints]; [workspace.dependencies]
  rules/            lib crate: pure rules engine, no I/O, no async
    src/  square.rs bitboard.rs position.rs movegen.rs turn.rs event.rs game.rs san.rs dice.rs lib.rs
    benches/        engine.rs (criterion: perft, random games)
    tests/          common/ movegen.rs turn.rs game.rs perft.rs random.rs
  server/           lib + bin crate (the lib lets integration tests build the router)
    src/  main.rs lib.rs  game/{mod,actor,hub,view}.rs  http/{mod,error,games,guest}.rs
    tests/          common/ game.rs http.rs parity.rs
  parity/main.go    Go game recorder (package main under the root go.mod)
```

## Rules crate

**Behaviour is identical to the Go rules engine.** The rules are settled (CLAUDE.md), so every rule interpretation the Go tests pin carries over unchanged (`docs/superpowers/plans/2026-10-03-02-rules-engine.md`):
- the pothole roll order;
- the reroll reasons (`king`, `pothole`, `exposes`, `checkmate`) and the 65-attempt cap leading to `no_pothole`;
- repair next to the Mamdani, the saving-roll targets, falls resetting the halfmove clock and castling rights, and the status order (mate/stalemate → 50-move → threefold → insufficient material);
- SAN (`Mb5`, `+`, `#`), and the start position with the Mamdani on a5.

The representation is Rust's own.

**Board: bitboards plus a mailbox.**
- `Bitboard(u64)` with `Copy` and bit operators.
- `Position` holds `by_color: [Bitboard; 2]`, `by_kind: [Bitboard; 6]` and `board: [Option<Piece>; 64]` (a mailbox) for O(1) "what is on this square".
- It also holds `mamdani: Option<Square>`, `potholes: [Option<Square>; 2]` (indexed by the roller's colour), `turn`, `castling: Castling` (a hand-written bit-set newtype), `ep: Option<Square>`, `halfmove: u16` and `fullmove: u16`.
- `Position` is `Copy`, about 150 bytes (`Option<Piece>` is one byte). Copy-make is cheap and never allocates.
- `blocked()` = all pieces | Mamdani | potholes, as one mask. This is the variant's key simplification: sliders stop at `blocked`, and landing squares exclude the potholes and the Mamdani.

**Attacks.**
- Knight, king and pawn attack tables are built by `const fn` at compile time.
- Sliders use classical ray attacks: a ray table per direction (also built at compile time), plus a first-blocker bitscan (`trailing_zeros`/`leading_zeros`).
- No magic bitboards. They are a large table-generation step for a gain we haven't measured a need for. Decide after the benches.
- The Mamdani's moves are queen rays over `blocked`, with no captures. The "clear queen line to the Mamdani" check for saving rolls uses a between-squares mask taken from the same ray table.

**Move generation.**
- Moves go into `MoveList = ArrayVec<Move, 320>`, so nothing touches the heap. 320 covers chess's 218-move maximum, the Mamdani's 27, and pseudo-legal moves the filter drops.
- `Position::after_move(m)` is public: the position after the move, close and repair steps, with no pothole roll. Perft and look-ahead use it.
- Legality uses copy-make and then `in_check`. This is **not** a Go carry-over: closing and repairing potholes after a move change the blockers, which makes pin-mask shortcuts unsound for this variant.
- `Move { from: Square, to: Square, promo: Option<Promo> }`. `Promo` is an enum of the four promotion kinds, so a pawn-to-king promotion cannot be represented.

**Typed API.**
- `Square` is a newtype over `u8` (0..64) with `FromStr`, `Display`, `Square::A1`…`Square::H8` constants, and `name()` returning a `&'static str`. There is no `-1` sentinel; `Option<Square>` is used instead.
- `Color` and `Kind` are enums. `Piece` packs a colour and kind into a `NonZeroU8`.
- `Occupant` is `Piece(Piece) | Mamdani`: whatever can stand on a square and fall.
- `Event` is an enum with one variant per kind, each carrying only its own fields (checked against `rules/event.go`):
  - `Moved { mv, occupant, color }`
  - `Captured { sq, piece }`
  - `PotholeClosed { sq }`
  - `Repaired { sq }`
  - `RolledPothole { roll, color }`
  - `Target { sq }`
  - `Reroll { sq, reason: RerollReason }`
  - `SavingRoll { sq, occupant, roll, saved, color }`
  - `Fell { sq, occupant }`
  - `PotholeOpened { sq, color }`
  - `NoPothole`
- `Outcome { winner: Option<Color>, reason: Reason }` uses a `Reason` enum, not strings. `Reason` includes `Resignation`, and `Game::resign(color)` sets it, so the server doesn't need its own result type.

**Dice.**
- `pub trait Dice { fn d8(&mut self) -> D8; }`, where `D8` is a newtype guaranteed to be in 1..=8 (`D8::new(u8) -> Option<D8>`).
- An out-of-range die cannot reach the engine, so Go's `ErrBadDie` disappears. `ScriptedDice::from_values(&[u8])` validates at construction.
- Past the end of its script, `ScriptedDice` rolls 1 and sets `ran_out()`, rather than panicking as Go's does. `Game::replay` turns that into `ReplayError::MissingDice`.
- A crate-private `Recording<D>` wraps any dice and keeps every roll for `Turn::dice`.
- `Game::play<D: Dice + ?Sized>(&mut self, mv, dice: &mut D)` is static dispatch for concrete dice, and also accepts `dyn Dice`, which the server uses.

**Game.**
- `Game { pos, turns: Vec<Turn>, outcome: Option<Outcome>, seen: HashMap<Key, u8> }`.
- `play` commits only on success.
- `Game::replay(start, &[Turn]) -> Result<Game, ReplayError>`.

**Errors.** `thiserror` types: `IllegalMove` (from `Position::apply`), `PlayError::{GameOver, Illegal}`, `ReplayError::{Play, MissingDice, UnusedDice}`, `FenError`, and `BadDie` (building a `D8` from a number outside 1..=8).

**Performance policy.** `cargo bench -p rules` (criterion) times perft from the start position to depth 4 and one seeded random game. To compare with Go without touching Go code (Go's perft helper is unexported), compare the suites both languages already have: the 10,000-random-games test and the perft test. First measurements on the author's machine, all cores:

| | Go | Rust |
| --- | --- | --- |
| 10,000 random games, invariants checked every ply | 20.6 s | 0.90 s |
| Perft suite (same depths) | 0.04 s | ~0.01 s |
| Perft start position, depth 4 (criterion) | — | 3.4 ms |
| One random game (criterion) | — | 0.48 ms |

## Server crate

**Dependencies:**
- `tokio` (multi-thread), `axum` and `axum-extra` (cookies);
- `tower-http` (`ServeDir`, `ServeFile`, `SetResponseHeader`, `RequestBodyLimit`, `TraceLayer`);
- `serde`, `serde_json`, `tokio-stream`, `tokio-util` (`CancellationToken`), `tracing`, `tracing-subscriber` and `rand`;
- `anyhow` in `main.rs` only.

### Game actor

There is one task per game, and it owns the game state with no locks. This is the model the roadmap requires so that the milestone 05 timers stay race-free.

**Hub and handles.**
- `Hub` is `Arc<HubInner>` with `games: Mutex<HashMap<Code, GameHandle>>`. It is a std mutex and is never held across an `.await`.
- `Hub::new(dice_factory, idle)`. Each game gets its own dice from the factory: in production a `StdRng` seeded from the OS; in tests, scripted dice.
- `Hub::create` retries on code collisions. `Hub::get` looks games up.
- `GameHandle` holds an `mpsc::Sender<Cmd>` (bounded, 32) and is cheap to clone.
- Handler-facing methods, such as `handle.make_move(guest, mv, seq).await -> Result<(), GameError>`, hide the `oneshot` plumbing.

**Commands.**
- `enum Cmd { Join{guest, reply}, Move{guest, mv, seq, reply}, Resign{guest, reply} }`.
- There is no `Leave` command. The actor sees how many subscribers each role has through the watch channels (`Sender::receiver_count`).

**The loop.** It runs `loop { select! { cmd = rx.recv() => …, () = sleep_until(deadline) => … } }`. On exit, an `OnExit` guard removes the game from the hub; being a drop guard, it also runs if the task panics.

**No panic catching.**
- The rules API is total: errors are `Result`s, and dice are valid by construction. A panic is a bug.
- If the actor dies anyway, its `mpsc` closes. Handlers turn the send error into `GameError::Gone` (500), and `Hub::get` treats a closed handle as absent, so later requests get 404.
- This replaces Go's `recover()`.

**Eviction.** This is new; the product spec asks for it and Go doesn't have it.
- The deadline is 24 h after the last command. When it passes, the game is dropped if it is over or nobody is watching (no watch receivers); otherwise the deadline moves on another 24 h. That covers finished games, games nobody joined, and abandoned games, while a game with players connected is never dropped.
- Tests use `#[tokio::test(start_paused = true)]`. No hand-rolled clock trait is needed, and milestone 05's clocks get the same benefit.

**Seats.**
- The creator is White, and the first other guest to join is Black.
- A reconnecting guest keeps their seat.
- Seats are deliberately not freed on disconnect, because that is what lets a player reload the page. Forfeiting an abandoned seat is milestone 05's 60 s rule.

**Move check order.** Seated → over → waiting → turn → `seq` → legal. Then:
1. compute SAN on the pre-move position;
2. play the move;
3. record it in the log, `lost` and stats;
4. broadcast.

`GameError` is a `thiserror` enum whose `Display` strings are the frontend's messages.

### Fan-out

- The JSON only varies by role: `you`, and `legal` for the player to move.
- The actor keeps one `watch::Sender<Arc<View>>` per role: white, black and spectator.
- A broadcast builds exactly three `View`s, one per role, whether or not anyone is watching that role. That keeps every receiver's current value fresh for the next join. The SSE handler serialises the view per connection with `Event::json_data`.
- Every spectator shares one `Arc<View>`.
- `watch` gives latest-value semantics: the game never blocks, and a slow reader skips to the newest state.

`View` is the actor's snapshot type, and it derives `Serialize`:
- `#[serde(rename_all = "camelCase")]` matches `savingRolls` and the other stats fields.
- `#[serde(skip_serializing_if = "Option::is_none")]` covers optional event fields.
- Event JSON is produced by `impl From<&rules::Event> for EventView`.

### HTTP

- **Routing:** an axum `Router` with nested `/api`.
- **Errors:**
  - `ApiError` is an enum that implements `IntoResponse`, producing `{"error": msg}` plus `state` for conflicts.
  - Handlers return `(CookieJar, Result<_, ApiError>)`, so a new guest cookie is set even on an error response, and `impl From<GameError> for ApiError` sets the status codes.
  - The JSON body extractor is axum's `Json` behind `#[derive(FromRequest)] #[from_request(via(Json), rejection(ApiError))]`, so malformed bodies get the same error shape.
  - Bodies are capped at 4 KiB with `DefaultBodyLimit`.
- **Guest:**
  - A `Guest(GuestId)` extractor built on `axum_extra::extract::CookieJar`.
  - Handlers return `(CookieJar, …)`, so the cookie is set only when it was missing.
  - `GuestId` is a validated newtype of 16 bytes, shown as hex.
- **SSE:**
  - `axum::response::sse::Sse` over `WatchStream` of the role's receiver (it yields the current view first), then `.take_until(shutdown.cancelled_owned())`. The response also sets `X-Accel-Buffering: no`.
  - Keep-alive uses `KeepAlive::new().interval(15s).text("ping")`. The interval lives in the app config so tests can shorten it.
  - Opening the stream is what joins the game.
- **Static files:**
  - `ServeDir::new(web_dir)` with `.precompressed_br().precompressed_gzip()` and a fallback of `ServeFile::new(index.html)` for SPA routes. This gives Range, ETag, Last-Modified and conditional requests for free.
  - `/_app/*` is a separate `ServeDir` with no fallback, so a missing hashed asset is a 404. (The Go server answers 200 with the SPA page, served `no-cache`, which a browser then fails to run as a script.)
  - Cache headers come from a small middleware: a successful `_app/immutable/*` response gets `public, max-age=31536000, immutable`, and everything else gets `no-cache`.
  - `WEB_DIR` defaults to `../web/build`, relative to the binary's working directory.
  - **Trade-off:** files are served from disk rather than embedded, so the binary is no longer self-contained. That's acceptable because the Rust server isn't deployed. If it ever is, add `rust-embed` behind a feature flag.
- **Shutdown:**
  - `axum::serve(...).with_graceful_shutdown(signal)` handles SIGINT and SIGTERM.
  - The shared `CancellationToken` is cancelled first so that SSE streams end, and other requests in flight get 10 s.
- **Configuration:**
  - `main.rs` reads `PORT` (default 8080) and `WEB_DIR` (default `../web/build`, which suits `cargo run` from `server_rs/`), and warns at startup if there is no `index.html`.
  - `tracing_subscriber` reads `RUST_LOG`.

## Errors and lints

- Library-style modules use `thiserror`, and `anyhow` appears only in `main.rs`.
- No `unwrap`/`expect` outside tests and `const` table construction.
- `[workspace.lints.clippy]`: `all = deny`, `pedantic = warn`, `unwrap_used = deny`, `expect_used = warn`. Tests allow these lints locally. Any lint that is turned off uses `#[expect(..)]` with a reason.
- CI runs `cargo fmt --check && cargo clippy --all-targets --locked -- -D warnings && cargo test --locked`.

## Testing

1. **Rules unit tests.**
   - Every Go rules test is ported with the same FENs, Mamdani squares, potholes and dice scripts.
   - The helpers are `setup(fen, mamdani, white_hole, black_hole)`, `dice(&[..])`, and `apply`, which asserts that the dice script is fully consumed.
   - Test names are descriptive, for example `pothole_next_to_mamdani_is_repaired`.
2. **Bitboard unit tests.** The attack tables are checked against a naive mailbox reference for every square and a set of blocker patterns.
3. **Perft.** Start position to depth 4 = 197,281, Kiwipete, and positions 3–5, all in plain chess. The deep counts are `#[ignore]` and run with `cargo test --release -- --ignored`.
4. **Random games.**
   - 500 seeded games by default and 10,000 under `--ignored`, using `rand_pcg`.
   - Invariants are checked after every ply, and replay must give the same position.
5. **Parity (differential).**
   - `server_rs/parity/main.go` uses only the exported Go API: `game.NewHub` with recorded seeded dice, `Hub.Create`, `Join` for white, black and a spectator, `Move`, and `Sub.C`.
   - It plays N random games from `View.Legal`. About 2.5% of turns are a move by the wrong player, and about 0.5% are a resignation, so refusals are compared too. It writes JSONL: `{seed, start, turns:[{guest, action, move, seq, dice, error, views:{white, black, spectator}}]}`. Each view's log is cut to its last entry; the log only grows, so this still checks every entry and keeps the file to about 400 KB per game.
   - `server/tests/parity.rs` drives a Rust actor with the recorded dice and compares each role's view after every turn as **parsed JSON**, ignoring `code` and sorting `legal`. It also checks that refusals carry the same message. A failure names the seed, the turn and the first differing JSON path.
   - With no `PARITY_FILE`, the test runs `go run ./server_rs/parity -n 30` itself, and skips if Go isn't installed. There are no committed fixtures.
   - The rules engine is checked through this test. A separate rules-only parity test would add nothing.
   - Result: 500 recorded games (about 100,000 turns, three views each) match. A deliberate one-character change to the dice log fails with `white.log[0].dice: go "d8 1" rust "d8:1"`.
6. **Actor tests.** Seats, reconnects and the check order. The broken-actor path (dropped `mpsc` → `Gone`) and eviction are tested with `tokio::time::pause()`.
7. **HTTP integration.**
   - A real `TcpListener` on port 0, a `reqwest` client with a cookie jar per player, and a small SSE frame reader. It is hand-written so that keep-alive comments can be seen.
   - It covers:
     - the friend game, including the 33 legal moves at the start;
     - error codes, and 409 carrying `state`;
     - cookie attributes;
     - keep-alive with a short interval;
     - static fallback and cache headers, from a temp dir, including a 404 for a missing hashed asset and a 405 for a POST to a page;
     - shutdown ending streams;
     - resignation.

## CI

- A new `rust` job in `.github/workflows/ci.yml` uses `dtolnay/rust-toolchain@stable` (clippy, rustfmt) and `Swatinem/rust-cache`, with working directory `server_rs`.
- The job sets up Go too, because the parity test runs the Go recorder.
- It runs the fmt/clippy/test command, then `go run ./server_rs/parity -n 300 -seed 1000`, and runs the parity test in release with `PARITY_FILE` pointing at the result.
- The existing jobs are untouched.

## Repo hygiene

- New files are confined to `server_rs/**`, this spec and its plan.
- `server_rs/.gitignore` ignores `/target/`.
- The edits to existing files are the CI job and one `.dockerignore` line (`server_rs/target`), which stops a local Docker build from sending gigabytes of Rust build output as context.
- Stage specific paths only. No Claude attribution in commits (CLAUDE.md).

## Later phase: persistence

This is not part of the first pass. It starts once parity is proven, so the parity tests compare like with like. It implements the product spec's §persistence. That is roadmap milestone 05 territory, so here the Rust server would get ahead of Go.

- **Schema:**
  - `games(code PRIMARY KEY, white, black, created_at, result)`
  - `turns(code, seq, mv, dice, PRIMARY KEY(code, seq))`
  - Only moves and rolls are stored, because rules plus fixed dice are deterministic, so `Game::replay` rebuilds any position.
- **Writer:** `rusqlite` with the `bundled` feature, on one dedicated blocking thread that owns the connection (WAL, `busy_timeout`). Actors send it requests over an `mpsc` channel with `oneshot` replies. There's no pool in front of a single-writer database.
- **Write ordering:**
  - The actor writes each turn *before* broadcasting it.
  - If the write fails, the move gets a 500 and nothing is broadcast, so clients never see a turn the database didn't keep.
  - Game creation, Black's seat, resignation and results are written the same way.
- **Startup:** before binding the listener, load unfinished games, replay each into an actor, and register it in the `Hub`.
- **Migrations:** an append-only list tracked in `schema_version`. It starts fresh, with no `honks`.
- **Tests:**
  - a restart in the middle of a game restores an identical view;
  - a failed write blocks the broadcast;
  - running migrations twice is safe.

## Risks

- **Bitboard bugs.** Sliders, castling paths through potholes, and en passant with a pothole are all risks. The defences are perft, the attack-table tests against a naive reference, and the parity test.
- **Rule divergence.** The defences are the parity test and the random-game invariants.
- **The frontend relying on a quirk this design drops**, such as `null` vs absent fields, or the 409 body shape. The defences are the end-to-end playthrough with the unchanged frontend, and a grep of `web/src` for every field it reads before the plan is written.
- **Serving from disk.** `WEB_DIR` must point at a built `web/build`. Startup logs a warning if `index.html` is missing.
