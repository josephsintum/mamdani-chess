<script lang="ts">
	import { formatClock } from './clock.ts';
	import type { Color } from './game.ts';
	import { pieceName } from './pieces.ts';

	let {
		color,
		you,
		lost,
		toMove,
		clockMs,
		ticking = false,
		pausedForDice = false,
		offline = false
	}: {
		color: Color;
		you: boolean;
		lost: string[];
		toMove: boolean;
		/** Time left, in ms; no clock is shown without it (the sandbox). */
		clockMs?: number;
		/** The clock is counting down right now. */
		ticking?: boolean;
		/** It's this side's turn, but the clock waits for the dice. */
		pausedForDice?: boolean;
		/** The player has no tab open on the game. */
		offline?: boolean;
	} = $props();
</script>

<div class="bar" class:to-move={toMove}>
	<span class="swatch {color}" aria-hidden="true"></span>
	<span class="who">
		<span class="name">
			{color === 'white' ? 'White' : 'Black'}{#if you}<span class="you">(you)</span>{/if}
			{#if offline}<span class="offline">Disconnected</span>{/if}
		</span>
		<span class="lost">
			Lost to potholes:
			{#if lost.length === 0}none{:else}<span class="glyphs">{#each lost as p, i (i)}<img src="/pieces/{p}.svg" alt={pieceName(p)} />{/each}</span>{/if}
		</span>
	</span>
	{#if pausedForDice}<span class="note">Paused for dice</span>{/if}
	{#if clockMs === undefined}
		{#if toMove}<span class="turn">To move</span>{/if}
	{:else}
		<span
			class="clock"
			class:active={toMove}
			class:low={ticking && clockMs < 20_000}
			role="timer"
			aria-label="{color === 'white' ? 'White' : 'Black'} clock: {formatClock(clockMs)}">{formatClock(clockMs)}</span
		>
	{/if}
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
		min-width: 0;
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
	.offline {
		margin-left: 8px;
		font-size: 13px;
		font-weight: 600;
		color: var(--hazard);
	}
	.lost {
		font-size: 13px;
		color: var(--text-muted);
	}
	.glyphs {
		display: inline-flex;
		gap: 1px;
		vertical-align: middle;
	}
	.glyphs img {
		width: 18px;
		height: 18px;
	}
	.note {
		font-size: 12px;
		color: var(--text-muted);
		text-align: right;
	}
	.turn {
		padding: 4px 10px;
		border: 1px solid var(--accent-line);
		border-radius: 999px;
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
	}
	.clock {
		min-width: 92px;
		padding: 6px 12px;
		box-sizing: border-box;
		border: 1px solid var(--line);
		border-radius: 8px;
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 22px;
		font-variant-numeric: tabular-nums;
		text-align: center;
		color: var(--text-body);
	}
	.clock.active {
		border-color: var(--accent);
		background: var(--accent);
		color: var(--accent-text);
	}
	.clock.active.low {
		border-color: var(--hazard);
		background: var(--hazard);
	}
</style>
