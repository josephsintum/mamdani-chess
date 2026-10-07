<script lang="ts">
	import { Animator } from './animator.svelte.ts';
	import DiceSummary from './DiceSummary.svelte';
	import { freeMoves } from './sandbox.ts';
	import { reducedMotion } from './motion.ts';
	import { framesOf, type Scene } from './scenes.ts';
	import ScriptedBoard from './ScriptedBoard.svelte';

	// One how-to-play scene on the real board: it plays while it's on screen,
	// waits, and plays again. With reduced motion it shows how the scene ends
	// until Play is pressed.
	let { scene, label }: { scene: Scene; label: string } = $props();

	const LOOP_GAP = 2000;
	const anim = new Animator();
	let showing = $state('');
	let playing = $state(false);
	let legal = $derived(showing && anim.view ? freeMoves(anim.view, true) : []);
	let run = 0; // a new run cancels the one playing
	let presses = $state(0); // Play/Replay presses: each spins the icon and fades the board in again

	// Every run gets its own seq numbers, so one-off effects keyed by seq
	// (repair celebrations, the mate burst) play again.
	let nextSeq = 0;
	function frames() {
		nextSeq += 1000;
		return framesOf(scene, nextSeq);
	}

	function showEnd() {
		const all = frames();
		anim.receive(all[all.length - 1].view);
		showing = '';
	}
	showEnd();

	const sleep = (ms: number) => new Promise((done) => setTimeout(done, ms));

	async function play() {
		const me = ++run;
		playing = true;
		for (;;) {
			for (const f of frames()) {
				if (me !== run) return;
				showing = f.show;
				if (f.view !== anim.view) anim.receive(f.view);
				while (anim.animating && me === run) await sleep(100);
				await sleep(f.hold);
			}
			if (me !== run) return;
			if (reducedMotion()) break;
			await sleep(LOOP_GAP);
		}
		playing = false;
	}

	function stop() {
		run++;
		anim.stop();
		playing = false;
	}

	function replay() {
		presses++;
		void play();
	}

	// Plays while on screen; reduced motion waits for Play.
	function watch(node: HTMLElement) {
		const seen = new IntersectionObserver(([e]) => {
			if (!e.isIntersecting) stop();
			else if (!playing && !reducedMotion()) void play();
		}, { threshold: 0.5 });
		seen.observe(node);
		return () => {
			seen.disconnect();
			stop();
		};
	}
</script>

<figure class="scene" {@attach watch}>
	{#key presses}
		<div class="board" class:rewound={presses > 0} inert>
			<ScriptedBoard {anim} {legal} {showing} id="scene-{scene.id}" ending={1} />
			<DiceSummary view={anim.view!} shown={anim.shown} />
		</div>
	{/key}
	<button type="button" class="replay" onclick={replay}>
		{#key presses}<span class="icon" class:spin={presses > 0} aria-hidden="true">{playing ? '↻' : '▶'}</span>{/key}
		{playing ? 'Replay' : 'Play'}
	</button>
	<figcaption class="sr-only">{label}</figcaption>
</figure>

<style>
	.scene {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin: 0;
		min-width: 0;
	}
	.board {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.replay {
		align-self: flex-end;
		min-height: 40px;
		padding: 0 14px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-size: 14px;
		font-weight: 600;
		cursor: pointer;
		display: flex;
		align-items: center;
		gap: 6px;
		transition:
			transform 0.1s ease,
			border-color 0.15s ease;
	}
	.replay:hover {
		border-color: var(--text-muted);
	}
	.replay:active {
		transform: scale(0.95);
		border-color: var(--accent);
	}
	.icon {
		display: inline-block;
	}
	/* A press winds the scene back: the icon spins a turn and the board fades in at the start. */
	.spin {
		animation: spin 0.5s ease-out;
	}
	.rewound {
		animation: rewind 0.35s ease-out;
	}
	@keyframes spin {
		from {
			transform: rotate(-360deg);
		}
	}
	@keyframes rewind {
		from {
			opacity: 0.3;
			transform: scale(0.98);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.spin,
		.rewound {
			animation: none;
		}
		.replay:active {
			transform: none;
		}
	}
</style>
