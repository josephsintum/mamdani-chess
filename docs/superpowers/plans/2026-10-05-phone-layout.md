# Phone Layout for the Game Screen Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** On phones in portrait (under 640 px wide), the game screen fits one screen with no scrolling: header, both player bars, a near-full-width board, a one-line dice card and a bottom bar with "Moves and rolls" and "Resign".

**Architecture:**
- **One page, two layouts.** The game page checks `new MediaQuery('max-width: 639px')` from `svelte/reactivity` (reactive, no `$effect`) and renders either the phone layout or today's desktop markup. Both layouts share all the state and handlers, so their behaviour can't drift apart.
- **New pure functions** in `board.ts`, unit-tested: `diceSummary()` (the dice card's chips and outcome line) and `pillFor()` (the player-bar pills).
- **New components:** `DiceSummary.svelte` (the card) and `MovesSheet.svelte` (a native `<dialog>` with the log, the current turn's dice steps and the end-of-game stats).
- **Board size:** the bars and board sit in a flexible region with `container-type: size`. The board is `min(100cqw − 16px, 100cqh − 92px)`, so it takes exactly the space that's left.

**Tech Stack:** SvelteKit 3, Svelte 5.57 (runes, `{@attach}`, `MediaQuery`), TypeScript, Vitest, Playwright (the play-test script).

**Spec:** `docs/superpowers/specs/2026-10-05-phone-layout-design.md`. Visual reference: the canvas row "Phone game: playtest build" (https://claude.ai/artifact/XVJqVp283CoZEHpmHDrSih), artboards `PhonePlaying`, `PhoneDice`, `PhoneMoves`, `PhoneResign`, `PhoneWaiting`, `PhoneOver`.

## How this plan was made

Every patch below was built and checked in a scratch copy. The scratch copy was checked in three ways:
- **Tests:** 77 Vitest tests.
- **Types and components:** `svelte-check` 0/0, and the autofixer clean on every touched component.
- **Browsers:** screenshots of all six states in WebKit at iPhone 15 size (393×659), compared by eye with the artboards. The play-test script ran 6/6 phone games clean, and 4/4 desktop games each in Chromium and WebKit, unchanged.

The plan was then dry-run task by task from `phone-layout` (`1a3dfdb`). Each task left `check` and `test` green, Task 2's tests failed first (17), and the final files are byte-identical to the scratch copy. Apply each patch from the repo root.

## Global Constraints

- **Frontend rules (CLAUDE.md):**
  - Svelte 5 runes, no `$effect`.
  - `#lib/…` imports carry their extension.
  - Every touched `.svelte` file passes `npx @sveltejs/mcp svelte-autofixer` with no issues and no suggestions.
- **The road-works look:** colours only from the tokens in `web/src/lib/theme/tokens.css`.
- **Desktop (640 px and wider) is unchanged** apart from the player-bar pills (spec, Success 4).
- **Out of scope:** clocks (milestone 05), spectator count and emoji reactions (06), home and the sandbox's own layout.
- **Git:** stage explicit paths and never use `git add -A`. No Claude attribution in commits.

## Review Focus

1. **A state that scrolls or hides the actions on a real phone.** Safari's visible height changes as its toolbars hide and show. Pinned by `100dvh` plus the container-query board, and by `playtest --phone` checking `scrollHeight <= innerHeight` after every ply (Task 6).
2. **The sheet and live updates.** A turn arriving while the sheet is open must not close it or steal focus, and focus must return to "Moves and rolls" when it closes. Pinned by the sheet opening and closing mid-game in every phone play-test (Task 6), and by the focus check in Task 5.
3. **The dice card as the dice play.** Each frame of the animation must show the right chips: a pending `?`, re-rolled squares, outcomes. Pinned by the `diceSummary` tests on mid-animation `shown` values (Task 2).
4. **The game ending on a phone.** The result card replaces the dice card, "New game" appears, and nothing waits for a move. Pinned by the play-test recognising the phone's result card (Task 6), which is exactly what failed before that fix.
5. **Desktop regressions from shared components.** `PlayerBar`, `Die` and the page are shared. Pinned by the desktop play-test runs in Chromium and WebKit (Task 6).

## File Structure

| File | Responsibility |
| --- | --- |
| `web/src/lib/board.ts`, `board.test.ts` | `diceSummary()`, `pillFor()` |
| `web/src/lib/game.test.ts` (new), `pieces.ts`, `pieces.test.ts` | One `pieceName()`, in `game.ts` |
| `web/src/lib/PlayerBar.svelte` | `pill`/`pillTone` replace `toMove`; `compact` one-row phone bar |
| `web/src/lib/Die.svelte` | `small` size for the dice card |
| `web/src/lib/DiceSummary.svelte` (new) | The phone's dice card |
| `web/src/lib/MovesSheet.svelte` (new) | The phone's "Moves and rolls" sheet |
| `web/src/routes/game/[code]/+page.svelte` | The phone layout branch and styles; pills on desktop |
| `web/src/routes/dev/board/+page.svelte` | The sandbox uses `pillFor()` |
| `web/scripts/playtest.js` | `--phone` |

---

### Task 1: One `pieceName()`

`game.ts` already exports `pieceName()`, and the copy added to `pieces.ts` in the pre-05 polish duplicates it (same output). Keep `game.ts`'s copy, move its test into a new `game.test.ts`, and point `Board.svelte` and `PlayerBar.svelte` at `game.ts`.

**Files:**
- Modify: `web/src/lib/pieces.ts`, `web/src/lib/pieces.test.ts`, `web/src/lib/Board.svelte`, `web/src/lib/PlayerBar.svelte`
- Create: `web/src/lib/game.test.ts`

**Interfaces:**
- Produces: `pieceName(code)` from `./game.ts` only.

- [ ] **Step 1: Apply the patch**

```bash
git apply <<'PATCH'
diff --git a/web/src/lib/Board.svelte b/web/src/lib/Board.svelte
index 09db26f..71a5827 100644
--- a/web/src/lib/Board.svelte
+++ b/web/src/lib/Board.svelte
@@ -1,8 +1,8 @@
 <script lang="ts">
 	import { blockedSquares, squareIndex, type Stage } from './board.ts';
-	import { squareName, type MoveJSON } from './game.ts';
+	import { pieceName, squareName, type MoveJSON } from './game.ts';
 	import { reducedMotion } from './motion.ts';
-	import { moveDuration, pieceName, reconcile, type PieceRef } from './pieces.ts';
+	import { moveDuration, reconcile, type PieceRef } from './pieces.ts';
 
 	let {
 		stage,
diff --git a/web/src/lib/PlayerBar.svelte b/web/src/lib/PlayerBar.svelte
index 38a9009..8e1fa52 100644
--- a/web/src/lib/PlayerBar.svelte
+++ b/web/src/lib/PlayerBar.svelte
@@ -1,6 +1,6 @@
 <script lang="ts">
 	import type { Color } from './game.ts';
-	import { pieceName } from './pieces.ts';
+	import { pieceName } from './game.ts';
 
 	let { color, you, lost, toMove }: { color: Color; you: boolean; lost: string[]; toMove: boolean } = $props();
 </script>
diff --git a/web/src/lib/game.test.ts b/web/src/lib/game.test.ts
new file mode 100644
index 0000000..af33c7c
--- /dev/null
+++ b/web/src/lib/game.test.ts
@@ -0,0 +1,10 @@
+import { describe, expect, it } from 'vitest';
+import { pieceName } from './game.ts';
+
+describe('pieceName', () => {
+	it('names a piece for screen readers', () => {
+		expect(pieceName('bN')).toBe('black knight');
+		expect(pieceName('wQ')).toBe('white queen');
+		expect(pieceName('M')).toBe('the Mamdani');
+	});
+});
diff --git a/web/src/lib/pieces.test.ts b/web/src/lib/pieces.test.ts
index 89b0b86..038b84d 100644
--- a/web/src/lib/pieces.test.ts
+++ b/web/src/lib/pieces.test.ts
@@ -1,6 +1,6 @@
 import { describe, expect, it } from 'vitest';
 import { squareIndex } from './board.ts';
-import { applyMove, moveDuration, pieceName, reconcile, settlesGuess, type PieceRef } from './pieces.ts';
+import { applyMove, moveDuration, reconcile, settlesGuess, type PieceRef } from './pieces.ts';
 
 /** A board with the given pieces, e.g. { e1: 'wK' }. */
 function boardOf(pieces: Record<string, string>): string[] {
@@ -118,11 +118,3 @@ describe('settlesGuess', () => {
 		expect(settlesGuess(null, { seq: 8 })).toBe(true);
 	});
 });
-
-describe('pieceName', () => {
-	it('names a piece for screen readers', () => {
-		expect(pieceName('bN')).toBe('black knight');
-		expect(pieceName('wQ')).toBe('white queen');
-		expect(pieceName('M')).toBe('the Mamdani');
-	});
-});
diff --git a/web/src/lib/pieces.ts b/web/src/lib/pieces.ts
index 026131e..b07230f 100644
--- a/web/src/lib/pieces.ts
+++ b/web/src/lib/pieces.ts
@@ -13,14 +13,6 @@ export interface PieceRef {
 
 const files = 'abcdefgh';
 
-const kinds: Record<string, string> = { K: 'king', Q: 'queen', R: 'rook', B: 'bishop', N: 'knight', P: 'pawn' };
-
-/** A piece's name for screen readers: "black knight", or "the Mamdani". */
-export function pieceName(code: string): string {
-	if (code === 'M') return 'the Mamdani';
-	return `${code[0] === 'w' ? 'white' : 'black'} ${kinds[code[1]]}`;
-}
-
 function squareName(index: number): string {
 	return files[index % 8] + (Math.floor(index / 8) + 1);
 }
PATCH
```

- [ ] **Step 2: Check**

Run: `pnpm --dir web check && pnpm --dir web test`
Expected: `0 ERRORS 0 WARNINGS`, `Tests  60 passed (60)`.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/pieces.ts web/src/lib/pieces.test.ts web/src/lib/game.test.ts web/src/lib/Board.svelte web/src/lib/PlayerBar.svelte
git commit -m "web: one pieceName, in game.ts"
```

---

### Task 2: `diceSummary()` and `pillFor()`

**Files:**
- Test: `web/src/lib/board.test.ts`
- Modify: `web/src/lib/board.ts`

**Interfaces:**
- Produces:
  - `interface DiceChip { text: string; kind: 'die' | 'square' | 'pending' | 'good' | 'bad' | 'plain' }`
  - `interface DiceSummary { who: Color | null; chips: DiceChip[]; line: string; tone: 'normal' | 'good' | 'hazard' | 'muted' }`
  - `diceSummary(view: View, shown: number): DiceSummary`
  - `pillFor(view: View, color: Color, animating: boolean): { text: string; tone: 'turn' | 'check' | 'muted' }`

- [ ] **Step 1: Write the failing tests**

```bash
git apply <<'PATCH'
diff --git a/web/src/lib/board.test.ts b/web/src/lib/board.test.ts
index a942e1b..9f6f6a3 100644
--- a/web/src/lib/board.test.ts
+++ b/web/src/lib/board.test.ts
@@ -1,5 +1,5 @@
 import { describe, expect, it } from 'vitest';
-import { blockedSquares, checkSquare, diceSteps, firstDiceStep, squareIndex, stageAt } from './board.ts';
+import { blockedSquares, checkSquare, diceSteps, diceSummary, firstDiceStep, pillFor, squareIndex, stageAt } from './board.ts';
 import type { EventJSON, View } from './game.ts';
 
 /** A view with the given pieces ({"e1": "wK"}), potholes and Mamdani square. */
@@ -187,3 +187,109 @@ describe('checkSquare', () => {
 		expect(checkSquare(v, stage, { animating: false, guessing: true })).toBe('');
 	});
 });
+
+describe('diceSummary', () => {
+	const sum = (last: EventJSON[], shown = last.length, extra: Partial<View> = {}, pieces: Record<string, string> = { g8: 'bN', e1: 'wK' }) => {
+		const d = diceSummary(makeView(pieces, { last, ...extra }), shown);
+		return { who: d.who, chips: d.chips.map((c) => `${c.text}:${c.kind}`), line: d.line, tone: d.tone };
+	};
+	const moved = { kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' } as const;
+
+	it('has nothing to show before the first roll', () => {
+		expect(sum([])).toEqual({ who: null, chips: [], line: 'The dice roll after every move.', tone: 'muted' });
+	});
+
+	it('shows the d8 rolling until it lands', () => {
+		expect(sum(fellOnG8, 1)).toEqual({ who: 'white', chips: ['?:pending'], line: 'Rolling the d8…', tone: 'muted' });
+	});
+
+	it('says an odd roll does nothing', () => {
+		expect(sum([moved, { kind: 'rolled_pothole', roll: 3, color: 'white' }])).toEqual({
+			who: 'white', chips: ['3:die'], line: 'Odd: nothing happens', tone: 'muted'
+		});
+	});
+
+	it('waits for the square after an even roll', () => {
+		expect(sum(fellOnG8, 2)).toEqual({
+			who: 'white', chips: ['2:die', '?:pending'], line: 'Even: a pothole opens · finding its square…', tone: 'normal'
+		});
+	});
+
+	it('marks re-rolled squares and keeps going', () => {
+		expect(sum(fellOnG8, 4).chips).toEqual(['2:die', 'e1↻:plain', '?:pending']);
+		expect(sum(fellOnG8, 4).line).toBe('Re-roll: kings never fall');
+	});
+
+	it('names a piece that falls, and the square', () => {
+		expect(sum(fellOnG8)).toEqual({
+			who: 'white', chips: ['2:die', 'e1↻:plain', 'g8:square', 'falls:bad'], line: 'Black knight falls into g8', tone: 'hazard'
+		});
+	});
+
+	it('opens a pothole on an empty square', () => {
+		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'd4' }, { kind: 'pothole_opened', sq: 'd4', color: 'white' }];
+		expect(sum(last)).toEqual({ who: 'white', chips: ['4:die', 'd4:square', 'opens:bad'], line: 'Pothole on d4 · closes when White moves', tone: 'hazard' });
+	});
+
+	it('repairs a pothole next to the Mamdani at once', () => {
+		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'b4' }, { kind: 'repaired', sq: 'b4' }];
+		expect(sum(last)).toEqual({ who: 'white', chips: ['4:die', 'b4:square', 'repaired:good'], line: 'The Mamdani repairs b4 at once', tone: 'good' });
+	});
+
+	it('shows a saving roll, saved or lost', () => {
+		const roll = (save: number, saved: boolean): EventJSON[] => [
+			moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'd5' },
+			{ kind: 'saving_roll', sq: 'd5', piece: 'bN', roll: save, saved, color: 'black' },
+			...(saved ? [] : ([{ kind: 'fell', sq: 'd5', piece: 'bN' }, { kind: 'pothole_opened', sq: 'd5', color: 'white' }] as EventJSON[]))
+		];
+		expect(sum(roll(3, true))).toEqual({ who: 'white', chips: ['4:die', 'd5:square', 'save 3:good'], line: 'Black knight saved', tone: 'good' });
+		expect(sum(roll(6, false))).toEqual({ who: 'white', chips: ['4:die', 'd5:square', 'save 6:bad'], line: 'Black knight falls into d5', tone: 'hazard' });
+	});
+
+	it('drops the Mamdani', () => {
+		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 8, color: 'white' }, { kind: 'target', sq: 'a5' },
+			{ kind: 'saving_roll', sq: 'a5', piece: 'M', roll: 2, saved: false, color: 'white' }, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5', color: 'white' }];
+		expect(sum(last).line).toBe('The Mamdani falls into a5');
+	});
+
+	it('says when no square could take a pothole', () => {
+		expect(sum([moved, { kind: 'rolled_pothole', roll: 2, color: 'white' }, { kind: 'no_pothole' }])).toEqual({
+			who: 'white', chips: ['2:die', 'none:plain'], line: 'No pothole: no square could take one', tone: 'muted'
+		});
+	});
+
+	it('says a game-ending move has no roll', () => {
+		const end = makeView({}, { last: [{ kind: 'moved', from: 'd8', to: 'h4', piece: 'bQ', color: 'black' }], status: 'over', result: { winner: 'black', draw: false, reason: 'checkmate' } });
+		expect(diceSummary(end, 1)).toEqual({ who: 'black', chips: [], line: 'No roll: the game is over', tone: 'muted' });
+	});
+});
+
+describe('pillFor', () => {
+	const v = (extra: Partial<View>) => makeView({}, { status: 'playing', turn: 'white', you: 'white', ...extra });
+
+	it('tells you it is your move, and the other bar whose move it is', () => {
+		expect(pillFor(v({}), 'white', false)).toEqual({ text: 'Your move', tone: 'turn' });
+		expect(pillFor(v({ you: 'black' }), 'white', false)).toEqual({ text: 'To move', tone: 'turn' });
+		expect(pillFor(v({}), 'black', false)).toEqual({ text: '', tone: 'turn' });
+	});
+
+	it('warns the side in check', () => {
+		expect(pillFor(v({ check: true }), 'white', false)).toEqual({ text: 'In check', tone: 'check' });
+	});
+
+	it('shows nothing while the dice play out', () => {
+		expect(pillFor(v({}), 'white', true).text).toBe('');
+	});
+
+	it('marks Black waiting to join', () => {
+		expect(pillFor(v({ status: 'waiting' }), 'black', false)).toEqual({ text: 'Waiting…', tone: 'muted' });
+		expect(pillFor(v({ status: 'waiting' }), 'white', false).text).toBe('');
+	});
+
+	it('marks the checkmated side at the end', () => {
+		const over = v({ status: 'over', result: { winner: 'black', draw: false, reason: 'checkmate' } });
+		expect(pillFor(over, 'white', false)).toEqual({ text: 'Checkmated', tone: 'check' });
+		expect(pillFor(over, 'black', false).text).toBe('');
+		expect(pillFor(v({ status: 'over', result: { winner: 'black', draw: false, reason: 'resignation' } }), 'white', false).text).toBe('');
+	});
+});
PATCH
```

- [ ] **Step 2: Run them to see them fail**

Run: `pnpm --dir web test`
Expected: FAIL: `Tests  17 failed | 60 passed (77)`, with `diceSummary is not a function`.

- [ ] **Step 3: Implement**

```bash
git apply <<'PATCH'
diff --git a/web/src/lib/board.ts b/web/src/lib/board.ts
index f4f2989..98c4a0e 100644
--- a/web/src/lib/board.ts
+++ b/web/src/lib/board.ts
@@ -207,3 +207,96 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 	});
 	return steps;
 }
