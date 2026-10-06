import { describe, expect, it } from 'vitest';
import { pickLine, quipper, QUIPS, quipFor } from './catchphrases.ts';
import type { EventJSON } from './game.ts';

const roll: EventJSON[] = [{ kind: 'moved', from: 'e2', to: 'e4' }, { kind: 'rolled_pothole', roll: 4 }, { kind: 'target', sq: 'a5' }];

describe('quipFor', () => {
	it('finds the Mamdani falling in, once that step is revealed', () => {
		const events = [...roll, { kind: 'saving_roll', sq: 'a5', piece: 'M', roll: 2, saved: false }, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }];
		expect(quipFor(events, 4)).toBeNull();
		expect(quipFor(events, 5)).toEqual({ kind: 'mamdaniFell', sq: 'a5', index: 4 });
		expect(quipFor(events, 6)).toEqual({ kind: 'mamdaniFell', sq: 'a5', index: 4 });
	});
	it('finds a queen falling in and the Mamdani repairing a hole', () => {
		expect(quipFor([...roll, { kind: 'fell', sq: 'd8', piece: 'bQ' }], 4)).toMatchObject({ kind: 'queenFell', sq: 'd8' });
		expect(quipFor([{ kind: 'moved', from: 'a5', to: 'b6' }, { kind: 'repaired', sq: 'c7' }], 2)).toMatchObject({ kind: 'repaired', sq: 'c7' });
	});
	it('finds a saving roll that saves a queen or a rook, not a pawn', () => {
		expect(quipFor([...roll, { kind: 'saving_roll', sq: 'a1', piece: 'wR', roll: 3, saved: true }], 4)).toMatchObject({ kind: 'saved', sq: 'a1' });
		expect(quipFor([...roll, { kind: 'saving_roll', sq: 'a2', piece: 'wP', roll: 3, saved: true }], 4)).toBeNull();
		expect(quipFor([...roll, { kind: 'fell', sq: 'a2', piece: 'wP' }], 4)).toBeNull();
	});
});

describe('pickLine', () => {
	it('never picks the same line twice in a row', () => {
		const lines = QUIPS.mamdaniFell.lines;
		for (const previous of lines) {
			for (const r of [0, 0.3, 0.6, 0.99]) expect(pickLine('mamdaniFell', previous, () => r)).not.toBe(previous);
		}
	});
	it('picks from the event\'s lines, the only one when there is one', () => {
		expect(QUIPS.mamdaniFell.lines).toContain(pickLine('mamdaniFell', undefined, () => 0.5));
		expect(pickLine('saved', 'That was close.', () => 0)).toBe('That was close.');
	});
});

describe('quipper', () => {
	const fall: EventJSON[] = [...roll, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }];
	it('keeps the same line for a moment while the board redraws', () => {
		const say = quipper(() => 0.5);
		const first = say(fall, 7, 4);
		expect(first).toMatchObject({ sq: 'a5', emoji: '😢', key: '7:3', delay: 550 });
		expect(say(fall, 7, 5)).toEqual(first);
	});
	it('waits for a fall to finish, but not for a repair', () => {
		const repair: EventJSON[] = [{ kind: 'moved', from: 'a5', to: 'b6' }, { kind: 'repaired', sq: 'c7' }];
		expect(quipper()(repair, 3, 2)).toMatchObject({ delay: 0 });
	});
	it('says nothing before the moment is revealed', () => {
		expect(quipper()(fall, 7, 3)).toBeNull();
	});
	it('never repeats the last line on the next moment', () => {
		const say = quipper(() => 0);
		const a = say(fall, 7, 4)!.line;
		const b = say(fall, 9, 4)!.line;
		expect(b).not.toBe(a);
	});
});
