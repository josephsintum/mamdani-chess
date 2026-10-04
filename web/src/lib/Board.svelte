<script lang="ts">
	import { squareName, type MoveJSON, type View } from './game.ts';

	let { view, onmove }: { view: View; onmove: (move: MoveJSON) => void } = $props();

	// U+FE0E asks for the text form: iOS can draw ♟ as a colour emoji that
	// ignores CSS colour, which would make both sides' pawns look the same.
	const glyphs: Record<string, string> = {
		K: '♚\uFE0E',
		Q: '♛\uFE0E',
		R: '♜\uFE0E',
		B: '♝\uFE0E',
		N: '♞\uFE0E',
		P: '♟\uFE0E'
	};
	const promoOptions = [
		{ promo: 'q', kind: 'Q' },
		{ promo: 'r', kind: 'R' },
		{ promo: 'b', kind: 'B' },
		{ promo: 'n', kind: 'N' }
	];

	// Selections remember the position they were made on (seq), so a new
	// position from the server clears any half-made move.
	let picked = $state<{ seq: number; sq: string } | null>(null);
	let pending = $state<{ seq: number; from: string; to: string } | null>(null);
	let selected = $derived(picked?.seq === view.seq ? picked.sq : null);
	let promoting = $derived(pending?.seq === view.seq ? pending : null);

	// White sees rank 8 at the top; Black sees the board flipped.
	let flipped = $derived(view.you === 'black');
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
	let targets = $derived(new Set(view.legal.filter((m) => m.from === selected).map((m) => m.to)));
	let movable = $derived(new Set(view.legal.map((m) => m.from)));
	let holes = $derived(new Set(view.potholes.map((p) => p.sq)));
	let lastMove = $derived(view.last.find((e) => e.kind === 'moved'));

	function tap(sq: string) {
		if (promoting) return;
		if (selected && targets.has(sq)) {
			const moves = view.legal.filter((m) => m.from === selected && m.to === sq);
			if (moves.length > 1) {
				pending = { seq: view.seq, from: selected, to: sq };
				return;
			}
			onmove(moves[0]);
			picked = null;
			return;
		}
		picked = movable.has(sq) && selected !== sq ? { seq: view.seq, sq } : null;
	}

	function promote(promo: string) {
		if (!promoting) return;
		onmove({ from: promoting.from, to: promoting.to, promo });
		pending = null;
		picked = null;
	}
</script>

<div class="board" class:flipped>
	{#each order as index (index)}
		{@const sq = squareName(index)}
		{@const piece = view.board[index]}
		{@const dark = (Math.floor(index / 8) + (index % 8)) % 2 === 0}
		<button
			class="square"
			class:dark
			class:last={lastMove?.from === sq || lastMove?.to === sq}
			class:selected={selected === sq}
			class:movable={movable.has(sq)}
			aria-label={sq}
			onclick={() => tap(sq)}
		>
			{#if holes.has(sq)}
				<span class="hole" aria-label="pothole"></span>
			{/if}
			{#if view.mamdani === sq}
				<span class="mamdani" aria-label="the Mamdani">M</span>
			{:else if piece}
				<span class="piece" class:white={piece[0] === 'w'}>{glyphs[piece[1]]}</span>
			{/if}
			{#if targets.has(sq)}
				<span class="dot" class:capture={!!piece}></span>
			{/if}
		</button>
	{/each}

	{#if promoting}
		<div class="promote" role="dialog" aria-label="Promote to">
			{#each promoOptions as option (option.promo)}
				<button onclick={() => promote(option.promo)}>
					<span class="piece" class:white={view.you === 'white'}>{glyphs[option.kind]}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.board {
		position: relative;
		display: grid;
		grid-template-columns: repeat(8, 1fr);
		width: min(100%, 560px);
		aspect-ratio: 1;
		border: 2px solid var(--line);
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
	.square.selected {
		outline: 3px solid var(--accent);
		outline-offset: -3px;
	}
	.piece {
		position: relative;
		font-size: clamp(24px, 8vmin, 52px);
		line-height: 1;
		color: var(--piece-dark);
		user-select: none;
	}
	.piece.white {
		color: var(--piece-light);
		-webkit-text-stroke: 1px var(--piece-dark);
	}
	.mamdani {
		position: relative;
		display: grid;
		place-items: center;
		width: 70%;
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
		border-radius: 46% 54% 50% 48%;
		background: var(--hole);
		box-shadow:
			0 0 0 3px var(--hazard),
			inset 0 4px 10px var(--bg);
	}
	.dot {
		position: absolute;
		width: 28%;
		aspect-ratio: 1;
		border-radius: 50%;
		background: var(--target-ring);
		opacity: 0.8;
		pointer-events: none;
	}
	.dot.capture {
		width: 86%;
		background: none;
		border: 4px solid var(--target-ring);
	}
	.promote {
		position: absolute;
		inset: 35% 10%;
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 8px;
		padding: 8px;
		background: var(--surface);
		border: 2px solid var(--accent);
	}
	.promote button {
		min-height: 44px;
		border: 0;
		background: var(--surface-2);
		cursor: pointer;
	}
</style>