+
+/** One chip in the phone's dice card: a die, a square, or an outcome. */
+export interface DiceChip {
+	text: string;
+	kind: 'die' | 'square' | 'pending' | 'good' | 'bad' | 'plain';
+}
+
+/** The phone's one-line version of a turn's dice (the steps are in the moves sheet). */
+export interface DiceSummary {
+	who: Color | null; // whose roll; null before the first one
+	chips: DiceChip[]; // as far as the dice have played
+	line: string;
+	tone: 'normal' | 'good' | 'hazard' | 'muted';
+}
+
+/**
+ * Sums up the turn's dice as far as they have played (shown events of
+ * view.last), for the phone's dice card: chips that fill in step by step
+ * and one outcome line.
+ */
+export function diceSummary(view: View, shown: number): DiceSummary {
+	const mover = view.last.find((e) => e.kind === 'moved')?.color ?? null;
+	if (!view.last.some((e) => e.kind === 'rolled_pothole')) {
+		if (mover && view.result) return { who: mover, chips: [], line: 'No roll: the game is over', tone: 'muted' };
+		return { who: null, chips: [], line: 'The dice roll after every move.', tone: 'muted' };
+	}
+	const chips: DiceChip[] = [];
+	let line = 'Rolling the d8…';
+	let tone: DiceSummary['tone'] = 'muted';
+	let pending = true; // the next die or square is still to come
+	let square = '';
+	for (const e of view.last.slice(0, shown)) {
+		switch (e.kind) {
+			case 'rolled_pothole': {
+				const even = (e.roll ?? 1) % 2 === 0;
+				chips.push({ text: String(e.roll), kind: 'die' });
+				[line, tone, pending] = even ? ['Even: a pothole opens · finding its square…', 'normal', true] : ['Odd: nothing happens', 'muted', false];
+				break;
+			}
+			case 'target':
+				square = e.sq ?? '';
+				chips.push({ text: square, kind: 'square' });
+				pending = false;
+				break;
+			case 'reroll': {
+				chips[chips.length - 1] = { text: `${e.sq}↻`, kind: 'plain' };
+				const why = rerollReasons[e.reason ?? ''] ?? e.reason ?? '';
+				[line, tone, pending] = [`Re-roll: ${why.charAt(0).toLowerCase()}${why.slice(1)}`, 'muted', true];
+				break;
+			}
+			case 'saving_roll':
+				chips.push({ text: `save ${e.roll}`, kind: e.saved ? 'good' : 'bad' });
+				if (e.saved) [line, tone] = [`${capitalize(pieceName(e.piece))} saved`, 'good'];
+				break;
+			case 'fell':
+				if (chips[chips.length - 1]?.kind === 'square') chips.push({ text: 'falls', kind: 'bad' });
+				[line, tone] = [`${capitalize(pieceName(e.piece))} falls into ${e.sq}`, 'hazard'];
+				break;
+			case 'pothole_opened':
+				// After a fall the fall is the news; on an empty square the hole is.
+				if (chips[chips.length - 1]?.kind === 'square') {
+					chips.push({ text: 'opens', kind: 'bad' });
+					[line, tone] = [`Pothole on ${e.sq} · closes when ${colorTitle(e.color)} moves`, 'hazard'];
+				}
+				break;
+			case 'repaired':
+				if (square === e.sq) {
+					chips.push({ text: 'repaired', kind: 'good' });
+					[line, tone] = [`The Mamdani repairs ${e.sq} at once`, 'good'];
+				}
+				break;
+			case 'no_pothole':
+				chips.push({ text: 'none', kind: 'plain' });
+				[line, tone, pending] = ['No pothole: no square could take one', 'muted', false];
+				break;
+		}
+	}
+	if (pending) chips.push({ text: '?', kind: 'pending' });
+	const roll = view.last.find((e) => e.kind === 'rolled_pothole');
+	return { who: roll?.color ?? mover, chips, line, tone };
+}
+
+/** The pill on a player's bar: whose move, check, waiting, checkmated. */
+export function pillFor(view: View, color: Color, animating: boolean): { text: string; tone: 'turn' | 'check' | 'muted' } {
+	if (view.status === 'waiting') return color === 'black' ? { text: 'Waiting…', tone: 'muted' } : { text: '', tone: 'muted' };
+	if (view.result) {
+		const mated = view.result.reason === 'checkmate' && !view.result.draw && view.result.winner !== color;
+		return mated ? { text: 'Checkmated', tone: 'check' } : { text: '', tone: 'turn' };
+	}
+	if (animating || view.turn !== color) return { text: '', tone: 'turn' };
+	if (view.check) return { text: 'In check', tone: 'check' };
+	return { text: view.you === color ? 'Your move' : 'To move', tone: 'turn' };
+}
PATCH
```

- [ ] **Step 4: Run them to see them pass**

Run: `pnpm --dir web check && pnpm --dir web test`
Expected: `0 ERRORS 0 WARNINGS`, `Tests  77 passed (77)`.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/board.ts web/src/lib/board.test.ts
git commit -m "web: diceSummary and pillFor for the phone's dice card and the player-bar pills"
```

