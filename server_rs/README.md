# server_rs: the Rust server (experiment)

A Rust implementation of the game server. It runs alongside the Go server and speaks the same API, so the SvelteKit frontend in `web/` runs unchanged against either one. The Go server is still the one that ships. The Rust server stops at the milestone 05 API: it has no guest names, quick match or live games list (milestone 06a), so the new home page and `/play` don't work against it, and its parity tests don't cover them. Design: [`docs/superpowers/specs/2026-10-04-rust-server-design.md`](../docs/superpowers/specs/2026-10-04-rust-server-design.md).

- `rules/` is the rules engine. It is pure (no I/O, no clock, no randomness of its own) and uses bitboards.
- `server/` is the HTTP server, built on tokio and axum. Each game is its own task, and views go out over SSE.
- `parity/` is a Go program that records games played on the Go server, so the Rust server can be checked against them.

## Run

```sh
cd web && pnpm build && cd ../server_rs
cargo run --release -p server        # http://localhost:8080, serving ../web/build
```

Environment variables:

| Variable | Default | What it does |
| --- | --- | --- |
| `PORT` | `8080` | Port to listen on. |
| `WEB_DIR` | `../web/build` | Where the built frontend is. |
| `RUST_LOG` | `info` | Log level. |

To work on the frontend with Vite instead, run `pnpm dev` in `web/`. It proxies `/api` to `:8080`, whichever server is listening there.

## Test

```sh
cargo fmt --check
cargo clippy --all-targets --locked -- -D warnings
cargo test                               # also replays 30 Go games, if Go is installed
cargo test --release -- --ignored        # deep perft and 10,000 random games
cargo bench -p rules                     # perft and whole-game timings
```

The parity test compares the two servers turn by turn: every view each role sees, and every refusal message. To run it on more games:

```sh
go run ./server_rs/parity -n 500 > /tmp/games.jsonl            # from the repo root
PARITY_FILE=/tmp/games.jsonl cargo test --release -p server --test parity
```
