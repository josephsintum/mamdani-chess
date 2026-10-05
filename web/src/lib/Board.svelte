<script lang="ts">
	import { blockedSquares, type Stage } from './board.ts';
	import { squareName, type MoveJSON } from './game.ts';

	let {
		stage,
		legal,
		lastMove = null,
		flipped = false,
		interactive = false,
		dim = false,
		onmove
	}: {
		stage: Stage;
		legal: MoveJSON[];
		lastMove?: { from: string; to: string } | null;
		flipped?: boolean;
		interactive?: boolean;
		dim?: boolean;
		onmove: (move: MoveJSON) => void;
	} = $props();

	// U+FE0E asks for the text form: iOS can draw ♟ as a colour emoji that
	// ignores CSS colour, which would make both sides' pawns look the same.
	const glyphs: Record<string, string> = {
		K: '♚︎',
		Q: '♛︎',
		R: '♜︎',
		B: '♝︎',
		N: '♞︎',
		P: '♟︎'
	};
	const promoOptions = [
		{ promo: 'q', kind: 'Q', name: 'Queen' },
		{ promo: 'r', kind: 'R', name: 'Rook' },
		{ promo: 'b', kind: 'B', name: 'Bishop' },
		{ promo: 'n', kind: 'N', name: 'Knight' }
	];

	let selected = $state<string | null>(null);
	let promoting = $state<{ from: string; to: string } | null>(null);
	let drag = $state<{ from: string; x: number; y: number; moved: boolean; pointer: number } | null>(null);
	let boardEl: HTMLDivElement | undefined = $state();

	// Drag follows the pointer across the whole board, so the listeners live
	// on the board element rather than on each square.
	function dragArea(node: HTMLDivElement) {
		boardEl = node;
		const cancel = () => (drag = null);
		node.addEventListener('pointermove', pointerMove);
		node.addEventListener('pointerup', pointerUp);
		node.addEventListener('pointercancel', cancel);
		return () => {
			node.removeEventListener('pointermove', pointerMove);
			node.removeEventListener('pointerup', pointerUp);
			node.removeEventListener('pointercancel', cancel);
			boardEl = undefined;
		};
	}

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
	let holes = $derived(new Set(stage.potholes.map((p) => p.sq)));
	let blocked = $derived(new Set(current ? blockedSquares(stage, current) : []));
	let dragPiece = $derived(drag?.moved ? pieceAt(drag.from) : '');

	function pieceAt(sq: string): string {
		if (sq === stage.mamdani) return 'M';
		const i = (Number(sq[1]) - 1) * 8 + (sq.charCodeAt(0) - 97);
		return stage.board[i];
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

	function promote(promo: string) {
		if (!pending) return;
		const { from, to } = pending;
		promoting = null;
		selected = null;
		onmove({ from, to, promo });
	}

	function cancelPromotion() {
		promoting = null;
	}

	let ghostStyle = $derived.by(() => {
		if (!drag?.moved || !boardEl) return '';
		const r = boardEl.getBoundingClientRect();
		const size = r.width / 8;
		return `width: ${size}px; height: ${size}px; left: ${drag.x - r.left - size / 2}px; top: ${drag.y - r.top - size / 2}px`;
	});
</script>

<div class="board" class:dim role="group" aria-label="Chessboard" {@attach dragArea}>
	{#each order as index, n (index)}
		{@const sq = squareName(index)}
		{@const piece = stage.mamdani === sq ? 'M' : stage.board[index]}
		{@const dark = (Math.floor(index / 8) + (index % 8)) % 2 === 0}
		{@const dragging = drag?.moved && drag.from === sq}
		<button
			class="square"
			class:dark
			class:last={lastMove?.from === sq || lastMove?.to === sq}
			class:selected={current === sq}
			class:movable={movable.has(sq)}
			aria-label={sq + (piece === 'M' ? ', the Mamdani' : '') + (holes.has(sq) ? ', pothole' : '')}
			onclick={() => tap(sq)}
			onpointerdown={(e) => pointerDown(e, sq)}
		>
			{#if holes.has(sq)}
				<span class="hole"></span>
			{/if}
			{#if stage.target === sq}
				<span class="target"></span>
			{/if}
			{#if piece === 'M'}
				<span class="mamdani" class:ghosted={dragging}>M</span>
			{:else if piece}
				<span class="piece" class:white={piece[0] === 'w'} class:ghosted={dragging}>{glyphs[piece[1]]}</span>
			{/if}
			{#if targets.has(sq)}
				<span class="dot" class:capture={!!piece}></span>
			{:else if blocked.has(sq)}
				<span class="blocked" title="Blocked by a pothole">×</span>
			{/if}
			{#if n % 8 === 0}<span class="rank-label">{sq[1]}</span>{/if}
			{#if n >= 56}<span class="file-label">{sq[0]}</span>{/if}
		</button>
	{/each}

	{#if dragPiece}
		<span class="drag" style={ghostStyle} aria-hidden="true">
			{#if dragPiece === 'M'}
				<span class="mamdani">M</span>
			{:else}
				<span class="piece" class:white={dragPiece[0] === 'w'}>{glyphs[dragPiece[1]]}</span>
			{/if}
		</span>
	{/if}

	{#if pending}
		<div class="promote" role="dialog" aria-label="Promote to">
			{#each promoOptions as option (option.promo)}
				<button aria-label={option.name} onclick={() => promote(option.promo)}>
					<span class="piece" class:white={pieceAt(pending.from)?.[0] === 'w'}>{glyphs[option.kind]}</span>
				</button>
			{/each}
			<button class="cancel" onclick={cancelPromotion}>Cancel</button>
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
	}
	.board.dim .square {
		opacity: 0.4;
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
		cursor: grab;
	}
	.square.selected {
		outline: 3px solid var(--accent);
		outline-offset: -3px;
	}
	.piece {
		position: relative;
		font-family: 'Apple Symbols', 'Segoe UI Symbol', 'Noto Sans Symbols 2', serif;
		font-size: clamp(24px, 8vmin, 54px);
		line-height: 1;
		color: var(--piece-dark);
	}
	.piece.white {
		color: var(--piece-light);
		-webkit-text-stroke: 1px var(--piece-dark);
	}
	.ghosted {
		opacity: 0.3;
	}
	.mamdani {
		position: relative;
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
	}
	.hole {
		position: absolute;
		inset: 12%;
		border-radius: 46% 54% 42% 58% / 55% 45% 55% 45%;
		background: var(--hole);
		box-shadow:
			0 0 0 3px var(--hazard),
			inset 0 4px 10px var(--bg);
	}
	.target {
		position: absolute;
		inset: 4%;
		border: 3px dashed var(--target-ring);
		border-radius: 6px;
		pointer-events: none;
	}
	.dot {
		position: absolute;
		width: 28%;
		aspect-ratio: 1;
		border-radius: 50%;
		background: var(--target-ring);
		opacity: 0.85;
		pointer-events: none;
	}
	.dot.capture {
		width: 86%;
		background: none;
		border: 4px solid var(--target-ring);
	}
	.blocked {
		position: absolute;
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: clamp(16px, 4vmin, 26px);
		color: var(--hazard);
		pointer-events: none;
	}
	.rank-label,
	.file-label {
		position: absolute;
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 11px;
		color: var(--board-dark);
		pointer-events: none;
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
	.drag {
		position: absolute;
		display: grid;
		place-items: center;
		pointer-events: none;
		z-index: 2;
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
		z-index: 3;
	}
	.promote button {
		min-height: 44px;
		border: 0;
		border-radius: 6px;
		background: var(--surface-2);
		cursor: pointer;
	}
	.promote .cancel {
		grid-column: 1 / -1;
		color: var(--text);
		font: inherit;
	}
</style>
