# Milestone 06c: Longer potholes — Design

Date: 2026-10-05
Status: Built and deployed 2026-10-05 (plan: [2026-10-05-06c-longer-potholes](../plans/2026-10-05-06c-longer-potholes.md))
Builds on: [the main design spec](2026-10-01-mamdani-chess-design.md), [RULES.md](../../../RULES.md) (live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB) and [the roadmap](../plans/2026-10-03-00-roadmap.md). Comes before milestone 07 (launch).

## Goal

Potholes close too fast: today each one closes after its roller's next move, so about one is open at a time and the board barely changes shape. This milestone makes potholes last 3 rounds with at most 5 open, lets a roll deliver checkmate, and makes the Mamdani's repairs an event on the board.

**Done when:** a whole game played through the UI shows holes counting down with cones, the cap closing the oldest hole, a repair celebration, and (in a scripted test) a mate by roll; the live rules doc, `RULES.md`, the engine, the server and the board all agree.

## Decisions

Taken with the user on 2026-10-05.

| Topic | Decision | Why |
| --- | --- | --- |
| How long a hole lasts (#4) | 3 rounds: it closes when its roller finishes their third move after opening it. | Playing in the sandbox, holes closed before they mattered. Three rounds triples the open holes without filling the board. |
| How many (#10) | At most 5 open. Opening a 6th closes the oldest at that moment. | A safety net, not the usual limit: in simulation it closes about one hole per game. The alternative, "holes last until the cap", keeps 5 open almost all the time and makes a hole vanish far away on nearly every roll, the moving hazard the rules doc says felt unfair. |
| A roll that would checkmate (#11) | No re-roll: a roll can deliver checkmate. Kings still never fall (#6). | The user's call. It almost never happens (about 1 game in 1,000 in simulation) and removes a re-roll case from the engine. Goals 2 and 3 are reworded to match. |
| Check held off only by your own hole (#16) | Still checkmate, but only for your holes on their last round. | Only those close on your next move. |
| Repetition | The holes' rounds left (and rollers) must match too. | Two positions with the same holes but different countdowns have different futures. The old key already told the holes' rollers apart. |
| The cap and the roller's king | If the cap closing the oldest hole would leave the roller in check, the target is re-rolled, as for a fall that would (the "exposes" re-roll). | Found while building: that hole may be the only thing shielding the roller's king, and a player must never end their own turn in check. |
| Games saved under the old rules | Retired, not replayed (option B). | Simpler than versioning the engine. Only playtest games exist. Milestone 08's history starts with the new rules. |
| Rounds-left display | One small traffic cone per round left, standing on the hole's front edge; the last one blinks. No owner marker on the board. | Tried in the sandbox: lit pips inside the hole, then cones outside it. The cones read at phone size and match the repair cone. An owner tab on the ring was tried and dropped because it didn't explain itself; ownership is in the tooltip and screen-reader label. |
| Repair celebration | When the Mamdani repairs a hole: a cone drops on it, the hole shrinks, a flash and sparks burst, and 👍 pops above the Mamdani (about 1.3 s). | Repairs nearly double (9 → 16 a game in simulation), and today a repair looks exactly like an ordinary close. |
| Where it lands | Before milestone 07. | Friends playtest the launch, so they should play the rules the user wants. |

## Simulation

A throwaway copy of `rules/` with three switches (rounds a hole lasts, the cap, the mate re-roll), checked first to play exactly like the real engine on 300 seeded games under today's rules. 2,000 games per variant. "Greedy" movers take a mate in one, else the biggest capture, else a random move.

| Per game, greedy movers | Today | **3 rounds, cap 5, rolls may mate** | Until cap 5 |
| --- | --- | --- | --- |
| Holes open on average | 0.9 | **2.5** | 4.5 |
| Moves with 5 holes open | 0% | **5%** | 73% |
| Holes closed by the cap | 0 | **~1** | 46 |
| The Mamdani's repairs | 9 | **16** | 27 |
| Pieces lost to potholes | 8.8 | **9.0** | 9.5 |
| Games won by a roll | 0% | **0.1%** | 0.2% |
| Moves per game / decisive | 80 / 16% | **80 / 16%** | 79 / 17% |

Random movers show the same pattern. Two findings to carry into the playtest:
- Longer holes don't claim more pieces: falls come from where holes open, not how long they stay. If games need more falls, that's a different knob (how often a hole opens).
- About 77% of bot games end with too little material to mate. Partly the bots (greedy captures drain the board), but with about 9 falls a game it's worth watching whether real games peter out.

The bots don't plan around holes, so how much longer holes shape play can only be judged in the playtest.

## Rules

| Rule | Today | New |
| --- | --- | --- |
| #4 Lifetime | Closes when its roller finishes their next move | Closes when its roller finishes their third move after opening it |
| #10 Count | One per even roll; at most 2 open (one each) | One per even roll; at most 5 open. When a hole actually opens with 5 already open, the oldest closes first. A roll that is saved, repaired or re-rolled away opens nothing, so nothing closes. |
| #11 Would checkmate | Re-roll | No re-roll |
| #16 Own hole holds off check | Checkmate (every move closes it) | Checkmate when only your holes on their last round hold it off |
| Repetition | Pieces, Mamdani, holes | Also each hole's rounds left |

Unchanged: repairs next to the Mamdani (after every move and the moment a hole opens there), saving rolls, re-rolls for kings and for "already a pothole", the "exposes the roller's king" re-roll (which now also covers the cap closing a hole that shields the roller's king), the 64 re-roll cap, blocking (sliders can't cross, knights jump, no castling through, no double step across, en passant cancelled). If the Mamdani falls, holes still close on schedule.

Turn order in RULES.md becomes: 1 Move. 2 Count down: each of your holes loses a round; any at zero closes. 3 Repair. 4 Roll for a pothole. 5 Place it; if it opens with 5 already open, the oldest closes first.

### Rules doc edits

The live doc is the source of truth; edit it first, then sync `RULES.md`.
- **Goal 2:** "Luck finishes, skill sets up. A roll can deliver mate, but kings never fall, so it only finishes a king that play has already cornered."
- **Goal 3:** "Most bad rolls have an answer: block, capture, or send the Mamdani."
- **Decision table:** rows #4, #10, #11, #16 rewritten as above, each with its reason ("playtest in the sandbox: holes closed too fast; simulation 2026-10-05: about 2.5 open, the cap closes about one a game, mates by roll about 1 game in 1,000").
- **Setup:** "A few checkers" becomes "Up to 5 markers for open potholes, and something to count rounds (three cones or coins each)".
- **Potholes:** "Would checkmate the next player" re-roll removed; lifetime, cap and countdown text added.
- **Winning and draws:** checkmate paragraph says a roll can now deliver mate; repetition includes rounds left.
- **Watch in test games:** add "Do games peter out?" (too little material left to mate).

## Engine (`rules/`)

- `Position.Potholes` becomes `[HoleCap]Hole`, with `Hole{Sq, By, Left, Seq}` (`Sq` is `NoSquare` for a free slot; `Seq` is the opening order). Constants `HoleRounds = 3`, `HoleCap = 5`. No settings struct: old games are retired, not replayed.
- `play`: after the move, each of the mover's holes loses a round; at zero it closes (`PotholeClosed`). Then `repair` as today, over all holes.
- `resolve`: opening with 5 open first closes the lowest `Seq` (`PotholeClosed` with `Color` = its roller), then `PotholeOpened`. A roll's events read `target → pothole_closed → pothole_opened`.
- `rerollReason`: the checkmate test and `ReasonCheckmate` go. After the roll, the game's status sees a mated side to move as checkmate, as for any move.
- `threatened` (#16): ignores only the side to move's holes with one round left.
- `Key`: holes in square order with their rounds left.
- `ParseFEN` and `StartPosition` start with no holes.
- The scratch simulation (see Prototype) is the starting point; it already handles all of the above.

Tests:
- Random games: at most 5 holes; every hole has 1 to 3 rounds left; a hole closes exactly after its roller's third move unless repaired or capped; replay still rebuilds every game.
- Scripted dice: the cap closes the oldest; a roll mates (a hole on the last escape square, and a blocking piece falling); #16 with a last-round hole and with a two-round hole; repetition differs by rounds left.
- Perft unchanged.
- Delete the tests pinning the old behaviour (one hole per player, the checkmate re-roll).

## Saved games and server

- **Migration 5:**
  ```sql
  ALTER TABLE games ADD COLUMN rules INTEGER NOT NULL DEFAULT 1;
  UPDATE games SET ended_at = CAST(strftime('%s','now') AS INTEGER) * 1000, result = 'retired'
   WHERE ended_at IS NULL;
  ```
  New games are inserted with `rules = 2`.
- `LoadForRestore` loads only `rules = 2`. A link to an old game gets the existing "Game not found" page.
- `retired` joins the result reasons the server knows; nothing displays it, since retired games are never loaded.
- JSON: every pothole (game view and live games list) gains `"left": 1|2|3`; `pothole_closed` gains `color`. No other protocol change.
- The `server_rs/` experiment is not updated (as in 06a).
- **Deploy:** one push to `main`. It ends any game in progress under the old rules, so not during a playtest.

## Board and text (`web/`)

**Board (`Board.svelte`)**
- Cones: one per round left on the hole's front edge (about 32% of the hole's width each); the last blinks (still under reduced motion). When a round passes, the cone that goes lifts and fades (about 300 ms).
- Cap close: `stageAt` keeps a hole closed by the cap on the board until its `pothole_closed` event is revealed, then it shrinks quietly, in step with the dice.
- Repair celebration as prototyped: cone drops (250 ms) → hole shrinks under it → cone lifts → flash and sparks → 👍 pops and wiggles above the Mamdani, below it on the top screen row. A repair by the Mamdani's move waits for its glide. Each plays to the end (about 2.4 s) even after the next move arrives (`out:linger`). Reduced motion: no cone, flash or sparks; a still 👍 for about 1 s. Instant mode: none.
- Celebrations come from `repaired` events revealed so far, keyed by `seq` and event index, and only when the turn was animated: the animator gains an `animated` flag, so a reload or reconnect never replays one.
- An ordinary close keeps today's quiet shrink, so the celebration only ever means "the Mamdani fixed it".
- Pothole squares: tooltip and `aria-label` "c4, pothole (White's, 2 rounds left)", or "…closes after their next move" on the last round.
- `MiniBoard` (home hero, live games): plain holes; cones don't survive at 26 px.

**Text**

| Where | Today | New |
| --- | --- | --- |
| Dice tray, hole opens | "It closes when White finishes their next move" | "It closes after 3 of White's moves" |
| Dice tray, cap | — | "At most 5 potholes: the oldest, on c4, closes" |
| Re-roll reasons | includes "the result would checkmate" | removed |
| Move log row | dice only | adds "repairs f6" for a repair by the Mamdani's move, and "c4 closes" for the cap |
| Result card, mate by roll | "Checkmate" | "Checkmate · by a pothole" when the roll delivered it |

**Sandbox (`/dev/board`)**: mirrors the rules (`HOLE_ROUNDS`, `HOLE_CAP` in `sandbox.ts`); a Random/Square choice for the target (Random throws two d8s and re-rolls past kings and open holes, as the server does); a typed target on a king re-rolls; a Repair preset. Its tests are rewritten for 3 rounds and the cap.

**Rules page (milestone 07)** is written from the new rules, including "each cone is a round left".

## Prototype

Built during the design, uncommitted in the working tree on 2026-10-05; the plan starts from these files:
- `web/src/lib/Board.svelte`: cones, the repair celebration (`repairs` prop, `born`/`linger`, `.fix` styles), pothole tooltips.
- `web/src/lib/sandbox.ts`, `web/src/routes/dev/board/+page.svelte`: 3-round holes with the cap, the Repair preset, the Random/Square target, king re-rolls.
- `web/src/lib/game.ts`: `left` on potholes.
- The simulation: `rules/` copied with `Life`, `Cap` and `MateReroll` switches, an equivalence test against the real engine, and the variant runs (in the session scratchpad, not the repo).

The same working tree also holds the Mamdani art on both boards (`/mamdani/piece.webp` as a rounded square), which belongs to milestone 07.

## Testing

- `go test -race -short ./...`, plus the full random-game suite once.
- `pnpm --dir web check`, `pnpm --dir web test`: `stageAt` with a cap close, the new text, a celebration that doesn't replay on reload.
- `pnpm --dir web playtest` in Chromium and WebKit, and `--phone`: whole games now carry about 2.5 holes.
- Cones and the celebration checked at phone size.

## Out of scope

- Changing how often a hole opens (falls per game).
- A custom page for retired games.
- Cones on the small boards.
- Sound.
- Updating `server_rs/`.
