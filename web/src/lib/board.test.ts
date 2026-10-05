import { describe, expect, it } from 'vitest';
import { blockedSquares, checkSquare, diceSteps, firstDiceStep, squareIndex, stageAt } from './board.ts';
import type { EventJSON, View } from './game.ts';

/** A view with the given pieces ({"e1": "wK"}), potholes and Mamdani square. */
function makeView(pieces: Record<string, string>, extra: Partial<View> = {}): View {
	const board = Array<string>(64).fill('');
	for (const [sq, piece] of Object.entries(pieces)) board[squareIndex(sq)] = piece;
	return {
		code: 'TEST01',
		status: 'playing',
		you: 'white',
		board,
		mamdani: '',
		potholes: [],
		turn: 'white',
		check: false,
		legal: [],
		last: [],
		log: [],
		lost: { white: [], black: [] },
		stats: { savingRolls: 0, saved: 0, repaired: 0, mamdaniFell: false },
		result: null,
		seq: 1,
		...extra
	};
}

describe('squareIndex', () => {
	it('maps a1 to 0 and h8 to 63', () => {
		expect(squareIndex('a1')).toBe(0);
		expect(squareIndex('e4')).toBe(28);
		expect(squareIndex('h8')).toBe(63);
	});
});

describe('blockedSquares', () => {
	it('marks the squares a rook could reach past a pothole', () => {
		const v = makeView({ a1: 'wR', h1: 'bN' }, { potholes: [{ sq: 'd1', by: 'black' }] });
		expect(blockedSquares(v, 'a1').sort()).toEqual(['e1', 'f1', 'g1', 'h1']);
	});

	it('stops at the mover’s own piece', () => {
		const v = makeView({ a1: 'wR', f1: 'wK' }, { potholes: [{ sq: 'd1', by: 'black' }] });
		expect(blockedSquares(v, 'a1')).toEqual(['e1']);
	});

	it('marks nothing when a piece stands before the pothole', () => {
		const v = makeView({ a1: 'wR', b1: 'wN' }, { potholes: [{ sq: 'd1', by: 'black' }] });
		expect(blockedSquares(v, 'a1')).toEqual([]);
	});

	it('follows diagonals for bishops and ignores the other lines', () => {
		const v = makeView({ c1: 'wB' }, { potholes: [{ sq: 'e3', by: 'white' }, { sq: 'c4', by: 'black' }] });
		expect(blockedSquares(v, 'c1').sort()).toEqual(['f4', 'g5', 'h6']);
	});

	it('never marks a capture for the Mamdani', () => {
		const v = makeView({ e8: 'bK' }, { mamdani: 'a4', potholes: [{ sq: 'c6', by: 'white' }] });
		expect(blockedSquares(v, 'a4').sort()).toEqual(['d7']);
	});

	it('marks nothing for pieces that jump or step', () => {
		const v = makeView({ b1: 'wN', e1: 'wK' }, { potholes: [{ sq: 'c3', by: 'black' }, { sq: 'e2', by: 'black' }] });
		expect(blockedSquares(v, 'b1')).toEqual([]);
		expect(blockedSquares(v, 'e1')).toEqual([]);
	});
});

