import { describe, expect, it } from 'vitest';
import { dicePace, dicePill, firstDiceStep, landsAt, oddRoll, stepDice, FILE_MS, MOVE_MS, playMs, RANK_DELAY_MS, RANK_MS, SAVE_MS, scanFrames, SCAN_MS, scanOf, THROW_MS, tumbleFaces, withRoll } from './dice.ts';
import TIMING from './dice-timing.json';
import type { EventJSON } from './game.ts';
import { viewOf } from './test-boards.ts';

const move: EventJSON = { kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' };
const view = (last: EventJSON[], seq = 7) => viewOf({ seq, last });

describe('the timing table', () => {
	it('times each step from the shared table', () => {
		expect(MOVE_MS).toBe(TIMING.moveMs);
		expect(playMs({ kind: 'rolled_pothole', roll: 3 })).toBe(TIMING.playMs.rolled_pothole_odd);
		expect(playMs({ kind: 'rolled_pothole', roll: 6 })).toBe(TIMING.playMs.rolled_pothole_even);
		expect(playMs({ kind: 'target', sq: 'd5' })).toBe(TIMING.playMs.target);
		expect(playMs({ kind: 'captured', sq: 'd5' })).toBe(TIMING.playMs.other);
	});
	it('waits for the move before the roll, then for the step just shown', () => {
		expect(dicePace(undefined)).toBe(MOVE_MS);
		expect(dicePace({ kind: 'target', sq: 'd5' })).toBe(TIMING.playMs.target);
	});
	it('fits the scan inside its step', () => {
		expect(SCAN_MS).toBeLessThan(TIMING.playMs.target);
	});
	it('shows what the dice decided once they land', () => {
		expect(landsAt({ kind: 'rolled_pothole', roll: 4 })).toBe(THROW_MS);
		expect(landsAt({ kind: 'target', sq: 'd5' })).toBe(SCAN_MS);
		expect(landsAt({ kind: 'saving_roll', sq: 'd5' })).toBe(SAVE_MS);
		expect(landsAt({ kind: 'fell', sq: 'd5' })).toBeUndefined();
	});
});

describe('the turn around its roll', () => {
	const capped: EventJSON[] = [
		move,
		{ kind: 'pothole_closed', sq: 'a3', color: 'white' },
		{ kind: 'repaired', sq: 'b5' },
		{ kind: 'rolled_pothole', roll: 2, color: 'white' },
		{ kind: 'target', sq: 'g8' },
		{ kind: 'pothole_closed', sq: 'c6', color: 'black' },
		{ kind: 'pothole_opened', sq: 'g8', color: 'white' }
	];
	it('starts the dice at the pothole roll, or the end when none was rolled', () => {
		expect(firstDiceStep(capped)).toBe(3);
		expect(firstDiceStep([move])).toBe(1);
	});
	it('tells the move’s closes and repairs from the roll’s', () => {
		expect([1, 2, 5].map((i) => withRoll(capped, i))).toEqual([false, false, true]);
		expect(withRoll([move, { kind: 'repaired', sq: 'b5' }], 1)).toBe(false);
	});
	it('reads an odd roll', () => {
		expect(oddRoll({ kind: 'rolled_pothole', roll: 3 })).toBe(true);
		expect(oddRoll({ kind: 'rolled_pothole', roll: 8 })).toBe(false);
	});
});

describe('tumbleFaces', () => {
	it('slows down, lands on the value at the end, and never shows it before', () => {
		for (const seed of [1, 7, 42, 99, 1234]) {
			for (const final of [1, 4, 8]) {
				const frames = tumbleFaces(final, 700, seed);
				expect(frames.at(-1)).toEqual({ at: 700, face: final });
				for (const f of frames.slice(0, -1)) {
					expect(f.face).not.toBe(final);
					expect(f.face).toBeGreaterThanOrEqual(1);
					expect(f.face).toBeLessThanOrEqual(8);
				}
				const gaps = frames.slice(1).map((f, i) => f.at - frames[i].at);
				expect(gaps[1]).toBeLessThan(gaps.at(-2)!); // early faces flick faster than late ones
			}
		}
	});
	it('follows its seed, so another throw tumbles differently', () => {
		expect(tumbleFaces(5, 850, 3)).not.toEqual(tumbleFaces(5, 850, 4));
	});
});

describe('scanFrames', () => {
	it('follows the file and rank dice and lands on the target only at the end', () => {
		for (const seed of [2, 5, 11, 300]) {
			const frames = scanFrames('d5', seed);
			expect(frames.at(-1)).toEqual({ at: SCAN_MS, sq: 'd5' });
			for (const f of frames.slice(0, -1)) expect(f.sq).not.toBe('d5');
			// Once the file die lands its column stays.
			for (const f of frames.filter((x) => x.at >= FILE_MS)) expect(f.sq[0]).toBe('d');
		}
	});
});

describe('the board during a roll', () => {
	it('says an odd roll, and lets the pill go', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 3 }]);
		expect(dicePill(v, 2)).toEqual({ key: '7:1', text: 'd8 3 · no pothole', tone: 'odd', at: 450, last: true });
	});
	it('says an even roll, then the square once the file and rank dice land', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 6 }, { kind: 'target', sq: 'd5' }, { kind: 'pothole_opened', sq: 'd5' }]);
		expect(dicePill(v, 2)).toMatchObject({ text: 'd8 6 · pothole', tone: 'pot', last: false });
		expect(dicePill(v, 3)).toEqual({ key: '7:1', text: 'd8 6 · pothole', tone: 'pot', then: { text: 'd5', tone: 'where', at: SCAN_MS }, last: true });
		// One pill from the roll to the square: the target step keeps the roll's
		// pill (by key), so it isn't drawn again and doesn't flicker.
		expect(dicePill(v, 3)?.key).toBe(dicePill(v, 2)?.key);
		// The hole opening keeps the target's pill.
		expect(dicePill(v, 4)?.key).toBe('7:1');
		expect(scanOf(v, 3)).toEqual({ sq: 'd5', key: '7:2', seed: 7 * 97 + 2 * 2 });
		expect(scanOf(v, 4)).toEqual({ sq: 'd5', key: '7:2', seed: 7 * 97 + 2 * 2 });
	});
	it('names a re-roll and a saving roll', () => {
		const v = view([
			move,
			{ kind: 'rolled_pothole', roll: 2 },
			{ kind: 'target', sq: 'e1' },
			{ kind: 'reroll', sq: 'e1', reason: 'king' },
			{ kind: 'target', sq: 'b4' },
			{ kind: 'saving_roll', sq: 'b4', piece: 'wN', roll: 5, saved: true }
		]);
		expect(dicePill(v, 4)).toMatchObject({ text: 'e1 · re-roll', tone: 'pot' });
		// After a re-roll the target's pill is a new one.
		expect(dicePill(v, 5)?.key).toBe('7:4');
		expect(dicePill(v, 6)).toEqual({ key: '7:5', text: 'White knight b4 · saving roll', tone: 'save', then: { text: 'White knight b4 · saved', tone: 'save', at: 450 }, last: true });
		expect(scanOf(v, 4)).toBeNull(); // the re-roll: no scan while the target blinks
		expect(scanOf(v, 5)?.sq).toBe('b4');
	});
	it('says a reset, with no re-roll blink', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 4 }, { kind: 'target', sq: 'd4' }, { kind: 'pothole_reset', sq: 'd4', color: 'white', was: 'black', left: 1 }]);
		expect(dicePill(v, 4)).toEqual({ key: '7:3', text: 'Pothole reset: 3 rounds', tone: 'pot', last: true });
		expect(scanOf(v, 4)?.sq).toBe('d4');
	});
	it('says nothing before the roll', () => {
		expect(dicePill(view([move, { kind: 'rolled_pothole', roll: 3 }]), 1)).toBeNull();
	});
});

