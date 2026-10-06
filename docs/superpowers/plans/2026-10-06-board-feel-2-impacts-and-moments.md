# Board Feel 2: Impacts and Moments Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give the board its impacts and big moments: a board shake by size, the capture knock-back, the fall into a pothole and the save, catchphrase bubbles, the checkmate burst, the win screen's tally and confetti, and "+5" on the clock (spec sections 11, 4, 5, 8, 6, 7 and 12).

**Architecture:** Pure helpers (geometry, timings, seeded particles, the moments worth a bubble, the increment) live in `feel.ts`, `board.ts`, `catchphrases.ts` and `clock.ts`, with Vitest tests. `Board.svelte` plays the board effects with Svelte out-transitions, the Web Animations API and CSS keyframes, driven by props (`quip`, `mated`) that the game page and the `/dev/board` sandbox compute from the view. The game page adds the result card's flip-in and count-up and a `Confetti.svelte` canvas; `PlayerBar.svelte` shows the "+5". Nothing changes on the server: every effect fits the existing 550 ms dice step.

**Tech Stack:** Svelte 5.57 / SvelteKit 3 (static), TypeScript, Vitest, Playwright (playtest script), Go server unchanged.

**Spec:** `docs/superpowers/specs/2026-10-06-board-feel-design.md` (sections 4, 5, 6, 7, 8, 11, 12 and "Everywhere"). Mockups: https://claude.ai/artifact/E6oCYQhp8xaDTZCVssczXy

**Starts from:** branch `board-feel-1` (plan 1, `docs/superpowers/plans/2026-10-06-board-feel-1-look-and-moves.md`), commit `fe74172`. Work on a new branch `board-feel-2` from it.

## Global Constraints

- Every effect is skipped under reduced motion and in instant mode (`reducedMotion()` from `motion.ts` is true for both) and shows its end state.
- A turn shown at once (first load, reconnect, a hidden tab) plays no one-off effect (burst, confetti, bubble): gate on `anim.animated`, as the repair celebration does.
- Shake sizes: a pawn or minor piece taken 1.5 px / 140 ms; a queen or rook taken 3 px / 200 ms; a fall 4.5 px / 280 ms; checkmate 7 px / 420 ms. One shake at a time; the bigger wins.
- A fall and a save fit one dice step (`STEP_MS`, 550 ms), so `pauseFor` on the server needs no change.
- Bubble lines, verbatim: the Mamdani falls in 😢 "Sorry, the wife is calling." · "Got paperwork to do." · "Ask Bruce Wayne for help." · the Mamdani repairs 👍 "Filled. Next!" · "Another one off the list."; a queen falls in 😱 "Not the queen!" · "Mind the gap."; a saving roll saves a queen or rook 😅 "That was close." Never the same line twice in a row. Text only, no sound.
- Confetti only for the winner, only when the game ended while they watched; never for a draw, a loss or a spectator.
- Svelte: avoid `$effect` (use attachments and `$derived`); imports use `#lib/...` with the file extension inside routes and `./x.ts` inside `lib`; run `npx @sveltejs/mcp svelte-autofixer <file>` on every component you change.
- The phone page (under 640 px) must never scroll, and the board must not move when the result card appears (at 393 px wide).
- Commits: stage only the files you changed (`git add <paths>`, never `-A`); no Claude attribution lines.

## Review Focus

