<script lang="ts">
	import { blockedSquares, CELEBRATION_MS, celebratedSoFar, markCelebrated, squareIndex, type Stage } from './board.ts';
	import { pieceName, squareName, type MoveJSON } from './game.ts';
	import { cellOf, coordinates, GLIDE_EASE, rippleDelay, trailColor, trailOf, TRAIL_FADE_MS, whipFrames, whiplash, WHIP_TAIL_MS } from './feel.ts';
	import { exitMs, reducedMotion } from './motion.ts';
	import { moveDuration, reconcile, type PieceRef } from './pieces.ts';

	let {
		stage,
		legal,
		lastMove = null,
		flipped = false,
		interactive = false,
		dim = false,
		check = '',
		saved = '',
		repairs = [],
		onmove
	}: {
		stage: Stage;
		legal: MoveJSON[];
		lastMove?: { from: string; to: string } | null;
		flipped?: boolean;
		interactive?: boolean;
		dim?: boolean;
		/** The king's square to glow when in check, or "". */
		check?: string;
		/** A square whose piece just survived a saving roll, or "". */
		saved?: string;
		/**
		 * Potholes the Mamdani has just repaired, to celebrate. `hole` is true
		 * when an open pothole is fixed by the Mamdani's move, false when the
		 * dice land next to it and the pothole never opens. Each key plays once.
		 */
		repairs?: { sq: string; key: string; hole: boolean }[];
		onmove: (move: MoveJSON) => void;
	} = $props();

	const promoOptions = [
		{ promo: 'q', kind: 'Q', name: 'Queen' },
		{ promo: 'r', kind: 'R', name: 'Rook' },
		{ promo: 'b', kind: 'B', name: 'Bishop' },
		{ promo: 'n', kind: 'N', name: 'Knight' }
	];

	let selected = $state<string | null>(null);
	let promoting = $state<{ from: string; to: string } | null>(null);
	let hovered = $state<string | null>(null);
	// x0, y0: where the press started; x, y: where the pointer is now; tilt:
	// how far the held piece leans toward the pull, in degrees.
	let drag = $state<{ from: string; x0: number; y0: number; x: number; y: number; moved: boolean; pointer: number; tilt: number } | null>(null);
	// A dragged piece dropped on a square it can't go to settles back with a
	// squash; n changes each time so the same square can bounce again.
	let bounce = $state({ sq: '', n: 0 });
	// The square a piece just settled back on: it stays selected, but without
	// replaying the pick-up wobble. Cleared on the next press.
	let settledBack = $state('');
	let boardEl: HTMLDivElement | undefined = $state();

	// Without moves to make (not your turn, dice still rolling) nothing is selectable.
	let active = $derived(interactive && legal.length > 0);
	let current = $derived(active ? selected : null);
	let pending = $derived(active ? promoting : null);

	let order = $derived.by(() => {
		const squares: number[] = [];
		for (let row = 0; row < 8; row++) {
			for (let col = 0; col < 8; col++) {
				const rank = flipped ? row : 7 - row;
				const file = flipped ? 7 - col : col;
				squares.push(rank * 8 + file);
			}
		}
		return squares;
	});
	let targets = $derived(new Set(legal.filter((m) => m.from === current).map((m) => m.to)));
	let movable = $derived(new Set(active ? legal.map((m) => m.from) : []));
	let blocked = $derived(new Set(current ? blockedSquares(stage, current) : []));
	let repairedSquares = $derived(new Set(repairs.map((r) => r.sq)));
	// Repairs already celebrated on a board before this one mounted (the game
	// page swaps boards at 640 px) don't play again.
	const playedBefore = celebratedSoFar();
	let fresh = $derived(repairs.filter((r) => !playedBefore.has(r.key)));
	// A pothole fixed by the Mamdani's move waits for the Mamdani to arrive.
	let glide = $derived(lastMove && lastMove.to === stage.mamdani && !reducedMotion() ? moveDuration(lastMove.from, lastMove.to) : 0);
	let sparks = Array.from({ length: 10 }, (_, i) => i * 36);

	// Each piece keeps an id across positions so it can glide (see pieces.ts).
	// prevPieces is plain bookkeeping for the next reconcile, not state.
	let prevPieces: PieceRef[] = [];
	let nextId = 0;
	let pieces = $derived.by(() => {
		const before = new Map(prevPieces.map((p) => [p.id, p.sq]));
		const next = reconcile(prevPieces, stage.board, stage.mamdani, () => ++nextId, lastMove ?? undefined);
		prevPieces = next;
		return next.map((p) => {
			const from = before.get(p.id);
			return { ...p, from, dur: from && !reducedMotion() ? moveDuration(from, p.sq) : 0 };
		});
	});

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
				node.animate(SQUASH, { duration: 140, easing: 'ease-out', composite: 'add' });
				return;
			}
			const key = `${p.id}:${p.sq}`;
			if (!p.from || p.from === p.sq || node.dataset.moved === key) return;
			node.dataset.moved = key;
			if (isDropped(p)) {
				node.animate(SQUASH, { duration: 140, easing: 'ease-out', composite: 'add' });
				return;
			}
			const w = whiplash(p.from, p.sq, flipped);
			if (w !== 0 && p.dur > 0) node.animate(whipFrames(w), { duration: p.dur + WHIP_TAIL_MS, easing: 'ease-in-out' });
		};
	}
	// Added on top of the piece's resting transform (composite: 'add'), so a
	// piece that stays picked up at 112% squashes from there, not from 100%.
	const SQUASH: Keyframe[] = [{ transform: 'scale(1.08, 0.92)' }, { transform: 'scale(1)' }];

	/** Column and row on screen for a square, 0..7 from the top left. */
	function cell(sq: string): { col: number; row: number } {
		return cellOf(sq, flipped);
	}
	let labels = $derived(coordinates(flipped));

	function place(sq: string): string {
		const { col, row } = cell(sq);
		return `--col: ${col}; --row: ${row}`;
	}

	function pieceStyle(p: PieceRef & { dur: number }): string {
		if (drag?.moved && drag.from === p.sq && boardEl) {
			const r = boardEl.getBoundingClientRect();
			const size = r.width / 8;
			return `transform: translate(${drag.x - r.left - size / 2}px, ${drag.y - r.top - size / 2}px); transition: none; z-index: 3; --tilt: ${drag.tilt.toFixed(1)}deg`;
		}
		return `${place(p.sq)}; --dur: ${p.dur}ms`;
	}

	function pieceAt(sq: string): string {
		return sq === stage.mamdani ? 'M' : stage.board[squareIndex(sq)];
	}

	function label(sq: string): string {
		const piece = pieceAt(sq);
		let text = sq;
		if (piece) text += `, ${pieceName(piece)}`;
		const hole = stage.potholes.find((h) => h.sq === sq);
		if (hole) {
			text += `, pothole (${hole.by === 'white' ? 'White' : 'Black'}’s`;
			text += hole.left === 1 ? ', closes after their next move' : `, ${hole.left} rounds left`;
			text += ')';
		}
		return text;
	}

	function moveTo(from: string, to: string) {
		const moves = legal.filter((m) => m.from === from && m.to === to);
		if (moves.length === 0) return false;
		if (moves.length > 1) {
			promoting = { from, to };
			return true;
		}
		selected = null;
		onmove(moves[0]);
		return true;
	}

	function tap(sq: string) {
		if (pending) return;
		if (current && targets.has(sq) && moveTo(current, sq)) return;
		selected = movable.has(sq) && current !== sq ? sq : null;
	}

	function squareFromPoint(x: number, y: number): string | null {
		if (!boardEl) return null;
		const r = boardEl.getBoundingClientRect();
		const col = Math.floor(((x - r.left) / r.width) * 8);
		const row = Math.floor(((y - r.top) / r.height) * 8);
		if (col < 0 || col > 7 || row < 0 || row > 7) return null;
		const rank = flipped ? row : 7 - row;
		const file = flipped ? 7 - col : col;
		return squareName(rank * 8 + file);
	}

	function pointerDown(e: PointerEvent, sq: string) {
		dropped = null;
		settledBack = '';
		if (pending || !movable.has(sq) || e.button !== 0) return;
		drag = { from: sq, x0: e.clientX, y0: e.clientY, x: e.clientX, y: e.clientY, moved: false, pointer: e.pointerId, tilt: 0 };
	}

	// The pointer is only captured once it has really moved. Capturing on
	// pointerdown would send the click to the board instead of the square,
	// and a plain tap would never select anything. After a drag the click
	// lands on the board too, so no square's tap handler runs.
	function pointerMove(e: PointerEvent) {
		if (!drag || e.pointerId !== drag.pointer) return;
		if (e.buttons === 0) {
			drag = null; // the button came up somewhere we didn't see
			return;
		}
		// Measured from the press, not the last event: a slow drag moves
		// less than 6px between events and would otherwise never start.
		const moved = drag.moved || Math.hypot(e.clientX - drag.x0, e.clientY - drag.y0) > 6;
		if (moved && !drag.moved) {
			boardEl?.setPointerCapture(e.pointerId);
			selected = drag.from;
		}
		// Lean toward the pull, from how fast the pointer moves sideways,
		// smoothed so a jittery drag doesn't wobble.
		const tilt = moved && !reducedMotion() ? Math.max(-10, Math.min(10, drag.tilt * 0.6 + (e.clientX - drag.x) * 1.2 * 0.4)) : 0;
		drag = { ...drag, x: e.clientX, y: e.clientY, moved, tilt };
	}

	function pointerUp(e: PointerEvent) {
		if (!drag || e.pointerId !== drag.pointer) return;
		const { from, moved } = drag;
		drag = null;
		if (!moved) return; // a tap: the square's click handler deals with it
		const to = squareFromPoint(e.clientX, e.clientY);
		if (to && to !== from) dropped = { from, to };
		if (!to || to === from || !moveTo(from, to)) {
			dropped = null;
			selected = from;
			bounce = { sq: from, n: bounce.n + 1 };
			settledBack = from;
		}
	}

	// Drag follows the pointer across the whole board, so the listeners live
	// on the board element rather than on each square.
	function dragArea(node: HTMLDivElement) {
		boardEl = node;
		const cancel = () => (drag = null);
		const leave = () => (hovered = null);
		node.addEventListener('pointermove', pointerMove);
		node.addEventListener('pointerup', pointerUp);
		node.addEventListener('pointercancel', cancel);
		node.addEventListener('lostpointercapture', cancel);
		node.addEventListener('pointerleave', leave);
		return () => {
			node.removeEventListener('pointermove', pointerMove);
			node.removeEventListener('pointerup', pointerUp);
			node.removeEventListener('pointercancel', cancel);
			node.removeEventListener('lostpointercapture', cancel);
			node.removeEventListener('pointerleave', leave);
			boardEl = undefined;
		};
	}

	function promote(promo: string) {
		if (!pending) return;
		const { from, to } = pending;
		promoting = null;
		selected = null;
		onmove({ from, to, promo });
	}

	function promoKey(e: KeyboardEvent) {
		if (e.key === 'Escape') promoting = null;
	}

	// Focus the first choice when the picker opens, so Escape and the arrow
	// keys work at once, without tabbing in.
	function focusFirst(node: HTMLElement) {
		node.querySelector('button')?.focus();
	}

	// Transitions. Each one collapses to nothing under reduced motion.
	const ms = (n: number) => (reducedMotion() ? 0 : n);

	/** A piece leaving the board: a fall shrinks into the hole, a capture fades. */
	function leave(_node: Element, { fell }: { fell: boolean }) {
		return fell
			? { duration: ms(450), css: (t: number) => `opacity: ${t}; transform-origin: 50% 50%; transform: scale(${0.25 + 0.75 * t}) rotate(${(1 - t) * 25}deg)` }
			: { duration: ms(300), css: (t: number) => `opacity: ${1 - (1 - t) ** 3}` };
	}

	/** A pothole cracks open. */
	function crack(_node: Element) {
		return { duration: ms(420), css: (t: number) => `transform: scale(${t < 0.7 ? t / 0.7 : 1 + 0.12 * Math.sin(((t - 0.7) / 0.3) * Math.PI)}) rotate(${(1 - t) * -20}deg); opacity: ${Math.min(1, t * 2)}` };
	}

	/**
	 * A pothole closes on its schedule, or the cap closes it. A repaired one
	 * hides at once: the celebration draws its own hole shrinking under the
	 * cone. (1 ms, not 0: with Svelte 5.57, a turn whose leaving holes mixed a
	 * 0 ms exit with a timed one left them all on the board.)
	 */
	function closeUp(_node: Element, { sq }: { sq: string }) {
		if (repairedSquares.has(sq)) return { duration: ms(1), css: () => 'opacity: 0' };
		return { duration: ms(350), css: (t: number) => `transform: scale(${t}); opacity: ${t}` };
	}

	/**
	 * A repair celebration plays to the end even when the next move arrives
	 * first (the opponent can reply as soon as the dice stop): the node stays
	 * until its CSS animations are done.
	 */
	function born(node: HTMLElement) {
		node.dataset.born = String(performance.now());
	}
	function linger(node: Element) {
		const age = performance.now() - Number((node as HTMLElement).dataset.born);
		return { duration: exitMs(age, CELEBRATION_MS) };
	}

	/** A round passes: one of a hole's cones lifts away. */
	function liftCone(_node: Element) {
		return { duration: ms(300), css: (t: number) => `opacity: ${t}; transform: translateY(${(1 - t) * -60}%)` };
	}

	/** The target ring drops onto its square. */
	function drop(_node: Element) {
		return { duration: ms(260), css: (t: number) => `transform: scale(${1.5 - 0.5 * t}); opacity: ${t}` };
	}