describe('stepDice', () => {
	it('throws one die for the roll, grey when odd and yellow when even', () => {
		expect(stepDice(view([move, { kind: 'rolled_pothole', roll: 3 }], 3), 1)).toEqual([{ value: 3, tone: 'dull', delay: 0, ms: 450, seed: 3 * 97 + 2 }]);
		expect(stepDice(view([move, { kind: 'rolled_pothole', roll: 6 }], 3), 1)[0].tone).toBe('pot');
	});
	it('throws the file and rank dice out of sync, with the scan\'s seeds', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 6 }, { kind: 'target', sq: 'd5' }], 3);
		const seed = 3 * 97 + 2 * 2;
		expect(stepDice(v, 2)).toEqual([
			{ value: 4, tone: 'where', delay: 0, ms: FILE_MS, seed: seed + 1 },
			{ value: 5, tone: 'where', delay: RANK_DELAY_MS, ms: RANK_MS, seed: seed + 2 }
		]);
	});
	it('throws a cream die for a saving roll, and none for the rest', () => {
		const v = view([move, { kind: 'saving_roll', sq: 'b4', piece: 'wN', roll: 5, saved: true }, { kind: 'fell', sq: 'b4', piece: 'wN' }], 3);
		expect(stepDice(v, 1)[0]).toMatchObject({ value: 5, tone: 'save' });
		expect(stepDice(v, 2)).toEqual([]);
	});
});
