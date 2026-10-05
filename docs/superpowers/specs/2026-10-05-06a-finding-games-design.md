# Milestone 06a: Finding games — Design

Date: 2026-10-05
Status: Draft for review
Builds on: [the main design spec](2026-10-01-mamdani-chess-design.md) and [the roadmap](../plans/2026-10-03-00-roadmap.md), row 06.

## Goal

Let people find a game without being sent a link: guest names, a real home page, quick match, join by code, and a list of live games to watch.

**Done when:** two strangers find each other through quick match. Concretely: two browsers that have never met tap Play online, land in the same game, and see each other's names; a third browser sees that game under Live now and can watch it.

Milestone 06 is split. This spec is **06a**. **06b** (a live watcher count over SSE, and emoji reactions) gets its own spec and plan after 06a ships.

## Decisions

| Topic | Decision | Why |
| --- | --- | --- |
| Names | Generated from `names/` (e.g. `pizza-rat-astoria`; how the words were chosen: [guest-names](../../guest-names.md)). A 🎲 button re-rolls; there is no free text. | All games are public and nobody moderates. Sites where strangers meet never let anonymous players pick a name: Lichess shows "Anonymous", chess.com guests get a generated `Guest…` name. Re-rolling gives some personality while strangers only ever see words from our list. |
| Where names live | A `guests` table keyed by the guest ID (the cookie's SHA-256). Stored, never derived from the ID. | A name derived from the ID would change for everyone whenever the word list changes. The cookie stays an opaque ID, so nobody can choose a name by editing it. |
| When a guest gets a name | Only when playing needs one: creating a friend game, joining quick match, taking a seat, or pressing 🎲. Visiting, browsing and watching never create one. | Every row in `guests` belongs to someone who played, so the table needs no pruning. |
| Names in games | Each seat keeps a snapshot of its player's name. | A re-roll mid-game doesn't rename anyone in that game, a restored game keeps its names, and old games never need rewriting. |
| Quick match | First come, first served; random colors; no rating. The queue is an SSE stream: open means queued, closed means gone. | A small pool makes waits the real problem, so any skill filter would only lengthen them. A closed tab can't leave a ghost in the queue. |
| Empty queue | Wait with no limit and a Cancel button. Home shows how many are looking; after 60 s the searching screen also offers "Play a friend instead". | Someone arriving later can see there's a person to match. |
| Live games | Games being played (both seated, not over), most watched first, then newest, at most 12. Polled every 10 s. | Friend games still waiting for their friend aren't for strangers. |
| Rust server | `server_rs/` stays at the 05 API. Its README notes the gap. | Keeping parity would double the server work for an experiment. |
| History | None in 06. Player history, ratings and leaderboards are milestone 08. | See the roadmap's milestone 08 section. 06 keeps everything 08 needs: `turns` and the name snapshots on seats. |

## Identity

```sql
-- migration 4
CREATE TABLE guests (
  id         TEXT PRIMARY KEY,   -- sha256 of the cookie, as games.white/black
  name       TEXT NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX guests_name ON guests(name);  -- for preferring unused names
ALTER TABLE games ADD COLUMN white_name TEXT;
ALTER TABLE games ADD COLUMN black_name TEXT;
```

- `store.EnsureGuest(ctx, id, draw) (name, error)` returns the guest's name. If there's no row yet, it inserts a fresh name (`INSERT … ON CONFLICT DO NOTHING`, then read back). Every path that needs a name goes through it.
- `store.RerollGuest(ctx, id, draw) (name, error)` gives the guest a fresh name and returns it.
- **A fresh name prefers names nobody has.** `draw` is `names.Random`. The store draws up to 5 candidates and keeps the first one that no other guest has and that differs from the guest's current name. If all 5 are taken, it uses the last one.
  - So names are unique in practice until roughly 10,000 guests, and repeats become possible gradually after that instead of failing.
  - It also means the second player in a game never gets the name of the player already seated.
  - Two guests named at the same moment can still collide, which is harmless: names are display-only and there's no unique index.
- Games saved before migration 4 have no names. Their player bars fall back to "White" and "Black".
- **The cookie is unchanged** (`server/guest.go`). It's still set on the first API request; only a name is deferred.

### API

| Method | Path | Answer |
| --- | --- | --- |
| `GET` | `/api/me` | `{name}`, or `{name: null}` for a guest without one. Never creates a name. |
| `POST` | `/api/me/name` | Re-rolls (or creates) the name and returns `{name}`. The new name is always different from the old one. |

### Seats

- `store.Game` gains `WhiteName` and `BlackName`; `CreateGame` writes them, `SeatBlack(ctx, code, guest, name)` writes Black's, and `LoadForRestore` reads them back.
- **Friend game:** `POST /api/games` calls `EnsureGuest` first and passes the name to `Hub.Create`.
- **Taking Black's seat:** `Game.Join` already seats Black inside the game goroutine and saves it with `SeatBlack`. The stream handler can't know beforehand whether the guest will be seated, so `Join` gets the name through a function `name(guest) (string, error)` that the hub is given (backed by `EnsureGuest`), and calls it only when it seats someone. Spectators never call it.
- **Quick match:** both names are known before the game is created (below).
- `View` gains `players: {white, black}` (names, `""` when unknown). `PlayerBar` shows the name, falling back to "White"/"Black", and keeps "(you)" and the online marker.
- **Names are up to 24 characters and 3 words** (`names.MaxLen`, `names.MaxWords`). Everywhere a name appears (player bar, header pill, live-game card) it sits on one line and ends in an ellipsis when it doesn't fit, with the full name in a `title`. On a phone's live-game card that applies to names over about 20 characters.

## Live games

- On every `broadcast`, a game stores a `Summary` in an `atomic.Pointer` on the `Game`:
  `{code, white, black, move, board, potholes, mamdani, last, watching, started}`.
  - `move` is the full-move number; `last` the latest move's from and to squares.
  - `watching` counts distinct guests with a stream open who aren't seated, so two tabs count once.
  - `started` is when Black took the seat; it orders ties.
- `Hub.List()` takes `mu`, copies the pointers of games whose status is playing, and sorts them. It never waits on a game's goroutine.
- `GET /api/games` → `{games: [...], looking: n}`, where `looking` is the number of guests in the quick-match queue.

## Quick match

A new package, `match`.

- **One goroutine owns the queue.** Commands arrive over a channel, as in `game`.
- **One entry per guest.** An entry holds the guest's name and their open streams, so two tabs share one place in line, and a guest can never be paired with themselves.
- **Pairing:** whenever the queue has two entries, take the two oldest, pick colors with `crypto/rand`, and call `create(white, black Player) (code, error)`. `Player` is `{ID, Name}`. The hub supplies `create`; it is `Hub.create` with both seats and names filled, as rematch already does, so White's 60 s first-move deadline starts at once.
- **On success:** send `matched {code}` on every stream of both entries and remove them. **On failure:** both entries stay at the front of the queue and the error is logged; the next arrival retries.
- **Leaving:** a stream's request context ends (Cancel, a closed tab, a dropped connection) → that stream is removed; the entry goes when its last stream does. A dropped connection that EventSource reconnects joins the back of the queue unless another tab kept the entry.
- **No-shows:** a matched guest who never opens the game is handled by the existing first-move abort (60 s, result `aborted`).
- **Looking count:** the queue goroutine publishes its length in an `atomic.Int64` for `GET /api/games`.

### `GET /api/match`

- Calls `EnsureGuest`, then `startSSE` and joins the queue.
- Events: `queued {looking}` once at the start, `matched {code}`, and the 15 s heartbeat (`writeHeartbeat`).
- After `matched`, the server **keeps the stream open** until the browser closes it (at most 10 s). If the server closed first, EventSource would reconnect and queue the guest again.

## Frontend

The canvas is the visual reference: artboards **Home**, **Home (phone)**, **Quick match: searching** and **Header component**. Two corrections to the canvas, made there as well: the home copy says "a d6 is rolled" but every die is a d8, and the name pill's pencil becomes 🎲.

### `Header.svelte`

- Logo (links home), Play, Rules (a link that 404s until milestone 07's Rules page; hide it until then), and the name pill: initials, name, 🎲.
- Pressing 🎲 calls `POST /api/me/name`; the new name flips in (no motion under reduced motion).
- A guest without a name sees no pill.
- On phones (under 640 px): the round initials button, which re-rolls on tap and announces the new name to screen readers.
- Used on `/` and `/play`. The game page keeps its own header.

### Home `/`

- **Hero:** eyebrow, headline, pitch, demo board with the pothole callout, as on the canvas. The demo board is the existing `Board` with `interactive={false}` and a fixed position.
- **Play online** → `/play`. Shows "N looking" when `looking > 0`.
- **Play a friend** → creates a game and goes to it, as today. The button keeps this accessible name: `web/scripts/playtest.js` clicks it.
- **Have a game code?** The input uppercases as you type and accepts only the code alphabet; Join is enabled at 6 characters and goes to `/game/CODE`. The game page already handles unknown codes. Normalising and checking live in `#lib/code.ts`, unit-tested.
- **Live now:** a card per game: mini board, both names, move number, watching count. The whole card links to `/game/CODE`. Polled every 10 s while the tab is visible (`visibilitychange`). On phones the cards scroll sideways. With no games: "No games right now. Start one." with the two play buttons' actions.
- **How it works:** the canvas's three steps, with d8.
- **Footer:** the Pot-Hole Chess credit, as on the canvas.

The mini board is `Board` with `interactive={false}`, no coordinates and no animation. If twelve of them make the page slow on a phone (checked while building), a small SVG thumbnail replaces it.

### Quick match `/play`

- Opens `EventSource('/api/match')`, with the elapsed-time ring, the chips (10+5, Standard rules, Random colors), Cancel, and "Keep this tab open".
- On `matched`: close the EventSource, then `goto('/game/CODE')`.
- Cancel closes the EventSource and goes home.
- The elapsed time counts from when the page opened, across reconnects.
- After 60 s, "Nobody yet. Play a friend instead" appears below Cancel; it creates a friend game (leaving the queue).

## Testing

- **Go**
  - `match`, under `testing/synctest`: two guests pair in arrival order; a guest with two tabs is one entry and both tabs get `matched`; one guest alone never pairs; closing the last stream leaves the queue; a failed `create` keeps both at the front; the looking count follows.
  - `store`: migration 4 on a database with 05 games (old games still restore, names empty); `EnsureGuest` creates once and then returns the same name; `RerollGuest` changes it; with a scripted `draw`, a fresh name skips candidates another guest already has and falls back to the last one when all 5 are taken.
  - `game`: a seated Black gets a name snapshot; a spectator calls no name function; a re-roll after seating doesn't change the game's names; `Hub.List` filters, orders and caps; `watching` counts guests, not tabs.
  - `server`: viewing endpoints (`GET /api/me`, `GET /api/games`, a spectator stream) create no `guests` row; `POST /api/games`, `GET /api/match` and taking a seat each create exactly one; `GET /api/match` end to end with two clients.
- **Web:** Vitest for `#lib/code.ts`; `pnpm --dir web check`, `test`, `build`; `npx @sveltejs/mcp svelte-autofixer` on every new or changed component.
- **Playtest:** `playtest.js` gains `--match`: both guests tap Play online and play the matched game. Run the default and `--match` in Chromium and WebKit, and with `--phone`.
- **By hand:** `localhost`, `[::1]` and `127.0.0.1` as three guests: two quick-match each other and see names; the third sees the game under Live now and watches it.

## Out of scope

- 06b: the live watcher count over SSE (`presence`) and emoji reactions.
- Player history, stats, ratings, leaderboards (milestone 08).
- Per-IP limits on the match stream and game creation (milestone 07, with the other limits).
- The Rules page (milestone 07).
