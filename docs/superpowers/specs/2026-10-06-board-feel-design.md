# Board Feel: Design

**Status:** design agreed item by item on 2026-10-05 and 06. Not built.
**Mockups:** https://claude.ai/artifact/E6oCYQhp8xaDTZCVssczXy (live, looping; its sections are numbered like this spec, and every board there is the agreed board: outside coordinates, mpchess with the white outline, blue last move, ease-out glide).
**Why:** the board should call attention to what the dice and the moves do, and feel as lively as chess.com's Arcade style, in our road-works look. Every effect here runs on every turn or at a big moment, so each one is short, and each one is skipped under reduced motion and in instant mode (`setInstant`).

## What is decided

| # | Change | Mockup |
| --- | --- | --- |
| 1 | Coordinates outside the board | 1 |
| 2 | mpchess pieces, 10% larger, with a white outline instead of a shadow | 2 |
| 3 | Ease-out glide, with a trail and a whiplash | 3 |
| 4 | Capture knock-back | 4 |
| 5 | Falling into a pothole, and the save | 5 |
| 6 | Checkmate burst | 6 |
| 7 | Win screen: result tally and confetti | 7 |
| 8 | Catchphrase bubbles (milestone 09) | 8 |
| 9 | Dice: a tray throw echoed on the board, the pothole scan riding on out-of-sync file and rank dice | 9 |
| 10 | Pick-up jiggle and tilt, legal moves ripple in | 10 |
| 11 | Board shake by size | 11 |
| 12 | +5 on the clock | 12 |

Already shipped on 2026-10-05: the last move is light blue (`--board-last-*`), so it never reads as a legal move (yellow).

## 1. Coordinates outside the board

- Rank numbers in a column left of the board, file letters in a row under it, each centred on its square: IBM Plex Mono, 12 px, `--text-muted`, on the page background. The squares carry no labels.
- They follow `flipped`: Black reads 8 to 1 down and h to a across.
- On a phone (under 640 px) the board gives up about 14 px of width for them; the page must still never scroll (`playtest --phone`).
- The dice light them: during a pothole roll, the letter and number the file and rank dice point to glow orange (section 9).

## 2. Pieces: mpchess, larger, with a white outline

- **Set:** lichess's mpchess by Maxime Chupin, GPLv3+, from `lila/public/piece/mpchess`. It replaces cburnett everywhere pieces are drawn: the board, lost pieces in the player bar, the home page's small boards (`MiniBoard.svelte`), the promotion picker, and the link preview card (`pnpm --dir web og-image` redraws it).
- **License:** `web/static/pieces/LICENSE.txt` becomes mpchess's notice (author, GPLv3+, source link); the Rules page (milestone 07) credits it. cburnett's BSD notice goes with its files.
- **Size:** drawn at 110% of today's piece size (`.piece img` from 92% to about 100% of the square), since mpchess has more padding in its viewBox; pawns must still clear the square's edges.
- **Outline, no shadow:** every piece's `drop-shadow(0 2px 2px)` becomes a crisp cream outline, like a sticker's white border, not a glow: four unblurred drop-shadows of the piece's own shape, offset 1.5 px right, left, down and up (`drop-shadow(1.5px 0 0 var(--piece-outline))` and so on), with a new `--piece-outline` token (`--piece-light`, #fbf8f0). The width scales down where pieces are small (lost pieces, home page boards: 1 px). The lost pieces' soft edge (shipped 2026-10-05) becomes the same outline. A lifted or dragged piece still grows 12%. The Mamdani tile keeps its yellow border and gets no edge.

## 3. Glide, trail and whiplash

