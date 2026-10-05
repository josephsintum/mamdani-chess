# Phone Layout for the Game Screen: Design

**Status:** approved design (canvas reviewed 2026-10-05), ready for a plan.
**Visual reference:** the canvas row "Phone game: playtest build" (https://claude.ai/artifact/XVJqVp283CoZEHpmHDrSih): artboards `PhonePlaying`, `PhoneDice`, `PhoneMoves`, `PhoneResign`, `PhoneWaiting`, `PhoneOver`, each 390×664.

## Why

The friends playtest will happen on phones. Today, on an iPhone 15 in Safari (393×659 visible), the game screen is a desktop column squeezed narrow:
- the title and status line take the top 163 px;
- the board is 345 px wide;
- the dice result, the heart of this variant, sits below the fold;
- the page is 1,193 px tall and scrolls.

## Decisions taken with the user (2026-10-05)

- **Scope: the game screen only**, in every state: waiting, playing, dice playing out, promotion, resign, game over. Home is rebuilt in milestone 06; the sandbox stays desktop-first.
- **Compact dice card** on phones: one row of chips that fill in as the dice play, plus one outcome line. The step-by-step explanation moves into the move log.
- **Bottom bar plus a sheet:** a bar pinned to the bottom holds "Moves and rolls" and "Resign". "Moves and rolls" opens a sheet with the full log.
- **Designed on the canvas first**, and approved.
- **Out of scope:** clocks (05), spectator count and emoji reactions (06). The future "Game (phone)" artboard keeps them; this design leaves their place empty.

## Success

On a phone in portrait (viewport width under 640 px), in Safari and Chrome:
1. **No scrolling.** Board, both player bars, the latest dice result and the actions fit in the visible area, down to 390×664. Shorter screens shrink the board rather than scroll.
2. **A near-full-width board:** `min(100vw − 16px, the height left over)`, so 374 px on a 390 px phone.
3. **Every state of the six artboards is reachable and matches them.**
4. **Desktop and tablet (640 px and wider) are unchanged** apart from the player-bar pills, which replace the "To move" badge on every layout.

## Layout

At widths under 640 px the game page renders the phone layout: a column filling `100dvh` (the visible height, which follows Safari's toolbars). From top to bottom:

| Part | Height | Contents |
| --- | --- | --- |
| Header | 48 | "Pothole Chess" (20 px display font, links home), the game code, a copy-link icon button (44×44). "Reconnecting…" appears as a chip here. |
| Opponent bar | 44 | `PlayerBar` with a pill. |
| Board | min(100vw − 16px, rest) | `Board`, unchanged, 8 px side gutters. |
| Your bar | 44 | `PlayerBar` with a pill. |
| Card slot | about 81 | Exactly one card (see below). |
| Bottom bar | 65 + safe area | Two 44 px buttons; pinned to the bottom; `padding-bottom: max(12px, env(safe-area-inset-bottom))`. |

The board's size comes from CSS: `--board: min(calc(100vw - 16px), calc(100dvh - var(--chrome)))`, where `--chrome` is the sum of the other rows (about 286 px with the bottom bar, about 245 px without it). It never scrolls; on very short screens (under about 560 px tall) it shrinks below 280 px, which is accepted.

### The card slot

One card at a time, in this priority:
1. **Waiting** (`status === 'waiting'`, you are White): the share box, with the link field and "Copy link". The bottom bar is hidden.
2. **Resign confirmation** (`confirmResign`): "Resign this game? *White wins.*" with "Keep playing" and "Yes, resign". The bottom bar is hidden.
3. **Game over** (`resultCard`): the headline ("YOU WIN", "WHITE WINS", "DRAW") with the kicker on the same row ("CHECKMATE · MOVE 2"), and one detail line.
4. **Error** (`error` set): the error text, `role="alert"`, in hazard colours, until the next view clears it.
5. **Otherwise, the dice card** (`DiceSummary`).

### The bottom bar

- **While playing:** "Moves and rolls" (filled) and "Resign" (outline). Spectators see "Moves and rolls" alone, full width.
- **Game over:** "New game" (primary) and "Moves and rolls".

### Player bars

`PlayerBar` gains a `pill` (text or `''`) and a `pillTone` (`turn`, `check`, `muted`). On every layout it replaces today's "To move" badge.

| State | Pill (tone) |
| --- | --- |
| Your turn | "Your move" (turn) |
| Opponent's turn | "To move" on their bar (turn) |
| Your turn and you're in check | "In check" (check) |
| Checkmated, at game end | "Checkmated" (check) |
| Waiting for Black | "Waiting…" on Black's bar (muted) |
| Dice playing out | none |

Lost pieces show as small piece images after the name, with `alt` text from `pieceName()`. On phones the "Lost to potholes: none" text is gone: no pieces lost means nothing shown. The desktop wording stays.

### Status line

On phones the visible status line ("Your move", "Dice are rolling…") is removed; the pills and the dice card carry it. The same text stays in a visually hidden `aria-live="polite"` element, so screen readers still hear each change.

## The dice card: `diceSummary()`

A pure function in `web/src/lib/board.ts`, unit-tested with Vitest:

```ts
export interface DiceChip { text: string; kind: 'die' | 'square' | 'pending' | 'good' | 'bad' | 'plain' }
export interface DiceSummary {
	who: Color | null;          // whose roll; null before the first roll
	chips: DiceChip[];          // left to right, as far as the dice have played
	line: string;               // one outcome sentence
	tone: 'normal' | 'good' | 'hazard' | 'muted';
}
export function diceSummary(view: View, shown: number): DiceSummary
```

It reads the same events as `diceSteps()`, up to `shown`, so the chips fill in step by step in time with the animation. The outcomes:

| Turn | Chips | Line |
| --- | --- | --- |
| No roll yet | none | "The dice roll after every move." (muted) |
| Odd roll | `◇3` | "Odd: nothing happens" (muted) |
| Even roll, square not yet rolled | `◇4 → [?]` (pending) | "Even: a pothole opens · finding its square…" |
| Pothole opens on an empty square | `◇4 → d4 → opens` (bad) | "Pothole on d4 · closes when White moves" (hazard) |
| Repaired at once (next to the Mamdani) | `◇4 → b4 → repaired` (good) | "The Mamdani repairs b4 at once" (good) |
| Re-rolls (king, already a pothole, exposes, checkmate) | the die, then each re-rolled square as a plain chip marked `↻` (`◇6 → e1↻ → h6 → opens`) | The final outcome's line; the re-roll reasons are in the sheet's step list |
| A piece falls | `◇4 → g8 → falls` (bad) | "Black knight falls into g8" (hazard) |
| A saving roll is saved | `◇4 → d5 → save 3` (good) | "Knight saved" (good) |
| A saving roll is lost | `◇4 → d5 → save 6` (bad) | "Knight falls into d5" (hazard) |
| The Mamdani falls | `◇8 → a5 → falls` (bad) | "The Mamdani falls into a5" (hazard) |
| A move that ends the game (no roll) | none | "No roll: the game is over" (muted) |

Potholes closed or repaired by the move itself, before the roll (today's `notes`), stay off the card so it keeps one line: the hole visibly closes on the board, and the sheet's step list names it.

`DiceSummary.svelte` renders it inside a `<section aria-label="White's roll">`, using `Die` for the die chip so it tumbles exactly as in the tray. On desktop, `DiceTray` is unchanged.

## The moves sheet: `MovesSheet.svelte`

- **What it is:** a modal sheet that slides up over the dimmed game, 440 px tall or 70 % of the screen, whichever is smaller. It holds a handle, the title "MOVES AND ROLLS", a "Close" button, and the move log, scrolling within the sheet.
- **The current turn:** its row is highlighted, and the full `DiceTray` steps for it show underneath, the detail the phone card leaves out.
- **At game end:** the stats (lost to potholes, saving rolls, repairs, the Mamdani) show at the top.
- **Opening:** focus moves to "Close". Escape, Close, or a tap on the dimmed backdrop closes it, and focus returns to the "Moves and rolls" button.
- **Behind it:** the game keeps updating, and new moves appear in the sheet live.
- **Motion:** slides up over 250 ms; instant with reduced motion or instant mode.
- **Markup:** a `<dialog>` element. The "Moves and rolls" button's click handler calls `showModal()`, so the browser handles the focus trap and Escape; its `close` event puts focus back on the button. No `$effect`.
- **No swipe-to-close.** The canvas shows a handle, but swipe gestures are left out; the handle is only visual.

## How the layout switches

`const phone = new MediaQuery('max-width: 639px')` from `svelte/reactivity` (Svelte 5.57) gives a reactive `phone.current` with no `$effect`. The game page branches once: `{#if phone.current}` renders the phone layout, `{:else}` today's markup. The pieces are shared components: `Board`, `PlayerBar`, `Die`, `MoveLog`, `DiceTray`, plus the new `DiceSummary` and `MovesSheet`. State and handlers (`move`, `copyLink`, `doResign`, `newGame`, `confirmResign`) are shared, so the two layouts can't drift apart in behaviour.

## Testing

- **Vitest:** `diceSummary()` covers every row of its table, including mid-animation `shown` values (chips filling in). `PlayerBar`'s pill rules come from a pure `pillFor(view, color, you, animating)` in `board.ts`, tested the same way.
- **The play-test script gains `--phone`.** It plays as an iPhone 15 (WebKit, touch, 393×659): taps instead of clicks, and drags with touch.
  - After every turn, it fails if the page scrolls (`scrollHeight > innerHeight`) or the board is narrower than `innerWidth − 20`.
  - It opens and closes the moves sheet once per game, and resigns through the phone confirmation.
  - `pnpm --dir web playtest --phone --games 6` runs it locally, and against the live URL with `--turn-ms 15000`.
- **Screenshots** of all six states at 393×659 in WebKit, compared by eye against the artboards.
- **Desktop regression:** the existing play-test runs (Chromium and WebKit at desktop size) must still pass unchanged.

## Risks

- **`100dvh` support:** Safari 16+ and Chrome 108+, which covers current phones. Older browsers fall back to `100vh`, where the bottom bar may sit under Safari's toolbar; accepted.
- **Landscape phones** (wider than 640 px but short) get the desktop single column and scroll; accepted for the playtest.
- **The sheet and live updates:** a new turn while the sheet is open must not close it or steal focus. Pinned by the play-test opening the sheet mid-game.
