<script lang="ts">
	import type { Color } from './game.ts';

	let { color, you, lost, toMove }: { color: Color; you: boolean; lost: string[]; toMove: boolean } = $props();

	const glyphs: Record<string, string> = { Q: '♛︎', R: '♜︎', B: '♝︎', N: '♞︎', P: '♟︎' };
</script>

<div class="bar" class:to-move={toMove}>
	<span class="swatch {color}" aria-hidden="true"></span>
	<span class="who">
		<span class="name">{color === 'white' ? 'White' : 'Black'}{#if you}<span class="you">(you)</span>{/if}</span>
		<span class="lost">
			Lost to potholes:
			{#if lost.length === 0}none{:else}<span class="glyphs" class:white={color === 'white'}>{lost.map((p) => glyphs[p[1]]).join('')}</span>{/if}
		</span>
	</span>
	{#if toMove}<span class="turn">To move</span>{/if}
</div>

<style>
	.bar {
		display: flex;
		align-items: center;
		gap: 12px;
		min-height: 52px;
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
	}
	.name {
		font-weight: 600;
		color: var(--text);
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
		font-family: 'Apple Symbols', 'Segoe UI Symbol', 'Noto Sans Symbols 2', serif;
		font-size: 16px;
		letter-spacing: 2px;
		color: var(--piece-dark);
		-webkit-text-stroke: 1px var(--text-muted);
	}
	.glyphs.white {
		color: var(--piece-light);
		-webkit-text-stroke: 0;
	}
	.turn {
		padding: 4px 10px;
		border: 1px solid var(--accent-line);
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
	}
</style>