- **Glide:** easing changes from easeInOutQuad `cubic-bezier(0.455, 0.03, 0.515, 0.955)` to easeOutQuart `cubic-bezier(0.25, 1, 0.5, 1)`: a piece leaves fast and settles gently. Durations stay 350 to 750 ms by distance (`moveDuration`). This leaves plan 04b's One Million Chessboards curve on purpose.
- **Trail:** every gliding piece (both players' moves, castling's rook, the Mamdani) draws a streak from its start square to where it is: 55% of a square wide, cream for White, charcoal for Black, soft yellow for the Mamdani; transparent at the start, solid at the piece. It grows with the glide's easing and duration, so its head stays under the piece, and fades 180 ms after landing. A dragged piece has no trail. Trails come from the board's list of moving pieces (`pieces`, which knows each one's previous square), with no timers.
- **Whiplash:** the piece pivots from its base (`transform-origin: 50% 85%`): it leans back as it launches (−0.6w at 12%), whips forward along the move as it brakes (+w at 62%), swings back a touch (−0.3w at 84%) and settles, finishing 160 ms after the glide. `w` is 5° times the move's sideways share (dx / distance), so a vertical move barely tilts.

## 4. Capture knock-back

The captured piece stays until the mover arrives (today it fades at once), then is knocked a third of a square along the move, spins 22°, shrinks to 85% and fades over 320 ms, while an orange ring (`--hazard`) flashes on the square and the board shakes by the captured piece's size (section 11). En passant knocks the pawn behind.

## 5. Falling into a pothole, and the save

- **Fall:** the hole cracks open under the piece, which stays on top; it teeters (two wobbles, 9°, 180 ms), then drops in over 370 ms: down 10%, scale to 15%, tip 40°, brightness to 25%, fade; dust puffs off the rim and the board shakes (section 11); the cones pop into place. The Mamdani falls the same way.
- **Saved by an odd saving roll** (the hole never opens): the hole cracks open, the piece teeters harder (14°, 260 ms) and hops out (up 22%, 380 ms) as the hole closes under it.
- Both fit the 550 ms dice step they play in (`STEP_MS`), so the clocks' pause (`pauseFor`) needs no change.

## 6. Checkmate burst

When a game ends in checkmate and the turn played out on screen (not a reload or reconnect): the mated king's square fades to red in two steps (45%, then 85%, about 0.5 s); 18 white and orange shards and two tumbling traffic cones burst across the board with the biggest shake (section 11) and fade over about 1 s; a red "#" badge pops onto the king's square. A mate by a pothole roll bursts once the dice stop. It plays once per game, like the repair celebration, and reuses its spark code.

## 7. Win screen: result tally and confetti

- **Tally (everyone):** the result card flips in (420 ms, a slight overshoot), then the game's numbers count up one at a time, each landing with a yellow pop, 150 ms apart: moves, saving rolls, saved by the Mamdani, potholes repaired, pieces lost to potholes. The numbers come from the view the game already sends (`stats`, `lost`, `seq`).
- **Confetti (the winner only):** as the card appears, confetti falls over the screen for about 2.5 s: about 80 strips in `--accent`, `--hazard` and cream, a few of them tiny traffic cones. Never for a draw, a loss or a spectator. One canvas over the page, removed when done.

## 8. Catchphrase bubbles (milestone 09)

- When the dice finish playing out a big moment, a speech bubble pops up from that square (cream, dark text, a tail pointing at the square) with an emoji and a line, stays about 2.8 s and fades. It stays inside the board's bounds near the edges. The same line goes to the toast region's live text for screen readers (`notify`), without a visible toast.
- Players and spectators alike see it; a reload doesn't replay it.
- Lines live in `web/src/lib/catchphrases.ts`, keyed by event, picked at random, never the same line twice in a row:

| Event | Emoji | Lines |
| --- | --- | --- |
| The Mamdani falls in | 😢 | "Sorry, my wife is calling." · "Got paperwork to do." · "Ask Batman for help." · "Your friendly neighborhood Spidey can handle this." |
| The Mamdani repairs a pothole | 👍 | "Filled. Next!" · "Another one off the list." |
| A queen falls in | 😱 | "Not the queen!" · "Mind the gap." |
| A saving roll saves a queen or rook | 😅 | "That was close." |

- Lines are the game piece's playful voice, never presented as real quotes (the roadmap's caution for a piece named after a real person). Text only, no sound.

## 9. Dice: a tray throw echoed on the board

Chosen on 2026-10-06 over rolling the dice along the coordinate rails (mockup 8).

