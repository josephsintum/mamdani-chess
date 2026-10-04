<script lang="ts">
	import type { LogEntry } from './game.ts';

	// rolling: the newest turn's dice are still playing out, so its result
	// stays hidden until the dice tray has shown it.
	let { log, rolling = false }: { log: LogEntry[]; rolling?: boolean } = $props();

	// Keep the newest move in view. Reading log.length makes the attachment
	// run again whenever a move is added.
	function follow(node: HTMLOListElement) {
		if (log.length) node.scrollTop = node.scrollHeight;
	}
</script>

<section class="log" aria-labelledby="log-heading">
	<h2 id="log-heading">Moves and rolls</h2>
	{#if log.length === 0}
		<p class="muted">No moves yet.</p>
	{:else}
		<ol {@attach follow}>
			{#each log as entry, i (i)}
				<li class:current={i === log.length - 1}>
					<span class="n">{i % 2 === 0 ? `${i / 2 + 1}.` : ''}</span>
					<span class="move">
						<span class="san">{entry.san}</span>
						{#if rolling && i === log.length - 1}
							<span class="dice">rolling…</span>
						{:else}
							<span class="dice" class:hazard={entry.dice.includes('→')}>{entry.dice || 'no roll'}</span>
						{/if}
					</span>
				</li>
			{/each}
		</ol>
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
		margin: 0;
		padding: 0;
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 4px;
		max-height: 520px;
		overflow-y: auto;
	}
	li {
		display: grid;
		grid-template-columns: 36px minmax(0, 1fr);
		gap: 8px;
		padding: 8px 10px;
		border-radius: 8px;
	}
	li.current {
		background: var(--accent-wash);
		box-shadow: inset 0 0 0 1px var(--accent-line);
	}
	.n {
		font-family: var(--font-mono);
		font-size: 13px;
		color: var(--text-muted);
	}
	.move {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.san {
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 15px;
		color: var(--text);
	}
	.dice {
		font-size: 13px;
		color: var(--text-muted);
	}
	.dice.hazard {
		color: var(--hazard-text);
	}
	li.current .dice {
		color: var(--accent);
	}
</style>
