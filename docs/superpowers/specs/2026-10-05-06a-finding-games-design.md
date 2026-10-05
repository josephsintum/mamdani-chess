# Milestone 06a: Finding games — Design

Date: 2026-10-05
Status: Built (plan: [2026-10-05-06a-finding-games](../plans/2026-10-05-06a-finding-games.md))
Builds on: [the main design spec](2026-10-01-mamdani-chess-design.md) and [the roadmap](../plans/2026-10-03-00-roadmap.md), row 06.

## Goal

Let people find a game without being sent a link: guest names, a real home page, quick match, join by code, and a list of live games to watch.

**Done when:** two strangers find each other through quick match. Concretely: two browsers that have never met tap Play online, land in the same game, and see each other's names; a third browser sees that game under Live now and can watch it.

Milestone 06 is split. This spec is **06a**. **06b** (a live watcher count over SSE, and emoji reactions) gets its own spec and plan after 06a ships.

## Decisions

| Topic | Decision | Why |
| --- | --- | --- |
| Names | Generated from `names/` (e.g. `pizza-rat-astoria`; how the words were chosen: [guest-names](../../guest-names.md)). A die offers 3 new names to pick from, 3 changes in any 24 hours; there is no free text. | All games are public and nobody moderates. Sites where strangers meet never let anonymous players pick a name: Lichess shows "Anonymous", chess.com guests get a generated `Guest…` name. Picking from offers gives some personality while strangers only ever see words from our list. |
| Where names live | A `guests` table keyed by the guest ID (the cookie's SHA-256). Stored, never derived from the ID. | A name derived from the ID would change for everyone whenever the word list changes. The cookie stays an opaque ID, so nobody can choose a name by editing it. |
| When a guest gets a name | Only when playing needs one: creating a friend game, joining quick match, or taking a seat. Visiting, browsing and watching never create one. | Every row in `guests` belongs to someone who played, so the table needs no pruning. |
| Names in games | Each seat keeps a snapshot of its player's name. | A name change mid-game doesn't rename anyone in that game, a restored game keeps its names, and old games never need rewriting. |
| Name changes | 3 in any 24 hours, counted from the first change in the window; the name given at first play doesn't count. Each change picks one of 3 offered names, and the offers stay the same until one is chosen. Enforced on the server. | A name only works if it stays put long enough to be recognised. Fixed offers mean closing and reopening the picker isn't a free re-roll, so nobody can fish for one combination. A limit kept only in the browser would be a hint anyone can clear. Clearing cookies still gives a new guest with a new random name; that's accepted for a guest-only site. |
| Quick match | First come, first served; random colors; no rating. The queue is an SSE stream: open means queued, closed means gone. A mutex guards the queue. | A small pool makes waits the real problem, so any skill filter would only lengthen them. A closed tab can't leave a ghost in the queue. |
| Empty queue | Wait with no limit and a Cancel button. Home shows how many are looking; after 60 s the searching screen also offers "Play a friend instead". | Someone arriving later can see there's a person to match. |
| Live games | Games being played (both seated, not over), most watched first, then newest, at most 12. Polled every 10 s. | Friend games still waiting for their friend aren't for strangers. |
| Rust server | `server_rs/` stays at the 05 API. Its README notes the gap. | Keeping parity would double the server work for an experiment. |
| History | None in 06. Player history, ratings and leaderboards are milestone 08. | See the roadmap's milestone 08 section. 06 keeps everything 08 needs: `turns` and the name snapshots on seats. |

## Identity

```sql
-- migration 4
CREATE TABLE guests (
  id            TEXT PRIMARY KEY,   -- sha256 of the cookie, as games.white/black
  name          TEXT NOT NULL,
  created_at    INTEGER NOT NULL,
  changes       INTEGER NOT NULL DEFAULT 0,  -- name changes in the current window
  changes_since INTEGER,                     -- when the window's first change was
  offers        TEXT                         -- names on offer, comma-separated
);
CREATE INDEX guests_name ON guests(name);  -- for preferring unused names
ALTER TABLE games ADD COLUMN white_name TEXT;
ALTER TABLE games ADD COLUMN black_name TEXT;
```

- `store.GuestName(ctx, id) (name, error)` returns the guest's name, or `""` without creating one.
- `store.EnsureGuest(ctx, id, draw) (name, error)` returns the guest's name. If there's no row yet, it inserts a fresh name (`INSERT … ON CONFLICT DO NOTHING`, then read back). Every path that needs a name goes through it.
- `store.Allowance(ctx, id, now)` returns the changes left (`store.NameChanges` = 3 per `store.NameWindow` = 24 h) and when the window ends.
- `store.NameOffers(ctx, id, draw, now)` returns the guest's 3 offers, drawing and saving them if there are none. They never include the current name. It returns `ErrNoName` before the guest has played, and `ErrNoChanges` (with the allowance) when none are left.
- `store.ChooseName(ctx, id, name, now)` takes one of the offers, uses a change and clears the offers. It returns `ErrNotOffered` for any other name, so nobody can set an arbitrary name through the API.
- **A fresh name prefers names nobody has.** `draw` is `names.Random`. The store draws up to 5 candidates and keeps the first one that no other guest has and that differs from the guest's current name. If all 5 are taken, it uses the last one.
  - So names are unique in practice until roughly 9,000 guests, and repeats become possible gradually after that instead of failing.
  - It also means the second player in a game never gets the name of the player already seated.
  - Two guests named at the same moment can still collide, which is harmless: names are display-only and there's no unique index.
- Games saved before migration 4 have no names. Their player bars fall back to "White" and "Black".
- **The cookie is unchanged** (`server/guest.go`). It's still set on the first API request; only a name is deferred.

### API

| Method | Path | Answer |
| --- | --- | --- |
| `GET` | `/api/me` | `{name, changesLeft, changesResetAt}`; `name` is null for a guest without one, and `changesResetAt` (Unix ms) is null while no change is used. Never creates a name. |
| `GET` | `/api/me/names` | `{names: [3], changesLeft, changesResetAt}`. 409 before the guest has played; 429 `{changesResetAt}` when no changes are left. |
| `POST` | `/api/me/name` | `{name}` must be one of the offers. Answers `{name, changesLeft, changesResetAt}`; 409 for a name not on offer; 429 when no changes are left. |

### Seats

- `store.Game` gains `WhiteName` and `BlackName`; `CreateGame` writes them, `SeatBlack(ctx, code, guest, name)` writes Black's, and `LoadForRestore` reads them back.
- **The game's `Store` interface gains `EnsureGuest`**, so the game package creates names itself: no name function is wired through the hub. A hub without a store (`nopStore`) gives no names.
- **Friend game:** `Hub.Create(creator)` calls `EnsureGuest` and seats White with the name.
- **Taking Black's seat:** `Game.Join` already seats Black inside the game goroutine. It now calls `EnsureGuest` and `SeatBlack` in the same save, only when it seats someone, so spectators never get a name. A failed save retires the game, as any failed save does.
- **Quick match:** `Hub.CreatePair(white, black)` calls `EnsureGuest` for both, then creates the game with both seats filled, as rematch does, so White's 60 s first-move deadline starts at once.
- **Rematch:** the new game copies the old game's name snapshots, swapped with the colors.
- `View` gains `players: {white, black}` (names, `""` when unknown). `PlayerBar` shows the name, falling back to "White"/"Black", and keeps "(you)" and the online marker.
- **Names are up to 24 characters and 3 words** (`names.MaxLen`, `names.MaxWords`). Everywhere a name appears (player bar, header pill, live-game card) it sits on one line and ends in an ellipsis when it doesn't fit, with the full name in a `title`. On a phone's live-game card that applies to names over about 20 characters.

## Live games

- A game publishes a `Live` in an `atomic.Pointer` on the `Game` (`game/live.go`): once when it's created or restored, and after every call into it, before the call returns. So a game is listed as soon as `Create` returns, and a spectator who just joined is already counted.
  `{code, white, black, move, board, mamdani, potholes, last, watching}`, plus its status and creation time, which aren't sent.
  - `move` is the full-move number; `last` is the latest move's from and to squares, or null.
  - `watching` counts distinct guests with a stream open who aren't seated, so two tabs count once.
- `Hub.List(max)` takes `mu`, copies the pointers of games whose status is playing, and sorts them: most watched first, then newest by creation time. It never waits on a game's goroutine.
- `GET /api/games` → `{games: [...], looking: n}`, where `looking` is the number of guests in the quick-match queue.

## Quick match

A new package, `match`.

- **A mutex guards the queue.** Pairing calls `create` under it; that's a few database calls per match, so joins wait at most a few milliseconds. A mutex needs no goroutine to start or stop, so tests don't need `synctest`.
- **One entry per guest.** An entry holds the guest's open tickets (one per stream), so two tabs share one place in line, and a guest can never be paired with themselves.
- **Pairing:** whenever the queue has two entries, take the two oldest, pick colors with `math/rand/v2`, and call `create(white, black string) (code, error)`. The server supplies `create`: `Hub.CreatePair`, which also names both players.
- **On success:** send `matched {code}` on every stream of both entries and remove them. **On failure:** both entries stay at the front of the queue and the error is logged; the next arrival retries.
- **Leaving:** a stream's request context ends (Cancel, a closed tab, a dropped connection) → that stream is removed; the entry goes when its last stream does. A dropped connection that EventSource reconnects joins the back of the queue unless another tab kept the entry.
- **No-shows:** a matched guest who never opens the game is handled by the existing first-move abort (60 s, result `aborted`).
- **Looking count:** the queue keeps its length in an `atomic.Int64` for `GET /api/games`.

### `GET /api/match`

- Calls `EnsureGuest`, then `startSSE` and joins the queue.
- Events: `queued {looking}` once at the start, `matched {code}`, and the 15 s heartbeat (`writeHeartbeat`).
- `matched` goes out with an SSE `retry: 3600000`, and then the server ends the stream. Without the long retry, EventSource would reconnect at once and queue the guest again before the page has gone to the game. The page also closes the EventSource as soon as `matched` arrives.

## Frontend

The canvas is the visual reference: artboards **Home**, **Home (phone)**, **Quick match: searching** and **Header component**. Two corrections to the canvas, made there as well: the home copy says "a d6 is rolled" but every die is a d8, and the name pill's pencil becomes a die (an inline SVG: the canvas avoids emoji).

### `Header.svelte`

- The logo (links home) and the name pill: initials, name, and a die button labelled with the changes left ("New name (2 changes left today)"). There's no Play or Rules link yet: Play is the home page itself, and the Rules page is milestone 07.
- Pressing the die opens a panel under the pill. At its top, under the title, it shows the changes left, as three pips and "2 of 3 left today", so they're in view while choosing. Then the 3 offers as buttons (focus moves to the first) and "Keep <name>". Choosing one changes the name, which flips in (no motion under reduced motion) and is announced to screen readers. Escape or a click outside closes the panel.
- With no changes left, the die is dimmed and labelled "New names again in 5 h". The panel shows "0 of 3 left today" and when names come back instead of offering names, and its button says OK.
- A guest without a name sees no pill.
- On phones (under 640 px), as on the canvas's phone artboard: a 24 px mark, no edition tag, and the name as its initials next to the die (screen readers still hear the whole name). A full name next to the title didn't fit: it covered the title on phones narrower than about 410 px.
- Used on `/` and `/play`. The game page keeps its own header.

### Home `/`

- **Hero:** eyebrow, headline, pitch, demo board with the pothole callout, as on the canvas. The demo board is a `MiniBoard` of the canvas's position; the callout reads "e6 · d8 4 → pothole at f4". Phones leave the board out, as the canvas does.
- **Play online** → `/play`. Shows "N looking" when `looking > 0`.
- **Play a friend** → creates a game and goes to it, as today. The button keeps this accessible name: `web/scripts/playtest.js` clicks it.
- **Have a game code?** The input uppercases as you type and accepts only the code alphabet; a pasted game link gives its code. Join is enabled at 6 characters and goes to `/game/CODE`. The game page already handles unknown codes. Normalising and checking live in `#lib/lobby.ts` (`normalizeCode`, `isCode`), unit-tested.
- **Live now:** a card per game: mini board, both names, move number, watching count. The whole card links to `/game/CODE`. Polled every 10 s while the tab is visible (`visibilitychange`). On phones the cards scroll sideways. With no games: "No games right now. Start one: play online or invite a friend."
- **How it works:** the canvas's three steps, checked against `RULES.md`: d8 throughout, and a saving roll needs a clear line from the Mamdani (the canvas said "could reach next turn").
- **Footer:** the Pot-Hole Chess credit, as on the canvas.

**The mini board is its own component, `MiniBoard.svelte`**, not `Board`. `Board`'s squares are `<button>`s, so twelve of them inside card links would put 768 buttons inside links: invalid HTML and a keyboard trap. `MiniBoard` is one `role="img"` with a label ("pizza-rat-astoria against bagel-soho, move 12"), drawn with plain elements: squares, the last move, potholes, pieces and the Mamdani, sized from the board's width (`cqw`).

### Quick match `/play`

- Opens `EventSource('/api/match')` (and opens a new one 2 s after an error answer, such as a proxy's 502 during a deploy), with the elapsed-time ring, the chips (10+5, Standard rules, Random colors), Cancel, and "Keep this tab open".
- On `matched`: close the EventSource, then `goto('/game/CODE')`.
- Cancel closes the EventSource and goes home.
- The elapsed time counts up from when the page opened (`formatElapsed`), across reconnects. `formatClock` counts down and shows tenths, so it doesn't fit.
- After 60 s, "Nobody yet. Play a friend instead" appears below Cancel; it creates a friend game (leaving the queue).

## Testing

- **Go**
  - `match`: two guests pair in arrival order; a guest with two tabs is one entry and both tabs get `matched`; one guest alone never pairs; closing the last stream leaves the queue; a failed `create` keeps both at the front; the looking count follows.
  - `store`: migration 4 on a database with 05 games (old games still restore, names empty); `EnsureGuest` creates once and then returns the same name; offers stay the same until one is chosen; only an offered name can be chosen; a fourth change within 24 hours is refused and three come back after; with a scripted `draw`, a fresh name skips candidates another guest already has and falls back to the last one when all 5 are taken.
  - `game`: a seated Black gets a name snapshot; a spectator gets no name; a name change after seating doesn't change the game's names; `Hub.List` filters, orders and caps; `watching` counts guests, not tabs.
  - `server`: viewing endpoints (`GET /api/me`, `GET /api/games`, a spectator stream) create no `guests` row; `POST /api/games`, `GET /api/match` and taking a seat each create exactly one; `GET /api/match` end to end with two clients; closing the stream leaves the queue; `matched` carries the long retry.
- **Web:** Vitest for `#lib/lobby.ts` (codes, initials, the FEN helper, elapsed time); `pnpm --dir web check`, `test`, `build`; `npx @sveltejs/mcp svelte-autofixer` on every new or changed component.
- **Playtest:** `playtest.js` gains `--match`: both guests tap Play online and play the matched game. Run the default and `--match` in Chromium and WebKit, and with `--phone`.
- **By hand:** `localhost`, `[::1]` and `127.0.0.1` as three guests: two quick-match each other and see names; the third sees the game under Live now and watches it.

## Out of scope

- 06b: the live watcher count over SSE (`presence`) and emoji reactions.
- Player history, stats, ratings, leaderboards (milestone 08).
- Per-IP limits on the match stream and game creation (milestone 07, with the other limits).
- The Rules page (milestone 07).