- **A capture by drag-and-drop:** the dropped piece is already on the square, so the taken piece is hit at once (`impact()` returns 0), not half a glide later. Checked by hand in Task 1, step 7.
- **A reload or reconnect after a checkmate, and a rematch offer arriving after it:** no burst, no confetti, no count-up replays; the red square and badge do not reappear after a reload. Covered by `endedHere`/`wonHere` tests (Task 4) and the `anim.animated` gate (Task 3); check by reloading in Task 6.
- **Two big moments in one turn** (the Mamdani repairs on its move, then the roll drops a queen): the bubble follows the latest revealed one and each gets its own key, so a second bubble pops. Covered by `quipFor` scanning from the latest revealed event (Task 2 tests).
- **A deploy restoring a clock** (the side to move's clock jumps back up and runs): no "+5". Covered by `bonusOf` ignoring a running clock (Task 5 test).
- **The phone result card with the tally at 320–393 px:** no page scroll and, at 393 px, the board stays put. Measured in Task 4, step 9; the tally hides under 360 px.

## Departures from the spec

Decided while building the scratch copy; each is in the code below.

- **Screen readers:** the bubble's line goes to a live region on the board (`<p class="sr-only" aria-live="polite">`), not to the toast region: `notify` would show a visible toast, and the spec wants none.
- **After a checkmate the result card waits** `BURST_HOLD_MS` (1.8 s, the same delay as the board's dim), and so do the count-up and the confetti: shown at once, the card covered the burst.
- **The desktop side panel's stats list becomes the tally** (moves, saving rolls, saved by the Mamdani, potholes repaired, pieces lost to potholes; "The Mamdani fell in" stays). Its per-side lost counts and "X of Y saved" go; the phone's moves sheet keeps its own list.
- **The phone result card** gets the tally as a one-line footnote, with tighter padding so the board doesn't move at 393 px; under 360 px the tally is hidden (the headline already wraps there).
- **Bubble timing:** after a fall the bubble waits for the fall (`FALL_MS`), so the piece drops in first.

---

### Task 1: Board shake, capture knock-back, the fall and the save (sections 11, 4, 5)

**Files:**
- Modify: `web/src/lib/feel.ts` (append), `web/src/lib/feel.test.ts`
- Modify: `web/src/lib/board.ts` (`stageAt`), `web/src/lib/board.test.ts`
- Modify: `web/src/lib/Board.svelte`
- Modify: `web/scripts/playtest.js` (`holesMatch`)

**Interfaces:**
- Consumes: `moveDuration(from, to)` from `pieces.ts`; `lastMove`, `dropped`, `flipped`, `saved` and `stage.target` already in `Board.svelte`; `reducedMotion()` from `motion.ts`.
- Produces: in `feel.ts`: `FALL_MS = 550`; `type Shake = { px: number; ms: number }`; `SHAKE: { minor, major, fall, mate }`; `captureShake(code: string): Shake`; `biggerShake(current: Shake | null, next: Shake): Shake`; `shakeFrames(px: number): Keyframe[]`; `knockOffset(from: string, to: string, flipped: boolean): { x: number; y: number }`. In `Board.svelte`: `shake(s: Shake, delay?: number)` and `impact(): number` (Task 3 uses both).

- [ ] **Step 1: Write the failing tests**

In `web/src/lib/feel.test.ts`, add `biggerShake, captureShake, knockOffset, SHAKE, shakeFrames` to the import from `./feel.ts`, and append:


```ts
describe('shakes', () => {
	it('shakes harder for a queen or rook than for a minor piece', () => {
		expect(captureShake('bP')).toEqual(SHAKE.minor);
		expect(captureShake('wN')).toEqual(SHAKE.minor);
		expect(captureShake('bQ')).toEqual(SHAKE.major);
		expect(captureShake('wR')).toEqual(SHAKE.major);
	});
	it('lets the bigger shake win', () => {
		expect(biggerShake(SHAKE.minor, SHAKE.fall)).toEqual(SHAKE.fall);
		expect(biggerShake(SHAKE.mate, SHAKE.minor)).toEqual(SHAKE.mate);
		expect(biggerShake(null, SHAKE.major)).toEqual(SHAKE.major);
	});
	it('zigzags, dies away and ends still', () => {
		const frames = shakeFrames(4);
		const xs = frames.map((k) => Number(/translate\((-?[\d.]+)px/.exec(String(k.transform))![1]));
		expect(Math.abs(xs[0])).toBe(4);
		expect(xs.at(-1)).toBe(0);
		for (let i = 1; i < xs.length - 1; i++) expect(Math.sign(xs[i])).toBe(-Math.sign(xs[i - 1]));
		for (let i = 1; i < xs.length; i++) expect(Math.abs(xs[i])).toBeLessThanOrEqual(Math.abs(xs[i - 1]));
	});
});

describe('knockOffset', () => {
	it('knocks the taken piece a third of a square along the move', () => {
		expect(knockOffset('a1', 'h1', false)).toEqual({ x: 33, y: 0 });
		expect(knockOffset('e2', 'e4', false)).toEqual({ x: 0, y: -33 });
		const k = knockOffset('g1', 'f3', false);
		expect(Math.hypot(k.x, k.y)).toBeCloseTo(33, 0);
	});
	it('follows the board when it is flipped', () => {
		expect(knockOffset('a1', 'h1', true)).toEqual({ x: -33, y: 0 });
	});
});
```


In `web/src/lib/board.test.ts`, add this test inside `describe('stageAt', ...)`, right after its first test ("keeps the fallen piece and hides the new pothole until they are revealed"). It uses that block's existing `final` fixture, whose turn has a fall on g8 (event 5) and then `pothole_opened` on g8 (event 6):


```ts
	it("opens a fall's hole in the same step the piece falls, so it drops into it", () => {
		const s = stageAt(final, 6);
		expect(s.board[squareIndex('g8')]).toBe('');
		expect(s.potholes).toEqual([{ sq: 'g8', by: 'white', left: 1 }]);
	});
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts src/lib/board.test.ts`
Expected: FAIL: the shake and knock tests with `captureShake is not a function` (or similar), and "opens a fall's hole in the same step" with `expected [] to deeply equal [ { sq: 'g8', … } ]`.

- [ ] **Step 3: Implement the helpers**

Append to `web/src/lib/feel.ts`:


```ts
/** How long a fall into a pothole plays: a teeter, then the drop. It fits one dice step (STEP_MS). */
export const FALL_MS = 550;

/** A board shake: how far it moves and for how long. */
export type Shake = { px: number; ms: number };

/** The shakes by size of moment: the bigger the moment, the bigger the shake. */
export const SHAKE = {
	minor: { px: 1.5, ms: 140 }, // a pawn, knight or bishop taken
	major: { px: 3, ms: 200 }, // a queen or rook taken
	fall: { px: 4.5, ms: 280 }, // a piece falls into a hole
	mate: { px: 7, ms: 420 } // checkmate
} as const satisfies Record<string, Shake>;

/** The shake for taking the piece `code` ("bQ", "wP"). */
export function captureShake(code: string): Shake {
	return code[1] === 'Q' || code[1] === 'R' ? SHAKE.major : SHAKE.minor;
}

/** One shake at a time: the bigger wins. */
export function biggerShake(current: Shake | null, next: Shake): Shake {
	return current && current.px >= next.px ? current : next;
}

/** A decaying zigzag of `px`, ending still. */
export function shakeFrames(px: number): Keyframe[] {
	const frames: Keyframe[] = [];
	for (let i = 0; i < 8; i++) {
		const f = 1 - i / 8;
		const x = Math.round((i % 2 ? -1 : 1) * px * f * 100) / 100;
		const y = Math.round((i % 3 === 1 ? 1 : i % 3 === 2 ? -1 : 0) * px * 0.5 * f * 100) / 100;
		frames.push({ transform: `translate(${x}px, ${y}px)` });
	}
	frames.push({ transform: 'translate(0px, 0px)' });
	return frames;
}

/** Where a taken piece is knocked: a third of a square along the move, in % of a square. */
export function knockOffset(from: string, to: string, flipped: boolean): { x: number; y: number } {
	const a = cellOf(from, flipped);
	const b = cellOf(to, flipped);
	const dx = b.col - a.col;
	const dy = b.row - a.row;
	const d = Math.hypot(dx, dy) || 1;
	const r = (v: number) => Math.round(v * 10) / 10 || 0;
	return { x: r((33 * dx) / d), y: r((33 * dy) / d) };
}
```


In `web/src/lib/board.ts`, `stageAt`, replace the line


```ts
		if (e.kind === 'pothole_opened' && !revealed) potholes = potholes.filter((p) => p.sq !== e.sq);
```


with


```ts
		// A fall's hole opens in the same step as the fall, so the piece drops
		// into it (the server sends the fall, then the hole).
		const prev = view.last[i - 1];
		const withFall = prev?.kind === 'fell' && prev.sq === e.sq && i - 1 < shown;
		if (e.kind === 'pothole_opened' && !revealed && !withFall) potholes = potholes.filter((p) => p.sq !== e.sq);
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts src/lib/board.test.ts`
Expected: PASS, all tests.

- [ ] **Step 5: Play them on the board**

In `web/src/lib/Board.svelte`:

1. Change the `./feel.ts` import from


```ts
	import { cellOf, coordinates, GLIDE_EASE, rippleDelay, trailColor, trailOf, TRAIL_FADE_MS, whipFrames, whiplash, WHIP_TAIL_MS } from './feel.ts';
```


   to:


```ts
	import { biggerShake, captureShake, cellOf, coordinates, FALL_MS, GLIDE_EASE, knockOffset, rippleDelay, SHAKE, shakeFrames, TRAIL_FADE_MS, trailColor, trailOf, type Shake, WHIP_TAIL_MS, whipFrames, whiplash } from './feel.ts';
```


2. In `motion()`, a saved piece teeters and hops out. Replace the start of the returned function, from `return (node: HTMLElement) => {` through `if (reducedMotion()) return;`, with:


```ts
		return (node: HTMLElement) => {
			const settled = bounce.sq === p.sq ? `bounce:${bounce.n}` : '';
			const savedHere = saved === p.sq;
			if (!saved) delete node.dataset.saved;
			if (reducedMotion()) return;
			// Saved by an odd saving roll: it teeters hard, then hops out as
			// the hole closes under it.
			if (savedHere && node.dataset.saved !== saved) {
				node.dataset.saved = saved;
				node.animate(SAVE_HOP, { duration: 640, easing: 'ease-out' });
				return;
			}
```


   and after `const SQUASH: Keyframe[] = ...;` add:


```ts
	const SAVE_HOP: Keyframe[] = [
		{ transform: 'rotate(0)' },
		{ transform: 'rotate(-14deg)', offset: 0.16 },
		{ transform: 'rotate(14deg)', offset: 0.32 },
		{ transform: 'rotate(0)', offset: 0.41 },
		{ transform: 'translateY(-22%) scale(1.08)', offset: 0.7 },
		{ transform: 'translateY(0) scale(1)' }
	];
```


3. Replace the whole `leave` function (under `// Transitions. Each one collapses to nothing under reduced motion.`):


```ts
	/** A piece leaving the board: a fall shrinks into the hole, a capture fades. */
	function leave(_node: Element, { fell }: { fell: boolean }) {
		return fell
			? { duration: ms(450), css: (t: number) => `opacity: ${t}; transform-origin: 50% 50%; transform: scale(${0.25 + 0.75 * t}) rotate(${(1 - t) * 25}deg)` }
			: { duration: ms(300), css: (t: number) => `opacity: ${1 - (1 - t) ** 3}` };
	}
```


   with:


```ts
	// One shake at a time on the whole frame: a bigger one replaces a smaller
	// one still running, a smaller one waits its turn out.
	let frameEl: HTMLDivElement | undefined;
	let shaking: { shake: Shake; anim: Animation } | null = null;
	function shake(s: Shake, delay = 0) {
		if (reducedMotion() || !frameEl) return;
		const running = shaking?.anim.playState === 'running' ? shaking.shake : null;
		if (biggerShake(running, s) !== s) return;
		shaking?.anim.cancel();
		shaking = { shake: s, anim: frameEl.animate(shakeFrames(s.px), { duration: s.ms, delay, easing: 'linear' }) };
	}

	/**
	 * When the move's mover reaches its square, so a taken piece is hit then:
	 * half its glide, since the ease-out glide has covered 94% of the way by
	 * then. A dropped piece is already there.
	 */
	function impact(): number {
		if (!lastMove || reducedMotion() || (dropped && dropped.from === lastMove.from && dropped.to === lastMove.to)) return 0;
		return Math.round(moveDuration(lastMove.from, lastMove.to) / 2);
	}

	/**
	 * A fall, `u` from 0 to 1 over 550 ms: two wobbles at the edge (180 ms),
	 * then a drop into the hole, shrinking, tipping and darkening.
	 */
	function fallFrame(u: number): string {
		const teeter = 180 / 550;
		if (u < teeter) return `transform-origin: 50% 50%; transform: rotate(${(9 * Math.sin((u / teeter) * 2 * Math.PI)).toFixed(2)}deg)`;
		const q = ((u - teeter) / (1 - teeter)) ** 2;
		return `transform-origin: 50% 50%; transform: translateY(${10 * q}%) scale(${1 - 0.85 * q}) rotate(${40 * q}deg); filter: brightness(${1 - 0.75 * q}); opacity: ${1 - q}`;
	}

	/**
	 * A piece leaving the board. A fall teeters, then drops into the hole and
	 * the board shakes. A capture waits for the mover to arrive, then is
	 * knocked a third of a square along the move, spins and fades, and the
	 * board shakes by the taken piece's size.
	 */
	function leave(node: Element, { fell, code }: { fell: boolean; code: string }) {
		// A taken piece waits under the mover, which lands on top of it.
		if (!fell && node.parentElement) node.parentElement.style.zIndex = '1';
		if (fell) {
			shake(SHAKE.fall, ms(180));
			return { duration: ms(FALL_MS), css: (_t: number, u: number) => fallFrame(u) };
		}
		const hit = impact();
		const k = lastMove ? knockOffset(lastMove.from, lastMove.to, flipped) : { x: 0, y: 0 };
		const spin = k.x < 0 ? -22 : 22;
		shake(captureShake(code), hit);
		return {
			delay: hit,
			duration: ms(320),
			css: (t: number, u: number) => {
				const e = 1 - (1 - u) ** 3;
				return `transform: translate(${k.x * e}%, ${k.y * e}%) rotate(${spin * e}deg) scale(${1 - 0.15 * e}); opacity: ${t}`;
			}
		};
	}

	/** The orange ring on a capture's square, as the taken piece is hit. */
	function ringOut(_node: Element, { fell }: { fell: boolean }) {
		if (fell || reducedMotion()) return { duration: 1, css: () => 'opacity: 0' };
		return { delay: impact(), duration: 360, css: (_t: number, u: number) => `opacity: ${1 - u}; transform: scale(${0.4 + 0.75 * u})` };
	}

	/** Dust puffing off the rim as a piece drops in. */
	function puff(_node: Element, { fell }: { fell: boolean }) {
		if (!fell || reducedMotion()) return { duration: 1, css: () => 'opacity: 0' };
		return { delay: 180, duration: 480, css: (_t: number, u: number) => `--p: ${u}; opacity: ${1 - u}` };
	}
	const DUST = [190, 215, 240, 265, 290, 315, 340, 355];
```


4. The frame registers itself for the shake. Change `<div class="frame" class:dim>` to:


```svelte
<div class="frame" class:dim {@attach (node) => void (frameEl = node)}>
```


5. In the first `<div class="layer" aria-hidden="true">` (the potholes layer), add first, before `{#each stage.potholes ...}`:


```svelte
		{#if saved && !reducedMotion()}
			{#key saved}
				<span class="slot" style={place(saved)}><span class="hole briefly"></span></span>
			{/key}
		{/if}
```


   and in the pieces layer, inside each `.piece-slot`, replace the `<span class="piece" out:leave={{ fell: stage.target === p.sq }} {@attach motion(p)}>` line with:


```svelte
				<span class="ring" out:ringOut={{ fell: stage.target === p.sq }}></span>
				<span class="dust" out:puff={{ fell: stage.target === p.sq }}>{#each DUST as a (a)}<i style="--a: {a}deg"></i>{/each}</span>
				<span class="piece" out:leave={{ fell: stage.target === p.sq, code: p.code }} {@attach motion(p)}>
```


6. Add to the end of the `<style>` block:


```css
	/* A capture: an orange ring on the square as the taken piece is hit. */
	.ring {
		position: absolute;
		inset: 0;
		border: 3px solid var(--hazard);
		border-radius: 50%;
		opacity: 0;
		pointer-events: none;
	}
	/* A fall: dust puffing off the hole's rim (--p runs 0 to 1). */
	.dust {
		position: absolute;
		inset: 0;
		opacity: 0;
		pointer-events: none;
	}
	.dust i {
		position: absolute;
		left: 50%;
		top: 62%;
		width: 11%;
		aspect-ratio: 1;
		border-radius: 50%;
		background: var(--text-body);
		transform: translate(-50%, -50%) rotate(var(--a)) translateX(calc(var(--p, 0) * 300%)) scale(calc(1 + var(--p, 0) * 0.8));
	}
	/* A save: the hole cracks open under the piece, then closes as it hops out. */
	.hole.briefly {
		animation: briefly 0.75s ease-in-out both;
	}
	@keyframes briefly {
		0% {
			transform: scale(0);
			opacity: 0;
		}
		20% {
			transform: scale(1.1);
			opacity: 1;
		}
		35%,
		55% {
			transform: scale(1);
			opacity: 1;
		}
		100% {
			transform: scale(0);
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.dust {
			display: none;
		}
	}
```


7. In `web/scripts/playtest.js`, `holesMatch`, the save's brief hole is not a pothole. Replace


```js
			document.querySelectorAll('.layer .slot > .hole:not(.patched)').length,
```


with


```js
			// Not counted: a repaired hole, or the one a saving roll flashes open.
			document.querySelectorAll('.layer .slot > .hole:not(.patched):not(.briefly)').length,
```


- [ ] **Step 6: Check the code**

Run: `pnpm --dir web check && npx @sveltejs/mcp svelte-autofixer web/src/lib/Board.svelte`
Expected: `0 ERRORS 0 WARNINGS`; the autofixer reports no issues.

- [ ] **Step 7: Watch it in the sandbox**

Run `go run ./cmd/server` and `pnpm --dir web dev`, open `http://localhost:5173/dev/board`.

- Tick "Move either side any time" and "Odd: nothing happens". Play e2-e4, d7-d5, then e4xd5 by clicking. Expected: the black pawn stays under the white pawn until it arrives, then is knocked up and to the left (along the move), spins and fades, an orange ring flashes on d5, the board shakes slightly.
- Repeat the capture by dragging the pawn. Expected: the knock and ring happen at once on the drop.
- Choose "Piece falls (no saving roll)", target `d7` (or any square with a piece), move. Expected: the hole cracks open under the piece, which stays on top; it wobbles twice, then drops in (shrinking, tipping, darkening), dust puffs off the rim, the board shakes, the cones appear.
- Choose a saving roll that saves (odd), aimed at a piece away from the Mamdani. Expected: a hole flashes open under the piece, it teeters and hops up, the hole closes, the piece stays.
- With the system set to reduce motion: none of the above moves; the piece is simply gone or stays.

- [ ] **Step 8: Run the suite and commit**

Run: `pnpm --dir web test`
Expected: PASS.

```bash
git add web/src/lib/feel.ts web/src/lib/feel.test.ts web/src/lib/board.ts web/src/lib/board.test.ts web/src/lib/Board.svelte web/scripts/playtest.js
git commit -m "web: captures knock the taken piece back, pieces teeter into potholes, saves hop out, and the board shakes by size"
```

---

### Task 2: Catchphrase bubbles (section 8)

**Files:**
- Create: `web/src/lib/catchphrases.ts`, `web/src/lib/catchphrases.test.ts`
- Modify: `web/src/lib/Board.svelte`, `web/src/routes/dev/board/+page.svelte`, `web/src/routes/game/[code]/+page.svelte`

**Interfaces:**
- Consumes: `FALL_MS` from `feel.ts` (Task 1); `EventJSON` from `game.ts`.
- Produces: `quipper(random?)` returning `(events: EventJSON[], seq: number, shown: number) => { sq: string; emoji: string; line: string; key: string; delay: number } | null`; Board prop `quip`.

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/catchphrases.test.ts`:


```ts
import { describe, expect, it } from 'vitest';
import { pickLine, quipper, QUIPS, quipFor } from './catchphrases.ts';
import type { EventJSON } from './game.ts';

const roll: EventJSON[] = [{ kind: 'moved', from: 'e2', to: 'e4' }, { kind: 'rolled_pothole', roll: 4 }, { kind: 'target', sq: 'a5' }];

describe('quipFor', () => {
	it('finds the Mamdani falling in, once that step is revealed', () => {
		const events = [...roll, { kind: 'saving_roll', sq: 'a5', piece: 'M', roll: 2, saved: false }, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }];
		expect(quipFor(events, 4)).toBeNull();
		expect(quipFor(events, 5)).toEqual({ kind: 'mamdaniFell', sq: 'a5', index: 4 });
		expect(quipFor(events, 6)).toEqual({ kind: 'mamdaniFell', sq: 'a5', index: 4 });
	});
	it('finds a queen falling in and the Mamdani repairing a hole', () => {
		expect(quipFor([...roll, { kind: 'fell', sq: 'd8', piece: 'bQ' }], 4)).toMatchObject({ kind: 'queenFell', sq: 'd8' });
		expect(quipFor([{ kind: 'moved', from: 'a5', to: 'b6' }, { kind: 'repaired', sq: 'c7' }], 2)).toMatchObject({ kind: 'repaired', sq: 'c7' });
	});
	it('finds a saving roll that saves a queen or a rook, not a pawn', () => {
		expect(quipFor([...roll, { kind: 'saving_roll', sq: 'a1', piece: 'wR', roll: 3, saved: true }], 4)).toMatchObject({ kind: 'saved', sq: 'a1' });
		expect(quipFor([...roll, { kind: 'saving_roll', sq: 'a2', piece: 'wP', roll: 3, saved: true }], 4)).toBeNull();
		expect(quipFor([...roll, { kind: 'fell', sq: 'a2', piece: 'wP' }], 4)).toBeNull();
	});
});

describe('pickLine', () => {
	it('never picks the same line twice in a row', () => {
		const lines = QUIPS.mamdaniFell.lines;
		for (const previous of lines) {
			for (const r of [0, 0.3, 0.6, 0.99]) expect(pickLine('mamdaniFell', previous, () => r)).not.toBe(previous);
		}
	});
	it('picks from the event\'s lines, the only one when there is one', () => {
		expect(QUIPS.mamdaniFell.lines).toContain(pickLine('mamdaniFell', undefined, () => 0.5));
		expect(pickLine('saved', 'That was close.', () => 0)).toBe('That was close.');
	});
});

describe('quipper', () => {
	const fall: EventJSON[] = [...roll, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }];
	it('keeps the same line for a moment while the board redraws', () => {
		const say = quipper(() => 0.5);
		const first = say(fall, 7, 4);
		expect(first).toMatchObject({ sq: 'a5', emoji: '😢', key: '7:3', delay: 550 });
		expect(say(fall, 7, 5)).toEqual(first);
	});
	it('waits for a fall to finish, but not for a repair', () => {
		const repair: EventJSON[] = [{ kind: 'moved', from: 'a5', to: 'b6' }, { kind: 'repaired', sq: 'c7' }];
		expect(quipper()(repair, 3, 2)).toMatchObject({ delay: 0 });
	});
	it('says nothing before the moment is revealed', () => {
		expect(quipper()(fall, 7, 3)).toBeNull();
	});
	it('never repeats the last line on the next moment', () => {
		const say = quipper(() => 0);
		const a = say(fall, 7, 4)!.line;
		const b = say(fall, 9, 4)!.line;
		expect(b).not.toBe(a);
	});
});
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `pnpm --dir web exec vitest run src/lib/catchphrases.test.ts`
Expected: FAIL: `Cannot find module './catchphrases.ts'` (Vitest may word it `Failed to resolve import`).

