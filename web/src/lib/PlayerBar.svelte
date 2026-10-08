<script lang="ts">
	import { bonusOf, formatClock } from './clock.ts';
	import { pieceName, sideName, type Color } from './game.ts';
	import { byKind, type BarPill } from './board.ts';
	import LogMark from './LogMark.svelte';
	import { reducedMotion } from './motion.ts';

	let {
		color,
		name = '',
		you,
		lost,
		taken = [],
		lead = 0,
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
		/** The other side's pieces this player has captured. */
		taken?: string[];
		/** How far ahead in material this player is; 0 when level or behind. */
		lead?: number;
		/** "Your move", "In check", "Waiting…", or "" for none (see pillFor). */
		pill?: string;
		pillTone?: BarPill['tone'];
		/** The phone's one-row bar: captures, lead and pothole count after the name. */
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

	let side = $derived(sideName(color));
	let took = $derived(byKind(taken));
	let gone = $derived(byKind(lost));
	let names = (codes: string[]) => codes.map((p) => pieceName(p)).join(', ') || 'none';
	// The phone's one label for its row of pieces.
	let summary = $derived(`Took: ${names(took)}${lead ? `, up ${lead}` : ''}. Lost to potholes: ${names(gone)}.`);
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
			{#if took.length > 0 || lead > 0 || gone.length > 0}<span class="sr-only">{summary}</span>{/if}
			{#if took.length > 0}<span class="taken" aria-hidden="true">{#each took as p, i (i)}<img src="/pieces/{p}.svg" alt="" class:same={p === took[i - 1]} />{/each}</span>{/if}
			{#if lead}<span class="lead" aria-hidden="true">+{lead}</span>{/if}
			{#if gone.length > 0}<span class="pothole" aria-hidden="true"><LogMark kind="hole" />{gone.length}</span>{/if}
		{:else}
			<span class="lost">
				Took:
				{#if took.length === 0}none{:else}<span class="taken">{#each took as p, i (i)}<img src="/pieces/{p}.svg" alt={pieceName(p)} class:same={p === took[i - 1]} />{/each}</span>{/if}
				{#if lead}<span class="lead">+{lead}</span>{/if}
			</span>
			<span class="lost">
				Lost to potholes:
				{#if gone.length === 0}none{:else}<span class="glyphs">{#each gone as p, i (i)}<img src="/pieces/{p}.svg" alt={pieceName(p)} class:same={p === gone[i - 1]} />{/each}</span>{/if}
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
		gap: 6px;
	}
	.compact .taken img {
		width: 16px;
		height: 16px;
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
	.glyphs,
	.taken {
		display: inline-flex;
		align-items: center;
		gap: 1px;
		vertical-align: middle;
		overflow: hidden;
	}
	/* On the phone the name gives way first, then the captured pieces,
	   which fade where they are cut; the lead and the pothole count stay. */
	.compact .name {
		flex-shrink: 2;
		min-width: min(64px, 100%);
	}
	.compact .taken {
		flex-shrink: 1;
		min-width: 0;
		mask-image: linear-gradient(to right, #000 calc(100% - 8px), transparent);
	}
	/* Short of room (a 320 px phone with its pill), the captured pieces go
	   rather than show cut in half; the lead and the pothole count stay, and
	   the row's label still lists everything. */
	.compact .who {
		container: who / inline-size;
		overflow: hidden;
	}
	@container who (max-width: 150px) {
		.taken {
			display: none;
		}
	}
	/* The smallest phone on its side, with a pill: only the name. */
	@container who (max-width: 120px) {
		.lead,
		.pothole {
			display: none;
		}
	}
	/* Pieces of one kind stack, as lichess shows them, so the row stays short. */
	img.same {
		margin-left: -9px;
	}
	.compact img.same {
		margin-left: -10px;
	}
	.lead {
		flex-shrink: 0;
		margin-left: 2px;
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}
	/* The phone's pieces lost to potholes: a hole and how many. */
	.pothole {
		display: flex;
		align-items: center;
		gap: 3px;
		flex-shrink: 0;
		margin-left: 4px;
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		color: var(--text-muted);
	}
	.glyphs img,
	.taken img {
		width: 18px;
		height: 18px;
		flex-shrink: 0;
		/* The board's white outline, 1 px at this size, so black pieces show
		   on the dark page. */
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
