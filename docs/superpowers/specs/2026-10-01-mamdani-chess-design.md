# Pothole Chess: Mamdani Edition — Design

Date: 2026-10-01
Status: Draft for review
Rules: [../../../RULES.md](../../../RULES.md) (live doc: https://claude.ai/code/artifact/50584f76-084a-4ea2-ae02-8534639e9b76)

## Goal

A browser game where friends — and strangers via quick match — play Pothole Chess: Mamdani Edition against each other, with spectators watching and reacting. Built as a demo: simple to run, simple to share, fun to watch.

**Success for the demo:** two friends can share a link, play a full game on phone or laptop, see every dice roll animate, and spectators can watch and react — without anyone needing an account.

## Decisions

| Topic | Decision |
| --- | --- |
| Identity | Guests only. Random editable name (e.g. "Pothole Pete") stored with a guest ID in a cookie. |
| Visibility | All games are public. |
| Time control | One fixed clock for every game: 10+5 (10 minutes each, +5 seconds per move). |
| Matchmaking | Quick match only: first-come-first-served queue, random colors. |
| Friend games | Create a game, share a link or 6-character code. |
| Chat | None. Emoji reactions from players and spectators, visible to everyone. |
| House rules | Configurable per friend game; quick match always uses the defaults in RULES.md. |
| Server | Go, single binary, serves the frontend too. |
| Transport | Server-Sent Events (server → browser) + JSON POST (browser → server). |
| Frontend | SvelteKit (Svelte 5) with `adapter-static`; custom board component, no chessground. |
| Storage | SQLite via `modernc.org/sqlite` (pure Go). |
| Hosting | One Railway service. |
| Visual design | "Road works" look: dark asphalt with road-marking yellow. Dark only for now. See §6. |

## Out of scope (for now)

Accounts, ratings, game history, private games, chat, draw offers, takebacks, premoves, time-control choice, open-challenge lobby, playing against a computer, sound settings.

---

## 1. Screens and flows

### Identity and codes
- First request assigns a guest cookie (`guest_id` + name). The name shows top-right and is editable.
- Each game has a 6-character code from an alphabet without lookalikes (no `0 O 1 I L`), e.g. `K7F3QZ`. The game URL is `/game/K7F3QZ`; "join with code" navigates there.

### Home `/`
- Header: logo, Rules link, guest name.
- Hero: one-line pitch and three actions — **Play online** (quick match), **Play a friend** (create game, with an optional *House rules* panel), **Join with code** (input + Go).
- **Live games**: cards for games in progress — players, move number, watcher count, mini board. Click to spectate. Refreshed every 10 s.
- **About**: 2–3 sentences on potholes and the Mamdani, link to Rules.

### Waiting
- **Quick match**: "Looking for an opponent…" with elapsed time and Cancel. On match, go to the game.
- **Friend game**: game page in a waiting state showing the code, a copy-link button, and "Waiting for your friend…". The game starts when a second guest opens the link.
- A third visitor to a full game is a spectator.

### Game `/game/:code`
- Board in the center. Player bars above and below: name, clock, pieces lost.
- **Dice tray** beside the board. After each move the clock pauses and the turn plays out in about 2 s: d6 → (if even) two d8 and the target square glows → (if eligible) saving d8 → crack and fall, or saved.
- **Log panel**: moves and rolls, e.g. `e4 · d6 4 → d3 · save 5 ✓`.
- **Reactions**: players have an emoji bar; their reactions float from their player bar. Spectator reactions rise in a strip at the board's edge, beside a 👀 watcher count. Fixed set: 😂 😱 🔥 🕳️ 👏 😭.
- **Resign** with an inline confirm.
- **Mobile**: board full width, player bars above and below, log in a collapsible drawer, emoji bar pinned at the bottom.

### Result
Overlay with the winner and reason (checkmate, timeout, resignation, forfeit, or draw and its type). **Rematch** (colors swap; both must accept) and **Home**. Spectators see the result and a link to another live game.

### Rules `/rules`
RULES.md as a page, with small board diagrams for potholes, the Mamdani, and saving rolls.

### Connection problems
- Own connection: "Reconnecting…" banner; the browser's EventSource reconnects and receives a fresh `state`.
- Opponent disconnected: "Opponent disconnected — they forfeit in 60s" countdown. Their clock keeps running; the game ends at whichever runs out first.

---

## 2. Server

### Packages
| Package | Responsibility |
| --- | --- |
| `rules` | Pure game logic. No I/O, no time, no randomness of its own. |
| `game` | One running game: seats, clocks, dice, subscribers, disconnect timers. |
| `match` | Quick-match queue. |
| `store` | SQLite persistence of games and their move/roll logs. |
| `http` | Routes, guest cookie, SSE, rate limits, static frontend. |

### Concurrency model
Each live game is one goroutine that owns its state. Handlers send it commands (move, react, resign, rematch, subscribe, unsubscribe) over a channel and get replies on a per-command channel. A hub maps game code → game. No shared mutable state, no locks inside game logic.

### API
| Method | Path | Purpose |
| --- | --- | --- |
| `PATCH` | `/api/me` | `{name}` — rename guest (1–20 chars). |
| `POST` | `/api/games` | `{settings?}` — create friend game, returns `{code}`. |
| `GET` | `/api/games` | Live games for the homepage. |
| `GET` | `/api/match` | SSE. Open stream = in queue; close = cancel. Emits `matched {code}`. |
| `GET` | `/api/games/:code/stream` | SSE. Takes the free seat if any, otherwise spectates. |
| `POST` | `/api/games/:code/move` | `{from, to, promo?, seq}`. A Mamdani move uses the Mamdani's square as `from`. |
| `POST` | `/api/games/:code/react` | `{emoji}` |
| `POST` | `/api/games/:code/resign` | — |
| `POST` | `/api/games/:code/rematch` | Offer or accept. On accept, both players get `rematch {code}`. |

### SSE events
- `state` — full snapshot after every change: board, potholes, Mamdani, clocks (remaining ms per side + turn start time + server time), side to move, legal moves for the side to move (sent only to that player), log, result, `seq`. Includes `last`: the ordered events of the latest turn, for animation. Sent immediately on connect, so reconnecting needs no replay.
- `reaction` — `{from, emoji, spectator}`. Ephemeral.
- `presence` — watcher count, opponent connected, forfeit deadline if any.
- `rematch` — offer or new game code.
- Heartbeat comment every 15 s.

### Clocks
- Server stores remaining time per side and when the current turn started; the browser counts down locally from those values.
- After a move, the next player's clock starts after a fixed resolution delay (2 s) so dice animations don't cost time. Increment (+5 s) is added to the mover after their move.
- The game goroutine runs a timer for the side to move; on expiry that side loses on time (or draws if the opponent has insufficient material).
- Time is injected (`Clock` interface) so tests can control it.

### Dice and persistence
- Dice use `crypto/rand`. Every roll is recorded in the log.
- Each turn's move and rolls are written to SQLite before the `state` broadcast.
- On startup, unfinished games are rebuilt by replaying their logs through `rules` with recorded dice. Clocks resume from stored values.

### Matchmaking
One goroutine owns a FIFO queue. It pairs the two longest-waiting guests, skipping a guest matched with themselves (two tabs). Colors are random. Leaving the stream removes the guest from the queue.

### Disconnects
The game counts open streams per player. At zero, a 60 s forfeit timer starts and a `presence` event announces the deadline. Any reconnection cancels it.

### Errors and abuse
- Illegal, out-of-turn, or stale-`seq` moves → `409` with the current `state`; the browser resyncs.
- Unknown game code → `404`; the page shows "Game not found" with a Home link.
- Reactions limited to the fixed emoji set and 1 per 2 s per guest per game; excess → `429`, ignored silently by the UI.
- Request bodies capped at 4 KB.
- Finished games stay viewable for 24 h, then leave memory (kept in SQLite).

---

## 3. Rules engine (`rules`)

### Data
- `Position`: 64 squares; Mamdani square (or none) and the square it last left; open potholes with opener color and close time; side to move; castling rights; en passant square; halfmove clock; fullmove number; position-hash history.
- `Settings`: pothole trigger (default: even on d6), pothole duration in turns (default: until opener's next move), sliders cross potholes (default: no), Mamdani enabled (default: yes). Defaults must match RULES.md.

### Move generation
- 8×8 board with direction tables. Pieces, potholes, and the Mamdani all block sliding lines; knights jump potholes but cannot land on them.
- Mamdani moves: queen lines to empty, non-pothole squares; not back to the square it just left; illegal if they leave the mover's king in check.
- A move is legal only if the mover's king is not in check **after** the move and the turn's close and repair steps.

### Turn resolution
`Apply(pos, move, dice) → (newPos, []Event, error)` where `dice` provides `D6()` and `D8()`. Steps follow RULES.md: move → close → repair → d6 → place (two d8) → resolve. Events: `Moved`, `Captured`, `Promoted`, `PotholeClosed`, `Repaired`, `RolledD6`, `Target`, `Reroll{reason}`, `SavingRoll{roll, saved}`, `Fell{piece}`, `PotholeOpened`.

Re-roll the target square when it is: a king; an open pothole; or a square whose piece falling would leave the player who just moved in check.

### Game end
Evaluated after resolution: checkmate; stalemate (Mamdani moves count as legal moves); 50-move rule (Mamdani moves don't reset, pothole losses do); threefold repetition (hash includes potholes and Mamdani); insufficient material.

### Rule clarifications to add to RULES.md
1. A move is illegal if your king would be in check after your pothole closes and nearby potholes are repaired.
2. If a piece falling would leave the player who just moved in check, re-roll the square.

---

## 4. Frontend

- SvelteKit with `adapter-static`; routes `/`, `/game/[code]`, `/rules`. Go serves the build with SPA fallback.
- `Board.svelte`: CSS 8×8 grid, pointer events for drag and tap-tap, legal-move dots from the server's list, potholes and the Mamdani as their own layers, Svelte transitions for falls, repairs, and captures. No rules logic.
- `DiceTray.svelte` animates `last` events in order; the board waits on it before showing the final position.
- One `gameStream` module wraps `EventSource` and exposes reactive state; actions are `fetch` POSTs.
- Piece art: cburnett SVGs (CC BY-SA 3.0, credited on the Rules page). Mamdani and pothole art are custom (see §6).
- Theme colors are CSS custom properties defined once on `:root`. No color literals in components, so a light theme can be added later.

---

## 5. Testing

- **Perft** with potholes and Mamdani disabled: start position (depth 4 = 197,281) and Kiwipete, to prove standard move generation.
- **Rule tests** with scripted dice, one or more per rule in RULES.md, including both clarifications.
- **Random-game invariants** over ~10,000 games: kings never removed; at most 2 potholes; Mamdani never on a piece or pothole; mover never left in check; replaying the log rebuilds the same position.
- **`game` tests** with a fake clock: timeouts, paused clock during resolution, increment, 60 s disconnect forfeit, rematch.
- **HTTP integration**: two SSE clients quick-match and play a short game end to end.
- **Playwright smoke test**: two browsers match and play a few moves.

---

## 6. Visual design

Design canvas: https://claude.ai/artifact/XVJqVp283CoZEHpmHDrSih. Every artboard on it is the reference: Home, Quick match, Play a friend (house rules), Friend game waiting, Game, Game over, Rules, Home (phone), Game (phone), plus the shared Board and Header components.

### Direction
"Road works": the game is about potholes, so the UI looks like a street at night. Asphalt-dark surfaces, road-marking yellow for actions and highlights, condensed signage-style headings.

### Theme
Dark only for the demo. Colors live in CSS custom properties so a light theme can be added later without touching components.

### Tokens
| Token | Value | Use |
| --- | --- | --- |
| `--bg` | `#141518` | Page |
| `--surface` | `#1E2024` | Cards, panels |
| `--surface-2` | `#2A2D32` | Buttons, inputs, raised elements |
| `--line` | `#3A3E44` | Borders, dividers |
| `--text` | `#F1EEE6` | Primary text |
| `--text-body` | `#C9C5BB` | Paragraphs |
| `--text-muted` | `#A8A49B` | Labels, meta |
| `--accent` | `#F2C230` | Primary buttons, highlights, active clock, saving roll (dark `#141518` text on it) |
| `--hazard` | `#FF7A3D` | Pothole events in the log and dice tray |
| `--board-light` / `--board-dark` | `#D9D3C4` / `#857E72` | Board squares |
| `--board-last` light / dark | `#E9D98B` / `#B5A24F` | Last-move squares |
| `--target-ring` | `#F2C230` | Dashed ring on the square the dice picked |

### Type
Barlow Condensed (700–800, uppercase) for headings and big buttons; IBM Plex Sans for body text; IBM Plex Mono for clocks, codes, dice values and move numbers.

### Signature elements
- **Pothole**: an irregular near-black hole with a deep inner shadow and a `--hazard` orange ring. Static.
- **Target square**: dashed yellow ring while the dice resolve.
- **Dice tray**: d6 as a pip die, d8s as diamonds, the saving roll highlighted in yellow; the clock shows "Paused for dice" meanwhile.
- **Spectator strip**: a narrow column beside the board where audience reactions float up, with the watcher count at the bottom.
- **The Mamdani**: a round token with a yellow ring. The current photo is a placeholder; the shipped game needs art we have rights to (illustration recommended). The traffic cone is the logo mark.

### Accessibility
Text contrast ≥ 4.5:1 (yellow is a fill, never small text on light). Touch targets ≥ 44 px. Emoji reactions have `aria-label`s.

## Next steps

1. Implementation plan.
