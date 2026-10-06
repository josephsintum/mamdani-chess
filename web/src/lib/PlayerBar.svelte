<script lang="ts">
	import { bonusOf, formatClock } from './clock.ts';
	import { pieceName, type Color } from './game.ts';
	import { reducedMotion } from './motion.ts';

	let {
		color,
		name = '',
		you,
		lost,
		pill = '',
		pillTone = 'turn',
		compact = false,
		clockMs,
		toMove = false,
		ticking = false,
		seq,
		pausedForDice = false,
		offline = false
	}: {
		color: Color;
		/** The player's name; the side ("White") when there's none. */
		name?: string;
		you: boolean;
		lost: string[];
		/** "Your move", "In check", "Waiting…", or "" for none (see pillFor). */
		pill?: string;
		pillTone?: 'turn' | 'check' | 'muted';
		/** The phone's one-row bar: lost pieces after the name, nothing when none. */
		compact?: boolean;
		/** Time left, in ms; no clock is shown without it (the sandbox). */
		clockMs?: number;
		/** It's this side's turn: the clock is highlighted. */
		toMove?: boolean;
		/** The clock is counting down right now. */
		ticking?: boolean;
		/** The game's move count (view.seq): "+5" shows only when a move lands. */
		seq?: number;
		/** It's this side's turn, but the clock waits for the dice. */
		pausedForDice?: boolean;
		/** The player has no tab open on the game. */
		offline?: boolean;
	} = $props();

	let side = $derived(color === 'white' ? 'White' : 'Black');
	let label = $derived(name || side);

	// The "+5": when this player moves, the increment floats up from their
	// clock and the time ticks up into place, one second at a time. `lastClock`
	// is plain bookkeeping between the clock's readings, not state.
	const TICK_UP_MS = 300;
	let lastClock: number | undefined;
	let lastSeq: number | undefined;
	let bonus: { ms: number; key: number } | null = null;
	let gained = $derived.by(() => {
		const moved = lastSeq !== undefined && seq !== lastSeq;
		const ms = clockMs === undefined ? 0 : bonusOf(lastClock, clockMs, { ticking, moved });
		lastClock = clockMs;
		lastSeq = seq;
		if (ms && !reducedMotion()) bonus = { ms, key: (bonus?.key ?? 0) + 1 };
		return bonus;
	});
	// How far behind the real time the clock shows, while it ticks up.
	let lag = $state(0);
	function tickUp(ms: number) {
		return () => {
			const steps = Math.max(1, Math.round(ms / 1000));
			let step = 0;
			lag = ms;
			let timer: ReturnType<typeof setInterval> | undefined;
			const start = setTimeout(() => {
				timer = setInterval(() => {
					step += 1;
					lag = step >= steps ? 0 : ms - step * 1000;
					if (step >= steps) clearInterval(timer);
				}, TICK_UP_MS / steps);
			}, 150);
			return () => {
				clearTimeout(start);
				clearInterval(timer);
				lag = 0;
			};
		};
	}
</script>

<div class="bar" class:compact>
	<span class="swatch {color}" aria-hidden="true"></span>
	<span class="who">
		<span class="name"
			><span class="label" title={label}>{label}</span>{#if you}<span class="you" class:sr-only={compact}>(you)</span>{/if}{#if offline}<span class="offline">{compact ? 'Offline' : 'Disconnected'}</span>{/if}</span
		>
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
	{#if pausedForDice && !compact}<span class="note">Paused for dice</span>{/if}
	{#if clockMs !== undefined}
		<span class="clock-wrap">
			{#key gained?.key}
				<span
					class="clock"
					class:active={toMove}
					class:low={ticking && clockMs < 20_000}
					class:bump={!!gained}
					role="timer"
					aria-label="{side} clock: {formatClock(clockMs)}">{formatClock(clockMs - lag)}</span
				>
				{#if gained}<span class="plus" aria-hidden="true" {@attach tickUp(gained.ms)}>+5</span>{/if}
			{/key}
		</span>
	{/if}
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
		display: flex;
		align-items: baseline;
		min-width: 0;
		font-weight: 600;
		color: var(--text);
		white-space: nowrap;
	}
	/* A long name ends in an ellipsis; "(you)" and "Offline" always show. */
	.label {
		overflow: hidden;
		text-overflow: ellipsis;
	}
	.you,
	.offline {
		flex-shrink: 0;
	}
	/* On a phone your bar is always the bottom one, so "(you)" is for screen
	   readers only and the name gets the room. */
	.you.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		margin: 0;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
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
		overflow: hidden;
	}
	.glyphs img {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
	}
	/* The board's white outline, 1 px at this size, so black pieces show on
	   the dark page. */
	.glyphs img {
		filter: drop-shadow(1px 0 0 var(--piece-outline)) drop-shadow(-1px 0 0 var(--piece-outline))
			drop-shadow(0 1px 0 var(--piece-outline)) drop-shadow(0 -1px 0 var(--piece-outline));
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
	.note {
		flex-shrink: 0;
		font-size: 12px;
		color: var(--text-muted);
	}
	.clock-wrap {
		position: relative;
		display: flex;
		flex-shrink: 0;
	}
	.clock {
		flex-shrink: 0;
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
	.compact .clock {
		min-width: 64px;
		height: 32px;
		padding: 0 8px;
		font-size: 16px;
		line-height: 30px;
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
	/* The increment: "+5" floats up off the clock, which bumps as the time lands. */
	.clock.bump {
		animation: clock-bump 0.2s ease-out 0.45s;
	}
	@keyframes clock-bump {
		40% {
			transform: scale(1.08);
		}
	}
	.plus {
		position: absolute;
		top: 0;
		left: 50%;
		z-index: 2;
		font-family: var(--font-mono);
		font-weight: 700;
		font-size: 16px;
		color: var(--accent);
		pointer-events: none;
		opacity: 0;
		animation: plus-up 0.6s ease-out forwards;
	}
	.compact .plus {
		font-size: 13px;
	}
	@keyframes plus-up {
		0% {
			opacity: 0;
			transform: translate(-50%, 0);
		}
		25% {
			opacity: 1;
		}
		100% {
			opacity: 0;
			transform: translate(-50%, -150%);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.clock.bump,
		.plus {
			animation: none;
		}
	}
</style>
