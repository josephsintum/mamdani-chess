<script lang="ts">
	import { moveRows } from './board.ts';
	import type { LogEntry } from './game.ts';
	import MoveText from './MoveText.svelte';

	// The phone's latest moves, drawn as the move list draws them, in the
	// spare height under the header on tall screens. The page's `recent`
	// container sizes it: rows fill it from the bottom, the oldest fade off
	// the top, and it hides when not even one row fits. Tapping it opens the
	// full list. rolling: the newest turn's dice are still playing out.
	let {
		log,
		rolling = false,
		onclick
	}: { log: LogEntry[]; rolling?: boolean; onclick: (e: MouseEvent & { currentTarget: HTMLButtonElement }) => void } = $props();

	const ROWS = 8; // more than the tallest phone has room for
	let rows = $derived(moveRows(log));
	let first = $derived(Math.max(0, rows.length - ROWS));
</script>

{#if log.length > 0}
	<button type="button" class="recent" {onclick} aria-label="Latest move {log[log.length - 1].san}; open the move list">
		{#each rows.slice(first) as row, k (first + k)}
			<span class="row">
				<span class="n">{first + k + 1}.</span>
				{#each [row.white, row.black] as p, j (j)}
					<span class="ply" class:current={p?.i === log.length - 1}>
						{#if p}<MoveText entry={p} rolling={rolling && p.i === log.length - 1} />{/if}
					</span>
				{/each}
			</span>
		{/each}
	</button>
{/if}

<style>
	.recent {
		display: flex;
		flex-direction: column;
		justify-content: flex-end;
		width: 100%;
		height: 100%;
		padding: 0 8px 4px;
		overflow: hidden;
		border: 0;
		background: none;
		color: inherit;
		font: inherit;
		text-align: left;
		cursor: pointer;
		mask-image: linear-gradient(transparent, #000 32px);
	}
	/* Not even one row fits: nothing at all, not a sliver. */
	@container recent (max-height: 35px) {
		.recent {
			display: none;
		}
	}
	.row {
		display: grid;
		flex-shrink: 0;
		grid-template-columns: 34px minmax(0, 1fr) minmax(0, 1fr);
		align-items: center;
		min-height: 32px;
		padding: 0 4px 0 10px;
		border-radius: 6px;
	}
	.row:nth-child(odd) {
		background: var(--surface);
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
</style>
