<script lang="ts">
	import { figurine } from './board.ts';
	import type { LogEntry } from './game.ts';
	import LogMark from './LogMark.svelte';

	// One move as the logs draw it: the piece's icon, the rest of its SAN,
	// then marks for what the dice did. Decorative: callers say it in words.
	// rolling: the dice are still playing out, so the marks wait.
	let { entry, rolling = false }: { entry: LogEntry; rolling?: boolean } = $props();

	let fig = $derived(figurine(entry));
</script>

<span class="san" aria-hidden="true"
	>{#if fig.icon === 'M'}<img class="mamdani" src="/mamdani/piece.webp" alt="" />{:else if fig.icon}<img src="/pieces/{fig.icon}.svg" alt="" />{/if}{fig.text}</span
>
{#if !rolling}
	{#if entry.repaired}<LogMark kind="repair" />{/if}
	{#each entry.fell ?? [] as piece, j (j)}<LogMark kind="fell" {piece} />{/each}
	{#if entry.opened}<LogMark kind="hole" />{/if}
{/if}

<style>
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
		/* The board's white outline, 1 px at this size, so black pieces show
		   on the dark page. */
		filter: drop-shadow(1px 0 0 var(--piece-outline)) drop-shadow(-1px 0 0 var(--piece-outline))
			drop-shadow(0 1px 0 var(--piece-outline)) drop-shadow(0 -1px 0 var(--piece-outline));
	}
	.san .mamdani {
		width: 16px;
		height: 16px;
		margin-right: 2px;
		box-sizing: border-box;
		border: 1px solid var(--accent);
		border-radius: 22%;
		object-fit: cover;
		filter: none;
	}
</style>