- [ ] **Step 3: Write the module**

Create `web/src/lib/catchphrases.ts`:


```ts
// What the board says at big moments: a speech bubble from the square, with
// an emoji and a line (spec: board feel, section 8). The lines are the game
// piece's playful voice, never presented as real quotes.

import { FALL_MS } from './feel.ts';
import type { EventJSON } from './game.ts';

export type QuipKind = 'mamdaniFell' | 'repaired' | 'queenFell' | 'saved';

export const QUIPS: Record<QuipKind, { emoji: string; lines: string[] }> = {
	mamdaniFell: {
		emoji: '😢',
		lines: ['Sorry, the wife is calling.', 'Got paperwork to do.', 'Ask Bruce Wayne for help.']
	},
	repaired: { emoji: '👍', lines: ['Filled. Next!', 'Another one off the list.'] },
	queenFell: { emoji: '😱', lines: ['Not the queen!', 'Mind the gap.'] },
	saved: { emoji: '😅', lines: ['That was close.'] }
};

/**
 * The latest big moment among the first `shown` events of a turn: the
 * Mamdani or a queen falling in, the Mamdani repairing a hole, or a saving
 * roll saving a queen or rook. Smaller moments (a pawn saved) say nothing.
 */
export function quipFor(events: EventJSON[], shown: number): { kind: QuipKind; sq: string; index: number } | null {
	for (let i = Math.min(shown, events.length) - 1; i >= 0; i--) {
		const e = events[i];
		if (!e.sq) continue;
		const kind = e.piece?.[1];
		if (e.kind === 'fell' && e.piece === 'M') return { kind: 'mamdaniFell', sq: e.sq, index: i };
		if (e.kind === 'fell' && kind === 'Q') return { kind: 'queenFell', sq: e.sq, index: i };
		if (e.kind === 'repaired') return { kind: 'repaired', sq: e.sq, index: i };
		if (e.kind === 'saving_roll' && e.saved && (kind === 'Q' || kind === 'R')) return { kind: 'saved', sq: e.sq, index: i };
	}
	return null;
}

/** A line for `kind`, picked at random but never `previous` again when there's another. */
export function pickLine(kind: QuipKind, previous: string | undefined, random: () => number = Math.random): string {
	const lines = QUIPS[kind].lines;
	const choices = lines.length > 1 ? lines.filter((l) => l !== previous) : lines;
	return choices[Math.min(choices.length - 1, Math.floor(random() * choices.length))];
}

/**
 * Remembers the line picked for each moment, so it stays the same while the
 * board redraws, and the line said last, so the next moment says another.
 * Returns the bubble for the first `shown` events of turn `seq`, or null.
 * After a fall it waits `delay` ms, the fall's length, so the piece drops first.
 */
export function quipper(random: () => number = Math.random) {
	const said = new Map<string, string>();
	let last: string | undefined;
	return (events: EventJSON[], seq: number, shown: number): { sq: string; emoji: string; line: string; key: string; delay: number } | null => {
		const q = quipFor(events, shown);
		if (!q) return null;
		const key = `${seq}:${q.index}`;
		let line = said.get(key);
		if (!line) {
			line = pickLine(q.kind, last, random);
			said.set(key, line);
			last = line;
		}
		const delay = q.kind === 'mamdaniFell' || q.kind === 'queenFell' ? FALL_MS : 0;
		return { sq: q.sq, emoji: QUIPS[q.kind].emoji, line, key, delay };
	};
}
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `pnpm --dir web exec vitest run src/lib/catchphrases.test.ts`
Expected: PASS (9 tests).

- [ ] **Step 5: Draw the bubble**

In `web/src/lib/Board.svelte`:

1. Add the prop: in the destructuring add `quip = null,` after `repairs = [],`, and in the type after the `repairs` member:


```ts
		/** A speech bubble for a big moment (catchphrases.ts); a new key pops a new one. */
		quip?: { sq: string; emoji: string; line: string; key: string; delay?: number } | null;
