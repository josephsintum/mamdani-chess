<script lang="ts">
	import { pieceName, type Color } from './game.ts';

	let {
		color,
		you,
		lost,
		pill = '',
		pillTone = 'turn',
		compact = false
	}: {
		color: Color;
		you: boolean;
		lost: string[];
		/** "Your move", "In check", "Waiting…", or "" for none (see pillFor). */
		pill?: string;
		pillTone?: 'turn' | 'check' | 'muted';
		/** The phone's one-row bar: lost pieces after the name, nothing when none. */
		compact?: boolean;
	} = $props();
</script>

<div class="bar" class:compact>
	<span class="swatch {color}" aria-hidden="true"></span>
	<span class="who">
		<span class="name">{color === 'white' ? 'White' : 'Black'}{#if you}<span class="you">(you)</span>{/if}</span>
		{#if compact}
			{#if lost.length > 0}
				<span class="glyphs" role="img" aria-label="Lost to potholes: {lost.map((p) => pieceName(p)).join(', ')}">
					{#each lost as p, i (i)}<img src="/pieces/{p}.svg" alt="" />{/each}
				</span>
			{/if}
		{:else}
			<span class="lost">
				Lost to potholes:
				{#if lost.length === 0}none{:else}<span class="glyphs">{#each lost as p, i (i)}<img src="/pieces/{p}.svg" alt={pieceName(p)} />{/each}</span>{/if}
			</span>
		{/if}
	</span>
	{#if pill}<span class="pill {pillTone}">{pill}</span>{/if}
</div>

<style>
	.bar {
		display: flex;
		align-items: center;
		gap: 12px;
		min-height: 52px;
	}
	.bar.compact {
		gap: 10px;
		min-height: 44px;
		height: 44px;
		padding: 0 12px;
	}
	.swatch {
		width: 14px;
		height: 14px;
		border-radius: 3px;
		flex-shrink: 0;
		box-shadow: 0 0 0 1px var(--line);
	}
	.swatch.white {
		background: var(--piece-light);
	}
	.swatch.black {
		background: var(--piece-dark);
	}
	.who {
		display: flex;
		flex-direction: column;
		gap: 2px;
		flex-grow: 1;
		min-width: 0;
	}
	.compact .who {
		flex-direction: row;
		align-items: center;
		gap: 8px;
	}
	.name {
		font-weight: 600;
		color: var(--text);
		white-space: nowrap;
	}
	.you {
		margin-left: 6px;
		font-weight: 400;
		color: var(--text-muted);
	}
	.lost {
		font-size: 13px;
		color: var(--text-muted);
	}
	.glyphs {
		display: inline-flex;
		gap: 1px;
		vertical-align: middle;
		overflow: hidden;
	}
	.glyphs img {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}
	.pill {
		display: flex;
		align-items: center;
		flex-shrink: 0;
		padding: 4px 10px;
		border: 1px solid var(--accent-line);
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
	}
	.compact .pill {
		height: 28px;
		box-sizing: border-box;
		padding: 0 12px;
		font-size: 13px;
	}
	.pill.check {
		border-color: var(--hazard);
		color: var(--hazard-text);
	}
	.pill.muted {
		border-color: var(--line);
		color: var(--text-muted);
	}
</style>
