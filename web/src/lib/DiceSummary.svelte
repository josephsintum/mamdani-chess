<script lang="ts">
	import { diceSummary } from './board.ts';
	import Die from './Die.svelte';
	import type { View } from './game.ts';

	// The phone's dice card: the turn's dice as a row of chips that fill in as
	// they play out, and one outcome line. The steps are in the moves sheet.
	let { view, shown }: { view: View; shown: number } = $props();

	let summary = $derived(diceSummary(view, shown));
	let side = $derived(summary.who ? (summary.who === 'white' ? 'White' : 'Black') : 'Dice');
	let title = $derived(summary.who ? `${side}’s roll` : 'Dice');
</script>

<section class="card" aria-label={title}>
	<div class="chips">
		<span class="who" aria-hidden="true">{side}</span>
		{#each summary.chips as chip, i (i)}
			{#if i > 0}<span class="arrow" aria-hidden="true">→</span>{/if}
			{#if chip.kind === 'die'}
				<Die value={Number(chip.text)} small />
			{:else}
				<span class="chip {chip.kind}">{chip.text}</span>
			{/if}
		{/each}
	</div>
	<p class="line {summary.tone}" aria-live="polite">{summary.line}</p>
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
		white-space: nowrap;
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
		overflow: hidden;
		font-size: 14px;
		line-height: 19px;
		white-space: nowrap;
		text-overflow: ellipsis;
		color: var(--text-body);
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
</style>
