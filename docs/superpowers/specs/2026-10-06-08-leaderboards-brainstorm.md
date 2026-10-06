# Milestone 08: leaderboards and the player page — brainstorm notes

**Status:** brainstorming, paused 2026-10-06. Not a spec yet. The decisions below are agreed; section 1 of the design is drafted and waiting for approval. Resume at "Where to pick up".

**Also read:** the roadmap's "Leaderboards: open questions" (`docs/superpowers/plans/2026-10-03-00-roadmap.md`): weekly titles, the design pass and the fun-fact table with sources. Achievements (milestone 09) are in the roadmap's "Achievements: brainstorm".

**Sketches:** `2026-10-06-08-leaderboards-sketches.html` in this folder (open it in a browser), also published at https://claude.ai/artifact/MMUKTL7M9pUKLD8PasthPF. Every screen is drawn on desktop and on a 390 px phone.

## Agreed

- **Five weekly titles:** GOAT (most wins), Mayor (most potholes repaired by your own Mamdani moves), Capitalist (most pieces taken per game, at least 5 games), Pothole Magnet (most pieces lost to potholes), Luckiest (best saving-roll rate, at least 20 rolls).
- **Titles are live:** whoever leads right now holds the title, so it can change hands mid-week, even mid-game. Weeks reset Monday 00:00 New York time; titles are vacant until someone is ranked. The leader at each reset is recorded as that week's winner.
- **Which games count:** quick match and friend games, but a guest is ranked only after playing 3 different opponents that week.
- **Leaderboard page:** direction A, the weekly list (Duolingo leagues + Strava Local Legend): title tabs that fit across a phone, the holder and time left, a chart of where everyone stands (hidden until about 20 players are ranked), the top five under a "title line", and your row among your neighbours. Directions B (chess.com lists) and C (your week first) were not chosen; C's race card moves to the game-over screen.
- **Also in 08:** a title badge on a holder's player bar, the race card on the game-over screen, the player's history, last week's winners.
- **Player page:** one per player, opened from any name: stats, "By the numbers" fun facts, this week's ranks, titles won, recent games. Milestones and feats join it in milestone 09. The Pothole Atlas was dropped.
- **Fun facts:** real, sourced NYC numbers next to the player's own (table in the roadmap). One about the game at every game end, wins and losses alike; four on the player page; one per title tab on the leaderboard; a city fact while waiting for a quick match. Never a goal, never the same fact twice in a row.
- **Build plan:** split into **08a** (per-player game rows, backfill, player page with history and fun facts) and **08b** (weekly list, live titles, badges, race card, last week's winners). Standings are added up on read: one query over 08a's rows, cached in memory and recomputed when a game ends; the page refreshes on load and on tab focus, no live stream. One spec for all of 08, then a plan each.
- **Working rule:** every design shows a phone view beside desktop; most players are on phones.

## Design section 1 (drafted, not yet approved): one row per player per game

When a game ends, the transaction that saves the result writes two rows, one per player, to a new `game_players` table. Old games get theirs by replaying their saved turns through `rules`. Every count comes from `rules/event.go` events:

| Column | Counted as |
| --- | --- |
| `guest`, `color`, `name` | The player and the name they sat down with (already saved on the seat) |
| `opponent`, `opponent_name` | The other seat; the "3 different opponents" rule counts these |
| `outcome`, `reason` | Win, loss or draw, and how it ended (checkmate, resignation, time, stalemate, repetition, 50-move rule). Games aborted before the first moves get no rows |
| `moves` | This player's moves |
| `captures` | `Captured` events on this player's turns, en passant included |
| `lost` | `Fell` events for this player's pieces; the Mamdani is nobody's piece |
| `opened` | `PotholeOpened` events this player rolled; a saved piece means no pothole opened |
| `repairs` | `Repaired` events on turns where this player moved the Mamdani, and only those before the pothole roll; a pothole that opens next to the Mamdani is luck and doesn't count |
| `repair_wait` | For those repairs, the total moves each pothole had been open (feeds "fixed within 1 turn; DOT takes 2 days") |
| `saving_rolls`, `saved` | `SavingRoll` events for this player's own pieces; Mamdani rolls excluded, as Luckiest requires |
| `mamdani_squares` | Squares the Mamdani travelled on this player's moves |
| `clock_ms` | Time this player spent thinking, from the clock values saved with each turn |
| `ended_at` | When the game ended; weeks are measured from it |

Indexes on `(guest, ended_at)` and `(ended_at)`. Achievement-only facts (a queen swallowed, a mate that needs the Mamdani) stay out; milestone 09 can replay games again for them. Open point: draws by agreement don't exist, so `reason` lists only today's endings.

## Where to pick up

1. Get section 1 approved (or changed).
2. Present the remaining sections one at a time, each with a phone view where it has UI:
   - 2: backfill (replaying saved games, idempotent, run at startup or as a command);
   - 3: the player page and its API, fun-fact selection ("most striking", no repeats, sources);
   - 4: weekly standings (the query, the 3-opponent rule, ties, the in-memory cache, recording each week's winners at the reset);
   - 5: the leaderboard page, badges on player bars, the race card and game-end fact on the result screen (sketch the result screen on phone and desktop);
   - 6: errors, edge cases (renamed guests, vacant titles, a guest with no games) and testing.
3. Write the spec to `docs/superpowers/specs/2026-10-06-08-leaderboards-design.md`, self-review it, ask for review, then write the 08a plan.

## Open questions

- Ties on a title: whoever reached the number first? (Proposed in conversation, not agreed.)
- Hide the leaderboard chart below about 20 ranked players, and drop the "···" gap when everyone fits on one screen (proposed).
- Milestone 09: adopt the Civic Rating ladder or not (roadmap).
