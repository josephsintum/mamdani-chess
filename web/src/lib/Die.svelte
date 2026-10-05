<script lang="ts">
	import { reducedMotion } from './motion.ts';

	// A d8 shown as a diamond. When it appears it tumbles through random
	// faces for a moment, then lands on its value.
	let { value, highlight = false, small = false }: { value: number; highlight?: boolean; small?: boolean } = $props();

	const TUMBLE_MS = 320;
	let face = $state(0); // 0 = settled on value

	function tumble(_node: HTMLElement) {
		if (reducedMotion()) return;
		const started = performance.now();
		const timer = setInterval(() => {
			face = performance.now() - started < TUMBLE_MS ? 1 + Math.floor(Math.random() * 8) : 0;
			if (face === 0) clearInterval(timer);
		}, 50);
		face = 1 + Math.floor(Math.random() * 8);
		return () => clearInterval(timer);
	}
</script>

<span class="die" class:highlight class:small class:rolling={face !== 0} {@attach tumble}>
	<!-- The tray is a live region: screen readers get the value, never the tumbling faces. -->
	<span aria-hidden="true">{face || value}</span>
	<span class="sr-only">{value}</span>
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
	.die.rolling {
		animation: tumble 0.32s ease-out;
	}
	@keyframes tumble {
		from {
			transform: rotate(-135deg) scale(0.6);
		}
		to {
			transform: rotate(45deg) scale(1);
		}
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
	.die.highlight {
		background: var(--accent);
	}
	.die.highlight span {
		color: var(--accent-text);
	}
	@media (prefers-reduced-motion: reduce) {
		.die.rolling {
			animation: none;
		}
	}
</style>
