# Board Feel 1: Look and Moves Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The board gets coordinates outside its squares, the mpchess pieces at 110% with a crisp white outline, an ease-out glide with a trail and a whiplash, and a pick-up that wobbles, tilts, ripples its legal moves in and lands with a squash.

**Architecture:** Pure geometry and timing live in a new `web/src/lib/feel.ts` (tested with Vitest); `Board.svelte` renders them. The coordinates wrap the board in a square frame. Trails are derived from the board's existing list of moving pieces and kept until a real move replaces them. A piece's one-off animations (whiplash, landing squash) run from one Svelte attachment that remembers what it last played. No server change.

**Tech Stack:** Svelte 5.57 on SvelteKit 3 (static adapter), TypeScript, Vitest, Playwright (playtest script), Web Animations API, CSS.

**Spec:** `docs/superpowers/specs/2026-10-06-board-feel-design.md`, sections 1, 2, 3 and 10. Plans 2 (impacts and big moments) and 3 (dice) follow this one. The live mockups: https://claude.ai/artifact/E6oCYQhp8xaDTZCVssczXy.

This plan was written from code built and checked in a scratch copy of `main` at `97d6cad`: unit tests 152/152, `pnpm --dir web check` clean, playtests with full animations Chromium 2/2, WebKit 1/1, phone 2/2.

## Global Constraints

- SvelteKit 3: use `$app/env`, not `$app/environment`; `#lib/...` imports need the file extension; inside `web/src/lib`, import siblings as `./feel.ts`.
- Svelte 5: avoid `$effect`; run `npx @sveltejs/mcp svelte-autofixer ./src/lib/<File>.svelte` (from `web/`) on every component you change, until it reports no issues.
- Board pointer rule (CLAUDE.md): capture the pointer only once a drag has moved more than 6 px from the press. Don't change that logic.
- Reduced motion and instant mode (`reducedMotion()` in `motion.ts`) skip every new effect. JS effects check `reducedMotion()`; CSS animations also sit under `@media (prefers-reduced-motion: reduce)`.
- Pieces: lichess mpchess by Maxime Chupin, GPLv3+, pinned to lichess commit `5ae58154b2be1033dcb0252173b4fff4bb02e73a`; drawn at 100% of the square (110% of the old 92%).
- Outline: `--piece-outline: #fbf8f0`, four unblurred `drop-shadow`s, 1.5 px on the board and promotion picker, 1 px on lost pieces and the home page's small boards. The Mamdani tile gets none.
- Glide: `cubic-bezier(0.25, 1, 0.5, 1)`, durations unchanged (`moveDuration`, 350 to 750 ms).
- Commits: stage only the files the task names (`git add <paths>`, never `-A`); no Claude attribution (no `Co-Authored-By`, no "Generated with").
- Run the playtest after any change to the board, game page or server protocol (CLAUDE.md).

## Review Focus

1. **Playing as Black (flipped board):** labels read 1 to 8 and h to a, and trails and whiplash follow the screen, not the square names. Covered by `feel.test.ts` (flipped cases) and a flipped screenshot in Task 5.
2. **A dice step arriving mid-glide:** the board recalculates its pieces 550 ms after a move, while a long glide (up to 750 ms) is still running; its trail must not vanish. Covered by Task 3, Step 7.
3. **Dragged moves:** a dropped piece gets no trail and no whiplash, lands with a squash, and an illegal drop settles back with a squash. Covered by Task 4, Step 6, and the playtest's dragged moves.
4. **Instant mode and reduced motion:** no trails, wobble, ripple or squash. Covered by Task 5, Step 3.
5. **Short phones:** the coordinates cost the board 14 px each way; the page must never scroll and the board must stay at least 88% of the screen's width. Covered by Task 1, Step 8 (`playtest --phone`).

---

### Task 1: Coordinates outside the board

**Files:**
- Create: `web/src/lib/feel.ts`
- Create: `web/src/lib/feel.test.ts`
- Modify: `web/src/lib/Board.svelte`
- Modify: `web/scripts/playtest.js:85`

**Interfaces:**
- Produces: `coordinates(flipped: boolean): { ranks: string[]; files: string[] }` and `cellOf(sq: string, flipped: boolean): { col: number; row: number }` in `web/src/lib/feel.ts`. Tasks 3 and 4 add to this file.

- [ ] **Step 1: Write the failing test**

Create `web/src/lib/feel.test.ts`:

```ts
import { describe, expect, it } from 'vitest';
import { cellOf, coordinates } from './feel.ts';

describe('coordinates', () => {
	it('reads 8 to 1 down and a to h across for White', () => {
		expect(coordinates(false)).toEqual({ ranks: ['8', '7', '6', '5', '4', '3', '2', '1'], files: ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'] });
	});
	it('reads 1 to 8 down and h to a across for Black', () => {
		expect(coordinates(true)).toEqual({ ranks: ['1', '2', '3', '4', '5', '6', '7', '8'], files: ['h', 'g', 'f', 'e', 'd', 'c', 'b', 'a'] });
	});
});

describe('cellOf', () => {
	it('puts a1 bottom left for White and top right for Black', () => {
		expect(cellOf('a1', false)).toEqual({ col: 0, row: 7 });
		expect(cellOf('a1', true)).toEqual({ col: 7, row: 0 });
		expect(cellOf('e4', false)).toEqual({ col: 4, row: 4 });
	});
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts`
Expected: FAIL, `Cannot find module './feel.ts'`.