- **The tray stays:** beside the board on desktop, across the top on a phone. Its dice are really thrown: each flies in spinning, bounces twice and settles in 450 ms, tumbling faces as it goes. The tray keeps its heading ("Black's roll") and a line saying what the roll means.
- **Colors say what each die decides.** The first die says whether a pothole opens: grey when odd, **road-works yellow** (`--accent`) when even. The file and rank dice say where: **hazard orange** (`--hazard`), like the scan and the hole's ring. A saving die is **cream** with dark text.
- **The board echoes it:** a pill on the board's top corner, in the first die's color. Odd roll: "d8 3 · no pothole", grey, fading after about 0.6 s (about 0.7 s in all). Even roll: "d8 6 · pothole", yellow; when the file and rank dice land it turns orange and shows the target square ("d5").
- **The pothole scan rides on the second throw, out of sync:** the file die is thrown, and the rank die 150 ms later. They tumble independently, each slowing down, and the orange scan square on the board always sits where their current faces point, with the matching letter and number lit on the outside coordinates. The file die settles first (after about 0.7 s) and locks its column; the scan then runs up and down that column until the rank die settles (about 0.3 s later), and it is already on the target. The hole cracks open. About 1.8 s in all; the scan and the second throw are one wait, not two.
- **Re-rolls** (a king, an existing hole, exposing the roller's king): the target blinks twice, and the file and rank dice are thrown again the same way. **A saving roll** is one more die thrown into the tray, with the board pill naming the piece and the square.

**Timing to check while planning:** the server pauses the next clock with `pauseFor` (`game/clock.go`), computed from one `StepTime` (550 ms) per event after the roll, at least 2 s. The new dice pacing is per kind of step, not uniform, so the plan must make the client's per-step durations and `pauseFor` come from one table (or keep the client within it), and test that a long roll (re-rolls plus a saving roll) never lets the next clock start mid-animation.

## 10. Pick-up jiggle and tilt, legal moves ripple in

- **Lift:** a selected piece grows 12% with a quick wobble (−6°, +5°, upright over 220 ms).
- **Drag tilt:** while dragged, it tilts toward the pull, up to 10°, from the pointer's sideways speed, easing back on release.
- **Ripple:** its legal squares pop in (scale 0 to 112% to 100%, 200 ms) in order of distance from the piece, 15 ms per step, instead of all at once.
- **Land:** a drop lands with a squash (108% × 92%, 140 ms); a drop on an illegal square settles back the same way.

## 11. Board shake by size

The whole board (frame and coordinates) shakes by the size of the moment, in a decaying zigzag: a pawn or minor piece taken, 1.5 px for 140 ms; a queen or rook taken, 3 px for 200 ms; a piece falling into a hole, 4.5 px for 280 ms; checkmate, 7 px for 420 ms. One shake at a time; the bigger wins.

## 12. +5 on the clock

When a player moves, their clock stops, a "+5" in `--accent` floats up from it (600 ms), and the time ticks up one second at a time into place (300 ms), with a small bump. Both players see it on the mover's clock.

## Everywhere

- Reduced motion and instant mode skip every effect here and show end states.
- A turn shown at once (first load, reconnect, a hidden tab) plays no one-off effect (burst, confetti, bubble), as today's repair celebration (`Animator.animated`).
- The `/dev/board` sandbox shares the board and animator, so it shows all of this for testing.

## Not in this design

- chess.com's move badges ("??", "!!"): they need an engine.
- Sound.
- chess.com's own piece art (theirs); mpchess is the open stand-in.

## Testing

- Unit tests: trail geometry (angle and length, flipped board), whiplash amplitude by direction, ripple order by distance, the shake picking the biggest event of a turn, the scan path ending on the target and never on it before the end, the catchphrase picker never repeating, confetti only for the winner, the burst only on an animated checkmate.
- Go test, if timing changes: `pauseFor` covers the new dice pacing for the longest roll.
- Before/after screenshots from one script per change, and a short GIF of each effect.
- Playtest with full animations: Chromium, WebKit and `--phone`; and a reduced-motion pass.

## Suggested build order

1. Coordinates outside (1).
2. mpchess, size, outline (2).
3. Glide curve, trail, whiplash (3).
4. Pick-up jiggle, tilt and ripple (10).
5. Board shake (11), then knock-back (4).
6. Fall and save (5).
7. Catchphrase bubbles (8).
8. Checkmate burst (6) and the win screen (7).
9. +5 on the clock (12).
10. Dice and scan (9).
