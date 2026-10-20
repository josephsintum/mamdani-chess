<script lang="ts">
	import type { Animator } from './animator.svelte.ts';
	import Board from './Board.svelte';
	import { boardEffects, checkSquare, stageAt } from './board.ts';
	import { quipper } from './catchphrases.ts';
	import type { MoveJSON } from './game.ts';

	// The board with a turn's dice and big moments played as in a game, for
	// pages that drive their own Animator: the /dev/board sandbox, the
	// how-to-play scenes and practice.
	let {
		anim,
		legal = [],
		flipped = false,
		interactive = false,
		showing = '',
		still = false,
		id,
		ending = 0,
		onmove = () => {},
		onrefuse
	}: {
		anim: Animator;
		legal?: MoveJSON[];
		flipped?: boolean;
		interactive?: boolean;
		/** A piece whose moves are shown as if picked up (Board's showing). */
		showing?: string;
		/** No effects: instant mode. */
		still?: boolean;
		/** Keys this board's one-off effects apart from other boards'. */
		id: string;
		/** Bump to play an ending set by hand (a result with no turn) again. */
		ending?: number;
		onmove?: (move: MoveJSON) => void;
		/** A tap on a piece that can't move now (Board's onrefuse). */
		onrefuse?: (sq: string) => void;
	} = $props();

	let view = $derived(anim.view!);
	let stage = $derived(stageAt(view, anim.shown));
	// Big moments say something, as in a game; an ending set by hand says
	// the winner's line.
	const say = quipper();
	let effects = $derived(
		boardEffects(view, anim.shown, {
			animating: anim.animating,
			animated: anim.animated,
			quiet: still,
			say,
			ending: ending > 0 ? { key: `${id}:end:${ending}`, winner: true } : null,
			// A mate shown at once (a reload) doesn't burst, as in a game.
			mate: anim.animated ? `${id}:${view.seq}:${ending}` : null
		})
	);
</script>

<Board
	{stage}
	{legal}
	{flipped}
	{showing}
	interactive={interactive && !anim.animating && view.status === 'playing'}
	dim={!!view.result && !anim.animating}
	check={checkSquare(view, stage, { animating: anim.animating, guessing: false })}
	{...effects}
	age={anim.age}
	{onmove}
	{onrefuse}
/>
