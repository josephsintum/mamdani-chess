<script lang="ts">
	import { figurine } from './board.ts';
	import type { LogEntry } from './game.ts';
	import LogMark from './LogMark.svelte';
	import { pieceSrc } from './pieces.ts';

	// One move as the logs draw it: the piece's icon, the rest of its SAN,
	// then marks for what the dice did. Decorative: `label` says it in words.
	// rolling: the dice are still playing out, so the marks wait. current:
	// the latest move, highlighted.
	let {
		entry,
		rolling = false,
		current = false,
		title,
		label
	}: { entry: LogEntry; rolling?: boolean; current?: boolean; title?: string; label?: string } = $props();

	let fig = $derived(figurine(entry));
</script>

<span class="ply" class:current {title}>
	{#if label}<span class="sr-only">{label}</span>{/if}
	<span class="san" aria-hidden="true"
		>{#if fig.icon}<img class:mamdani={fig.icon === 'M'} src={pieceSrc(fig.icon)} alt="" />{/if}{fig.text}</span
	>
	{#if !rolling}
		{#if entry.repaired}<LogMark kind="repair" />{/if}
		{#each entry.fell ?? [] as piece, j (j)}<LogMark kind="fell" {piece} />{/each}
		{#if entry.opened}<LogMark kind="hole" />{/if}
	{/if}
</span>

<style>
	.ply {
		display: flex;
		align-items: center;
		gap: 5px;
		justify-self: start;
		min-width: 0;
		padding: 3px 6px;
		border-radius: 6px;
	}
	.ply.current {
		background: var(--accent-wash);
		box-shadow: inset 0 0 0 1px var(--accent-line);
	}
	.san {
		display: inline-flex;
		align-items: center;
		gap: 2px;
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 14px;
		color: var(--text);
		white-space: nowrap;
	}
	.san img {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
		/* The board's white outline, so black pieces show on the dark page. */
		filter: var(--piece-outline-1);
	}
	.san .mamdani {
		width: 16px;
		height: 16px;
		margin-right: 2px;
		border: 1px solid var(--accent);
		border-radius: 22%;
		object-fit: cover;
		filter: none;
	}
</style>