---

### Task 3: Pills in the player bars

`PlayerBar` takes `pill`/`pillTone` (from `pillFor`) instead of `toMove`, and gains `compact`, the phone's one-row bar. The desktop game page and the sandbox switch to `pillFor`.

**Files:**
- Modify: `web/src/lib/PlayerBar.svelte` (replaced), `web/src/routes/game/[code]/+page.svelte`, `web/src/routes/dev/board/+page.svelte`

**Interfaces:**
- Consumes: `pillFor` (Task 2).
- Produces: `<PlayerBar color you lost pill? pillTone? compact? />`.

- [ ] **Step 1: Apply the patch**

```bash
git apply <<'PATCH'
diff --git a/web/src/lib/PlayerBar.svelte b/web/src/lib/PlayerBar.svelte
index 8e1fa52..720f7d1 100644
--- a/web/src/lib/PlayerBar.svelte
+++ b/web/src/lib/PlayerBar.svelte
@@ -1,20 +1,43 @@
 <script lang="ts">
-	import type { Color } from './game.ts';
-	import { pieceName } from './game.ts';
+	import { pieceName, type Color } from './game.ts';
 
-	let { color, you, lost, toMove }: { color: Color; you: boolean; lost: string[]; toMove: boolean } = $props();
+	let {
+		color,
+		you,
+		lost,
+		pill = '',
+		pillTone = 'turn',
+		compact = false
+	}: {
+		color: Color;
+		you: boolean;
+		lost: string[];
+		/** "Your move", "In check", "Waiting…", or "" for none (see pillFor). */
+		pill?: string;
+		pillTone?: 'turn' | 'check' | 'muted';
+		/** The phone's one-row bar: lost pieces after the name, nothing when none. */
+		compact?: boolean;
+	} = $props();
 </script>
 
-<div class="bar" class:to-move={toMove}>
+<div class="bar" class:compact>
 	<span class="swatch {color}" aria-hidden="true"></span>
 	<span class="who">
 		<span class="name">{color === 'white' ? 'White' : 'Black'}{#if you}<span class="you">(you)</span>{/if}</span>
-		<span class="lost">
-			Lost to potholes:
-			{#if lost.length === 0}none{:else}<span class="glyphs">{#each lost as p, i (i)}<img src="/pieces/{p}.svg" alt={pieceName(p)} />{/each}</span>{/if}
-		</span>
+		{#if compact}
+			{#if lost.length > 0}
+				<span class="glyphs" role="img" aria-label="Lost to potholes: {lost.map((p) => pieceName(p)).join(', ')}">
+					{#each lost as p, i (i)}<img src="/pieces/{p}.svg" alt="" />{/each}
+				</span>
+			{/if}
+		{:else}
+			<span class="lost">
+				Lost to potholes:
+				{#if lost.length === 0}none{:else}<span class="glyphs">{#each lost as p, i (i)}<img src="/pieces/{p}.svg" alt={pieceName(p)} />{/each}</span>{/if}
+			</span>
+		{/if}
 	</span>
-	{#if toMove}<span class="turn">To move</span>{/if}
+	{#if pill}<span class="pill {pillTone}">{pill}</span>{/if}
 </div>
 
 <style>
@@ -24,6 +47,12 @@
 		gap: 12px;
 		min-height: 52px;
 	}
+	.bar.compact {
+		gap: 10px;
+		min-height: 44px;
+		height: 44px;
+		padding: 0 12px;
+	}
 	.swatch {
 		width: 14px;
 		height: 14px;
@@ -42,10 +71,17 @@
 		flex-direction: column;
 		gap: 2px;
 		flex-grow: 1;
+		min-width: 0;
+	}
+	.compact .who {
+		flex-direction: row;
+		align-items: center;
+		gap: 8px;
 	}
 	.name {
 		font-weight: 600;
 		color: var(--text);
+		white-space: nowrap;
 	}
 	.you {
 		margin-left: 6px;
@@ -60,12 +96,17 @@
 		display: inline-flex;
 		gap: 1px;
 		vertical-align: middle;
+		overflow: hidden;
 	}
 	.glyphs img {
 		width: 18px;
 		height: 18px;
+		flex-shrink: 0;
 	}
-	.turn {
+	.pill {
+		display: flex;
+		align-items: center;
+		flex-shrink: 0;
 		padding: 4px 10px;
 		border: 1px solid var(--accent-line);
 		border-radius: 999px;
@@ -73,4 +114,18 @@
 		font-weight: 600;
 		color: var(--accent);
 	}
+	.compact .pill {
+		height: 28px;
+		box-sizing: border-box;
+		padding: 0 12px;
+		font-size: 13px;
+	}
+	.pill.check {
+		border-color: var(--hazard);
+		color: var(--hazard-text);
+	}
+	.pill.muted {
+		border-color: var(--line);
+		color: var(--text-muted);
+	}
 </style>
diff --git a/web/src/routes/dev/board/+page.svelte b/web/src/routes/dev/board/+page.svelte
index 184b5b8..8fce653 100644
--- a/web/src/routes/dev/board/+page.svelte
+++ b/web/src/routes/dev/board/+page.svelte
@@ -4,7 +4,7 @@
 	import MoveLog from '#lib/MoveLog.svelte';
 	import PlayerBar from '#lib/PlayerBar.svelte';
 	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
-	import { stageAt } from '#lib/board.ts';
+	import { pillFor, stageAt } from '#lib/board.ts';
 	import type { Color, MoveJSON, View } from '#lib/game.ts';
 	import { setInstant } from '#lib/motion.ts';
 	import { freeMoves, playTurn, positions, type RollScript } from '#lib/sandbox.ts';
@@ -173,7 +173,7 @@
 		</div>
 
 		<div class="board-col">
-			<PlayerBar color={top} you={you === top} lost={stage.lost[top]} toMove={!view.result && view.turn === top && !anim.animating} />
+			<PlayerBar color={top} you={you === top} lost={stage.lost[top]} pill={pillFor(view, top, anim.animating).text} pillTone={pillFor(view, top, anim.animating).tone} />
 			<Board
 				{stage}
 				{legal}
@@ -184,7 +184,7 @@
 				saved={savedSquare}
 				onmove={move}
 			/>
-			<PlayerBar color={bottom} you={you === bottom} lost={stage.lost[bottom]} toMove={!view.result && view.turn === bottom && !anim.animating} />
+			<PlayerBar color={bottom} you={you === bottom} lost={stage.lost[bottom]} pill={pillFor(view, bottom, anim.animating).text} pillTone={pillFor(view, bottom, anim.animating).tone} />
 		</div>
 
 		<div class="side">
diff --git a/web/src/routes/game/[code]/+page.svelte b/web/src/routes/game/[code]/+page.svelte
index 4354722..4026741 100644
--- a/web/src/routes/game/[code]/+page.svelte
+++ b/web/src/routes/game/[code]/+page.svelte
@@ -9,7 +9,7 @@
 	import MoveLog from '#lib/MoveLog.svelte';
 	import PlayerBar from '#lib/PlayerBar.svelte';
 	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
-	import { checkSquare, stageAt } from '#lib/board.ts';
+	import { checkSquare, pillFor, stageAt } from '#lib/board.ts';
 	import { applyMove, settlesGuess } from '#lib/pieces.ts';
 	import { createGame, reasons, resign, sendMove, type Color, type MoveJSON, type View } from '#lib/game.ts';
 
@@ -84,6 +84,8 @@
 	});
 	let playing = $derived(view?.status === 'playing');
 	let isPlayer = $derived(you === 'white' || you === 'black');
+	let topPill = $derived(view ? pillFor(view, top, animating) : { text: '', tone: 'turn' as const });
+	let bottomPill = $derived(view ? pillFor(view, bottom, animating) : { text: '', tone: 'turn' as const });
 
 	async function move(m: MoveJSON) {
 		if (!view || busy) return;
@@ -202,7 +204,7 @@
 			</div>
 
 			<div class="board-col">
-				<PlayerBar color={top} you={you === top} lost={stage.lost[top]} toMove={playing && view.turn === top && !animating} />
+				<PlayerBar color={top} you={you === top} lost={stage.lost[top]} pill={topPill.text} pillTone={topPill.tone} />
 				<div class="board-wrap">
 					<Board
 						{stage}
@@ -227,7 +229,8 @@
 					color={bottom}
 					you={you === bottom}
 					lost={stage.lost[bottom]}
-					toMove={playing && view.turn === bottom && !animating}
+					pill={bottomPill.text}
+					pillTone={bottomPill.tone}
 				/>
 			</div>
 
PATCH
```

