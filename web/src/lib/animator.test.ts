import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Animator } from './animator.svelte.ts';
import { MOVE_MS, playMs } from './dice.ts';
import type { EventJSON, View } from './game.ts';

const roll: EventJSON[] = [
	{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
	{ kind: 'rolled_pothole', roll: 2, color: 'white' },
	{ kind: 'target', sq: 'd4' },
	{ kind: 'pothole_opened', sq: 'd4', color: 'white' }
];

function view(seq: number, last: EventJSON[] = []): View {
	return { seq, last } as View;
}

describe('Animator', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('shows the first view at once', () => {
		const a = new Animator(100);
		a.receive(view(3, roll));
		expect(a.shown).toBe(roll.length);
		expect(a.animating).toBe(false);
	});

	it('plays the next turn one dice step at a time', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.receive(view(1, roll));
		expect(a.shown).toBe(1); // the move itself, before the roll
		expect(a.animating).toBe(true);
		vi.advanceTimersByTime(100);
		expect(a.shown).toBe(2);
		vi.advanceTimersByTime(200);
		expect(a.shown).toBe(roll.length);
		expect(a.animating).toBe(false);
	});

	it('marks only a turn that plays out as animated, so one-off effects never replay', () => {
		const a = new Animator(100);
		a.receive(view(3, roll)); // first load
		expect(a.animated).toBe(false);
		a.receive(view(4, roll)); // the next turn
		expect(a.animated).toBe(true);
		vi.advanceTimersByTime(1000);
		a.receive(view(4, roll)); // an update for the same turn keeps it
		expect(a.animated).toBe(true);
		a.receive(view(6, roll)); // a reconnect that skipped a turn
		expect(a.animated).toBe(false);
		a.receive(view(7, roll), { hidden: true }); // a hidden tab
		expect(a.animated).toBe(false);
	});

	it('jumps straight to a view that skips turns', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.receive(view(2, roll));
		expect(a.shown).toBe(roll.length);
	});

	it('cuts an animation short when a newer view arrives', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.receive(view(1, roll));
		a.receive(view(3, roll));
		expect(a.shown).toBe(roll.length);
		vi.advanceTimersByTime(1000); // the old timer must not touch the new view
		expect(a.shown).toBe(roll.length);
	});

	it('skips the animation for a hidden tab and can finish early', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.receive(view(1, roll), { hidden: true });
		expect(a.shown).toBe(roll.length);
		a.receive(view(2, roll));
		a.finish();
		expect(a.animating).toBe(false);
	});

	it('reveals a live turn step by step, or all at once when it jumps; never on load', () => {
		const a = new Animator(100);
		const calls: [number, number, number, boolean | undefined][] = [];
		a.onreveal = (v, from, to, jumped) => calls.push([v.seq, from, to, jumped]);
		a.receive(view(0, roll)); // the first view: shown at once, silently
		a.receive(view(1, roll), { hidden: true }); // live, in a hidden tab
		a.receive(view(2, roll)); // live: the move, then one step at a time
		vi.advanceTimersByTime(100);
		a.finish(); // back mid-roll: the rest at once
		a.receive(view(9, roll)); // turns skipped (a reconnect): silent
		expect(calls).toEqual([
			[1, 0, 4, true],
			[2, 0, 1, undefined],
			[2, 1, 2, undefined],
			[2, 2, 4, true]
		]);
	});

	it('keeps playing when an update for the same turn arrives', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.receive(view(1, roll));
		vi.advanceTimersByTime(100);
		a.receive(view(1, roll)); // the opponent went offline, say
		expect(a.shown).toBe(2);
		expect(a.animating).toBe(true);
		vi.advanceTimersByTime(100);
		expect(a.shown).toBe(3);
	});

	it('stops at the end of a shorter turn for the same move', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.receive(view(1, roll));
		a.receive(view(1, [])); // resigned during the roll: nothing left to show
		expect(a.shown).toBe(0);
		expect(a.animating).toBe(false);
	});

	it('takes a new step time for the next turn', () => {
		const a = new Animator(100);
		a.receive(view(0));
		a.pace = 1000;
		a.receive(view(1, roll));
		vi.advanceTimersByTime(999);
		expect(a.shown).toBe(1);
		vi.advanceTimersByTime(1);
		expect(a.shown).toBe(2);
	});
});

describe('Animator with the dice table', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => vi.useRealTimers());

	it('waits for the move, then for each step as long as it plays', () => {
		const a = new Animator();
		a.receive(view(0));
		a.receive(view(1, roll));
		expect(a.shown).toBe(1);
		vi.advanceTimersByTime(MOVE_MS - 1);
		expect(a.shown).toBe(1);
		vi.advanceTimersByTime(1);
		expect(a.shown).toBe(2); // the roll
		vi.advanceTimersByTime(playMs(roll[1]));
		expect(a.shown).toBe(3); // the target
		vi.advanceTimersByTime(playMs(roll[2]) - 1);
		expect(a.shown).toBe(3); // the scan still runs
		vi.advanceTimersByTime(1);
		expect(a.shown).toBe(4);
	});
});
