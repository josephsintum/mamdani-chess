<script lang="ts">
	import { isDark } from './feel.ts';
	import { squareName, type MoveJSON } from './game.ts';
	import type { Pothole } from './wire.gen.ts';

	// A board to look at, not to play on: the live games list and the home
	// page's hero. No buttons, so a card that holds one stays one link.
	let {
		board,
		potholes = [],
		mamdani = '',
		last = null,
		label
	}: {
		board: string[]; // 64 entries, index 0 = a1
		potholes?: Pothole[];
		mamdani?: string;
		last?: MoveJSON | null;
		/** What a screen reader hears instead of 64 squares. */
		label: string;
	} = $props();

	// a8 first, row by row, as the board is drawn.
	const order = Array.from({ length: 64 }, (_, n) => (7 - Math.floor(n / 8)) * 8 + (n % 8));
	let holes = $derived(new Set(potholes.map((h) => h.sq)));
</script>

<div class="mini" role="img" aria-label={label}>
	{#each order as index (index)}
		{@const sq = squareName(index)}
		<span
			class="square"
			class:dark={isDark(index)}
			class:last={last?.from === sq || last?.to === sq}
		>
			{#if holes.has(sq)}<span class="hole"></span>{/if}
			{#if mamdani === sq}
				<img class="mamdani" src="/mamdani/piece.webp" alt="" draggable="false" />
			{:else if board[index]}
				<img src="/pieces/{board[index]}.svg" alt="" draggable="false" />
			{/if}
		</span>
	{/each}
</div>

<style>
	.mini {
		display: grid;
		/* Fixed tracks: rows sized by their pieces would squash empty ranks. */
		grid-template-columns: repeat(8, minmax(0, 1fr));
		grid-template-rows: repeat(8, minmax(0, 1fr));
		width: 100%;
		aspect-ratio: 1;
		border-radius: 6px;
		overflow: hidden;
		box-shadow: 0 0 0 1px var(--hole);
		/* Sizes inside scale with the board: cqw is 1% of its width. */
		container-type: inline-size;
	}
	.square {
		position: relative;
		display: grid;
		place-items: center;
		background: var(--board-light);
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
	.square > * {
		grid-area: 1 / 1;
	}
	/* The board's pieces at their size, with a 1 px white outline. */
	img {
		width: 100%;
		height: 100%;
		filter: drop-shadow(1px 0 0 var(--piece-outline)) drop-shadow(-1px 0 0 var(--piece-outline))
			drop-shadow(0 1px 0 var(--piece-outline)) drop-shadow(0 -1px 0 var(--piece-outline));
	}
	.hole {
		width: 76%;
		height: 76%;
		border-radius: 46% 54% 42% 58% / 55% 45% 55% 45%;
		background: var(--hole);
		box-shadow: 0 0 0 max(1.5px, 0.5cqw) var(--hazard);
	}
	.mamdani {
		filter: none; /* its yellow border is its outline */
		box-sizing: border-box;
		width: 74%;
		height: auto;
		aspect-ratio: 1;
		object-fit: cover;
		border-radius: 22%;
		border: max(1.5px, 0.5cqw) solid var(--accent);
		background: var(--surface);
	}
</style>