</script>

{#snippet coneShape()}
	<path class="cone-body" d="M17.5 4h5l9.5 29h-24z" />
	<path class="cone-band" d="M14.6 13h10.8l1.7 5H12.9zM11.6 22h16.8l1.7 5H9.9z" />
	<rect class="cone-base" x="4" y="32" width="32" height="5" rx="1.5" />
{/snippet}

<!-- Coordinates sit outside the board, so the squares stay clean. The
     label column and row are the same size, so the frame stays square. -->
<div class="frame" class:dim>
<div class="ranks" aria-hidden="true">{#each labels.ranks as r (r)}<span>{r}</span>{/each}</div>
<div class="board" class:dim role="group" aria-label="Chessboard" style="--glide-ease: {GLIDE_EASE}; --trail-fade: {TRAIL_FADE_MS}ms" {@attach dragArea}>
	{#each order as index (index)}
		{@const sq = squareName(index)}
		{@const dark = (Math.floor(index / 8) + (index % 8)) % 2 === 0}
		<button
			class="square"
			class:dark
			class:last={lastMove?.from === sq || lastMove?.to === sq}
			class:legal={targets.has(sq)}
			style={current && targets.has(sq) ? `--ripple: ${rippleDelay(current, sq)}ms` : undefined}
			class:selected={current === sq}
			class:movable={movable.has(sq)}
			class:check={check === sq}
			aria-label={label(sq)}
			title={stage.potholes.some((h) => h.sq === sq) ? label(sq) : undefined}
			aria-pressed={current === sq}
			onclick={() => tap(sq)}
			onpointerdown={(e) => pointerDown(e, sq)}
			onpointerenter={() => (hovered = sq)}
		>
			{#if blocked.has(sq)}
				<span class="blocked" title="Blocked by a pothole">×</span>
			{/if}
			{#if saved === sq}
				<span class="saved"></span>
			{/if}
		</button>
	{/each}

	<div class="layer" aria-hidden="true">
		{#each stage.potholes as h (h.sq)}
			<span class="slot" style={place(h.sq)}>
				<span class="hole" in:crack out:closeUp={{ sq: h.sq }}>
					<!-- One cone per round left, on the hole's front edge. -->
					<span class="cones">
						{#each { length: h.left }, n (n)}
							<svg class="mini-cone" class:last={h.left === 1} viewBox="0 0 40 40" out:liftCone>
								{@render coneShape()}
							</svg>
						{/each}
					</span>
				</span>
			</span>
		{/each}
		{#if stage.target}
			{#key stage.target}
				<span class="slot" style={place(stage.target)}><span class="target" in:drop></span></span>
			{/key}
		{/if}
	</div>

	<div class="layer" aria-hidden="true">
		<!-- Under the pieces: a streak from where each moving piece started. -->
		{#each trails as t (t.key)}
			<span class="trail" style="--x: {t.x}; --y: {t.y}; --len: {t.length}; --angle: {t.angle}deg; --dur: {t.dur}ms; --color: {t.color}"></span>
		{/each}
		{#each pieces as p (p.id)}
			<span
				class="slot piece-slot"
				class:lifted={movable.has(p.sq) && (hovered === p.sq || current === p.sq) && !drag?.moved}
				class:picked={current === p.sq && !drag?.moved && settledBack !== p.sq}
				class:dragging={drag?.moved && drag.from === p.sq}
				style={pieceStyle(p)}
			>
				<span class="piece" out:leave={{ fell: stage.target === p.sq }} {@attach motion(p)}>
					{#if p.code === 'M'}
						<img class="mamdani" src="/mamdani/piece.webp" alt="" draggable="false" />
					{:else}
						<img src="/pieces/{p.code}.svg" alt="" draggable="false" />
					{/if}
				</span>
			</span>
		{/each}
	</div>

	<div class="layer celebrate" aria-hidden="true">
		{#each fresh as r (r.key)}
			<span
				class="slot fix"
				style="{place(r.sq)}; --delay: {r.hole ? glide : 0}ms"
				{@attach (node) => {
					born(node);
					markCelebrated(r.key);
				}}
				out:linger
			>
				{#if r.hole}<span class="hole patched"></span>{/if}
				<svg class="cone" viewBox="0 0 40 40">
					{@render coneShape()}
				</svg>
				<span class="flash"></span>
				{#each sparks as a, i (a)}<span class="spark" class:far={i % 2 === 0} style="--a: {a}deg"></span>{/each}
			</span>
		{/each}
		{#if fresh.length > 0 && stage.mamdani}
			{#key fresh[0].key}
				<span class="slot fix" class:top={cell(stage.mamdani).row === 0} style="{place(stage.mamdani)}; --delay: {fresh[0].hole ? glide : 0}ms" {@attach born} out:linger|global><span class="thumb">👍</span></span>
			{/key}
		{/if}
	</div>

	{#if pending}
		<div class="promote" role="dialog" aria-label="Promote to" tabindex="-1" onkeydown={promoKey} {@attach focusFirst}>
			{#each promoOptions as option (option.promo)}
				<button aria-label={option.name} onclick={() => promote(option.promo)}>
					<img src="/pieces/{pieceAt(pending.from)[0]}{option.kind}.svg" alt="" draggable="false" />
				</button>
			{/each}
			<button class="cancel" onclick={() => (promoting = null)}>Cancel</button>
		</div>
	{/if}
</div>
<div class="files" aria-hidden="true">{#each labels.files as f (f)}<span>{f}</span>{/each}</div>
</div>

<style>
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
	.board {
		position: relative;
		display: grid;
		grid-template-columns: repeat(8, 1fr);
		width: 100%;
		aspect-ratio: 1;
		border-radius: 8px;
		overflow: hidden;
		box-shadow:
			0 0 0 1px var(--hole),
			0 14px 36px var(--hole);
		touch-action: none;
		user-select: none;
		-webkit-touch-callout: none;
	}
	.board.dim {
		filter: saturate(0.6) brightness(0.55);
		transition: filter 0.4s ease;
	}
	.square {
		position: relative;
		display: grid;
		place-items: center;
		padding: 0;
		border: 0;
		background: var(--board-light);
		cursor: default;
		aspect-ratio: 1;
		font: inherit;
	}
	.square.dark {
		background: var(--board-dark);
	}
	.square.last {
		background: var(--board-last-light);
	}
	.square.dark.last {
		background: var(--board-last-dark);
	}
	.square.movable {
		cursor: pointer;
	}
	.square.legal {
		cursor: pointer;
	}
	/* Full-square fills, One Million Chessboards style, in road-works yellow. */
	.square::after {
		content: '';
		position: absolute;
		inset: 0;
		background: transparent;
		transition: background-color 0.15s ease;
		pointer-events: none;
	}
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
	.square.selected::after {
		background: var(--selected-fill);
	}
	.square.check::after {
		background: radial-gradient(circle, var(--hazard) 0%, transparent 72%);
	}
	.saved {
		position: absolute;
		inset: 0;
		background: radial-gradient(circle, var(--accent) 0%, transparent 70%);
		animation: pulse 0.9s ease-out both;
		pointer-events: none;
		z-index: 1;
	}
	@keyframes pulse {
		0% {
			opacity: 0;
			transform: scale(0.6);
		}
		35% {
			opacity: 0.9;
		}
		100% {
			opacity: 0;
			transform: scale(1.25);
		}
	}
	.blocked {
		position: absolute;
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: clamp(16px, 4vmin, 26px);
		color: var(--hazard);
		pointer-events: none;
		z-index: 1;
	}

	/* Layers over the squares: potholes, the target ring, then pieces. They
	   never take clicks; the squares underneath do. */
	.layer {
		position: absolute;
		inset: 0;
		pointer-events: none;
	}
	.slot {
		position: absolute;
		top: 0;
		left: 0;
		display: grid;
		place-items: center;
		width: 12.5%;
		height: 12.5%;
		transform: translate(calc(var(--col) * 100%), calc(var(--row) * 100%));
	}
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
	.piece-slot.dragging {
		z-index: 3;
	}
	.piece {
		display: grid;
		place-items: center;
		width: 100%;
		height: 100%;
		/* The whiplash pivots from the base, so the head swings. */
		transform-origin: 50% 85%;
		transition: transform 0.3s ease;
	}
	.lifted .piece,
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
	/* mpchess pieces fill the square (110% of the old 92%), with a crisp
	   white outline: their own shape offset 1.5 px four ways, no blur. */
	.piece img {
		width: 100%;
		height: 100%;
		filter: drop-shadow(1.5px 0 0 var(--piece-outline)) drop-shadow(-1.5px 0 0 var(--piece-outline))
			drop-shadow(0 1.5px 0 var(--piece-outline)) drop-shadow(0 -1.5px 0 var(--piece-outline));
	}
	.piece .mamdani {
		filter: none; /* its yellow border is its outline */
		box-sizing: border-box;
		width: 74%;
		height: auto;
		aspect-ratio: 1;
		object-fit: cover;
		border-radius: 22%;
		border: 3px solid var(--accent);
		background: var(--surface);
	}
	.hole {
		width: 76%;
		height: 76%;
		border-radius: 46% 54% 42% 58% / 55% 45% 55% 45%;
		background: var(--hole);
		box-shadow:
			0 0 0 3px var(--hazard),
			inset 0 4px 10px var(--bg);
	}
	/* Rounds left: one traffic cone per round, standing on the hole's front
	   edge; the last one blinks. */
	.hole {
		position: relative;
		display: grid;
		place-items: center;
	}
	.cones {
		position: absolute;
		left: 50%;
		bottom: -16%;
		display: flex;
		justify-content: center;
		gap: 2%;
		width: 116%;
		transform: translateX(-50%);
	}
	.mini-cone {
		width: 32%;
		aspect-ratio: 1;
		overflow: visible;
		filter: drop-shadow(0 1px 1px var(--hole));
	}
	.mini-cone path,
	.mini-cone rect {
		stroke-width: 2.5;
	}
	.mini-cone.last {
		animation: blink 1s ease-in-out infinite;
	}
	@keyframes blink {
		50% {
			opacity: 0.25;
		}
	}
	.target {
		width: 92%;
		height: 92%;
		border: 3px dashed var(--target-ring);
		border-radius: 6px;
	}

	.promote {
		position: absolute;
		inset: 32% 8%;
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 8px;
		padding: 8px;
		background: var(--surface);
		border: 2px solid var(--accent);
		border-radius: 10px;
		z-index: 4;
	}
	.promote button {
		min-height: 44px;
		border: 0;
		border-radius: 6px;
		background: var(--surface-2);
		cursor: pointer;
	}
	.promote img {
		width: 80%;
		height: 80%;
		filter: drop-shadow(1.5px 0 0 var(--piece-outline)) drop-shadow(-1.5px 0 0 var(--piece-outline))
			drop-shadow(0 1.5px 0 var(--piece-outline)) drop-shadow(0 -1.5px 0 var(--piece-outline));
	}
	.promote .cancel {
		grid-column: 1 / -1;
		color: var(--text);
		font: inherit;
	}
	/* The Mamdani's repair: a cone drops on the hole, the hole shrinks under
	   it, the cone lifts, sparks burst and the Mamdani gives a thumbs up.
	   About 1.3 s after --delay; it never holds up the turn. */
	.fix {
		container-type: size;
		z-index: 4;
	}
	.fix > * {
		grid-area: 1 / 1;
	}
	.patched {
		animation: patch 0.4s ease-in calc(var(--delay) + 250ms) both;
	}
	@keyframes patch {
		to {
			transform: scale(0);
			opacity: 0;
		}
	}
	.cone {
		width: 62%;
		height: 62%;
		overflow: visible;
		animation: cone 0.85s var(--delay) both;
	}
	.cone-body {
		fill: var(--hazard);
	}
	.cone-band {
		fill: var(--piece-light);
	}
	.cone-body,
	.cone-band,
	.cone-base {
		stroke: var(--piece-dark);
		stroke-width: 1.5;
		stroke-linejoin: round;
	}
	.cone-base {
		fill: var(--hazard);
	}
	@keyframes cone {
		0% {
			transform: translateY(-90%) scale(1.1);
			opacity: 0;
			animation-timing-function: cubic-bezier(0.5, 0, 1, 0.6);
		}
		22% {
			transform: translateY(0) scale(1, 0.86);
			opacity: 1;
		}
		30% {
			transform: translateY(-6%) scale(1);
		}
		76% {
			transform: translateY(0) scale(1);
			opacity: 1;
			animation-timing-function: ease-in;
		}
		100% {
			transform: translateY(-40%) scale(0.7);
			opacity: 0;
		}
	}
	.flash {
		width: 100%;
		height: 100%;
		border-radius: 50%;
		background: radial-gradient(circle, var(--accent) 0%, transparent 65%);
		opacity: 0;
		animation: flash 0.55s ease-out calc(var(--delay) + 620ms) forwards;
	}
	@keyframes flash {
		0% {
			transform: scale(0.3);
			opacity: 0.95;
		}
		100% {
			transform: scale(1.9);
			opacity: 0;
		}
	}
	.spark {
		width: 22%;
		aspect-ratio: 1;
		background: var(--accent);
		clip-path: polygon(50% 0, 62% 38%, 100% 50%, 62% 62%, 50% 100%, 38% 62%, 0 50%, 38% 38%);
		opacity: 0;
		animation: spark 0.5s ease-out calc(var(--delay) + 650ms) forwards;
	}
	@keyframes spark {
		0% {
			transform: rotate(var(--a)) translateY(0) scale(0.4);
			opacity: 1;
		}
		70% {
			opacity: 1;
		}
		100% {
			transform: rotate(var(--a)) translateY(-300%) scale(1);
			opacity: 0;
		}
	}
	.spark.far {
		width: 28%;
		animation-name: spark-far;
		animation-duration: 0.6s;
	}
	@keyframes spark-far {
		0% {
			transform: rotate(var(--a)) translateY(0) scale(0.4);
			opacity: 1;
		}
		70% {
			opacity: 1;
		}
		100% {
			transform: rotate(var(--a)) translateY(-360%) scale(1.1) rotate(45deg);
			opacity: 0;
		}
	}
	.thumb {
		font-size: 66cqh;
		line-height: 1;
		transform-origin: 50% 100%;
		filter: drop-shadow(0 2px 2px var(--hole));
		animation: thumb 1.1s calc(var(--delay) + 700ms) both;
	}
	/* On the top row the thumb would leave the board, so it drops in below. */
	.top .thumb {
		animation-name: thumb-below;
	}
	@keyframes thumb-below {
		0% {
			transform: translateY(20cqh) scale(0);
			opacity: 0;
			animation-timing-function: cubic-bezier(0.3, 1.6, 0.6, 1);
		}
		30% {
			transform: translateY(62cqh) scale(1.15) rotate(-12deg);
			opacity: 1;
		}
		45% {
			transform: translateY(62cqh) scale(1) rotate(10deg);
		}
		78% {
			transform: translateY(62cqh) scale(1) rotate(0);
			opacity: 1;
		}
		100% {
			transform: translateY(70cqh) scale(0.9);
			opacity: 0;
		}
	}
	@keyframes thumb {
		0% {
			transform: translateY(-20cqh) scale(0);
			opacity: 0;
			animation-timing-function: cubic-bezier(0.3, 1.6, 0.6, 1);
		}
		30% {
			transform: translateY(-62cqh) scale(1.15) rotate(-12deg);
			opacity: 1;
		}
		45% {
			transform: translateY(-62cqh) scale(1) rotate(10deg);
		}
		58% {
			transform: translateY(-64cqh) scale(1) rotate(-4deg);
		}
		78% {
			transform: translateY(-68cqh) scale(1) rotate(0);
			opacity: 1;
		}
		100% {
			transform: translateY(-82cqh) scale(0.9);
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.mini-cone.last {
			animation: none;
		}
		.cone,
		.spark,
		.flash,
		.patched {
			display: none;
		}
		.thumb,
		.top .thumb {
			--rise: -58cqh;
			animation: hold 1s both;
		}
		.top .thumb {
			--rise: 58cqh;
		}
		@keyframes hold {
			0%,
			90% {
				transform: translateY(var(--rise));
				opacity: 1;
			}
			100% {
				transform: translateY(var(--rise));
				opacity: 0;
			}
		}
	}
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
	@media (prefers-reduced-motion: reduce) {
		.board.dim,
		.piece-slot,
		.piece,
		.square::after {
			transition: none;
		}
		.saved {
			animation: none;
		}
	}
</style>
