import { describe, expect, it } from 'vitest';
import { BLINK_MS, dicePace, dicePill, stepDice, FILE_MS, MOVE_MS, playMs, RANK_DELAY_MS, RANK_MS, scanFrames, SCAN_MS, scanOf, tumbleFaces, turnMs } from './dice.ts';
import TIMING from './dice-timing.json';
import type { EventJSON, View } from './game.ts';

const move: EventJSON = { kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' };

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
	it('plays a whole turn in the time the server pauses the clock for', () => {
		const last: EventJSON[] = [move, { kind: 'rolled_pothole', roll: 2 }, { kind: 'target', sq: 'd5' }, { kind: 'pothole_opened', sq: 'd5' }];
		expect(turnMs(last)).toBe(MOVE_MS + 600 + 1250 + 550);
		expect(turnMs([move])).toBe(0);
	});
	it('fits the scan inside its step', () => {
		expect(SCAN_MS).toBe(RANK_DELAY_MS + RANK_MS);
		expect(FILE_MS).toBeLessThan(SCAN_MS);
		expect(SCAN_MS).toBeLessThan(TIMING.playMs.target);
	});
	it('blinks the target twice within the re-roll step', () => {
		expect(BLINK_MS).toBeGreaterThan(0);
		expect(2 * BLINK_MS).toBeLessThanOrEqual(TIMING.playMs.reroll);
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
	it('is the same for the same seed, so the tray and the board agree', () => {
		expect(tumbleFaces(5, 850, 3)).toEqual(tumbleFaces(5, 850, 3));
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
	const view = (last: EventJSON[], seq = 7) => ({ seq, last, board: Array(64).fill(''), mamdani: '', potholes: [] }) as unknown as View;
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
	it('says nothing before the roll', () => {
		expect(dicePill(view([move, { kind: 'rolled_pothole', roll: 3 }]), 1)).toBeNull();
	});
});

describe('stepDice', () => {
	const view = (last: EventJSON[]) => ({ seq: 3, last }) as unknown as View;
	it('throws one die for the roll, grey when odd and yellow when even', () => {
		expect(stepDice(view([move, { kind: 'rolled_pothole', roll: 3 }]), 1)).toEqual([{ value: 3, tone: 'dull', delay: 0, ms: 450, seed: 3 * 97 + 2 }]);
		expect(stepDice(view([move, { kind: 'rolled_pothole', roll: 6 }]), 1)[0].tone).toBe('pot');
	});
	it('throws the file and rank dice out of sync, with the scan\'s seeds', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 6 }, { kind: 'target', sq: 'd5' }]);
		const seed = 3 * 97 + 2 * 2;
		expect(stepDice(v, 2)).toEqual([
			{ value: 4, tone: 'where', delay: 0, ms: FILE_MS, seed: seed + 1 },
			{ value: 5, tone: 'where', delay: RANK_DELAY_MS, ms: RANK_MS, seed: seed + 2 }
		]);
	});
	it('throws a cream die for a saving roll, and none for the rest', () => {
		const v = view([move, { kind: 'saving_roll', sq: 'b4', piece: 'wN', roll: 5, saved: true }, { kind: 'fell', sq: 'b4', piece: 'wN' }]);
		expect(stepDice(v, 1)[0]).toMatchObject({ value: 5, tone: 'save' });
		expect(stepDice(v, 2)).toEqual([]);
	});
});
