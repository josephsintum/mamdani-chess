<script lang="ts">
	import type { DicePill } from './dice.ts';
	import { catchUp } from './motion.ts';

	// The pill on the board's corner saying what the dice mean (dice.ts
	// dicePill). A new key plays it again; `age` (Animator.age of its step)
	// starts a pill drawn late that far in.
	let { pill, age }: { pill: DicePill; age: number } = $props();

	/** How long the turn's last pill stays: an odd roll's briefly, a square or a saving roll's a little longer. */
	function hold(p: DicePill): number {
		return p.tone === 'odd' && !p.then ? 600 : (p.then?.at ?? 0) + 1400;
	}
</script>

{#key pill.key}
	<span class="pills" class:last={pill.last} {@attach catchUp(age)} style="--at: {pill.at ?? 0}ms; --hold: {(pill.at ?? 0) + hold(pill)}ms; --then: {pill.then?.at ?? 0}ms" aria-hidden="true">
		<span class="pill {pill.tone}" class:swap={!!pill.then}>{pill.text}</span>
		{#if pill.then}<span class="pill {pill.then.tone} next">{pill.then.text}</span>{/if}
	</span>
{/key}

<style>
	.pills {
		position: absolute;
		top: 6px;
		right: 6px;
		z-index: 7;
		display: grid;
		justify-items: end;
		pointer-events: none;
		animation: pill-in 0.16s ease-out var(--at) both;
	}
	.pills.last {
		animation:
			pill-in 0.16s ease-out var(--at) both,
			pill-out 0.22s ease-in var(--hold) forwards;
	}
	.pill {
		grid-area: 1 / 1;
		padding: 5px 9px;
		border-radius: 999px;
		font: 600 12px/1 var(--font-mono);
		white-space: nowrap;
		background: var(--surface);
		color: var(--text);
		box-shadow: 0 0 0 1px var(--line);
	}
	.pill.pot {
		background: var(--accent);
		color: var(--accent-text);
		box-shadow: none;
	}
	.pill.where {
		background: var(--hazard);
		color: var(--accent-text);
		box-shadow: none;
	}
	.pill.save {
		background: var(--piece-light);
		color: var(--piece-dark);
		box-shadow: none;
	}
	.pill.swap {
		animation: pill-out 0.15s ease-in var(--then) forwards;
	}
	.pill.next {
		opacity: 0;
		animation: pill-in 0.18s ease-out var(--then) forwards;
	}
	@keyframes pill-in {
		from {
			opacity: 0;
			transform: scale(0.8);
		}
		to {
			opacity: 1;
		}
	}
	@keyframes pill-out {
		to {
			opacity: 0;
		}
	}
	/* Reduced motion: the pill shows what the dice decided, then goes. */
	@media (prefers-reduced-motion: reduce) {
		.pills,
		.pill.next {
			animation: pill-in 1ms linear both;
		}
		.pill.swap {
			animation: pill-out 1ms linear forwards;
		}
		.pills.last {
			animation: pill-out 1ms linear var(--hold) forwards;
		}
	}
</style>
