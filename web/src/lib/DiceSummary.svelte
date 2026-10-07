<script lang="ts">
	import { diceSummary } from './board.ts';
	import { SAVE_MS, SCAN_MS } from './dice.ts';
	import Die from './Die.svelte';
	import { sideName, type View } from './game.ts';

	// The phone's dice card: the turn's dice as a row of chips that fill in as
	// they play out, and one outcome line. The steps are in the moves sheet.
	// wrap: chips that don't fit take a second line (pages whose layout can
	// grow); the game's card keeps one height, so there they clip.
	let { view, shown, wrap = false }: { view: View; shown: number; wrap?: boolean } = $props();

	let summary = $derived(diceSummary(view, shown));
	let side = $derived(summary.who ? sideName(summary.who) : 'Dice');
	let title = $derived(summary.who ? `${side}’s roll` : side);
</script>

<section class="card" aria-label={title}>
	<div class="chips" class:wrap>
		<span class="who" aria-hidden="true">{side}</span>
		{#each summary.chips as chip, i (i)}
			{#if i > 0}<span class="arrow" aria-hidden="true">→</span>{/if}
			{#if chip.dice?.length}
				<!-- The thrown dice, then what they decide once they land: the square, or saved / lost. -->
				{#each chip.dice as d, j (j)}<Die die={d} small />{/each}
				{#if chip.kind === 'square'}
					<span class="chip square late" style:--late="{SCAN_MS}ms">{chip.text}</span>
				{:else if chip.kind === 'good' || chip.kind === 'bad'}
					<span class="chip {chip.kind} late" style:--late="{SAVE_MS}ms">{chip.kind === 'good' ? 'saved' : 'lost'}</span>
				{/if}
			{:else}
				<span class="chip {chip.kind}">{chip.text}</span>
			{/if}
		{/each}
	</div>
	<p class="line {summary.tone}" aria-live="polite">
		{#key summary.line}<span class="text" class:late={summary.lineAt} style:--late="{summary.lineAt ?? 0}ms">{summary.line}</span>{/key}
	</p>
</section>

<style>
	.card {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin: 6px 8px 0;
		padding: 10px 12px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 12px;
	}
	.chips {
		display: flex;
		align-items: center;
		gap: 6px;
		min-height: 26px;
		overflow: hidden; /* a turn too long for a very narrow phone clips, never wraps */
		font-family: var(--font-mono);
		font-size: 13px;
		font-weight: 600;
		color: var(--text);
	}
	.who {
		margin-right: 4px;
		font-family: var(--font-display);
		font-size: 15px;
		font-weight: 700;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--text-muted);
	}
	.who,
	.arrow,
	.chip,
	.chips > :global(.die) {
		flex-shrink: 0;
		white-space: nowrap;
	}
	.arrow {
		color: var(--text-muted);
	}
	.chip {
		display: flex;
		align-items: center;
		justify-content: center;
		box-sizing: border-box;
		height: 26px;
		min-width: 26px;
		padding: 0 8px;
		border-radius: 6px;
		background: var(--line);
	}
	.chip.pending {
		background: none;
		border: 2px dashed var(--accent);
		color: var(--accent);
	}
	.chip.good {
		background: var(--accent);
		color: var(--accent-text);
	}
	.chip.bad {
		background: var(--hazard);
		color: var(--accent-text);
	}
	.line {
		margin: 0;
		font-size: 14px;
		line-height: 19px;
		white-space: nowrap;
		color: var(--text-body);
	}
	/* The ellipsis is the text's own, so it fades in with it. */
	.text {
		display: block;
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.chips.wrap {
		flex-wrap: wrap;
		row-gap: 6px;
	}
	/* Phones up to 420 px: tighter, so a usual turn fits on one line. */
	@media (max-width: 420px) {
		.chips {
			gap: 4px;
		}
		.chip {
			padding: 0 6px;
		}
		.who {
			margin-right: 2px;
		}
	}
	.line.muted {
		color: var(--text-muted);
	}
	.line.good {
		color: var(--accent);
	}
	.line.hazard {
		color: var(--hazard-text);
	}
	/* What the dice decide shows once they land. */
	.late {
		animation: chip-in 0.2s ease-out var(--late) both;
	}
	@keyframes chip-in {
		from {
			opacity: 0;
			transform: scale(0.6);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.late {
			animation: none;
		}
	}
</style>