- [ ] **Step 3: Write the helpers**

Create `web/src/lib/feel.ts`:

```ts
// How the board looks and moves: pure helpers for the board's frame and
// effects (docs/superpowers/specs/2026-10-06-board-feel-design.md).

const RANKS = ['8', '7', '6', '5', '4', '3', '2', '1'];
const FILES = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

/** The labels beside the board in screen order: ranks top to bottom, files left to right. */
export function coordinates(flipped: boolean): { ranks: string[]; files: string[] } {
	return flipped ? { ranks: [...RANKS].reverse(), files: [...FILES].reverse() } : { ranks: RANKS, files: FILES };
}

/** A square's column and row on screen, 0 to 7 from the top left. */
export function cellOf(sq: string, flipped: boolean): { col: number; row: number } {
	const file = sq.charCodeAt(0) - 97;
	const rank = Number(sq[1]) - 1;
	return flipped ? { col: 7 - file, row: rank } : { col: file, row: 7 - rank };
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts`
Expected: PASS, 3 tests.

- [ ] **Step 5: Frame the board with the labels**

In `web/src/lib/Board.svelte`:

1. Add the import after the `game.ts` import:

```ts
	import { cellOf, coordinates } from './feel.ts';
```

2. Replace the body of `cell()` and add the labels right after it:

```ts
	/** Column and row on screen for a square, 0..7 from the top left. */
	function cell(sq: string): { col: number; row: number } {
		return cellOf(sq, flipped);
	}
	let labels = $derived(coordinates(flipped));
```

3. Change `{#each order as index, n (index)}` to `{#each order as index (index)}`, and delete these two lines inside the square button:

```svelte
			{#if n % 8 === 0}<span class="rank-label">{sq[1]}</span>{/if}
			{#if n >= 56}<span class="file-label">{sq[0]}</span>{/if}
```

4. Replace the board's opening tag:

```svelte
<div class="board" class:dim role="group" aria-label="Chessboard" {@attach dragArea}>
```

with:

```svelte
<!-- Coordinates sit outside the board, so the squares stay clean. The
     label column and row are the same size, so the frame stays square. -->
<div class="frame" class:dim>
<div class="ranks" aria-hidden="true">{#each labels.ranks as r (r)}<span>{r}</span>{/each}</div>
<div class="board" class:dim role="group" aria-label="Chessboard" {@attach dragArea}>
```

5. After the board's closing `</div>` (the last line before `<style>`), add:

```svelte
<div class="files" aria-hidden="true">{#each labels.files as f (f)}<span>{f}</span>{/each}</div>
</div>
```

6. In `<style>`, delete the whole `.rank-label, .file-label { … }`, `.square.dark .rank-label, .square.dark .file-label { … }`, `.rank-label { … }` and `.file-label { … }` rules, and add before `.board {`:

```css
	/* The board with its coordinates: a column of ranks on the left and a
	   row of files underneath, the same size so the whole frame is square. */
	.frame {
		--coord: 14px;
		display: grid;
		grid-template-columns: var(--coord) minmax(0, 1fr);
		grid-template-rows: auto var(--coord);
		gap: 4px;
		width: 100%;
	}
	.ranks,
	.files {
		display: grid;
		font-family: var(--font-mono);
		font-weight: 500;
		font-size: 12px;
		line-height: 1;
		color: var(--text-muted);
		text-align: center;
		user-select: none;
	}
	.ranks {
		grid-template-rows: repeat(8, 1fr);
		align-items: center;
	}
	.files {
		grid-column: 2;
		grid-template-columns: repeat(8, 1fr);
		align-items: end;
	}
	.frame.dim .ranks,
	.frame.dim .files {
		opacity: 0.55;
	}
	/* On a phone the board is as big as fits, so the labels go compact: the
	   frame then costs the board 14 px each way. */
	@media (max-width: 640px) {
		.frame {
			--coord: 12px;
			gap: 2px;
		}
		.ranks,
		.files {
			font-size: 10px;
		}
	}
```

- [ ] **Step 6: Let the phone check allow for the labels**

In `web/scripts/playtest.js`, replace:

```js
		if (board < innerWidth * 0.9) return `the board is ${Math.round(board)}px on a ${innerWidth}px screen`;
```

with:

```js
		// Near full width: the coordinates outside the board take 14 px of it
		// (12 px labels and a 2 px gap), so on a short screen it's about 89%.
		if (board < innerWidth * 0.88) return `the board is ${Math.round(board)}px on a ${innerWidth}px screen`;
```

On an iPhone 15 the phone board is limited by height, not width: it measured 346 px of 393 (88%) with 18 px labels, about 350 px (89%) with the compact ones, and about 364 px before. The 14 px is the spec's "about 14 px of width on a phone".