- [ ] **Step 2: Autofix and check**

Run: `for f in web/src/lib/PlayerBar.svelte web/src/routes/dev/board/+page.svelte 'web/src/routes/game/[code]/+page.svelte'; do npx -y @sveltejs/mcp svelte-autofixer "$f"; done; pnpm --dir web check && pnpm --dir web test`
Expected: `issues: []` and `suggestions: []` for each; `0 ERRORS 0 WARNINGS`; `Tests  77 passed (77)`.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/PlayerBar.svelte 'web/src/routes/game/[code]/+page.svelte' web/src/routes/dev/board/+page.svelte
git commit -m "web: player-bar pills (your move, in check, waiting, checkmated)"
```

---

### Task 4: The phone's dice card and moves sheet

**Files:**
- Modify: `web/src/lib/Die.svelte` (a `small` size)
- Create: `web/src/lib/DiceSummary.svelte`, `web/src/lib/MovesSheet.svelte`

**Interfaces:**
- Consumes: `diceSummary` (Task 2); `MoveLog`, `DiceTray`, `reducedMotion`.
- Produces:
  - `<DiceSummary view shown />`
  - `<MovesSheet view shown rolling />`, with `open(from: HTMLElement | null)` (focus returns to `from` on close)
  - `<Die value highlight? small? />`

- [ ] **Step 1: Apply the patch**

```bash
git apply <<'PATCH'
diff --git a/web/src/lib/DiceSummary.svelte b/web/src/lib/DiceSummary.svelte
new file mode 100644
index 0000000..5a691f5
--- /dev/null
+++ b/web/src/lib/DiceSummary.svelte
@@ -0,0 +1,105 @@
+<script lang="ts">
+	import { diceSummary } from './board.ts';
+	import Die from './Die.svelte';
+	import type { View } from './game.ts';
+
+	// The phone's dice card: the turn's dice as a row of chips that fill in as
+	// they play out, and one outcome line. The steps are in the moves sheet.
+	let { view, shown }: { view: View; shown: number } = $props();
+
+	let summary = $derived(diceSummary(view, shown));
+	let title = $derived(summary.who ? `${summary.who === 'white' ? 'White' : 'Black'}’s roll` : 'Dice');
+</script>
+
+<section class="card" aria-label={title}>
+	<div class="chips">
+		<span class="who">{title}</span>
+		{#each summary.chips as chip, i (i)}
+			{#if i > 0}<span class="arrow" aria-hidden="true">→</span>{/if}
+			{#if chip.kind === 'die'}
+				<Die value={Number(chip.text)} small />
+			{:else}
+				<span class="chip {chip.kind}">{chip.text}</span>
+			{/if}
+		{/each}
+	</div>
+	<p class="line {summary.tone}">{summary.line}</p>
+</section>
+
+<style>
+	.card {
+		display: flex;
+		flex-direction: column;
+		gap: 8px;
+		margin: 6px 8px 0;
+		padding: 10px 12px;
+		background: var(--surface);
+		border: 1px solid var(--surface-2);
+		border-radius: 12px;
+	}
+	.chips {
+		display: flex;
+		align-items: center;
+		gap: 6px;
+		min-height: 26px;
+		font-family: var(--font-mono);
+		font-size: 13px;
+		font-weight: 600;
+		color: var(--text);
+	}
+	.who {
+		margin-right: 4px;
+		font-family: var(--font-display);
+		font-size: 15px;
+		font-weight: 700;
+		letter-spacing: 0.04em;
+		text-transform: uppercase;
+		color: var(--text-muted);
+	}
+	.arrow {
+		color: var(--text-muted);
+	}
+	.chip {
+		display: flex;
+		align-items: center;
+		justify-content: center;
+		box-sizing: border-box;
+		height: 26px;
+		min-width: 26px;
+		padding: 0 8px;
+		border-radius: 6px;
+		background: var(--line);
+		white-space: nowrap;
+	}
+	.chip.pending {
+		background: none;
+		border: 2px dashed var(--accent);
+		color: var(--accent);
+	}
+	.chip.good {
+		background: var(--accent);
+		color: var(--accent-text);
+	}
+	.chip.bad {
+		background: var(--hazard);
+		color: var(--accent-text);
+	}
+	.line {
+		margin: 0;
+		overflow: hidden;
+		font-size: 14px;
+		line-height: 19px;
+		white-space: nowrap;
+		text-overflow: ellipsis;
+		color: var(--text-body);
+	}
+	.line.muted {
+		color: var(--text-muted);
+	}
+	.line.good {
+		color: var(--accent);
+	}
+	.line.hazard {
+		color: var(--hazard-text);
+	}
+</style>
diff --git a/web/src/lib/Die.svelte b/web/src/lib/Die.svelte
index d100b4f..1a16d61 100644
--- a/web/src/lib/Die.svelte
+++ b/web/src/lib/Die.svelte
@@ -3,7 +3,7 @@
 
 	// A d8 shown as a diamond. When it appears it tumbles through random
 	// faces for a moment, then lands on its value.
-	let { value, highlight = false }: { value: number; highlight?: boolean } = $props();
+	let { value, highlight = false, small = false }: { value: number; highlight?: boolean; small?: boolean } = $props();
 
 	const TUMBLE_MS = 320;
 	let face = $state(0); // 0 = settled on value
@@ -20,7 +20,7 @@
 	}
 </script>
 
-<span class="die" class:highlight class:rolling={face !== 0} {@attach tumble}>
+<span class="die" class:highlight class:small class:rolling={face !== 0} {@attach tumble}>
 	<!-- The tray is a live region: screen readers get the value, never the tumbling faces. -->
 	<span aria-hidden="true">{face || value}</span>
 	<span class="sr-only">{value}</span>
@@ -37,6 +37,16 @@
 		background: var(--line);
 		border-radius: 5px;
 	}
+	/* The phone's dice card: fits a 26px chip row. */
+	.die.small {
+		width: 22px;
+		height: 22px;
+		margin: 0 4px;
+		border-radius: 4px;
+	}
+	.die.small span {
+		font-size: 13px;
+	}
 	.die.rolling {
 		animation: tumble 0.32s ease-out;
 	}
diff --git a/web/src/lib/MovesSheet.svelte b/web/src/lib/MovesSheet.svelte
new file mode 100644
index 0000000..43107c8
--- /dev/null
+++ b/web/src/lib/MovesSheet.svelte
@@ -0,0 +1,146 @@
+<script lang="ts">
+	import DiceTray from './DiceTray.svelte';
+	import type { View } from './game.ts';
+	import { reducedMotion } from './motion.ts';
+	import MoveLog from './MoveLog.svelte';
+
+	// The phone's "Moves and rolls": a sheet over the game with the log, the
+	// current turn's dice step by step, and the stats once the game is over.
+	// A native <dialog>: showModal() traps focus and closes on Escape.
+	let { view, shown, rolling }: { view: View; shown: number; rolling: boolean } = $props();
+
+	let dialog: HTMLDialogElement | undefined;
+	let still = $state(false); // no slide under reduced motion or instant mode
+	let returnTo: HTMLElement | null = null;
+
+	/** Opens the sheet; focus goes back to `from` when it closes. */
+	export function open(from: HTMLElement | null) {
+		returnTo = from;
+		still = reducedMotion();
+		dialog?.showModal();
+	}
+
+	function keep(node: HTMLDialogElement) {
+		dialog = node;
+		return () => (dialog = undefined);
+	}
+
+	function close() {
+		dialog?.close();
+	}
+
+	// A tap on the dimmed backdrop lands on the dialog itself.
+	function backdrop(e: MouseEvent) {
+		if (e.target === dialog) close();
+	}
+</script>
+
+<dialog {@attach keep} class="sheet" class:still aria-labelledby="log-heading" onclick={backdrop} onclose={() => returnTo?.focus()}>
+	<div class="body">
+		<span class="handle" aria-hidden="true"></span>
+		<button type="button" class="close" onclick={close}>Close</button>
+		{#if view.result}
+			<dl class="stats">
+				<div><dt>Lost to potholes</dt><dd>White {view.lost.white.length} · Black {view.lost.black.length}</dd></div>
+				<div><dt>Saving rolls</dt><dd>{view.stats.saved} of {view.stats.savingRolls} saved</dd></div>
+				<div><dt>Repaired by the Mamdani</dt><dd>{view.stats.repaired} {view.stats.repaired === 1 ? 'pothole' : 'potholes'}</dd></div>
+				{#if view.stats.mamdaniFell}<div><dt>The Mamdani</dt><dd>fell in</dd></div>{/if}
+			</dl>
+		{/if}
+		<MoveLog log={view.log} {rolling} />
+		<DiceTray {view} {shown} />
+	</div>
+</dialog>
+
+<style>
+	.sheet {
+		position: fixed;
+		inset: auto 0 0;
+		width: 100%;
+		max-width: 100%;
+		height: min(440px, 70dvh);
+		max-height: 100%;
+		margin: 0;
+		padding: 0;
+		box-sizing: border-box;
+		border: 0;
+		border-top: 1px solid var(--line);
+		border-radius: 16px 16px 0 0;
+		background: var(--surface);
+		color: var(--text-body);
+		box-shadow: 0 -12px 32px var(--hole);
+	}
+	.sheet[open] {
+		animation: slide-up 250ms ease-out;
+	}
+	.sheet.still[open] {
+		animation: none;
+	}
+	.sheet::backdrop {
+		background: rgb(8 9 10 / 62%);
+	}
+	@keyframes slide-up {
+		from {
+			transform: translateY(100%);
+		}
+	}
+	.body {
+		position: relative;
+		display: flex;
+		flex-direction: column;
+		gap: 12px;
+		height: 100%;
+		box-sizing: border-box;
+		padding: 56px 12px max(16px, env(safe-area-inset-bottom));
+		overflow-y: auto;
+	}
+	/* The sheet scrolls; its sections keep their height. */
+	.body > :global(*) {
+		flex-shrink: 0;
+	}
+	.handle {
+		position: absolute;
+		top: 8px;
+		left: calc(50% - 20px);
+		width: 40px;
+		height: 4px;
+		border-radius: 2px;
+		background: var(--line);
+	}
+	.close {
+		position: absolute;
+		top: 8px;
+		right: 4px;
+		height: 44px;
+		padding: 0 14px;
+		border: 0;
+		background: none;
+		color: var(--accent);
+		font: inherit;
+		font-weight: 600;
+		cursor: pointer;
+	}
+	.stats {
+		display: grid;
+		gap: 6px;
+		margin: 0;
+		font-size: 14px;
+	}
+	.stats div {
+		display: flex;
+		justify-content: space-between;
+		gap: 12px;
+	}
+	.stats dt {
+		color: var(--text-muted);
+	}
+	.stats dd {
+		margin: 0;
+		color: var(--text);
+	}
+	@media (prefers-reduced-motion: reduce) {
+		.sheet[open] {
+			animation: none;
+		}
+	}
+</style>
PATCH
```

- [ ] **Step 2: Autofix and check**

Run: `for f in web/src/lib/Die.svelte web/src/lib/DiceSummary.svelte web/src/lib/MovesSheet.svelte; do npx -y @sveltejs/mcp svelte-autofixer "$f"; done; pnpm --dir web check && pnpm --dir web test`
Expected: clean autofixer runs; `0 ERRORS 0 WARNINGS`; `Tests  77 passed (77)`.

- [ ] **Step 3: Commit**

```bash
git add web/src/lib/Die.svelte web/src/lib/DiceSummary.svelte web/src/lib/MovesSheet.svelte
git commit -m "web: the phone's dice card and moves sheet"
```

---

### Task 5: The phone layout

Below 640 px wide the game page renders the phone layout. The card slot shows one card, in this priority: waiting share box, resign confirmation, result, error, then the dice card. The waiting share box and the resign confirmation hide the bottom bar. On phones the board also dims while waiting, matching the artboard.

**Files:**
- Modify: `web/src/routes/game/[code]/+page.svelte`

**Interfaces:**
- Consumes: `DiceSummary`, `MovesSheet` (Task 4); `PlayerBar compact` (Task 3); `pillFor` (Task 2).

- [ ] **Step 1: Apply the patch**

```bash
git apply <<'PATCH'
diff --git a/web/src/routes/game/[code]/+page.svelte b/web/src/routes/game/[code]/+page.svelte
index 4026741..6bbc5d3 100644
--- a/web/src/routes/game/[code]/+page.svelte
+++ b/web/src/routes/game/[code]/+page.svelte
@@ -1,12 +1,15 @@
 <script lang="ts">
 	import { onMount } from 'svelte';
+	import { MediaQuery } from 'svelte/reactivity';
 	import { fly } from 'svelte/transition';
 	import { reducedMotion, setInstant } from '#lib/motion.ts';
 	import { dev } from '$app/env';
 	import { page } from '$app/state';
 	import Board from '#lib/Board.svelte';
+	import DiceSummary from '#lib/DiceSummary.svelte';
 	import DiceTray from '#lib/DiceTray.svelte';
 	import MoveLog from '#lib/MoveLog.svelte';
+	import MovesSheet from '#lib/MovesSheet.svelte';
 	import PlayerBar from '#lib/PlayerBar.svelte';
 	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
 	import { checkSquare, pillFor, stageAt } from '#lib/board.ts';
@@ -19,6 +22,9 @@
 	const instant = dev && page.url.searchParams.has('instant');
 	setInstant(instant);
 	const anim = new Animator(instant ? 0 : STEP_MS);
+	// Phones in portrait get their own layout (canvas row "Phone game: playtest build").
+	const phone = new MediaQuery('max-width: 639px');
+	let sheet: MovesSheet | undefined = $state();
 	let view = $derived(anim.view);
 	let shown = $derived(anim.shown);
 	// Your move, shown before the server confirms it (One Million Chessboards
@@ -163,6 +169,91 @@
 	<title>Game {code} · Pothole Chess</title>
 </svelte:head>
 
+{#if phone.current && view && stage && !notFound}
+	<div class="phone">
+		<header class="ph-head">
+			<a href="/" class="ph-logo">Pothole Chess</a>
+			<span class="ph-meta">
+				{#if !connected && !lost}<span class="ph-chip warn">Reconnecting…</span>{/if}
+				{#if you === 'spectator'}<span class="ph-chip">Watching</span>{/if}
+				<span class="ph-code">{copyHint && view.status !== 'waiting' ? copyHint : code}</span>
+				<button type="button" class="ph-icon" aria-label="Copy game link" onclick={copyLink}>
+					<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="12" height="12" rx="2"></rect><path d="M5 15V5a2 2 0 0 1 2-2h10"></path></svg>
+				</button>
+			</span>
+		</header>
+		{#if lost}
+			<p class="ph-lost" role="alert">
+				Lost the connection. <a href={`/game/${code}`} data-sveltekit-reload>Reload</a> or <a href="/">start a new game</a>.
+			</p>
+		{/if}
+		<!-- The pills and the dice card show this; screen readers hear it. -->
+		<p class="sr-only" aria-live="polite">{status}</p>
+
+		<div class="ph-play">
+			<PlayerBar compact color={top} you={you === top} lost={stage.lost[top]} pill={topPill.text} pillTone={topPill.tone} />
+			<div class="ph-board">
+				<Board
+					{stage}
+					legal={view.legal}
+					{lastMove}
+					flipped={bottom === 'black'}
+					interactive={!animating && !busy && !optimistic && playing}
+					dim={!!resultCard || view.status === 'waiting'}
+					check={checked}
+					saved={savedSquare}
+					onmove={move}
+				/>
+			</div>
+			<PlayerBar compact color={bottom} you={you === bottom} lost={stage.lost[bottom]} pill={bottomPill.text} pillTone={bottomPill.tone} />
+		</div>
+
+		{#if view.status === 'waiting' && you === 'white'}
+			<section class="ph-card" aria-label="Invite a friend">
+				<label for="link" class="ph-title">Send this link to your friend</label>
+				<div class="ph-row">
+					<input id="link" readonly value={page.url.href} />
+					<button type="button" class="primary" onclick={copyLink}>Copy link</button>
+				</div>
+				{#if copyHint}<span class="muted">{copyHint}</span>{/if}
+			</section>
+		{:else if confirmResign}
+			<section class="ph-card danger-line" aria-label="Resign">
+				<span class="ph-title">Resign this game? <span class="muted">{you === 'white' ? 'Black' : 'White'} wins.</span></span>
+				<div class="ph-two">
+					<button type="button" class="outline" onclick={() => (confirmResign = false)}>Keep playing</button>
+					<button type="button" class="danger" onclick={doResign} disabled={busy}>Yes, resign</button>
+				</div>
+			</section>
+		{:else if resultCard}
+			<section class="ph-card accent-line" role="status" aria-label="Game over" in:fly={{ y: 24, duration: reducedMotion() ? 0 : 400 }}>
+				<span class="ph-result"><span class="ph-headline">{resultCard.title}</span><span class="ph-kicker">{resultCard.kicker}</span></span>
+				<span class="ph-detail">{resultCard.detail}</span>
+			</section>
+		{:else if error}
+			<p class="ph-card ph-error" role="alert">{error}</p>
+		{:else}
+			<DiceSummary {view} {shown} />
+		{/if}
+
+		{#if view.status !== 'waiting' && !confirmResign}
+			<nav class="ph-nav" aria-label="Game actions" class:single={!resultCard && !(isPlayer && playing)}>
+				{#if resultCard}<button type="button" class="primary" onclick={newGame} disabled={busy}>New game</button>{/if}
+				<button type="button" class="solid" onclick={(e) => sheet?.open(e.currentTarget)}>
+					<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M8 6h13"></path><path d="M8 12h13"></path><path d="M8 18h13"></path><path d="M3 6h.01"></path><path d="M3 12h.01"></path><path d="M3 18h.01"></path></svg>
+					Moves and rolls
+				</button>
+				{#if !resultCard && isPlayer && playing}
+					<button type="button" class="outline" onclick={() => (confirmResign = true)} disabled={busy}>
+						<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 22V4"></path><path d="M4 4h12l-2 4 2 4H4"></path></svg>
+						Resign
+					</button>
+				{/if}
+			</nav>
+		{/if}
+	</div>
+	<MovesSheet bind:this={sheet} {view} {shown} rolling={animating} />
+{:else}
 <main>
 	<header>
 		<a href="/" class="logo">Pothole Chess</a>
@@ -270,8 +361,225 @@
 		</div>
 	{/if}
 </main>
+{/if}
 
 <style>
+	/* Phone layout (under 640px): one screen, no scrolling. */
+	.phone {
+		display: flex;
+		flex-direction: column;
+		height: 100vh;
+		height: 100dvh;
+		overflow: hidden;
+	}
+	.ph-head {
+		display: flex;
+		align-items: center;
+		justify-content: space-between;
+		flex-shrink: 0;
+		height: 48px;
+		padding: 0 4px 0 12px;
+	}
+	.ph-logo {
+		font-family: var(--font-display);
+		font-size: 20px;
+		font-weight: 800;
+		letter-spacing: 0.02em;
+		text-transform: uppercase;
+		color: var(--text);
+		text-decoration: none;
+	}
+	.ph-meta {
+		display: flex;
+		align-items: center;
+		gap: 6px;
+		min-width: 0;
+	}
+	.ph-code {
+		overflow: hidden;
+		font-family: var(--font-mono);
+		font-size: 14px;
+		font-weight: 600;
+		white-space: nowrap;
+		text-overflow: ellipsis;
+		color: var(--text-muted);
+	}
+	.ph-chip {
+		padding: 2px 8px;
+		border: 1px solid var(--line);
+		border-radius: 999px;
+		font-size: 12px;
+		color: var(--text-muted);
+	}
+	.ph-chip.warn {
+		border-color: var(--hazard);
+		color: var(--hazard-text);
+	}
+	.ph-icon {
+		display: flex;
+		align-items: center;
+		justify-content: center;
+		width: 44px;
+		height: 44px;
+		padding: 0;
+		border: 0;
+		background: none;
+		color: var(--text-body);
+		cursor: pointer;
+	}
+	.ph-lost {
+		margin: 0 8px 4px;
+		font-size: 14px;
+		color: var(--hazard-text);
+	}
+	/* Bars and board take the space left over; the board is as big as fits. */
+	.ph-play {
+		display: flex;
+		flex-direction: column;
+		justify-content: center;
+		flex: 1 1 auto;
+		min-height: 0;
+		container-type: size;
+	}
+	.ph-board {
+		width: min(calc(100cqw - 16px), calc(100cqh - 92px));
+		margin: 2px auto;
+	}
+	.ph-card {
+		display: flex;
+		flex-direction: column;
+		gap: 10px;
+		flex-shrink: 0;
+		margin: 6px 8px 0;
+		padding: 12px;
+		background: var(--surface);
+		border: 1px solid var(--surface-2);
+		border-radius: 12px;
+	}
+	.ph-card.accent-line {
+		gap: 4px;
+		border-color: var(--accent-line);
+	}
+	.ph-card.danger-line {
+		border-color: var(--hazard);
+	}
+	.ph-title {
+		font-weight: 600;
+		color: var(--text);
+	}
+	.ph-row {
+		display: flex;
+		gap: 8px;
+	}
+	.ph-row input {
+		flex: 1;
+		min-width: 0;
+		height: 44px;
+		box-sizing: border-box;
+		padding: 0 10px;
+		border: 1px solid var(--line);
+		border-radius: 10px;
+		background: var(--bg);
+		color: var(--text-body);
+		font-family: var(--font-mono);
+		font-size: 13px;
+	}
+	.ph-row .primary {
+		flex-grow: 0;
+	}
+	.ph-two {
+		display: grid;
+		grid-template-columns: repeat(2, minmax(0, 1fr));
+		gap: 8px;
+	}
+	.ph-result {
+		display: flex;
+		align-items: baseline;
+		justify-content: space-between;
+		gap: 8px;
+	}
+	.ph-headline {
+		font-family: var(--font-display);
+		font-size: 28px;
+		font-weight: 800;
+		line-height: 1;
+		text-transform: uppercase;
+		color: var(--accent);
+	}
+	.ph-kicker {
+		font-family: var(--font-mono);
+		font-size: 12px;
+		font-weight: 600;
+		letter-spacing: 0.04em;
+		text-transform: uppercase;
+		color: var(--text-muted);
+	}
+	.ph-detail {
+		font-size: 14px;
+	}
+	.ph-error {
+		margin-bottom: 0;
+		font-size: 14px;
+		color: var(--hazard-text);
+	}
+	.ph-nav {
+		display: grid;
+		grid-template-columns: repeat(2, minmax(0, 1fr));
+		gap: 8px;
+		flex-shrink: 0;
+		margin-top: 8px;
+		padding: 8px 8px max(12px, env(safe-area-inset-bottom));
+		border-top: 1px solid var(--surface-2);
+	}
+	.ph-nav.single {
+		grid-template-columns: 1fr;
+	}
+	.phone button {
+		display: flex;
+		align-items: center;
+		justify-content: center;
+		gap: 8px;
+		min-height: 44px;
+		padding: 0 14px;
+		border-radius: 10px;
+		font: inherit;
+		font-weight: 600;
+		cursor: pointer;
+	}
+	.phone .solid {
+		border: 0;
+		background: var(--surface-2);
+		color: var(--text);
+	}
+	.phone .outline {
+		border: 1px solid var(--line);
+		background: none;
+		color: var(--text);
+	}
+	.phone .primary {
+		border: 0;
+		background: var(--accent);
+		color: var(--accent-text);
+	}
+	.phone .danger {
+		border: 0;
+		background: var(--hazard);
+		color: var(--accent-text);
+	}
+	.phone .ph-icon {
+		padding: 0;
+		border: 0;
+		background: none;
+		color: var(--text-body);
+	}
+	.sr-only {
+		position: absolute;
+		width: 1px;
+		height: 1px;
+		overflow: hidden;
+		clip-path: inset(50%);
+		white-space: nowrap;
+	}
 	main {
 		max-width: 1400px;
 		margin: 0 auto;
PATCH
```

- [ ] **Step 2: Autofix and check**

Run: `npx -y @sveltejs/mcp svelte-autofixer 'web/src/routes/game/[code]/+page.svelte'; pnpm --dir web check && pnpm --dir web test; pnpm --dir web build`
Expected: a clean autofixer run; `0 ERRORS 0 WARNINGS`; `Tests  77 passed (77)`; `Wrote site to "build"`.

- [ ] **Step 3: Look at every state on a phone**

With `go run ./cmd/server` and `pnpm --dir web dev` running, open a game on an iPhone, or in WebKit at iPhone 15 size, as two guests (`localhost` and `[::1]`, or two browser contexts). Walk through: waiting, your move, dice playing out, "Moves and rolls" open, resign confirmation, game over.
Expected:
- each matches its artboard, and none scrolls;
- the board is 365–377 px wide on a 393×659 screen;
- opening the sheet focuses "Close", and Escape or Close returns focus to "Moves and rolls".

- [ ] **Step 4: Commit**

```bash
git add 'web/src/routes/game/[code]/+page.svelte'
git commit -m "web: phone layout for the game screen"
```

---

### Task 6: `playtest --phone`, checks, and docs

`--phone` plays as an iPhone 15 (WebKit by default) with real taps. After every ply it fails if either player's page scrolls or the board is under 90% of the screen's width. At ply 10 it opens and closes the moves sheet. It recognises the phone's result card (`section[aria-label="Game over"]`) as well as the desktop one.

**Files:**
- Modify: `web/scripts/playtest.js`, `docs/superpowers/specs/2026-10-05-phone-layout-design.md`, `CLAUDE.md`

- [ ] **Step 1: Apply the patch**

```bash
git apply <<'PATCH'
diff --git a/web/scripts/playtest.js b/web/scripts/playtest.js
index d3b2cbb..f8842f1 100644
--- a/web/scripts/playtest.js
+++ b/web/scripts/playtest.js
@@ -8,24 +8,26 @@
 //   pnpm --dir web playtest                       # 6 games in Chromium
 //   pnpm --dir web playtest --browser webkit      # Safari's engine
 //   pnpm --dir web playtest --games 12 --drag 0.5 # half the moves by dragging
+//   pnpm --dir web playtest --phone               # as iPhones: taps, the phone layout
 //
 // Against a production build (no ?instant: the dice play out in full), allow
 // each turn longer:
 //
 //   pnpm --dir web playtest --base https://<domain> --games 2 --max-plies 30 --turn-ms 15000
 
-import { chromium, webkit } from 'playwright';
+import { chromium, devices, webkit } from 'playwright';
 import { parseArgs } from 'node:util';
 
 const { values: opts } = parseArgs({
 	options: {
 		games: { type: 'string', default: '6' },
-		browser: { type: 'string', default: 'chromium' },
+		browser: { type: 'string' }, // chromium; webkit with --phone
 		drag: { type: 'string', default: '0.3' }, // share of moves made by dragging
 		base: { type: 'string', default: 'http://localhost:5173' },
 		'max-plies': { type: 'string', default: '1000' },
 		'turn-ms': { type: 'string', default: '3000' }, // how long a move may take to land
-		headed: { type: 'boolean', default: false }
+		headed: { type: 'boolean', default: false },
+		phone: { type: 'boolean', default: false } // play as an iPhone 15, in the phone layout
 	}
 });
 const GAMES = Number(opts.games);
@@ -33,8 +35,11 @@ const DRAG = Number(opts.drag);
 const MAX_PLIES = Number(opts['max-plies']);
 const TURN_MS = Number(opts['turn-ms']);
 const engines = { chromium, webkit };
+opts.browser ??= opts.phone ? 'webkit' : 'chromium';
 if (!engines[opts.browser]) throw new Error(`--browser must be one of ${Object.keys(engines).join(', ')}`);
 
+// The result card: beside the board on desktop, in the card slot on a phone.
+const RESULT = '.result, section[aria-label="Game over"]';
 const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
 const pick = (xs) => xs[Math.floor(Math.random() * xs.length)];
 
@@ -55,6 +60,39 @@ async function clickMove(page) {
 	});
 }
 
+/** Taps a random movable piece, then a random target, as a finger would. */
+async function tapMove(page) {
+	const from = pick(await page.locator('.square.movable').all());
+	const fromLabel = await from.getAttribute('aria-label');
+	await from.tap();
+	const tos = await page.locator('.square.legal').all();
+	if (!tos.length) return { err: `no targets for ${fromLabel}` };
+	const to = pick(tos);
+	const toLabel = await to.getAttribute('aria-label');
+	await to.tap();
+	return { from: fromLabel, to: toLabel };
+}
+
+/** The phone layout's rules: one screen, no scrolling, a board near full width. */
+async function phoneLayout(page) {
+	return page.evaluate(() => {
+		if (!document.querySelector('.phone')) return 'not in the phone layout';
+		if (document.documentElement.scrollHeight > innerHeight + 1) return `the page scrolls (${document.documentElement.scrollHeight} > ${innerHeight})`;
+		const board = document.querySelector('.board')?.getBoundingClientRect().width ?? 0;
+		if (board < innerWidth * 0.9) return `the board is ${Math.round(board)}px on a ${innerWidth}px screen`;
+		return '';
+	});
+}
+
+/** Opens "Moves and rolls" and closes it again. */
+async function sheetOpensAndCloses(page) {
+	await page.getByRole('button', { name: 'Moves and rolls' }).tap();
+	if (!(await page.locator('dialog.sheet[open]').count())) return 'the moves sheet did not open';
+	await page.locator('dialog.sheet').getByRole('button', { name: 'Close' }).tap();
+	if (await page.locator('dialog.sheet[open]').count()) return 'the moves sheet did not close';
+	return '';
+}
+
 /** Drags a random movable piece to a random target with the real mouse. */
 async function dragMove(page) {
 	const from = pick(await page.locator('.square.movable').all());
@@ -75,7 +113,8 @@ async function dragMove(page) {
 }
 
 async function playGame(browser, n) {
-	const contexts = [await browser.newContext(), await browser.newContext()];
+	const device = opts.phone ? { ...devices['iPhone 15'], defaultBrowserType: undefined } : {};
+	const contexts = [await browser.newContext(device), await browser.newContext(device)];
 	const [w, b] = await Promise.all(contexts.map((c) => c.newPage()));
 	const errors = [];
 	for (const [p, who] of [
@@ -94,7 +133,7 @@ async function playGame(browser, n) {
 	await b.goto(url);
 
 	const movable = (p) => p.locator('.square.movable').count();
-	const over = async () => (await w.locator('.result').count()) > 0 && (await b.locator('.result').count()) > 0;
+	const over = async () => (await w.locator(RESULT).count()) > 0 && (await b.locator(RESULT).count()) > 0;
 	let plies = 0;
 	let drags = 0;
 	let idle = 0;
@@ -120,7 +159,7 @@ async function playGame(browser, n) {
 			continue;
 		}
 		idle = 0;
-		const move = Math.random() < DRAG ? await dragMove(mover) : await clickMove(mover);
+		const move = opts.phone ? await tapMove(mover) : Math.random() < DRAG ? await dragMove(mover) : await clickMove(mover);
 		if (move.err) {
 			errors.push(`ply ${plies}: ${move.err}`);
 			break;
@@ -140,9 +179,16 @@ async function playGame(browser, n) {
 		}
 		if (move.dragged) drags++;
 		plies++;
+		if (opts.phone) {
+			const bad = (await phoneLayout(w)) || (await phoneLayout(b)) || (plies === 10 ? await sheetOpensAndCloses(other) : '');
+			if (bad) {
+				errors.push(`ply ${plies}: ${bad}`);
+				break;
+			}
+		}
 	}
-	const result = (await w.locator('.result').innerText().catch(() => '')).replace(/\s*\n+\s*/g, ' · ');
-	const resultBlack = (await b.locator('.result').innerText().catch(() => '')).replace(/\s*\n+\s*/g, ' · ');
+	const result = (await w.locator(RESULT).innerText().catch(() => '')).replace(/\s*\n+\s*/g, ' · ');
+	const resultBlack = (await b.locator(RESULT).innerText().catch(() => '')).replace(/\s*\n+\s*/g, ' · ');
 	if (result !== resultBlack) errors.push(`the two sides show different results: "${result}" / "${resultBlack}"`);
 	const lost = await w.locator('.glyphs').evaluateAll((gs) => gs.map((g) => g.querySelectorAll('img').length));
 	await Promise.all(contexts.map((c) => c.close()));
PATCH
```

- [ ] **Step 2: Phone and desktop play-tests**

With both servers running:

```bash
pnpm --dir web playtest --phone --games 6
pnpm --dir web playtest --games 4
pnpm --dir web playtest --games 4 --browser webkit
```

Expected: `webkit: 6/6 games finished cleanly`, `chromium: 4/4 games finished cleanly`, `webkit: 4/4 games finished cleanly`.

- [ ] **Step 3: Sync the spec with what was built**

In `docs/superpowers/specs/2026-10-05-phone-layout-design.md`:
- In the dice table, change `"Knight saved" (good)` to `"Black knight saved" (good)`; the line names the piece's colour, like the fall line.
- Replace the paragraph starting "The board's size comes from CSS: `--board: …`" with:

```markdown
The bars and board sit in a flexible region with `container-type: size`; the board is `min(100cqw − 16px, 100cqh − 92px)` (92 = the two bars plus margins), so it takes exactly the space the header, card and bottom bar leave. On very short screens (under about 560 px tall) it shrinks below 280 px, which is accepted.
```

- In Testing, change "or the board is narrower than `innerWidth − 20`" to "or the board is narrower than 90% of `innerWidth` (a height-limited board is 365 px on a 393 px phone)".
- In Testing, change "taps instead of clicks, and drags with touch" to "taps instead of clicks (touch drags are left to the hands-on phone check: Playwright has no touch-move)".
- Under "The card slot", item 1, add: "The board dims while waiting."

- [ ] **Step 4: CLAUDE.md**

In "Working notes", after the **Playtest** bullet, add:

```markdown
- **Phones:** under 640 px wide the game page renders its phone layout (canvas row "Phone game: playtest build"). Check phone changes with `pnpm --dir web playtest --phone`, which fails if the page ever scrolls.
```

- [ ] **Step 5: Commit**

```bash
git add web/scripts/playtest.js docs/superpowers/specs/2026-10-05-phone-layout-design.md CLAUDE.md
git commit -m "Phone layout: playtest --phone, spec synced with the build"
```

---

## Done when

- `pnpm --dir web check && pnpm --dir web test && pnpm --dir web build` pass (77 tests).
- `pnpm --dir web playtest --phone --games 6` finishes 6/6 clean, and the desktop play-tests are unchanged (Chromium and WebKit).
- All six phone states match their artboards on a real phone, with no scrolling.
