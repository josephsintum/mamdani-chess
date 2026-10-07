<script lang="ts">
	import type { Animator } from './animator.svelte.ts';
	import Board from './Board.svelte';
	import { checkSquare, matedKing, repairsShown, stageAt } from './board.ts';
	import { contextOf, endQuip, quipper } from './catchphrases.ts';
	import { dicePill, scanOf } from './dice.ts';
	import type { MoveJSON } from './game.ts';

	// The board with a turn's dice and big moments played as in a game, for
	// views made in the browser (sandbox.ts) rather than sent by the server:
	// the /dev/board sandbox and the how-to-play scenes.
	let {
		anim,
		legal = [],
		flipped = false,
		interactive = false,
		showing = '',
		still = false,
		id,
		ending = 0,
		onmove = () => {}
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
	} = $props();

	let view = $derived(anim.view!);
	let stage = $derived(stageAt(view, anim.shown));
	let lastMove = $derived.by(() => {
		const m = view.last.find((e) => e.kind === 'moved');
		return m?.from && m?.to ? { from: m.from, to: m.to } : null;
	});
	let savedSquare = $derived(view.last.find((e, i) => e.kind === 'saving_roll' && e.saved && i < anim.shown)?.sq ?? '');
	// A turn that is playing out, with animations on.
	let live = $derived(anim.animated && !still);
	// The Mamdani's repairs, celebrated only on a turn that is playing out.
	let repairs = $derived(live ? repairsShown(view, anim.shown) : []);
	// The dice on the board, as in a game.
	let pill = $derived(live ? dicePill(view, anim.shown) : null);
	let scan = $derived(live ? scanOf(view, anim.shown) : null);
	let reroll = $derived(anim.animating && view.last[anim.shown - 1]?.kind === 'reroll');
	// Big moments say something, as in a game.
	const say = quipper();
	let quip = $derived.by(() => {
		if (still) return null;
		const end = ending > 0 && !anim.animating ? endQuip(view) : null;
		if (end) return say(view.last, view.seq, anim.shown, { end, key: `${id}:end:${ending}`, winner: true });
		return anim.animated ? say(view.last, view.seq, anim.shown, undefined, contextOf(view)) : null;
	});
	let mated = $derived.by(() => {
		const sq = !still && !anim.animating ? matedKing(view) : '';
		return sq ? { sq, key: `${id}:${view.seq}:${ending}` } : null;
	});
</script>

<Board
	{stage}
	{legal}
	{lastMove}
	{flipped}
	{showing}
	interactive={interactive && !anim.animating && view.status === 'playing'}
	dim={!!view.result && !anim.animating}
	check={checkSquare(view, stage, { animating: anim.animating, guessing: false })}
	saved={savedSquare}
	{repairs}
	{quip}
	{mated}
	{pill}
	{scan}
	{reroll}
	{onmove}
/>
