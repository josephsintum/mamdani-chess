# Mamdani Chess

A browser chess variant to play with friends. It is [Pot-Hole Chess](https://www.chessvariants.com/boardrules.dir/potholechess.html) (Peter Spicer and Michael Chamberlain, 2001) plus one new piece: the Mamdani.

- **Potholes.** After every move you roll a d8. On an even roll, two more d8s pick a square and a pothole opens there. Whatever stands on it falls in and is lost; kings never fall. The pothole blocks the square until the player who rolled it finishes their next move.
- **The Mamdani.** A neutral piece that either player can move instead of their own. It moves like a queen but never captures, blocks lines like any piece, repairs potholes on the squares around it, and gives pieces it can reach a saving roll.

Chess comes first: the dice bring the chaos, and the Mamdani is how you fight back. The full rules, with every decision and why it was made, are in [RULES.md](RULES.md).

<p align="center">
  <img src="https://raw.githubusercontent.com/josephsintum/mamdani-chess/pr-screenshots/readme/gameplay-desktop.png" alt="A game on a laptop: three potholes are open, the Mamdani stands on a6, and the dice have just dropped a white pawn into a new pothole on e2" width="68%">
  <img src="https://raw.githubusercontent.com/josephsintum/mamdani-chess/pr-screenshots/readme/gameplay-phone.png" alt="The same moment on Black's phone: the board turned for Black, and the dice card saying the white pawn falls into e2" width="26%">
</p>

<p align="center"><b>Play it at <a href="https://mamdanichess.com">mamdanichess.com</a></b></p>

## How it works

- **Guests only.** No accounts: you get a random New York name like `babylon-governor`, and can trade it for another up to three times a day.
- **Ways to play.** Quick match with a stranger, or send a friend a link or a 6-character code.
- **Clock.** One time control for every game: 10 minutes each, plus 5 seconds per move. The clock waits while the dice play out.
- **Public games.** Every game is public: the home page lists the live ones, and anyone can open a game and watch. Emoji reactions for spectators are planned.
- **Saved games.** Games are saved as they go, so a server restart doesn't lose them. A game ends with a result card and a rematch button.

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
- **Chess sounds:** the SFX set by [Enigmahack](https://github.com/Enigmahack), from [Lichess](https://github.com/lichess-org/lila/tree/master/public/sound/sfx), under the GNU AGPL v3 or later. Every sound file, with its source and licence, is listed in [`web/src/lib/sounds/CREDITS.md`](web/src/lib/sounds/CREDITS.md).

## License

Copyright (C) 2026 Joseph Sintum.

Mamdani Chess is free software under the [GNU Affero General Public License v3.0 or later](LICENSE). You can use, change and share it. If you run a changed version for other people, over a network or otherwise, you must offer them its source under the same license.

Some files in this repo are not covered by that license:

- **Chess pieces** (`web/static/pieces/`) keep their own license, the GNU GPL v3 or later, as their [notice](web/static/pieces/LICENSE.txt) says. The GPL v3 and the AGPL v3 allow the two to be combined.
- **The Mamdani art** (`web/static/mamdani/`) is not licensed for reuse. Don't copy it into your own projects.
- **Other sound clips** (marked "Unconfirmed" in [`web/src/lib/sounds/CREDITS.md`](web/src/lib/sounds/CREDITS.md)) come from free sound sites and are believed to be in the public domain. If a clip's licence can't be confirmed, or it isn't in the public domain, [message the developer](https://github.com/josephsintum/mamdani-chess/issues) and it will be replaced.
- **Sound clips from Pixabay** (`die.mp3`, `dice.mp3` and `closed.mp3` in `web/src/lib/sounds/`) are under the [Pixabay Content License](https://pixabay.com/service/license-summary/), not the AGPL. It allows them in the game but not handing them out as files on their own, so don't reuse them outside it.
- **The names "Mamdani" and "Mamdani Chess"** are not licensed either. The license covers the code, not any person's name or likeness. Mamdani Chess is unofficial and not affiliated with Zohran Mamdani.