- [ ] **Step 7: Check the component**

Run (from `web/`): `npx @sveltejs/mcp svelte-autofixer ./src/lib/Board.svelte`
Expected: `issues: []`.
Run: `pnpm --dir web check`
Expected: `0 ERRORS 0 WARNINGS`.

- [ ] **Step 8: Look, then playtest the phone**

Start `go run ./cmd/server` and `pnpm --dir web dev`. Open http://localhost:5173/dev/board: ranks 8 to 1 run down the left of the board, files a to h along the bottom, and no square has a label. Then run:

Run: `pnpm --dir web playtest --phone --games 2`
Expected: `2/2 games finished cleanly` (the page never scrolls; the board is at least 88% wide).

- [ ] **Step 9: Commit**

```bash
git add web/src/lib/feel.ts web/src/lib/feel.test.ts web/src/lib/Board.svelte web/scripts/playtest.js
git commit -m "web: coordinates outside the board, so the squares stay clean"
```

---

### Task 2: mpchess pieces, 110%, with a white outline

**Files:**
- Modify: `web/static/pieces/*.svg` (all 12, replaced)
- Create: `web/static/pieces/GPL-3.0.txt`
- Modify: `web/static/pieces/LICENSE.txt` (replaced)
- Modify: `web/src/lib/theme/tokens.css`
- Modify: `web/src/lib/Board.svelte`, `web/src/lib/PlayerBar.svelte`, `web/src/lib/MiniBoard.svelte`
- Modify: `web/static/og.png` (redrawn)

**Interfaces:**
- Produces: the CSS token `--piece-outline` in `tokens.css`, used by every piece outline.

- [ ] **Step 1: Take the before screenshots**