const fellOnG8: EventJSON[] = [
	{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
	{ kind: 'rolled_pothole', roll: 2, color: 'white' },
	{ kind: 'target', sq: 'e1' },
	{ kind: 'reroll', sq: 'e1', reason: 'king' },
	{ kind: 'target', sq: 'g8' },
	{ kind: 'fell', sq: 'g8', piece: 'bN' },
	{ kind: 'pothole_opened', sq: 'g8', color: 'white' }
];

describe('firstDiceStep', () => {
	it('starts at the pothole roll', () => {
		expect(firstDiceStep(fellOnG8)).toBe(1);
	});
	it('is the end when no dice were rolled', () => {
		expect(firstDiceStep([{ kind: 'moved', from: 'e2', to: 'e4' }])).toBe(1);
	});
});

describe('stageAt', () => {
	// The final position: the knight is gone and g8 holds White's pothole.
	const final = makeView({ e4: 'wP', e1: 'wK' }, { last: fellOnG8, potholes: [{ sq: 'g8', by: 'white' }] });

	it('keeps the fallen piece and hides the new pothole until they are revealed', () => {
		const s = stageAt(final, 1);
		expect(s.board[squareIndex('g8')]).toBe('bN');
		expect(s.potholes).toEqual([]);
		expect(s.target).toBe('');
	});

	it('rings the latest target while the dice play out', () => {
		expect(stageAt(final, 3).target).toBe('e1');
		expect(stageAt(final, 5).target).toBe('g8');
	});

	it('keeps a fall out of the lost list until it is revealed', () => {
		const v = { ...final, lost: { white: [], black: ['bP', 'bN'] } };
		expect(stageAt(v, 1).lost).toEqual({ white: [], black: ['bP'] });
		expect(stageAt(v, fellOnG8.length).lost).toEqual({ white: [], black: ['bP', 'bN'] });
	});

	it('shows the final position once every step is revealed', () => {
		const s = stageAt(final, fellOnG8.length);
		expect(s.board[squareIndex('g8')]).toBe('');
		expect(s.potholes).toEqual([{ sq: 'g8', by: 'white' }]);
		expect(s.target).toBe('');
	});

	it('puts a fallen Mamdani back until its fall is revealed', () => {
		const last: EventJSON[] = [
			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
			{ kind: 'rolled_pothole', roll: 4, color: 'white' },
			{ kind: 'target', sq: 'a5' },
			{ kind: 'saving_roll', sq: 'a5', piece: 'M', roll: 2, saved: false, color: 'white' },
			{ kind: 'fell', sq: 'a5', piece: 'M' },
			{ kind: 'pothole_opened', sq: 'a5', color: 'white' }
		];
		const v = makeView({}, { last, mamdani: '', potholes: [{ sq: 'a5', by: 'white' }] });
		expect(stageAt(v, 4).mamdani).toBe('a5');
		expect(stageAt(v, 6).mamdani).toBe('');
	});
});

describe('diceSteps', () => {
	it('describes each revealed step of the roll', () => {
		const v = makeView({ e4: 'wP', e1: 'wK' }, { last: fellOnG8 });
		const steps = diceSteps(v, fellOnG8.length);
		expect(steps.map((s) => s.title)).toEqual([
			'Even. A pothole opens',
			'Square e1',
			'Re-roll',
			'Square g8',
			'Black knight falls in',
			'Pothole opens on g8'
		]);
		expect(steps[0].dice).toEqual([2]);
		expect(steps[1].dice).toEqual([5, 1]);
		expect(steps[1].detail).toBe('File 5 = e, rank 1. White king is there');
		expect(steps[2].detail).toBe('Kings never fall');
		expect(steps[3].detail).toBe('File 7 = g, rank 8. Black knight is there');
	});

	it('only shows steps that have been revealed', () => {
		const v = makeView({}, { last: fellOnG8 });
		expect(diceSteps(v, 3).map((s) => s.title)).toEqual(['Even. A pothole opens', 'Square e1']);
	});

	it('describes a saving roll and an odd roll', () => {
		const saved: EventJSON[] = [
			{ kind: 'moved', from: 'e7', to: 'e5', piece: 'bP', color: 'black' },
			{ kind: 'rolled_pothole', roll: 4, color: 'black' },
			{ kind: 'target', sq: 'd2' },
			{ kind: 'saving_roll', sq: 'd2', piece: 'wP', roll: 5, saved: true, color: 'white' }
		];
		const v = makeView({ d2: 'wP' }, { last: saved, mamdani: 'a5' });
		const steps = diceSteps(v, saved.length);
		expect(steps[2]).toMatchObject({ title: 'Odd. White pawn is saved', dice: [5], tone: 'good' });
		expect(steps[2].detail).toBe('The Mamdani on a5 has a clear line to d2.');
		const odd = makeView({}, { last: [saved[0], { kind: 'rolled_pothole', roll: 7, color: 'black' }] });
		expect(diceSteps(odd, 2)[0]).toMatchObject({ title: 'Odd. No pothole', dice: [7], tone: 'muted' });
	});
});

describe('checkSquare', () => {
	const v = makeView({ e1: 'wK', e8: 'bK', e5: 'bQ' }, { check: true, turn: 'white' });
	const stage = stageAt(v, v.last.length);

	it('is the checked king’s square', () => {
		expect(checkSquare(v, stage, { animating: false, guessing: false })).toBe('e1');
	});

	it('is empty when nobody is in check', () => {
		expect(checkSquare({ ...v, check: false }, stage, { animating: false, guessing: false })).toBe('');
	});

	it('waits for the dice, and for the server while your move is in flight', () => {
		expect(checkSquare(v, stage, { animating: true, guessing: false })).toBe('');
		expect(checkSquare(v, stage, { animating: false, guessing: true })).toBe('');
	});
});
