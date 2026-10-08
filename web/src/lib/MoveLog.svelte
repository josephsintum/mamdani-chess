<script lang="ts">
	import { moveRows, type Ply } from './board.ts';
	import type { LogEntry } from './game.ts';
	import LogMark from './LogMark.svelte';
	import MoveText from './MoveText.svelte';

	// rolling: the newest turn's dice are still playing out, so its result
	// stays hidden until the dice tray has shown it.
	let { log, rolling = false }: { log: LogEntry[]; rolling?: boolean } = $props();

	let rows = $derived(moveRows(log));

	// Keep the newest move in view. Reading log.length makes the attachment
	// run again whenever a move is added.
	function follow(node: HTMLOListElement) {
		if (log.length) node.scrollTop = node.scrollHeight;
	}
</script>

{#snippet ply(p: Ply | undefined)}
	{#if p}
		{@const hidden = rolling && p.i === log.length - 1}
		{@const dice = hidden ? 'rolling…' : p.dice || 'no roll'}
		<span class="ply" class:current={p.i === log.length - 1} title={dice}>
			<span class="sr-only">{p.san}, {dice}</span>
			<MoveText entry={p} rolling={hidden} />
		</span>
	{:else}
		<span></span>
	{/if}
{/snippet}

<section class="log" aria-labelledby="log-heading">
	<h2 id="log-heading">Moves and rolls</h2>
	{#if log.length === 0}
		<p class="muted">No moves yet.</p>
	{:else}
		<ol {@attach follow}>
			{#each rows as row, n (n)}
				<li>
					<span class="n">{n + 1}.</span>
					{@render ply(row.white)}
					{@render ply(row.black)}
				</li>
			{/each}
		</ol>
		<p class="legend" aria-hidden="true">
			<span><LogMark kind="hole" /> pothole</span>
			<span><LogMark kind="fell" piece="wP" /> fell</span>
			<span><LogMark kind="repair" /> repaired</span>
		</p>
	{/if}
</section>

<style>
	.log {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 18px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		min-height: 0;
	}
	h2 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 700;
		font-size: 22px;
		text-transform: uppercase;
		color: var(--text);
	}
	.muted {
		margin: 0;
		color: var(--text-muted);
		font-size: 14px;
	}
	ol {
		margin: 0 -8px;
		padding: 0;
		list-style: none;
		/* With the legend, as tall as the old list's 520 px. */
		max-height: 488px;
		overflow-y: auto;
	}
	li {
		display: grid;
		grid-template-columns: 30px minmax(0, 1fr) minmax(0, 1fr);
		align-items: center;
		min-height: 32px;
		padding: 0 4px 0 8px;
	}
	li:nth-child(even) {
		background: var(--surface-2);
	}
	.n {
		font-family: var(--font-mono);
		font-size: 13px;
		color: var(--text-muted);
	}
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
	.legend {
		display: flex;
		flex-wrap: wrap;
		gap: 4px 14px;
		margin: 0;
		font-size: 12px;
		color: var(--text-muted);
	}
	.legend span {
		display: inline-flex;
		align-items: center;
		gap: 5px;
	}
</style>
