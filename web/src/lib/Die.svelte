<script lang="ts">
	import { untrack } from 'svelte';
	import { THROW_MS, tumbleFaces, type DieSpec } from './dice.ts';
	import { reducedMotion } from './motion.ts';

	// A d8 shown as a diamond, thrown into the tray: it flies in spinning,
	// bounces twice and settles, its face tumbling on the seeded schedule the
	// board's scan follows, slowing until it lands on its value.
	let { die, small = false }: { die: DieSpec; small?: boolean } = $props();

	let face = $state(0); // 0: landed, showing the value

	// Thrown once, when the die appears: the tray redraws its steps as the
	// roll plays, with equal specs, and must not throw it again.
	function roll(node: HTMLElement) {
		const d = untrack(() => die);
		if (reducedMotion()) return;
		const frames = tumbleFaces(d.value, d.ms, d.seed);
		face = frames[0].face;
		const timers = frames.map((f) => setTimeout(() => (face = f.at >= d.ms ? 0 : f.face), d.delay + f.at));
		node.animate(
			[
				{ transform: 'translate(-28px, -14px) rotate(-215deg) scale(0.7)', opacity: 0 },
				{ transform: 'translate(0, -8px) rotate(-15deg) scale(1)', opacity: 1, offset: 0.5 },
				{ transform: 'translate(0, 0) rotate(35deg)', offset: 0.72 },
				{ transform: 'translate(0, -3px) rotate(45deg)', offset: 0.86 },
				{ transform: 'translate(0, 0) rotate(45deg)' }
			],
			{ duration: THROW_MS, delay: d.delay, easing: 'ease-out', fill: 'backwards' }
		);
		return () => timers.forEach(clearTimeout);
	}
</script>

<!-- A pothole roll's die shows even (yellow) or odd only once it lands. -->
<span class="die {face && (die.tone === 'pot' || die.tone === 'dull') ? '' : die.tone}" class:small {@attach roll}>
	<!-- The tray is a live region: screen readers get the value, never the tumbling faces. -->
	<span aria-hidden="true">{face || die.value}</span>
	<span class="sr-only">{die.value}</span>
</span>

<style>
	.die {
		display: grid;
		place-items: center;
		width: 30px;
		height: 30px;
		margin: 6px;
		transform: rotate(45deg);
		background: var(--line);
		border-radius: 5px;
	}
	/* The phone's dice card: fits a 26px chip row. */
	.die.small {
		width: 22px;
		height: 22px;
		margin: 0 4px;
		border-radius: 4px;
	}
	.die.small span {
		font-size: 13px;
	}
	.die span {
		transform: rotate(-45deg);
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 15px;
		color: var(--text);
	}
	.die .sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	/* What each die decides: an odd pothole roll is grey, an even one yellow;
	   the file and rank dice are orange; a saving die is cream. */
	.die.dull span {
		color: var(--text-muted);
	}
	.die.pot {
		background: var(--accent);
	}
	.die.where {
		background: var(--hazard);
	}
	.die.save {
		background: var(--piece-light);
	}
	.die.pot span,
	.die.where span,
	.die.save span {
		color: var(--accent-text);
	}
</style>
