<script lang="ts">
	import { pieceSrc } from './pieces.ts';

	// The promotion choices, over the board. color: the pawn's, "w" or "b".
	let { color, onpick, oncancel }: { color: string; onpick: (promo: string) => void; oncancel: () => void } = $props();

	const OPTIONS = [
		{ promo: 'q', kind: 'Q', name: 'Queen' },
		{ promo: 'r', kind: 'R', name: 'Rook' },
		{ promo: 'b', kind: 'B', name: 'Bishop' },
		{ promo: 'n', kind: 'N', name: 'Knight' }
	];

	function key(e: KeyboardEvent) {
		if (e.key === 'Escape') oncancel();
	}

	// Focus the first choice when the picker opens, so Escape and the arrow
	// keys work at once, without tabbing in.
	function focusFirst(node: HTMLElement) {
		node.querySelector('button')?.focus();
	}
</script>

<div class="promote" role="dialog" aria-label="Promote to" tabindex="-1" onkeydown={key} {@attach focusFirst}>
	{#each OPTIONS as option (option.promo)}
		<button aria-label={option.name} onclick={() => onpick(option.promo)}>
			<img src={pieceSrc(color + option.kind)} alt="" draggable="false" />
		</button>
	{/each}
	<button class="cancel" onclick={oncancel}>Cancel</button>
</div>

<style>
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
		z-index: 7; /* above the board's shade and celebrate layer */
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
		filter: var(--piece-outline-2);
	}
	.promote .cancel {
		grid-column: 1 / -1;
		color: var(--text);
		font: inherit;
	}
</style>
