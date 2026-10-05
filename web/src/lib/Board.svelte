<script lang="ts">
	import { blockedSquares, squareIndex, type Stage } from './board.ts';
	import { squareName, type MoveJSON } from './game.ts';
	import { reducedMotion } from './motion.ts';
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
		onmove: (move: MoveJSON) => void;
	} = $props();

	const promoOptions = [
		{ promo: 'q', kind: 'Q', name: 'Queen' },
		{ promo: 'r', kind: 'R', name: 'Rook' },
		{ promo: 'b', kind: 'B', name: 'Bishop' },
		{ promo: 'n', kind: 'N', name: 'Knight' }
	];
	const names: Record<string, string> = { K: 'king', Q: 'queen', R: 'rook', B: 'bishop', N: 'knight', P: 'pawn' };

	let selected = $state<string | null>(null);
	let promoting = $state<{ from: string; to: string } | null>(null);
	let hovered = $state<string | null>(null);
	let drag = $state<{ from: string; x: number; y: number; moved: boolean; pointer: number } | null>(null);
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
			return { ...p, dur: from && !reducedMotion() ? moveDuration(from, p.sq) : 0 };
		});
	});

	/** Column and row on screen for a square, 0..7 from the top left. */
	function cell(sq: string): { col: number; row: number } {
		const file = sq.charCodeAt(0) - 97;
		const rank = Number(sq[1]) - 1;
		return flipped ? { col: 7 - file, row: rank } : { col: file, row: 7 - rank };
	}

	function place(sq: string): string {
		const { col, row } = cell(sq);
		return `--col: ${col}; --row: ${row}`;
	}

	function pieceStyle(p: PieceRef & { dur: number }): string {
		if (drag?.moved && drag.from === p.sq && boardEl) {
			const r = boardEl.getBoundingClientRect();
			const size = r.width / 8;
			return `transform: translate(${drag.x - r.left - size / 2}px, ${drag.y - r.top - size / 2}px); transition: none; z-index: 3`;
		}
		return `${place(p.sq)}; --dur: ${p.dur}ms`;
	}

	function pieceAt(sq: string): string {
		return sq === stage.mamdani ? 'M' : stage.board[squareIndex(sq)];
	}

	function label(sq: string): string {
		const piece = pieceAt(sq);
		let text = sq;
		if (piece === 'M') text += ', the Mamdani';
		else if (piece) text += `, ${piece[0] === 'w' ? 'white' : 'black'} ${names[piece[1]]}`;
		if (stage.potholes.some((h) => h.sq === sq)) text += ', pothole';
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
		if (pending || !movable.has(sq) || e.button !== 0) return;
		drag = { from: sq, x: e.clientX, y: e.clientY, moved: false, pointer: e.pointerId };
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
		const moved = drag.moved || Math.hypot(e.clientX - drag.x, e.clientY - drag.y) > 6;
		if (moved && !drag.moved) {
			boardEl?.setPointerCapture(e.pointerId);
			selected = drag.from;
		}
		drag = { ...drag, x: e.clientX, y: e.clientY, moved };
	}

	function pointerUp(e: PointerEvent) {
		if (!drag || e.pointerId !== drag.pointer) return;
		const { from, moved } = drag;
		drag = null;
		if (!moved) return; // a tap: the square's click handler deals with it
		const to = squareFromPoint(e.clientX, e.clientY);
		if (!to || to === from || !moveTo(from, to)) selected = from;
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

	// Transitions. Each one collapses to nothing under reduced motion.
	const ms = (n: number) => (reducedMotion() ? 0 : n);

	/** A piece leaving the board: a fall shrinks into the hole, a capture fades. */
	function leave(_node: Element, { fell }: { fell: boolean }) {
		return fell
			? { duration: ms(450), css: (t: number) => `opacity: ${t}; transform: scale(${0.25 + 0.75 * t}) rotate(${(1 - t) * 25}deg)` }
			: { duration: ms(300), css: (t: number) => `opacity: ${1 - (1 - t) ** 3}` };
	}

	/** A pothole cracks open. */
	function crack(_node: Element) {
		return { duration: ms(420), css: (t: number) => `transform: scale(${t < 0.7 ? t / 0.7 : 1 + 0.12 * Math.sin(((t - 0.7) / 0.3) * Math.PI)}) rotate(${(1 - t) * -20}deg); opacity: ${Math.min(1, t * 2)}` };
	}

	/** A pothole closes (its roller moved again, or the Mamdani repaired it). */
	function closeUp(_node: Element) {
		return { duration: ms(350), css: (t: number) => `transform: scale(${t}); opacity: ${t}` };
	}

	/** The target ring drops onto its square. */
	function drop(_node: Element) {
		return { duration: ms(260), css: (t: number) => `transform: scale(${1.5 - 0.5 * t}); opacity: ${t}` };
	}
</script>

<div class="board" class:dim role="group" aria-label="Chessboard" {@attach dragArea}>
	{#each order as index, n (index)}
		{@const sq = squareName(index)}
		{@const dark = (Math.floor(index / 8) + (index % 8)) % 2 === 0}
		<button
			class="square"
			class:dark
			class:last={lastMove?.from === sq || lastMove?.to === sq}
			class:legal={targets.has(sq)}
			class:selected={current === sq}
			class:movable={movable.has(sq)}
			class:check={check === sq}
			aria-label={label(sq)}
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
			{#if n % 8 === 0}<span class="rank-label">{sq[1]}</span>{/if}
			{#if n >= 56}<span class="file-label">{sq[0]}</span>{/if}
		</button>
	{/each}

	<div class="layer" aria-hidden="true">
		{#each stage.potholes as h (h.sq)}
			<span class="slot" style={place(h.sq)}><span class="hole" in:crack out:closeUp></span></span>
		{/each}
		{#if stage.target}
			{#key stage.target}
				<span class="slot" style={place(stage.target)}><span class="target" in:drop></span></span>
			{/key}
		{/if}
	</div>

	<div class="layer" aria-hidden="true">
		{#each pieces as p (p.id)}
			<span
				class="slot piece-slot"
				class:lifted={movable.has(p.sq) && (hovered === p.sq || current === p.sq) && !drag?.moved}
				class:dragging={drag?.moved && drag.from === p.sq}
				style={pieceStyle(p)}
			>
				<span class="piece" out:leave={{ fell: stage.target === p.sq }}>
					{#if p.code === 'M'}
						<span class="mamdani">M</span>
					{:else}
						<img src="/pieces/{p.code}.svg" alt="" draggable="false" />
					{/if}
				</span>
			</span>
		{/each}
	</div>

	{#if pending}
		<div class="promote" role="dialog" aria-label="Promote to" tabindex="-1" onkeydown={promoKey}>
			{#each promoOptions as option (option.promo)}
				<button aria-label={option.name} onclick={() => promote(option.promo)}>
					<img src="/pieces/{pieceAt(pending.from)[0]}{option.kind}.svg" alt="" draggable="false" />
				</button>
			{/each}
			<button class="cancel" onclick={() => (promoting = null)}>Cancel</button>
		</div>
	{/if}
</div>

<style>
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
	.square.legal::after {
		background: var(--legal-fill);
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
	.rank-label,
	.file-label {
		position: absolute;
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 11px;
		color: var(--board-dark);
		pointer-events: none;
		z-index: 1;
	}
	.square.dark .rank-label,
	.square.dark .file-label {
		color: var(--board-light);
	}
	.rank-label {
		top: 3px;
		left: 4px;
	}
	.file-label {
		bottom: 2px;
		right: 4px;
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
	/* The glide: easeInOutQuad, duration by distance (pieces.ts moveDuration). */
	.piece-slot {
		transition: transform var(--dur, 0ms) cubic-bezier(0.455, 0.03, 0.515, 0.955);
		z-index: 2;
	}
	.piece-slot.dragging {
		z-index: 3;
	}
	.piece {
		display: grid;
		place-items: center;
		width: 100%;
		height: 100%;
		transition: transform 0.3s ease;
	}
	.lifted .piece,
	.dragging .piece {
		transform: scale(1.12);
	}
	.piece img {
		width: 92%;
		height: 92%;
		filter: drop-shadow(0 2px 2px var(--hole));
	}
	.mamdani {
		display: grid;
		place-items: center;
		width: 72%;
		aspect-ratio: 1;
		border-radius: 50%;
		background: var(--surface);
		border: 3px solid var(--accent);
		color: var(--accent);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: clamp(14px, 4vmin, 28px);
		box-shadow: 0 2px 4px var(--hole);
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
	}
	.promote .cancel {
		grid-column: 1 / -1;
		color: var(--text);
		font: inherit;
	}
	@media (prefers-reduced-motion: reduce) {
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