```


2. In the `<div class="layer celebrate" aria-hidden="true">` layer, after the `{#each fresh as r (r.key)} ... {/each}` block and before `{#if fresh.length > 0 && stage.mamdani}`, add:


```svelte
		{#if quip}
			{#key quip.key}
				<span class="slot fix quip" class:top={cell(quip.sq).row < 2} class:left={cell(quip.sq).col === 0} class:right={cell(quip.sq).col === 7} style="{place(quip.sq)}; --quip-delay: {quip.delay ?? 0}ms">
					<span class="bubble"><em>{quip.emoji}</em> {quip.line}</span>
				</span>
			{/key}
		{/if}
```


3. After that layer's closing `</div>`, add the line screen readers hear:


```svelte
	<!-- Screen readers hear the bubble's line; the bubble itself is drawn above. -->
	<p class="sr-only" aria-live="polite">{quip?.line ?? ''}</p>
```


4. Add to the end of the `<style>` block:


```css
	/* A big moment: a speech bubble from the square, for about 2.8 s. */
	.quip {
		z-index: 6;
		overflow: visible;
	}
	.bubble {
		position: absolute;
		left: 50%;
		bottom: 88%;
		translate: -50% 0;
		width: max-content;
		max-width: min(280px, 72vw);
		padding: 7px 11px;
		border-radius: 12px;
		background: var(--piece-light);
		color: var(--piece-dark);
		font: 600 14px/1.25 var(--font-body);
		box-shadow: 0 4px 14px var(--hole);
		transform-origin: 50% 100%;
		animation:
			bubble-in 0.26s ease-out var(--quip-delay, 0ms) both,
			bubble-out 0.25s ease-in calc(var(--quip-delay, 0ms) + 2.8s) forwards;
	}
	.bubble em {
		font-style: normal;
		font-size: 17px;
	}
	/* Its tail points at the square's centre (50cqw: half the square). */
	.bubble::after {
		content: '';
		position: absolute;
		top: 100%;
		left: 50%;
		translate: -50% 0;
		border: 7px solid transparent;
		border-top-color: var(--piece-light);
		border-bottom: 0;
	}
	.left .bubble {
		left: 0;
		translate: 0 0;
		transform-origin: 0 100%;
	}
	.left .bubble::after {
		left: 50cqw;
	}
	.right .bubble {
		left: auto;
		right: 0;
		translate: 0 0;
		transform-origin: 100% 100%;
	}
	.right .bubble::after {
		left: auto;
		right: 50cqw;
		translate: 50% 0;
	}
	.top .bubble {
		bottom: auto;
		top: 88%;
		transform-origin: 50% 0;
	}
	.top .bubble::after {
		top: auto;
		bottom: 100%;
		border: 7px solid transparent;
		border-bottom-color: var(--piece-light);
		border-top: 0;
	}
	@keyframes bubble-in {
		0% {
			scale: 0.6;
			opacity: 0;
		}
		70% {
			scale: 1.06;
			opacity: 1;
		}
		100% {
			scale: 1;
		}
	}
	@keyframes bubble-out {
		to {
			opacity: 0;
			visibility: hidden;
		}
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	/* Reduced motion: the bubble just shows, then goes. */
	@media (prefers-reduced-motion: reduce) {
		.bubble {
			animation: bubble-out 0.25s ease-in calc(var(--quip-delay, 0ms) + 2.8s) forwards;
		}
	}
```


In `web/src/routes/dev/board/+page.svelte`, add `import { quipper } from '#lib/catchphrases.ts';` after the `#lib/board.ts` import, after the `let repairs = ...` line add:


```ts
	// Big moments say something, as in a game.
	const say = quipper();
	let quip = $derived(anim.animated && !instant ? say(view.last, view.seq, anim.shown) : null);
```


and pass `{quip}` to `<Board ... />` after `{repairs}`.

In `web/src/routes/game/[code]/+page.svelte`, add `import { quipper } from '#lib/catchphrases.ts';` after the `#lib/board.ts` import; after the `let repairs = ...` line add:


```ts
	// A speech bubble for a big moment: only on a turn that is playing out, like the repairs.
	const say = quipper();
	let quip = $derived(view && anim.animated && !instant ? say(view.last, view.seq, shown) : null);
```


and pass `{quip}` to both `<Board ... />` elements (phone and desktop) after `{repairs}`.

- [ ] **Step 6: Check the code**

Run: `pnpm --dir web check && npx @sveltejs/mcp svelte-autofixer web/src/lib/Board.svelte && npx @sveltejs/mcp svelte-autofixer 'web/src/routes/game/[code]/+page.svelte'`
Expected: `0 ERRORS 0 WARNINGS`; the autofixer reports no issues (it may suggest `SvelteSet` for the page's `streams` set, which is not reactive state: leave it).

- [ ] **Step 7: Watch it in the sandbox**

In `/dev/board`, tick "Mamdani falls" and move a pawn. Expected: the Mamdani teeters and drops in first, then a cream bubble with 😢 and one of the four lines pops up from the hole, its tail pointing at the square, stays about 2.8 s and fades. On the a- or h-file the bubble stays inside the board; on the top two rows it hangs below the square. Do it again: a different line. With reduced motion, the bubble shows without the pop and still fades.

- [ ] **Step 8: Run the suite and commit**

Run: `pnpm --dir web test`
Expected: PASS.

```bash
git add web/src/lib/catchphrases.ts web/src/lib/catchphrases.test.ts web/src/lib/Board.svelte web/src/routes/dev/board/+page.svelte 'web/src/routes/game/[code]/+page.svelte'
git commit -m "web: big moments say something: a speech bubble from the square, for players and spectators"
```

---

### Task 3: Checkmate burst (section 6)

**Files:**
- Modify: `web/src/lib/feel.ts`, `web/src/lib/feel.test.ts`
- Modify: `web/src/lib/board.ts`, `web/src/lib/board.test.ts`
- Modify: `web/src/lib/theme/tokens.css`
- Modify: `web/src/lib/Board.svelte`, `web/src/routes/dev/board/+page.svelte`, `web/src/routes/game/[code]/+page.svelte`

**Interfaces:**
- Consumes: `shake()`, `impact()` and `SHAKE` (Task 1); `markCelebrated`, `celebratedSoFar`/`playedBefore` and the `coneShape` snippet already in `Board.svelte`.
- Produces: `burstShards(count?, seed?)`, `BURST_HOLD_MS = 1800` in `feel.ts`; `matedKing(view: View): string` in `board.ts`; Board prop `mated: { sq: string; key: string } | null`; the page's `mated` (Task 4 reads it).

- [ ] **Step 1: Write the failing tests**

In `web/src/lib/feel.test.ts` add `burstShards` to the import and append:


```ts
describe('burstShards', () => {
	it('throws 18 shards all the way round, one in three orange', () => {
		const shards = burstShards();
		expect(shards).toHaveLength(18);
		expect(shards.filter((s) => s.hazard)).toHaveLength(6);
		for (const s of shards) {
			expect(s.dist).toBeGreaterThanOrEqual(2);
			expect(s.dist).toBeLessThanOrEqual(5);
		}
		const angles = shards.map((s) => s.angle).sort((a, b) => a - b);
		expect(angles[0]).toBeLessThan(40);
		expect(angles.at(-1)).toBeGreaterThan(320);
	});
	it('is the same burst every time, so a test can pin it', () => {
		expect(burstShards()).toEqual(burstShards());
	});
});
```


In `web/src/lib/board.test.ts` add `matedKing` to the import from `./board.ts` and append:


```ts
describe('matedKing', () => {
	it("finds the mated king's square", () => {
		const v = makeView({ e8: 'bK', e1: 'wK' }, { status: 'over', result: { winner: 'white', draw: false, reason: 'checkmate' } });
		expect(matedKing(v)).toBe('e8');
	});
	it('is empty for any other ending', () => {
		const v = makeView({ e8: 'bK', e1: 'wK' }, { status: 'over', result: { winner: 'white', draw: false, reason: 'resignation' } });
		expect(matedKing(v)).toBe('');
		expect(matedKing(makeView({ e8: 'bK' }))).toBe('');
	});
});
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts src/lib/board.test.ts`
Expected: FAIL: `burstShards is not a function`, `matedKing is not a function`.

- [ ] **Step 3: Implement**

Append to `web/src/lib/feel.ts`:


```ts
/**
 * The shards of a checkmate burst: an angle all the way round (degrees), how
 * far each flies (squares), its spin, and whether it's orange. Seeded, so the
 * burst is the same every time.
 */
export function burstShards(count = 18, seed = 7): { angle: number; dist: number; spin: number; hazard: boolean }[] {
	let s = seed;
	const rand = () => ((s = (s * 16807) % 2147483647) - 1) / 2147483646;
	return Array.from({ length: count }, (_, i) => ({
		angle: Math.round(((i + rand() * 0.6) / count) * 360),
		dist: Math.round((2 + rand() * 3) * 10) / 10,
		spin: Math.round(rand() * 540 - 270),
		hazard: i % 3 === 0
	}));
}

/**
 * How long a checkmate burst holds the board, in ms, before it dims and the
 * result card shows (with the tally and the winner's confetti).
 */
export const BURST_HOLD_MS = 1800;
```


In `web/src/lib/board.ts`, import `squareName` from `./game.ts` (the import becomes `import { pieceName, squareName, type Color, type EventJSON, type View } from './game.ts';`) and append:


```ts
/** The mated king's square once a game ends in checkmate, else "". */
export function matedKing(view: View): string {
	const r = view.result;
	if (!r || r.reason !== 'checkmate' || r.draw || !r.winner) return '';
	const king = r.winner === 'white' ? 'bK' : 'wK';
	const i = view.board.indexOf(king);
	return i < 0 ? '' : squareName(i);
}
```


In `web/src/lib/theme/tokens.css`, after `--trail-mamdani: ...;` add:


```css
	--mate: #d8413a; /* the mated king's square and badge */
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts src/lib/board.test.ts`
Expected: PASS.

- [ ] **Step 5: Burst on the board**

In `web/src/lib/Board.svelte`:

1. Add `BURST_HOLD_MS, burstShards` to the `./feel.ts` import (alphabetical: `biggerShake, BURST_HOLD_MS, burstShards, captureShake, ...`).
2. Add the prop: `mated = null,` after `quip = null,`, and in the type after `quip`:


```ts
		/** The mated king's square, burst once per game (by key). */
		mated?: { sq: string; key: string } | null;
```


3. After `const DUST = [...]` add `const SHARDS = burstShards();`.
4. The board dims only after the burst. Change the `<div class="board" ...>` opening tag to:


```svelte
<div class="board" class:dim class:late={!!mated && !playedBefore.has(mated.key)} role="group" aria-label="Chessboard" style="--glide-ease: {GLIDE_EASE}; --trail-fade: {TRAIL_FADE_MS}ms; --burst-hold: {BURST_HOLD_MS}ms" {@attach dragArea}>
```


   and in the style, after the `.board.dim { ... }` rule, add:


```css
	/* A checkmate burst plays first, then the board dims. */
	.board.dim.late {
		transition-delay: var(--burst-hold);
	}
```


5. In the potholes layer, after the `{#if saved && !reducedMotion()} ... {/if}` block from Task 1, add the red square under the king:


```svelte
		{#if mated && !playedBefore.has(mated.key)}
			<span class="slot" style="{place(mated.sq)}; --delay: {impact()}ms"><span class="wash"></span></span>
		{/if}
```


6. In the celebrate layer, after the `{#if quip} ... {/if}` block, add:


```svelte
		{#if mated && !playedBefore.has(mated.key)}
			{#key mated.key}
				<span
					class="slot fix burst"
					class:bottom={cell(mated.sq).row === 7}
					style="{place(mated.sq)}; --delay: {impact()}ms"
					{@attach () => {
						markCelebrated(mated.key);
						shake(SHAKE.mate, impact() + 500);
					}}
				>
					{#each SHARDS as s, i (i)}<span class="shard" class:hazard={s.hazard} style="--a: {s.angle}deg; --d: {s.dist}; --s: {s.spin}deg"></span>{/each}
					{#each [-1, 1] as dir (dir)}<svg class="flycone" style="--dir: {dir}" viewBox="0 0 40 40">{@render coneShape()}</svg>{/each}
					<span class="badge">#</span>
				</span>
			{/key}
		{/if}
```


7. In the style, before `/* A big moment: a speech bubble ...` add:


```css
	/* Checkmate: the king's square turns red in two steps, under the king. */
	.wash {
		position: absolute;
		inset: 0;
		background: var(--mate);
		opacity: 0.85;
		animation: wash 0.52s var(--delay, 0ms) both;
	}
	@keyframes wash {
		0% {
			opacity: 0;
		}
		40%,
		60% {
			opacity: 0.45;
		}
		100% {
			opacity: 0.85;
		}
	}
	/* Then white and orange shards and two cones burst across the board, and
	   a red badge marks the king. */
	.shard {
		position: absolute;
		left: 44%;
		top: 37%;
		width: 12%;
		height: 26%;
		border-radius: 2px;
		background: var(--piece-light);
		opacity: 0;
		animation: shard 1s cubic-bezier(0.15, 0.7, 0.3, 1) calc(var(--delay, 0ms) + 500ms) forwards;
	}
	.shard.hazard {
		background: var(--hazard);
	}
	@keyframes shard {
		from {
			opacity: 1;
			transform: rotate(var(--a)) translateY(0) rotate(0deg);
		}
		to {
			opacity: 0;
			transform: rotate(var(--a)) translateY(calc(var(--d) * -385%)) rotate(var(--s));
		}
	}
	.flycone {
		--up: 1;
		width: 100%;
		height: 100%;
		overflow: visible;
		opacity: 0;
		animation: flycone 1.1s ease-out calc(var(--delay, 0ms) + 500ms) forwards;
	}
	.bottom .flycone {
		--up: -1;
	}
	@keyframes flycone {
		0% {
			opacity: 1;
			transform: translate(0, 0) rotate(0deg) scale(0.6);
		}
		55% {
			opacity: 1;
			transform: translate(calc(var(--dir) * 160%), calc(var(--up) * 140%)) rotate(calc(var(--dir) * 200deg)) scale(1);
		}
		100% {
			opacity: 0;
			transform: translate(calc(var(--dir) * 230%), calc(var(--up) * 320%)) rotate(calc(var(--dir) * 330deg)) scale(1);
		}
	}
	.badge {
		place-self: start end;
		display: grid;
		place-items: center;
		width: 36%;
		aspect-ratio: 1;
		margin: 4%;
		border-radius: 50%;
		background: var(--mate);
		box-shadow: 0 0 0 2px var(--bg);
		color: var(--piece-light);
		font-family: var(--font-mono);
		font-weight: 700;
		font-size: 22cqh;
		line-height: 1;
		animation: badge 0.38s ease-out calc(var(--delay, 0ms) + 800ms) both;
	}
	@keyframes badge {
		0% {
			transform: scale(0);
		}
		60% {
			transform: scale(1.25);
		}
		100% {
			transform: scale(1);
		}
	}
	/* Reduced motion: the red square and the badge, nothing flying. */
	@media (prefers-reduced-motion: reduce) {
		.shard,
		.flycone {
			display: none;
		}
		.wash,
		.badge {
			animation: none;
		}
	}
```


In `web/src/routes/dev/board/+page.svelte`, add `matedKing` to the `#lib/board.ts` import, after the `let quip = ...` line add:


```ts
	// The Result card buttons end the game at once: each checkmate bursts once.
	let endings = $state(0);
	let mated = $derived.by(() => {
		const sq = !instant ? matedKing(view) : '';
		return sq ? { sq, key: `sandbox:${endings}` } : null;
	});
```


in `end(result)`, add `endings += 1;` as its first line, and pass `{mated}` to `<Board ... />` after `{quip}`.

In `web/src/routes/game/[code]/+page.svelte`, add `matedKing` to the `#lib/board.ts` import, and after the `let quip = ...` line add:


```ts
	// The checkmate burst, once the dice stop, on a turn that played out here.
	let mated = $derived.by(() => {
		const sq = view && anim.animated && !instant && !animating ? matedKing(view) : '';
		return sq ? { sq, key: `${code}:mate` } : null;
	});
```


Pass `{mated}` to both `<Board ... />` elements after `{quip}`. `animating` is declared above this point (`let animating = $derived(anim.animating);`); if it is not, move the `mated` block below it.

- [ ] **Step 6: Check the code**

Run: `pnpm --dir web check && npx @sveltejs/mcp svelte-autofixer web/src/lib/Board.svelte && npx @sveltejs/mcp svelte-autofixer web/src/routes/dev/board/+page.svelte`
Expected: `0 ERRORS 0 WARNINGS`; no issues.

- [ ] **Step 7: Watch it in the sandbox**

In `/dev/board`, press the Checkmate result button. Expected: the mated king's square turns red in two steps; white and orange shards and two traffic cones fly out, the board shakes hardest, a red "#" badge pops onto the king's square, and only then (about 1.8 s) the board dims. Press it again: it bursts again (a new key). With reduced motion: the red square and the badge, nothing flies, the board dims at once.

- [ ] **Step 8: Run the suite and commit**

Run: `pnpm --dir web test`
Expected: PASS.

```bash
git add web/src/lib/feel.ts web/src/lib/feel.test.ts web/src/lib/board.ts web/src/lib/board.test.ts web/src/lib/theme/tokens.css web/src/lib/Board.svelte web/src/routes/dev/board/+page.svelte 'web/src/routes/game/[code]/+page.svelte'
git commit -m "web: checkmate bursts on the king's square: red square, shards, flying cones and a # badge"
```

---

### Task 4: Win screen: result tally and confetti (section 7)

**Files:**
- Create: `web/src/lib/Confetti.svelte`
- Modify: `web/src/lib/feel.ts`, `web/src/lib/feel.test.ts`, `web/src/lib/board.ts`, `web/src/lib/board.test.ts`
- Modify: `web/src/routes/game/[code]/+page.svelte`, `web/scripts/playtest.js`

**Interfaces:**
- Consumes: `mated` (Task 3) and `BURST_HOLD_MS` (Task 3); `reducedMotion()`; `instant`, `anim`, `resultCard`, `receive()` in the page.
- Produces: `tallyOf(view): { label; short; value }[]`, `endedHere(prev, next)`, `wonHere(prev, next)` in `board.ts`; `CONFETTI_MS`, `type Confetto`, `confetti(count?, seed?)`, `countAt(value, t)` in `feel.ts`; `<Confetti delay? ondone />`.

- [ ] **Step 1: Write the failing tests**

In `web/src/lib/board.test.ts` add `endedHere, tallyOf, wonHere` to the import and append (before `describe('matedKing'`):


```ts
describe('tallyOf', () => {
	it("counts up the game's numbers for the result card", () => {
		const v = makeView({}, { seq: 61, stats: { savingRolls: 6, saved: 4, repaired: 3, mamdaniFell: false }, lost: { white: ['wP', 'wN'], black: ['bP', 'bB', 'bQ'] } });
		expect(tallyOf(v)).toEqual([
			{ label: 'Moves', short: 'moves', value: 31 },
			{ label: 'Saving rolls', short: 'rolls', value: 6 },
			{ label: 'Saved by the Mamdani', short: 'saved', value: 4 },
			{ label: 'Potholes repaired', short: 'repaired', value: 3 },
			{ label: 'Pieces lost to potholes', short: 'lost', value: 5 }
		]);
	});
});

describe('endedHere and wonHere', () => {
	const playing = makeView({}, { status: 'playing', you: 'white' });
	const over = (winner: 'white' | 'black', extra: Partial<View> = {}) =>
		makeView({}, { status: 'over', you: 'white', result: { winner, draw: false, reason: 'checkmate' }, ...extra });
	it('is true only when this page saw the game in play', () => {
		expect(endedHere(playing, over('white'))).toBe(true);
		expect(endedHere(null, over('white'))).toBe(false); // a reload of a finished game
		expect(endedHere(over('white'), over('white'))).toBe(false); // a later update, a rematch offer say
		expect(endedHere(playing, playing)).toBe(false);
	});
	it('throws confetti for the winner only', () => {
		expect(wonHere(playing, over('white'))).toBe(true);
		expect(wonHere(playing, over('black'))).toBe(false);
		expect(wonHere(null, over('white'))).toBe(false);
		const spectating = makeView({}, { status: 'playing', you: 'spectator' });
		expect(wonHere(spectating, over('white', { you: 'spectator' }))).toBe(false);
		const draw = makeView({}, { status: 'over', you: 'white', result: { draw: true, reason: 'stalemate' } });
		expect(wonHere(playing, draw)).toBe(false);
	});
});
```


In `web/src/lib/feel.test.ts` add `confetti, CONFETTI_MS, countAt` to the import and append:


```ts
describe('confetti', () => {
	it('throws 80 strips in three colors, a few of them traffic cones', () => {
		const pieces = confetti();
		expect(pieces).toHaveLength(80);
		expect(pieces.filter((c) => c.cone)).toHaveLength(5);
		expect(new Set(pieces.map((c) => c.tone))).toEqual(new Set(['accent', 'hazard', 'cream']));
		expect(confetti()).toEqual(pieces); // seeded: the same every time
	});
	it('lands every piece before it is cleared away', () => {
		for (const c of confetti()) {
			expect(c.x).toBeGreaterThanOrEqual(0);
			expect(c.x).toBeLessThanOrEqual(1);
			expect(c.delay + c.fall).toBeLessThanOrEqual(CONFETTI_MS);
		}
	});
});

describe('countAt', () => {
	it('counts from 0 up to the value, never past it', () => {
		expect(countAt(7, 0)).toBe(0);
		expect(countAt(7, 1)).toBe(7);
		expect(countAt(7, 2)).toBe(7);
		const steps = [0, 0.2, 0.4, 0.6, 0.8, 1].map((t) => countAt(7, t));
		expect(steps).toEqual([...steps].sort((a, b) => a - b));
		expect(countAt(0, 0.5)).toBe(0);
	});
});
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts src/lib/board.test.ts`
Expected: FAIL: `tallyOf is not a function`, `endedHere is not a function`, `wonHere is not a function`, `confetti is not a function`, `countAt is not a function`.

- [ ] **Step 3: Implement the helpers**

Append to `web/src/lib/board.ts` (before `matedKing`):


```ts
/** The result card's numbers, counted up one at a time when it appears. */
export function tallyOf(view: View): { label: string; short: string; value: number }[] {
	return [
		{ label: 'Moves', short: 'moves', value: Math.ceil(view.seq / 2) },
		{ label: 'Saving rolls', short: 'rolls', value: view.stats.savingRolls },
		{ label: 'Saved by the Mamdani', short: 'saved', value: view.stats.saved },
		{ label: 'Potholes repaired', short: 'repaired', value: view.stats.repaired },
		{ label: 'Pieces lost to potholes', short: 'lost', value: view.lost.white.length + view.lost.black.length }
	];
}

/**
 * Whether the game ended while this page watched it being played, rather
 * than being opened (or reopened) after it was over. Only then does the
 * result card count up and, for the winner, throw confetti.
 */
export function endedHere(prev: View | null, next: View): boolean {
	return prev?.status === 'playing' && !!next.result;
}

/** Confetti: the game ended here and this viewer won it. Never for a draw or a spectator. */
export function wonHere(prev: View | null, next: View): boolean {
	return endedHere(prev, next) && !next.result?.draw && !!next.result?.winner && next.result.winner === next.you;
}
```


Append to `web/src/lib/feel.ts`:


```ts
/** How long the winner's confetti falls, in ms. */
export const CONFETTI_MS = 2500;

export type Confetto = {
	/** Where it starts across the screen, 0 to 1. */
	x: number;
	/** When it starts falling, and how long it takes to cross the screen (ms). */
	delay: number;
	fall: number;
	/** How far it sways side to side (px), and how fast it spins (degrees a second). */
	sway: number;
	spin: number;
	/** Its size (px); a cone is drawn in a box this size. */
	w: number;
	h: number;
	tone: 'accent' | 'hazard' | 'cream';
	cone: boolean;
};

/**
 * The winner's confetti: strips in road-works yellow, hazard orange and
 * cream, one in sixteen a tiny traffic cone. Seeded, so it is the same every
 * time; every piece is through the screen before CONFETTI_MS.
 */
export function confetti(count = 80, seed = 11): Confetto[] {
	let s = seed;
	const rand = () => ((s = (s * 16807) % 2147483647) - 1) / 2147483646;
	const tones = ['accent', 'hazard', 'cream'] as const;
	return Array.from({ length: count }, (_, i) => {
		const cone = i % 16 === 15;
		const delay = Math.round(rand() * 500);
		return {
			x: Math.round(rand() * 1000) / 1000,
			delay,
			fall: Math.round(1400 + rand() * (CONFETTI_MS - 1400 - delay)),
			sway: Math.round(10 + rand() * 30),
			spin: Math.round(rand() * 720 - 360),
			w: cone ? 12 : Math.round(6 + rand() * 4),
			h: cone ? 14 : Math.round(10 + rand() * 6),
			tone: tones[i % 3],
			cone
		};
	});
}

/** A count-up's value at `t` (0 to 1 of the way through), easing out. */
export function countAt(value: number, t: number): number {
	const p = Math.min(1, Math.max(0, t));
	return Math.round(value * (1 - (1 - p) ** 3));
}
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts src/lib/board.test.ts`
Expected: PASS.

- [ ] **Step 5: The confetti canvas**

Create `web/src/lib/Confetti.svelte`:


```svelte
<script lang="ts">
	import { confetti, CONFETTI_MS, type Confetto } from './feel.ts';

	// The winner's confetti: one canvas over the page for CONFETTI_MS, then
	// `ondone` so the page removes it. The page shows it only to the winner,
	// only when the game ended while they watched, never under reduced motion.
	let { delay = 0, ondone }: { delay?: number; ondone: () => void } = $props();

	const PIECES = confetti();
	const FADE_MS = 300;

	function cone(ctx: CanvasRenderingContext2D, c: Confetto, colors: Record<string, string>) {
		const { w, h } = c;
		ctx.fillStyle = colors.hazard;
		ctx.beginPath();
		ctx.moveTo(0, -h / 2);
		ctx.lineTo(w * 0.35, h * 0.35);
		ctx.lineTo(-w * 0.35, h * 0.35);
		ctx.closePath();
		ctx.fill();
		ctx.fillStyle = colors.cream;
		ctx.fillRect(-w * 0.2, -h * 0.05, w * 0.4, h * 0.16);
		ctx.fillStyle = colors.hazard;
		ctx.fillRect(-w / 2, h * 0.35, w, h * 0.15);
	}

	function fall(canvas: HTMLCanvasElement) {
		const ctx = canvas.getContext('2d');
		if (!ctx) {
			ondone();
			return;
		}
		const css = getComputedStyle(canvas);
		const colors: Record<string, string> = {
			accent: css.getPropertyValue('--accent').trim() || '#f2c230',
			hazard: css.getPropertyValue('--hazard').trim() || '#ff7a3d',
			cream: css.getPropertyValue('--piece-light').trim() || '#fbf8f0'
		};
		const dpr = window.devicePixelRatio || 1;
		let width = 0;
		let height = 0;
		const size = () => {
			width = window.innerWidth;
			height = window.innerHeight;
			canvas.width = Math.round(width * dpr);
			canvas.height = Math.round(height * dpr);
		};
		size();
		window.addEventListener('resize', size);
		const start = performance.now() + delay;
		let frame = requestAnimationFrame(function draw(now) {
			const t = Math.max(0, now - start);
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			ctx.clearRect(0, 0, width, height);
			ctx.globalAlpha = Math.min(1, (CONFETTI_MS - t) / FADE_MS);
			for (const c of PIECES) {
				const p = (t - c.delay) / c.fall;
				if (p <= 0 || p >= 1) continue;
				ctx.save();
				ctx.translate(c.x * width + Math.sin(p * Math.PI * 3) * c.sway, -20 + p * (height + 40));
				ctx.rotate(((c.spin * t) / 1000) * (Math.PI / 180));
				if (c.cone) cone(ctx, c, colors);
				else {
					// A strip turning over: its width swings, like paper.
					ctx.fillStyle = colors[c.tone];
					ctx.fillRect((-c.w / 2) * Math.cos(p * 9), -c.h / 2, c.w * Math.cos(p * 9), c.h);
				}
				ctx.restore();
			}
			if (t < CONFETTI_MS) frame = requestAnimationFrame(draw);
			else ondone();
		});
		return () => {
			cancelAnimationFrame(frame);
			window.removeEventListener('resize', size);
		};
	}
</script>

<canvas class="confetti" aria-hidden="true" {@attach fall}></canvas>

<style>
	.confetti {
		position: fixed;
		inset: 0;
		z-index: 50;
		width: 100vw;
		height: 100vh;
		pointer-events: none;
	}
</style>
```


- [ ] **Step 6: The result card flips in and counts up; the winner gets confetti**

In `web/src/routes/game/[code]/+page.svelte`:

1. Imports: replace `import { fly } from 'svelte/transition';` with `import { backOut } from 'svelte/easing';`; add `import Confetti from '#lib/Confetti.svelte';` after the `Board` import; add `endedHere`, `tallyOf`, `wonHere` to the `#lib/board.ts` import; add `import { BURST_HOLD_MS, countAt } from '#lib/feel.ts';` after the `#lib/catchphrases.ts` import.

2. After `let copyHint = $state('');` add:


```ts
	// The game ended while this page watched it: the tally counts up, and the
	// winner gets confetti (cleared once it has fallen).
	let endedLive = $state(false);
	let cheer = $state(false);
```


3. In `receive(next)`, after `const prev = anim.view;` add:


```ts
		if (endedHere(prev, next)) {
			endedLive = true;
			cheer = wonHere(prev, next) && !instant && !reducedMotion();
		}
```


4. Before `let resultCard = $derived.by(() => {` add:


```ts
	// The result card flips in, with a slight overshoot. After a checkmate it
	// first waits out the burst (the card would cover it), and so do the tally
	// and the confetti.
	const FLIP_MS = 420;
	let hold = $derived(mated ? BURST_HOLD_MS : 0);
	function flipIn(_node: Element, { lift = '' }: { lift?: string } = {}) {
		const wait = hold;
		return {
			duration: reducedMotion() ? 0 : wait + FLIP_MS,
			css: (t: number) => {
				const p = Math.max(0, (t * (wait + FLIP_MS) - wait) / FLIP_MS);
				return `transform: ${lift} perspective(700px) rotateX(${-75 * (1 - backOut(p))}deg); opacity: ${Math.min(1, p * 2)}`;
			}
		};
	}

	// The tally's numbers count up one at a time once the card is in, each
	// landing with a pop. A game opened after it ended shows them at once, and
	// each number counts once: a later update to the finished game (a rematch
	// offer, the opponent leaving) redraws the tally, and shows it as it is.
	const COUNT_MS = 250;
	const COUNT_GAP_MS = 150;
	const counted: boolean[] = [];
	function countUp(value: number, i: number) {
		return (node: HTMLElement) => {
			node.textContent = String(value);
			if (!endedLive || instant || reducedMotion() || counted[i]) return;
			counted[i] = true;
			node.textContent = '0';
			let frame = 0;
			const timer = setTimeout(() => {
				const start = performance.now();
				frame = requestAnimationFrame(function step(now) {
					const t = (now - start) / COUNT_MS;
					node.textContent = String(countAt(value, t));
					if (t < 1) frame = requestAnimationFrame(step);
					else node.classList.add('pop');
				});
			}, hold + FLIP_MS + i * (COUNT_MS + COUNT_GAP_MS));
			return () => {
				clearTimeout(timer);
				cancelAnimationFrame(frame);
			};
		};
	}
```


5. Phone result card: replace


```svelte
			<section class="ph-card accent-line" role="status" aria-label="Game over" in:fly={{ y: 24, duration: reducedMotion() ? 0 : 400 }}>
				<span class="ph-result"><span class="ph-headline">{resultCard.title}</span><span class="ph-kicker">{resultCard.kicker}</span></span>
				<span class="ph-detail">{rematchNote || resultCard.detail}</span>
			</section>
```


   with


```svelte
			<section class="ph-card accent-line" role="status" aria-label="Game over" in:flipIn>
				<span class="ph-result"><span class="ph-headline">{resultCard.title}</span><span class="ph-kicker">{resultCard.kicker}</span></span>
				<span class="ph-detail">{rematchNote || resultCard.detail}</span>
				<span class="ph-tally">
					{#each tallyOf(view) as row, i (row.short)}<span><b {@attach countUp(row.value, i)}></b> {row.short}</span>{/each}
				</span>
			</section>
```


6. Desktop result card: change `<div class="result" role="status" in:fly={{ y: -24, duration: reducedMotion() ? 0 : 500 }}>` to:


```svelte
						<div class="result" role="status" in:flipIn={{ lift: 'translate(-50%, -50%)' }}>
```


   and replace the side column's stats list


```svelte
					<dl class="stats">
						<div><dt>Lost to potholes</dt><dd>White {view.lost.white.length} · Black {view.lost.black.length}</dd></div>
						<div><dt>Saving rolls</dt><dd>{view.stats.saved} of {view.stats.savingRolls} saved</dd></div>
						<div>
							<dt>Repaired by the Mamdani</dt>
							<dd>{view.stats.repaired} {view.stats.repaired === 1 ? 'pothole' : 'potholes'}</dd>
						</div>
						{#if view.stats.mamdaniFell}<div><dt>The Mamdani</dt><dd>fell in</dd></div>{/if}
					</dl>
```


   with the tally


```svelte
					<dl class="stats">
						{#each tallyOf(view) as row, i (row.label)}
							<div><dt>{row.label}</dt><dd {@attach countUp(row.value, i)}></dd></div>
						{/each}
						{#if view.stats.mamdaniFell}<div><dt>The Mamdani</dt><dd>fell in</dd></div>{/if}
					</dl>
```


7. After the closing `{/if}` of the whole page (just before `<style>`), add:


```svelte
{#if cheer && resultCard}<Confetti delay={hold} ondone={() => (cheer = false)} />{/if}
```


8. Styles. Replace the rule


```css
	.ph-card.accent-line {
		gap: 4px;
		border-color: var(--accent-line);
	}
```


   with:


```css
	/* The result card: tighter, so its three lines (result, detail, tally)
	   fit the bottom block's height and the board doesn't move. */
	.ph-card.accent-line {
		gap: 2px;
		padding: 6px 12px;
		border-color: var(--accent-line);
	}
	.ph-card.accent-line .ph-detail {
		line-height: 1.25;
	}
```


   after the `.ph-detail` rule (`font-size: 14px;`) add:


```css
	/* The tally, one line under the result. */
	.ph-tally {
		overflow: hidden;
		line-height: 1.1;
		font-family: var(--font-mono);
		font-size: 11px;
		white-space: nowrap;
		text-overflow: ellipsis;
		color: var(--text-muted);
	}
	/* Too narrow for it: the headline already wraps. The moves sheet has the numbers. */
	@media (max-width: 359px) {
		.ph-tally {
			display: none;
		}
	}
	.ph-tally > span + span::before {
		content: ' · ';
	}
	.ph-tally b {
		display: inline-block;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: var(--text);
	}
```


   and replace the `.stats dd { ... }` rule with:


```css
	.stats dd {
		margin: 0;
		font-family: var(--font-mono);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: var(--text);
	}
	/* A tally number lands with a yellow pop. */
	.stats dd:global(.pop),
	.ph-tally b:global(.pop) {
		animation: tally-pop 0.3s ease-out;
	}
	@keyframes tally-pop {
		0% {
			color: var(--accent);
			transform: scale(1.45);
		}
		100% {
			transform: scale(1);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.stats dd:global(.pop),
		.ph-tally b:global(.pop) {
			animation: none;
		}
	}
```


In `web/scripts/playtest.js`, the phone card's tally may still be counting when the two sides are compared. Replace


```js
	const result = (await w.locator(RESULT).innerText().catch(() => '')).replace(/\s*\n+\s*/g, ' · ');
	const resultBlack = (await b.locator(RESULT).innerText().catch(() => '')).replace(/\s*\n+\s*/g, ' · ');
```


with


```js
	// The result, without the phone card's tally, which may still be counting up.
	const resultText = (p) =>
		p
			.locator(RESULT)
			.evaluate((el) => [...el.children].filter((c) => !c.classList.contains('ph-tally')).map((c) => c.innerText).join('\n'))
			.catch(() => '');
	const result = (await resultText(w)).replace(/\s*\n+\s*/g, ' · ');
	const resultBlack = (await resultText(b)).replace(/\s*\n+\s*/g, ' · ');
```


- [ ] **Step 7: Check the code**

Run: `pnpm --dir web check && npx @sveltejs/mcp svelte-autofixer web/src/lib/Confetti.svelte && npx @sveltejs/mcp svelte-autofixer 'web/src/routes/game/[code]/+page.svelte'`
Expected: `0 ERRORS 0 WARNINGS`; no issues beyond the `SvelteSet` suggestion for `streams`.

- [ ] **Step 8: Play fool's mate**

Run the Go server and the dev server; open a game in one browser (White) and its link in a private window (Black). Play 1. f3 e5 2. g4 Qh4# (start a new game if a pothole gets in the way). Expected, on Black's screen: the burst on e1, then the board dims and the card flips in with a slight overshoot, the five tally numbers count up one at a time with a yellow pop (Moves 2), and confetti (yellow, orange and cream strips, a few cones) falls for about 2.5 s, then the canvas is gone. On White's screen: the same, without confetti. Reload Black's page: the card and numbers show at once, no burst, no confetti.

- [ ] **Step 9: The phone card fits**

In a 393 × 760 window, measure `document.querySelector('.ph-bottom').offsetHeight` while playing and after resigning. Expected: the same height (155), and the tally reads `2 moves · 0 rolls · ...` on one line. At 320 px wide the tally is hidden. Run `pnpm --dir web playtest --phone --games 2`; expected: all games clean.

- [ ] **Step 10: Run the suite and commit**

Run: `pnpm --dir web test`
Expected: PASS.

```bash
git add web/src/lib/Confetti.svelte web/src/lib/feel.ts web/src/lib/feel.test.ts web/src/lib/board.ts web/src/lib/board.test.ts 'web/src/routes/game/[code]/+page.svelte' web/scripts/playtest.js
git commit -m "web: the result card flips in and counts up the game's numbers; the winner gets confetti"
```

---

### Task 5: +5 on the clock (section 12)

**Files:**
- Modify: `web/src/lib/clock.ts`, `web/src/lib/clock.test.ts`, `web/src/lib/PlayerBar.svelte`

**Interfaces:**
- Consumes: `PlayerBar`'s `clockMs` and `ticking` props; `reducedMotion()`.
- Produces: `bonusOf(prev: number | undefined, next: number, ticking: boolean): number` in `clock.ts`.

- [ ] **Step 1: Write the failing test**

In `web/src/lib/clock.test.ts` add `bonusOf` to the import and append:


```ts
describe('bonusOf', () => {
	it("finds the mover's increment when their clock jumps up and stops", () => {
		expect(bonusOf(41_300, 46_100, false)).toBe(4_800);
		expect(bonusOf(41_300, 46_300, false)).toBe(5_000);
	});
	it('ignores ticking down, the first reading, and a running clock', () => {
		expect(bonusOf(41_300, 41_200, false)).toBe(0);
		expect(bonusOf(undefined, 46_300, false)).toBe(0);
		// A deploy restores the side to move's clock, which then runs.
		expect(bonusOf(41_300, 46_300, true)).toBe(0);
	});
	it('ignores jumps that are not an increment', () => {
		expect(bonusOf(41_300, 42_300, false)).toBe(0);
		expect(bonusOf(41_300, 101_300, false)).toBe(0); // a new game's clock
	});
});
```


- [ ] **Step 2: Run it to make sure it fails**

Run: `pnpm --dir web exec vitest run src/lib/clock.test.ts`
Expected: FAIL: `bonusOf is not a function`.

- [ ] **Step 3: Implement**

Append to `web/src/lib/clock.ts`:


```ts
/**
 * The time a player just gained from the 5 s increment, from their clock's
 * last reading and this one, or 0: the clock jumped up by about the
 * increment (less what the move's trip to the server took) and stopped,
 * since it's now the other side's turn.
 */
export function bonusOf(prev: number | undefined, next: number, ticking: boolean): number {
	if (prev === undefined || ticking) return 0;
	const gain = next - prev;
	return gain >= 3_000 && gain <= 6_000 ? gain : 0;
}
```


- [ ] **Step 4: Run it to make sure it passes**

Run: `pnpm --dir web exec vitest run src/lib/clock.test.ts`
Expected: PASS.

- [ ] **Step 5: Float "+5" off the clock**

In `web/src/lib/PlayerBar.svelte`:

1. Imports become:


```ts
	import { bonusOf, formatClock } from './clock.ts';
	import { pieceName, type Color } from './game.ts';
	import { reducedMotion } from './motion.ts';
```


2. After `let label = $derived(name || side);` add:


```ts
	// The "+5": when this player moves, the increment floats up from their
	// clock and the time ticks up into place, one second at a time. `lastClock`
	// is plain bookkeeping between the clock's readings, not state.
	const TICK_UP_MS = 300;
	let lastClock: number | undefined;
	let bonus: { ms: number; key: number } | null = null;
	let gained = $derived.by(() => {
		const ms = clockMs === undefined ? 0 : bonusOf(lastClock, clockMs, ticking);
		lastClock = clockMs;
		if (ms && !reducedMotion()) bonus = { ms, key: (bonus?.key ?? 0) + 1 };
		return bonus;
	});
	// How far behind the real time the clock shows, while it ticks up.
	let lag = $state(0);
	function tickUp(ms: number) {
		return () => {
			const steps = Math.max(1, Math.round(ms / 1000));
			let step = 0;
			lag = ms;
			let timer: ReturnType<typeof setInterval> | undefined;
			const start = setTimeout(() => {
				timer = setInterval(() => {
					step += 1;
					lag = step >= steps ? 0 : ms - step * 1000;
					if (step >= steps) clearInterval(timer);
				}, TICK_UP_MS / steps);
			}, 150);
			return () => {
				clearTimeout(start);
				clearInterval(timer);
				lag = 0;
			};
		};
	}
```


3. Replace the clock markup


```svelte
	{#if clockMs !== undefined}
		<span
			class="clock"
			class:active={toMove}
			class:low={ticking && clockMs < 20_000}
			role="timer"
			aria-label="{side} clock: {formatClock(clockMs)}">{formatClock(clockMs)}</span
		>
	{/if}
```


   with


```svelte
	{#if clockMs !== undefined}
		<span class="clock-wrap">
			{#key gained?.key}
				<span
					class="clock"
					class:active={toMove}
					class:low={ticking && clockMs < 20_000}
					class:bump={!!gained}
					role="timer"
					aria-label="{side} clock: {formatClock(clockMs)}">{formatClock(clockMs - lag)}</span
				>
				{#if gained}<span class="plus" aria-hidden="true" {@attach tickUp(gained.ms)}>+5</span>{/if}
			{/key}
		</span>
	{/if}
```


4. In the style, before `.clock {` add `.clock-wrap`, and after `.clock.active.low { ... }` add the float and bump:


```css
	.clock-wrap {
		position: relative;
		display: flex;
		flex-shrink: 0;
	}
```

```css
	/* The increment: "+5" floats up off the clock, which bumps as the time lands. */
	.clock.bump {
		animation: clock-bump 0.2s ease-out 0.45s;
	}
	@keyframes clock-bump {
		40% {
			transform: scale(1.08);
		}
	}
	.plus {
		position: absolute;
		top: 0;
		left: 50%;
		z-index: 2;
		font-family: var(--font-mono);
		font-weight: 700;
		font-size: 16px;
		color: var(--accent);
		pointer-events: none;
		opacity: 0;
		animation: plus-up 0.6s ease-out forwards;
	}
	.compact .plus {
		font-size: 13px;
	}
	@keyframes plus-up {
		0% {
			opacity: 0;
			transform: translate(-50%, 0);
		}
		25% {
			opacity: 1;
		}
		100% {
			opacity: 0;
			transform: translate(-50%, -150%);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.clock.bump,
		.plus {
			animation: none;
		}
	}
```


- [ ] **Step 6: Check the code**

Run: `pnpm --dir web check && npx @sveltejs/mcp svelte-autofixer web/src/lib/PlayerBar.svelte`
Expected: `0 ERRORS 0 WARNINGS`; no issues.

- [ ] **Step 7: Watch it**

Play a few moves in a friend game (two windows). Expected: when a player moves, a yellow "+5" floats up off their clock and the time ticks up a second at a time (10:00 → 10:01 → 10:03 → 10:05), with a small bump; both players see it on the mover's clock. With reduced motion the time simply changes.

- [ ] **Step 8: Run the suite and commit**

Run: `pnpm --dir web test`
Expected: PASS.

```bash
git add web/src/lib/clock.ts web/src/lib/clock.test.ts web/src/lib/PlayerBar.svelte
git commit -m "web: a move's +5 floats off the mover's clock and the time ticks up into place"
```

---

### Task 6: Whole games with full animations

**Files:** none (a check; fix and re-run if anything fails).

- [ ] **Step 1: Build and serve the production bundle**

Run: `pnpm --dir web build && go build -o /tmp/pc-server ./cmd/server && PORT=8095 DB_PATH=/tmp/pc.db /tmp/pc-server` (in the background).
Expected: the server logs that it is listening on :8095.

- [ ] **Step 2: Playtest Chromium, WebKit and the phone with full animations**

Run, one after the other:

```bash
pnpm --dir web playtest --base http://localhost:8095 --turn-ms 15000 --games 4 --max-plies 80
pnpm --dir web playtest --base http://localhost:8095 --turn-ms 15000 --games 4 --max-plies 80 --browser webkit
pnpm --dir web playtest --base http://localhost:8095 --turn-ms 15000 --games 4 --max-plies 80 --phone
```

Expected: each ends with `4/4 games finished cleanly` (a production build ignores `?instant`, so every animation plays).

- [ ] **Step 3: Reduced motion and reload**

Repeat Task 4, step 8 with the system set to reduce motion. Expected: the red square and "#" badge without anything flying, the card and numbers at once, no confetti, no "+5" float. Then reload a finished game: no burst, no confetti, no count-up.

- [ ] **Step 4: Run every suite**

Run: `pnpm --dir web check && pnpm --dir web test && pnpm --dir web build && go test -race -short ./...`
Expected: all pass.

