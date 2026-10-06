# Pothole Chess: Mamdani Edition

A browser chess variant to play with friends. It is [Pot-Hole Chess](https://www.chessvariants.com/boardrules.dir/potholechess.html) (Peter Spicer and Michael Chamberlain, 2001) plus one new piece: the Mamdani.

- **Potholes.** After every move you roll a d8. On an even roll, two more d8s pick a square and a pothole opens there. Whatever stands on it falls in and is lost; kings never fall. The pothole blocks the square until the player who rolled it finishes their next move.
- **The Mamdani.** A neutral piece that either player can move instead of their own. It moves like a queen but never captures, blocks lines like any piece, repairs potholes on the squares around it, and gives pieces it can reach a saving roll.

Chess comes first: the dice bring the chaos, and the Mamdani is how you fight back. The full rules, with every decision and why it was made, are in [RULES.md](RULES.md).

## Status

Early development. You can play a full game: press **Play a friend**, send the link, and play in two browsers. Every roll plays out in a dice tray, and the game ends with a result card. There is no clock yet, and games live in memory, so they vanish when the server restarts.

| Done | Next |
| --- | --- |
| Rules engine (`rules/`): move generation, potholes, saving rolls, game end and replay. It is tested against published chess move counts (perft) and 10,000 random games. | A playtest with friends, then clocks and saved games |
| App: Go server, live updates over Server-Sent Events, SvelteKit frontend with a tap-to-move board, CI and a Dockerfile. | Clocks, saved games, quick match, spectators and emoji reactions, then a public launch |

The plan is in [docs/superpowers/plans/2026-10-03-00-roadmap.md](docs/superpowers/plans/2026-10-03-00-roadmap.md).

## How it will work

- **Guests only.** No accounts; you get a random name like `pizza-rat-astoria`.
- **Ways to play.** Quick match with a stranger, or send a friend a link or a 6-character code.
- **Clock.** One time control for every game: 10 minutes each, plus 5 seconds per move.
- **Public games.** Every game is public; spectators watch live and react with emoji instead of chat.

## Stack

- **Server:** Go. It sends live updates to the browser with Server-Sent Events, takes moves as JSON POSTs, and stores games in SQLite (pure Go, no cgo).
- **Frontend:** SvelteKit 3 and Svelte 5, built as a static app and served by the Go binary. The chess board is a custom component.
- **Deploy:** one Docker image, intended for Railway.

The design is in [docs/superpowers/specs/2026-10-01-mamdani-chess-design.md](docs/superpowers/specs/2026-10-01-mamdani-chess-design.md).

## Running it locally

You need Go 1.27 or newer, Node 22.17 or newer, and pnpm 10.

```sh
# Terminal 1: the Go server on :8080 (data goes to data/mamdani.db)
go run ./cmd/server

# Terminal 2: the frontend dev server on :5173, which forwards /api to :8080
pnpm --dir web install
pnpm --dir web dev
```

Then open http://localhost:5173.

To work on the board without a second browser, open http://localhost:5173/dev/board (dev server only). One browser plays both sides, moves ignore check, and you pick what the dice do next: a pothole, a fall, a saving roll and so on. It uses the same board, dice tray and animation code as real games.

To build a single binary with the frontend embedded:

```sh
pnpm --dir web build
go build -tags embedweb -o bin/server ./cmd/server
./bin/server
```

## Tests

```sh
go test ./...            # Go tests; the rules engine's random games take about 20 s
go test -short ./...     # a quicker run with 500 random games
pnpm --dir web check     # Svelte and TypeScript type checks
```

## Layout

| Path | What's there |
| --- | --- |
| `rules/` | The rules engine. Pure Go: no I/O, no clock, dice passed in. |
| `server/` | HTTP routes, Server-Sent Events, static file serving |
| `store/` | SQLite and migrations |
| `cmd/server/` | The server binary |
| `web/` | The SvelteKit app |
| `names/` | Random guest names |
| `docs/superpowers/` | Design spec and implementation plans |

## Credits

- **Pot-Hole Chess** by Peter Spicer and Michael Chamberlain, published on [The Chess Variant Pages](https://www.chessvariants.com/boardrules.dir/potholechess.html) (2001).
- **The Mamdani** comes from a [patch video](https://www.instagram.com/reels/Dd8tV_MxEQL/).
- **Chess pieces:** the "mpchess" set by [Maxime Chupin](https://github.com/chupinmaxime), as published in [lichess](https://github.com/lichess-org/lila/tree/5ae58154b2be1033dcb0252173b4fff4bb02e73a/public/piece/mpchess), under the GNU GPL v3 or later, with every line 1.2 times as heavy ([notice](web/static/pieces/LICENSE.txt), [license](web/static/pieces/GPL-3.0.txt)).
