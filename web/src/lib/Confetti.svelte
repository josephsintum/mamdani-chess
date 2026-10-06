<script lang="ts">
	import { confetti, CONFETTI_MS, type Confetto } from './feel.ts';

	// The winner's confetti: one canvas over the page for CONFETTI_MS, then
	// `ondone` so the page removes it. The page shows it only to the winner,
	// only when the game ended while they watched, never under reduced motion.
	let { delay = 0, ondone }: { delay?: number; ondone: () => void } = $props();

	const PIECES = confetti();
	const FADE_MS = 300;

	function cone(ctx: CanvasRenderingContext2D, c: Confetto, colors: Record<string, string>) {
		const { w, h } = c;
		ctx.fillStyle = colors.hazard;
		ctx.beginPath();
		ctx.moveTo(0, -h / 2);
		ctx.lineTo(w * 0.35, h * 0.35);
		ctx.lineTo(-w * 0.35, h * 0.35);
		ctx.closePath();
		ctx.fill();
		ctx.fillStyle = colors.cream;
		ctx.fillRect(-w * 0.2, -h * 0.05, w * 0.4, h * 0.16);
		ctx.fillStyle = colors.hazard;
		ctx.fillRect(-w / 2, h * 0.35, w, h * 0.15);
	}

	function fall(canvas: HTMLCanvasElement) {
		const ctx = canvas.getContext('2d');
		if (!ctx) {
			ondone();
			return;
		}
		const css = getComputedStyle(canvas);
		const colors: Record<string, string> = {
			accent: css.getPropertyValue('--accent').trim() || '#f2c230',
			hazard: css.getPropertyValue('--hazard').trim() || '#ff7a3d',
			cream: css.getPropertyValue('--piece-light').trim() || '#fbf8f0'
		};
		const dpr = window.devicePixelRatio || 1;
		let width = 0;
		let height = 0;
		const size = () => {
			width = window.innerWidth;
			height = window.innerHeight;
			canvas.width = Math.round(width * dpr);
			canvas.height = Math.round(height * dpr);
		};
		size();
		window.addEventListener('resize', size);
		const start = performance.now() + delay;
		let frame = requestAnimationFrame(function draw(now) {
			const t = Math.max(0, now - start);
			ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
			ctx.clearRect(0, 0, width, height);
			ctx.globalAlpha = Math.min(1, (CONFETTI_MS - t) / FADE_MS);
			for (const c of PIECES) {
				const p = (t - c.delay) / c.fall;
				if (p <= 0 || p >= 1) continue;
				ctx.save();
				ctx.translate(c.x * width + Math.sin(p * Math.PI * 3) * c.sway, -20 + p * (height + 40));
				ctx.rotate(((c.spin * t) / 1000) * (Math.PI / 180));
				if (c.cone) cone(ctx, c, colors);
				else {
					// A strip turning over: its width swings, like paper.
					ctx.fillStyle = colors[c.tone];
					ctx.fillRect((-c.w / 2) * Math.cos(p * 9), -c.h / 2, c.w * Math.cos(p * 9), c.h);
				}
				ctx.restore();
			}
			if (t < CONFETTI_MS) frame = requestAnimationFrame(draw);
			else ondone();
		});
		return () => {
			cancelAnimationFrame(frame);
			window.removeEventListener('resize', size);
		};
	}
</script>

<canvas class="confetti" aria-hidden="true" {@attach fall}></canvas>

<style>
	.confetti {
		position: fixed;
		inset: 0;
		z-index: 50;
		width: 100vw;
		height: 100vh;
		pointer-events: none;
	}
</style>