With the dev server running, screenshot (browser screenshot, or Playwright's `locator('.board').screenshot()`) these three, and keep them for Task 5's before/after pairs:
- http://localhost:5173/dev/board as it opens (the starting position);
- the same board after choosing "Piece falls (no saving roll)" with target `b7` in the sandbox's dice panel and playing any White move, so the player bar shows a lost black pawn;
- the same board with "You are" set to Black.

- [ ] **Step 2: Replace the pieces with mpchess, pinned**

```bash
SHA=5ae58154b2be1033dcb0252173b4fff4bb02e73a
for c in wK wQ wR wB wN wP bK bQ bR bB bN bP; do
  curl -sfL -o web/static/pieces/$c.svg "https://raw.githubusercontent.com/lichess-org/lila/$SHA/public/piece/mpchess/$c.svg"
done
curl -sfL -o web/static/pieces/GPL-3.0.txt https://raw.githubusercontent.com/spdx/license-list-data/main/text/GPL-3.0-or-later.txt
grep -L "<script" web/static/pieces/*.svg | wc -l
```

Expected: the last line prints `12` (no piece file contains a script). `GPL-3.0.txt` starts with `GNU GENERAL PUBLIC LICENSE` and is about 34 KB. (gnu.org refused scripted downloads when this plan was written; SPDX serves the same text.)

- [ ] **Step 3: Replace the license notice**

Replace `web/static/pieces/LICENSE.txt` with:

```text
Chess piece images: the "mpchess" set by Maxime Chupin
(https://github.com/chupinmaxime), as published in lichess
(https://github.com/lichess-org/lila/tree/5ae58154b2be1033dcb0252173b4fff4bb02e73a/public/piece/mpchess).

These files are free software: you can redistribute them and/or modify them
under the terms of the GNU General Public License as published by the Free
Software Foundation, either version 3 of the License, or (at your option) any
later version. They are distributed without any warranty; see GPL-3.0.txt,
next to this file, for the full license.

Pothole Chess draws them 10% larger, with a cream outline added by CSS; the
files themselves are unchanged.
```

- [ ] **Step 4: Add the outline token**

In `web/src/lib/theme/tokens.css`, after `--piece-dark: #141518;`, add:

```css
	--piece-outline: #fbf8f0; /* the white border around every piece */
```

- [ ] **Step 5: Outline and enlarge the board's pieces**

In `web/src/lib/Board.svelte`, replace:

```css
	.piece img {
		width: 92%;
		height: 92%;
		filter: drop-shadow(0 2px 2px var(--hole));
	}
```

with:

```css
	/* mpchess pieces fill the square (110% of the old 92%), with a crisp
	   white outline: their own shape offset 1.5 px four ways, no blur. */
	.piece img {
		width: 100%;
		height: 100%;
		filter: drop-shadow(1.5px 0 0 var(--piece-outline)) drop-shadow(-1.5px 0 0 var(--piece-outline))
			drop-shadow(0 1.5px 0 var(--piece-outline)) drop-shadow(0 -1.5px 0 var(--piece-outline));
	}
```

In the same file, make `.piece .mamdani` start with `filter: none;`:

```css
	.piece .mamdani {
		filter: none; /* its yellow border is its outline */
		box-sizing: border-box;
```

and add the outline to `.promote img`:

```css
	.promote img {
		width: 80%;
		height: 80%;
		filter: drop-shadow(1.5px 0 0 var(--piece-outline)) drop-shadow(-1.5px 0 0 var(--piece-outline))
			drop-shadow(0 1.5px 0 var(--piece-outline)) drop-shadow(0 -1.5px 0 var(--piece-outline));
	}
```

- [ ] **Step 6: Outline the lost pieces and the home page's boards**

In `web/src/lib/PlayerBar.svelte`, remove ` class:black={p[0] === 'b'}` from both `<img>` tags, and replace the `.glyphs img.black` rule (with its comment) with:

```css
	/* The board's white outline, 1 px at this size, so black pieces show on
	   the dark page. */
	.glyphs img {
		filter: drop-shadow(1px 0 0 var(--piece-outline)) drop-shadow(-1px 0 0 var(--piece-outline))
			drop-shadow(0 1px 0 var(--piece-outline)) drop-shadow(0 -1px 0 var(--piece-outline));
	}
```

In `web/src/lib/MiniBoard.svelte`, replace:

```css
	img {
		width: 92%;
		height: 92%;
	}
```

with:

```css
	/* The board's pieces at their size, with a 1 px white outline. */
	img {
		width: 100%;
		height: 100%;
		filter: drop-shadow(1px 0 0 var(--piece-outline)) drop-shadow(-1px 0 0 var(--piece-outline))
			drop-shadow(0 1px 0 var(--piece-outline)) drop-shadow(0 -1px 0 var(--piece-outline));
	}
```

and make its `.mamdani` rule start with `filter: none; /* its yellow border is its outline */`.

- [ ] **Step 7: Redraw the link-preview card**

Run: `pnpm --dir web og-image`
Expected: `wrote static/og.png` and `wrote static/apple-touch-icon.png`. Open `web/static/og.png`: the board corner shows the mpchess pieces.

- [ ] **Step 8: Check and look**

Run (from `web/`): `npx @sveltejs/mcp svelte-autofixer` on `./src/lib/Board.svelte`, `./src/lib/PlayerBar.svelte` and `./src/lib/MiniBoard.svelte`
Expected: `issues: []` for each.
Run: `pnpm --dir web check`
Expected: `0 ERRORS 0 WARNINGS`.
Look at http://localhost:5173/dev/board: bold mpchess pieces fill their squares, black ones with a crisp cream border (no haze), and the Mamdani tile keeps its yellow border with no outline.

- [ ] **Step 9: Commit**

```bash
git add web/static/pieces web/src/lib/theme/tokens.css web/src/lib/Board.svelte web/src/lib/PlayerBar.svelte web/src/lib/MiniBoard.svelte web/static/og.png
git commit -m "web: mpchess pieces, 10% larger, with a white outline instead of a shadow"
```

---

### Task 3: Ease-out glide, trail and whiplash

**Files:**
- Modify: `web/src/lib/feel.ts`, `web/src/lib/feel.test.ts`
- Modify: `web/src/lib/theme/tokens.css`
- Modify: `web/src/lib/Board.svelte`

**Interfaces:**
- Consumes: `cellOf(sq, flipped)` from Task 1.
- Produces in `feel.ts`: `GLIDE_EASE: string`, `TRAIL_FADE_MS = 180`, `WHIP_TAIL_MS = 160`, `trailOf(from: string, to: string, flipped: boolean): { x: number; y: number; length: number; angle: number }`, `whiplash(from: string, to: string, flipped: boolean): number`, `whipFrames(w: number): Keyframe[]`, `trailColor(code: string): string`. Produces in `Board.svelte`: the `motion(p)` attachment and the `dropped` bookkeeping that Task 4 extends.

- [ ] **Step 1: Write the failing tests**

In `web/src/lib/feel.test.ts`, change the import to:

```ts
import { cellOf, coordinates, trailColor, trailOf, whipFrames, whiplash } from './feel.ts';
```

and append:

```ts
describe('trailOf', () => {
	it("runs from the start square's centre, in squares and degrees", () => {
		expect(trailOf('a1', 'h1', false)).toEqual({ x: 0.5, y: 7.5, length: 7, angle: 0 });
		const up = trailOf('e2', 'e4', false);
		expect(up).toMatchObject({ x: 4.5, y: 6.5, length: 2 });
		expect(up.angle).toBeCloseTo(-90);
	});
	it('follows the board when it is flipped', () => {
		const t = trailOf('e2', 'e4', true);
		expect(t).toMatchObject({ x: 3.5, y: 1.5, length: 2 });
		expect(t.angle).toBeCloseTo(90);
	});
	it('measures a knight move along its diagonal', () => {
		expect(trailOf('g1', 'f3', false).length).toBeCloseTo(Math.sqrt(5));
	});
});

describe('whiplash', () => {
	it('swings 5 degrees on a move straight across', () => {
		expect(whiplash('a1', 'h1', false)).toBeCloseTo(5);
		expect(whiplash('h1', 'a1', false)).toBeCloseTo(-5);
	});
	it('barely tilts a move straight up or down', () => {
		expect(whiplash('e2', 'e4', false)).toBeCloseTo(0);
	});
	it('scales with how sideways the move is, and flips with the board', () => {
		expect(whiplash('a1', 'h8', false)).toBeCloseTo(5 / Math.SQRT2);
		expect(whiplash('a1', 'h1', true)).toBeCloseTo(-5);
	});
	it('is zero for a piece that did not move', () => {
		expect(whiplash('e4', 'e4', false)).toBe(0);
	});
});

describe('whipFrames', () => {
	it('leans back, whips forward, swings back and settles upright', () => {
		const angles = whipFrames(5).map((k) => k.transform);
		expect(angles).toEqual(['rotate(0deg)', 'rotate(-3deg)', 'rotate(5deg)', 'rotate(-1.5deg)', 'rotate(0deg)']);
	});
});

describe('trailColor', () => {
	it('tints the trail by the piece', () => {
		expect(trailColor('wN')).toBe('var(--trail-white)');
		expect(trailColor('bB')).toBe('var(--trail-black)');
		expect(trailColor('M')).toBe('var(--trail-mamdani)');
	});
});
```

- [ ] **Step 2: Run them to verify they fail**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts`
Expected: FAIL, 9 failed (the new functions don't exist), 3 passed.

- [ ] **Step 3: Write the helpers**

Append to `web/src/lib/feel.ts`:

```ts

/** The glide's easing: a piece leaves fast and settles gently (easeOutQuart). */
export const GLIDE_EASE = 'cubic-bezier(0.25, 1, 0.5, 1)';
/** How long a trail takes to fade once its piece lands. */
export const TRAIL_FADE_MS = 180;
/** How long the whiplash runs on after the glide, settling the piece. */
export const WHIP_TAIL_MS = 160;

/**
 * A trail from the centre of `from` toward `to` on screen: where it starts
 * (x, y) and its length, in squares, and its angle in degrees (0 points
 * right, 90 down).
 */
export function trailOf(from: string, to: string, flipped: boolean): { x: number; y: number; length: number; angle: number } {
	const a = cellOf(from, flipped);
	const b = cellOf(to, flipped);
	const dx = b.col - a.col;
	const dy = b.row - a.row;
	return { x: a.col + 0.5, y: a.row + 0.5, length: Math.hypot(dx, dy), angle: (Math.atan2(dy, dx) * 180) / Math.PI };
}

/**
 * The whiplash's peak in degrees: 5 times the sideways share of the move on
 * screen, positive moving right. A move straight up or down barely tilts.
 */
export function whiplash(from: string, to: string, flipped: boolean): number {
	const a = cellOf(from, flipped);
	const b = cellOf(to, flipped);
	const dx = b.col - a.col;
	const dist = Math.hypot(dx, b.row - a.row);
	return dist === 0 ? 0 : (5 * dx) / dist;
}

/**
 * The whiplash for peak `w`: the piece leans back as it launches, whips
 * forward along the move as it brakes, swings back a touch and settles.
 */
export function whipFrames(w: number): Keyframe[] {
	const r = (deg: number) => `rotate(${Math.round(deg * 100) / 100 || 0}deg)`;
	return [
		{ transform: r(0) },
		{ transform: r(-w * 0.6), offset: 0.12 },
		{ transform: r(w), offset: 0.62 },
		{ transform: r(-w * 0.3), offset: 0.84 },
		{ transform: r(0) }
	];
}

/** A trail's color, from the piece that leaves it ("wN", "bB", "M"). */
export function trailColor(code: string): string {
	if (code === 'M') return 'var(--trail-mamdani)';
	return code[0] === 'w' ? 'var(--trail-white)' : 'var(--trail-black)';
}
```

- [ ] **Step 4: Run them to verify they pass**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts`
Expected: PASS, 12 tests.

- [ ] **Step 5: Add the trail colors**

In `web/src/lib/theme/tokens.css`, after the `--piece-outline` line, add:

```css
	--trail-white: #fbf8f099; /* the streak behind a gliding piece */
	--trail-black: #1415188c;
	--trail-mamdani: #f2c2308c;
```

- [ ] **Step 6: Draw the trails, glide ease-out, whiplash**

In `web/src/lib/Board.svelte`:

1. Change the `feel.ts` import to:

```ts
	import { cellOf, coordinates, GLIDE_EASE, trailColor, trailOf, TRAIL_FADE_MS, whipFrames, whiplash, WHIP_TAIL_MS } from './feel.ts';
```

2. In the `pieces` derivation, keep each piece's previous square: change `return { ...p, dur: … };` to:

```ts
			return { ...p, from, dur: from && !reducedMotion() ? moveDuration(from, p.sq) : 0 };
```

3. Right after the `pieces` derivation, add:

```ts
	// The move you just dragged: that piece is already where you put it, so
	// it gets no trail and no whiplash. Forgotten on your next press.
	let dropped: { from: string; to: string } | null = null;
	const isDropped = (p: { from?: string; sq: string }) => dropped !== null && p.from === dropped.from && p.sq === dropped.to;

	// Trails behind the pieces that just moved. A dice step recalculates the
	// pieces (with nothing moving) while a long glide is still going, so the
	// last trails stay until a real move replaces them; prevTrails is plain
	// bookkeeping, like prevPieces.
	type Trail = ReturnType<typeof trailOf> & { key: string; dur: number; color: string };
	let prevTrails: Trail[] = [];
	let trails = $derived.by(() => {
		const moving = pieces.filter((p) => p.dur > 0 && p.from && !isDropped(p));
		if (moving.length === 0) return prevTrails;
		prevTrails = moving.map((p) => ({ ...trailOf(p.from!, p.sq, flipped), key: `${p.id}:${p.sq}`, dur: p.dur, color: trailColor(p.code) }));
		return prevTrails;
	});

	/**
	 * A piece's one-off animations: the whiplash when it glides. The
	 * attachment re-runs whenever the board recalculates, so it remembers
	 * what it last played.
	 */
	function motion(p: { id: number; from?: string; sq: string; dur: number }) {
		return (node: HTMLElement) => {
			if (reducedMotion()) return;
			const key = `${p.id}:${p.sq}`;
			if (!p.from || p.from === p.sq || node.dataset.moved === key) return;
			node.dataset.moved = key;
			if (isDropped(p)) return;
			const w = whiplash(p.from, p.sq, flipped);
			if (w !== 0 && p.dur > 0) node.animate(whipFrames(w), { duration: p.dur + WHIP_TAIL_MS, easing: 'ease-in-out' });
		};
	}
```

4. In `pointerDown`, make the first line `dropped = null;`. In `pointerUp`, replace:

```ts
		const to = squareFromPoint(e.clientX, e.clientY);
		if (!to || to === from || !moveTo(from, to)) selected = from;
```

with:

```ts
		const to = squareFromPoint(e.clientX, e.clientY);
		if (to && to !== from) dropped = { from, to };
		if (!to || to === from || !moveTo(from, to)) {
			dropped = null;
			selected = from;
		}
```

5. Give the board its easing and fade time: in the `<div class="board" …>` tag, add before `{@attach dragArea}`:

```svelte
style="--glide-ease: {GLIDE_EASE}; --trail-fade: {TRAIL_FADE_MS}ms"
```

6. In the pieces layer, before `{#each pieces as p (p.id)}`, add:

```svelte
		<!-- Under the pieces: a streak from where each moving piece started. -->
		{#each trails as t (t.key)}
			<span class="trail" style="--x: {t.x}; --y: {t.y}; --len: {t.length}; --angle: {t.angle}deg; --dur: {t.dur}ms; --color: {t.color}"></span>
		{/each}
```

and on `<span class="piece" out:leave={…}>` add `{@attach motion(p)}`.

7. In `<style>`, replace the `.piece-slot` rule and its comment with:

```css
	/* The glide: ease-out (GLIDE_EASE in feel.ts), duration by distance
	   (pieces.ts moveDuration). */
	.piece-slot {
		transition: transform var(--dur, 0ms) var(--glide-ease);
		z-index: 2;
	}
	/* The streak behind a gliding piece: 55% of a square wide, from its start
	   square's centre, growing with the glide so its head stays under the
	   piece, then fading. Transparent at the start, solid at the piece. */
	.trail {
		position: absolute;
		left: calc(var(--x) * 12.5%);
		top: calc(var(--y) * 12.5% - 3.4375%);
		width: calc(var(--len) * 12.5%);
		height: 6.875%;
		border-radius: 999px;
		background: linear-gradient(90deg, transparent, var(--color));
		transform-origin: 0 50%;
		z-index: 1;
		animation:
			trail-grow var(--dur) var(--glide-ease) both,
			trail-fade var(--trail-fade) linear var(--dur) forwards;
	}
	@keyframes trail-grow {
		from {
			transform: rotate(var(--angle)) scaleX(0);
		}
		to {
			transform: rotate(var(--angle)) scaleX(1);
		}
	}
	@keyframes trail-fade {
		to {
			opacity: 0;
		}
	}
```

and add `transform-origin: 50% 85%;` (with the comment `/* The whiplash pivots from the base, so the head swings. */`) to the `.piece` rule, before its `transition`.

- [ ] **Step 7: Check, then watch a glide over a dice step**

Run (from `web/`): `npx @sveltejs/mcp svelte-autofixer ./src/lib/Board.svelte`, expecting `issues: []`; then `pnpm --dir web check`, expecting `0 ERRORS 0 WARNINGS`.

In http://localhost:5173/dev/board, play g1-f3 by two taps and screenshot about 140 ms later: the knight is nearly on f3 with a cream streak back to g1. After about a second, `document.querySelectorAll('.trail')` has one element with computed opacity `0`. Then, in the sandbox's dice panel, choose "Pothole opens" with target `h6`, leave "Slow dice (3×)" unticked, tick "Move either side any time", and play a2-a4, Ra1-a3, then Ra3-h3 (a long rook move, 750 ms): the streak keeps growing to h3 while the dice step updates the board 550 ms in, and does not vanish early.

- [ ] **Step 8: Commit**

```bash
git add web/src/lib/feel.ts web/src/lib/feel.test.ts web/src/lib/theme/tokens.css web/src/lib/Board.svelte
git commit -m "web: ease-out glide with a trail and a whiplash"
```

---

### Task 4: Pick-up wobble and tilt, legal moves ripple in, landing squash

**Files:**
- Modify: `web/src/lib/feel.ts`, `web/src/lib/feel.test.ts`
- Modify: `web/src/lib/Board.svelte`

**Interfaces:**
- Consumes: `motion(p)`, `dropped` and `isDropped` from Task 3.
- Produces: `rippleDelay(from: string, to: string): number` in `feel.ts`.

- [ ] **Step 1: Write the failing test**

In `web/src/lib/feel.test.ts`, add `rippleDelay` to the import, and append:

```ts
describe('rippleDelay', () => {
	it('waits 15 ms per square of distance from the picked-up piece', () => {
		expect(rippleDelay('e2', 'e3')).toBe(15);
		expect(rippleDelay('e2', 'e4')).toBe(30);
		expect(rippleDelay('a1', 'h8')).toBe(Math.round(15 * 7 * Math.SQRT2));
	});
	it('pops the nearer squares first, whichever way the board faces', () => {
		const near = rippleDelay('d4', 'e5'),
			far = rippleDelay('d4', 'h8');
		expect(near).toBeLessThan(far);
	});
});
```

- [ ] **Step 2: Run it to verify it fails**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts`
Expected: FAIL, 2 failed, 12 passed.

- [ ] **Step 3: Write the helper**

Append to `web/src/lib/feel.ts`:

```ts

/**
 * How long a legal square waits before popping in when a piece is picked
 * up: 15 ms per square of distance, so the moves ripple out from the piece.
 */
export function rippleDelay(from: string, to: string): number {
	const df = to.charCodeAt(0) - from.charCodeAt(0);
	const dr = Number(to[1]) - Number(from[1]);
	return Math.round(15 * Math.hypot(df, dr));
}
```

- [ ] **Step 4: Run it to verify it passes**

Run: `pnpm --dir web exec vitest run src/lib/feel.test.ts`
Expected: PASS, 14 tests.

- [ ] **Step 5: Wobble, tilt, ripple and squash on the board**

In `web/src/lib/Board.svelte`:

1. Add `rippleDelay` to the `feel.ts` import.

2. Replace the `drag` state declaration and its comment with:

```ts
	// x0, y0: where the press started; x, y: where the pointer is now; tilt:
	// how far the held piece leans toward the pull, in degrees.
	let drag = $state<{ from: string; x0: number; y0: number; x: number; y: number; moved: boolean; pointer: number; tilt: number } | null>(null);
	// A dragged piece dropped on a square it can't go to settles back with a
	// squash; n changes each time so the same square can bounce again.
	let bounce = $state({ sq: '', n: 0 });
```

3. Replace the whole `motion()` function from Task 3 (and its comment) with:

```ts
	/**
	 * A piece's one-off animations: the whiplash when it glides, a squash
	 * when you drop it on a square, or when it settles back from one it
	 * can't go to. The attachment re-runs whenever the board recalculates,
	 * so it remembers what it last played.
	 */
	function motion(p: { id: number; from?: string; sq: string; dur: number }) {
		return (node: HTMLElement) => {
			const settled = bounce.sq === p.sq ? `bounce:${bounce.n}` : '';
			if (reducedMotion()) return;
			if (settled && node.dataset.bounce !== settled) {
				node.dataset.bounce = settled;
				node.animate(SQUASH, { duration: 140, easing: 'ease-out' });
				return;
			}
			const key = `${p.id}:${p.sq}`;
			if (!p.from || p.from === p.sq || node.dataset.moved === key) return;
			node.dataset.moved = key;
			if (isDropped(p)) {
				node.animate(SQUASH, { duration: 140, easing: 'ease-out' });
				return;
			}
			const w = whiplash(p.from, p.sq, flipped);
			if (w !== 0 && p.dur > 0) node.animate(whipFrames(w), { duration: p.dur + WHIP_TAIL_MS, easing: 'ease-in-out' });
		};
	}
	const SQUASH: Keyframe[] = [{ transform: 'scale(1.08, 0.92)' }, { transform: 'scale(1)' }];
```

4. In `pointerDown`, add `tilt: 0` to the new `drag` object. In `pointerMove`, replace `drag = { ...drag, x: e.clientX, y: e.clientY, moved };` with:

```ts
		// Lean toward the pull, from how fast the pointer moves sideways,
		// smoothed so a jittery drag doesn't wobble.
		const tilt = moved ? Math.max(-10, Math.min(10, drag.tilt * 0.6 + (e.clientX - drag.x) * 1.2 * 0.4)) : 0;
		drag = { ...drag, x: e.clientX, y: e.clientY, moved, tilt };
```

5. In `pointerUp`'s failure branch (from Task 3), add `bounce = { sq: from, n: bounce.n + 1 };` after `selected = from;`.

6. In `pieceStyle`, add `; --tilt: ${drag.tilt.toFixed(1)}deg` to the end of the dragging style string, so it reads:

```ts
			return `transform: translate(${drag.x - r.left - size / 2}px, ${drag.y - r.top - size / 2}px); transition: none; z-index: 3; --tilt: ${drag.tilt.toFixed(1)}deg`;
```

7. On the piece slot, after `class:lifted={…}`, add:

```svelte
				class:picked={current === p.sq && !drag?.moved}
```

and on the square button, after `class:legal={targets.has(sq)}`, add:

```svelte
			style={current && targets.has(sq) ? `--ripple: ${rippleDelay(current, sq)}ms` : undefined}
```

8. In `<style>`, replace the `.dragging .piece` rule with:

```css
	.dragging .piece {
		transform: scale(1.12) rotate(var(--tilt, 0deg));
		transition: transform 0.12s ease-out;
	}
	/* Picked up: a quick wobble as it lifts. */
	.picked .piece {
		animation: wobble 0.22s ease-out;
	}
	@keyframes wobble {
		0% {
			transform: scale(1);
		}
		30% {
			transform: scale(1.14) rotate(-6deg);
		}
		65% {
			transform: scale(1.12) rotate(5deg);
		}
		100% {
			transform: scale(1.12) rotate(0);
		}
	}
```

replace the `.square.legal::after` rule with:

```css
	/* Legal squares pop in, nearest the piece first (rippleDelay). */
	.square.legal::after {
		background: var(--legal-fill);
		animation: ripple 0.2s ease-out var(--ripple, 0ms) both;
	}
	@keyframes ripple {
		0% {
			transform: scale(0);
			opacity: 0;
		}
		70% {
			transform: scale(1.12);
			opacity: 1;
		}
		100% {
			transform: scale(1);
		}
	}
```

and add, before the last `@media (prefers-reduced-motion: reduce)` block:

```css
	@media (prefers-reduced-motion: reduce) {
		.picked .piece,
		.square.legal::after,
		.trail {
			animation: none;
		}
		.trail {
			display: none;
		}
	}
```

- [ ] **Step 6: Check, then try a pick-up, a drag and an illegal drop**

Run (from `web/`): `npx @sveltejs/mcp svelte-autofixer ./src/lib/Board.svelte`, expecting `issues: []`; then `pnpm --dir web check`, expecting `0 ERRORS 0 WARNINGS`.

In http://localhost:5173/dev/board after 1. e4 e5: tap the f1 bishop and screenshot about 45 ms later: e2 is fully lit, d3 and c4 are still growing, b5 is just starting. Drag the d1 queen sideways toward h5 and hold: she leans toward the pull, up to 10°. Drop her on a square she can't reach: she settles back on d1 with a squash, and the square's label still reads `d1, white queen`. Drag a legal move: it lands with a squash, no trail, no whiplash.

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/feel.ts web/src/lib/feel.test.ts web/src/lib/Board.svelte
git commit -m "web: pick-up wobble and tilt, legal moves ripple in, drops land with a squash"
```

---

### Task 5: Check the whole board

**Files:** none changed (unless a check fails).

- [ ] **Step 1: Run every test and check**

Run: `pnpm --dir web test`
Expected: all pass (152 when this plan was written).
Run: `pnpm --dir web check`
Expected: `0 ERRORS 0 WARNINGS`.
Run: `pnpm --dir web build`
Expected: `✔ done`.

- [ ] **Step 2: Playtest with full animations**

Build the frontend, run `go run ./cmd/server` on another port so instant mode is off (a production build), e.g. `PORT=8093 DB_PATH=/tmp/feel1.db go run ./cmd/server`, then:

Run: `pnpm --dir web playtest --base http://localhost:8093 --games 2 --max-plies 30 --turn-ms 15000`
Expected: `chromium: 2/2 games finished cleanly`, with dragged moves in each game.
Run: `pnpm --dir web playtest --base http://localhost:8093 --games 1 --max-plies 24 --turn-ms 15000 --browser webkit`
Expected: `webkit: 1/1 games finished cleanly`.
Run: `pnpm --dir web playtest --base http://localhost:8093 --games 2 --max-plies 24 --turn-ms 15000 --phone`
Expected: `2/2 games finished cleanly`.

- [ ] **Step 3: Instant mode shows no effects**

In http://localhost:5173/dev/board, tick Instant and play a move by tapping: no `.trail` element appears (`document.querySelectorAll('.trail').length` is `0` on a fresh page), and the piece jumps without a whiplash.

- [ ] **Step 4: Before and after, as Black too**

Take the same three screenshots as Task 2, Step 1 (start position, lost pawn in the player bar, "You are" Black), and show them next to the befores in pairs. As Black, the labels read 1 to 8 down and h to a across, and a trail still runs from the piece's start square to its landing square.

- [ ] **Step 5: Mark the spec**

In `docs/superpowers/specs/2026-10-06-board-feel-design.md`, change `**Status:** design agreed item by item on 2026-10-05 and 06. Not built.` to `**Status:** design agreed item by item on 2026-10-05 and 06. Sections 1, 2, 3 and 10 built (plan 1); the rest follow in plans 2 and 3.`

```bash
git add docs/superpowers/specs/2026-10-06-board-feel-design.md
git commit -m "Docs: board feel plan 1 built"
```
