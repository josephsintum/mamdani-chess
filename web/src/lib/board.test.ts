import { describe, expect, it } from 'vitest';
import {
	blockedSquares,
	CELEBRATION_MS,
	celebratedSoFar,
	checkSquare,
	diceSteps,
	diceSummary,
	firstDiceStep,
	markCelebrated,
	matedByRoll,
	matedKing,
	pillFor,
	repairsShown,
	squareIndex,
	stageAt
} from './board.ts';
import { moveDuration } from './pieces.ts';
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
		clock: { whiteMs: 600_000, blackMs: 600_000, now: 0 },
		online: { white: true, black: true },
		players: { white: '', black: '' },
		rematch: {},
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
		const v = makeView({ a1: 'wR', h1: 'bN' }, { potholes: [{ sq: 'd1', by: 'black', left: 1 }] });
		expect(blockedSquares(v, 'a1').sort()).toEqual(['e1', 'f1', 'g1', 'h1']);
	});

	it('stops at the mover’s own piece', () => {
		const v = makeView({ a1: 'wR', f1: 'wK' }, { potholes: [{ sq: 'd1', by: 'black', left: 1 }] });
		expect(blockedSquares(v, 'a1')).toEqual(['e1']);
	});

	it('marks nothing when a piece stands before the pothole', () => {
		const v = makeView({ a1: 'wR', b1: 'wN' }, { potholes: [{ sq: 'd1', by: 'black', left: 1 }] });
		expect(blockedSquares(v, 'a1')).toEqual([]);
	});

	it('follows diagonals for bishops and ignores the other lines', () => {
		const v = makeView({ c1: 'wB' }, { potholes: [{ sq: 'e3', by: 'white', left: 1 }, { sq: 'c4', by: 'black', left: 1 }] });
		expect(blockedSquares(v, 'c1').sort()).toEqual(['f4', 'g5', 'h6']);
	});

	it('never marks a capture for the Mamdani', () => {
		const v = makeView({ e8: 'bK' }, { mamdani: 'a4', potholes: [{ sq: 'c6', by: 'white', left: 1 }] });
		expect(blockedSquares(v, 'a4').sort()).toEqual(['d7']);
	});

	it('marks nothing for pieces that jump or step', () => {
		const v = makeView({ b1: 'wN', e1: 'wK' }, { potholes: [{ sq: 'c3', by: 'black', left: 1 }, { sq: 'e2', by: 'black', left: 1 }] });
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
	const final = makeView({ e4: 'wP', e1: 'wK' }, { last: fellOnG8, potholes: [{ sq: 'g8', by: 'white', left: 1 }] });

	it('keeps the fallen piece and hides the new pothole until they are revealed', () => {
		const s = stageAt(final, 1);
		expect(s.board[squareIndex('g8')]).toBe('bN');
		expect(s.potholes).toEqual([]);
		expect(s.target).toBe('');
	});

	it("opens a fall's hole in the same step the piece falls, so it drops into it", () => {
		const s = stageAt(final, 6);
		expect(s.board[squareIndex('g8')]).toBe('');
		expect(s.potholes).toEqual([{ sq: 'g8', by: 'white', left: 1 }]);
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
		expect(s.potholes).toEqual([{ sq: 'g8', by: 'white', left: 1 }]);
		expect(s.target).toBe('');
	});

	it('keeps a hole the cap closes until the new one opens', () => {
		// Five holes were open; White's roll opens a sixth on d4 and the oldest, c4, closes.
		const last: EventJSON[] = [
			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
			{ kind: 'rolled_pothole', roll: 2, color: 'white' },
			{ kind: 'target', sq: 'd4' },
			{ kind: 'pothole_closed', sq: 'c4', color: 'black' },
			{ kind: 'pothole_opened', sq: 'd4', color: 'white' }
		];
		const v = makeView({}, { last, potholes: [{ sq: 'd4', by: 'white', left: 3 }] });
		expect(stageAt(v, 3).potholes).toEqual([{ sq: 'c4', by: 'black', left: 1 }]);
		expect(stageAt(v, 5).potholes).toEqual([{ sq: 'd4', by: 'white', left: 3 }]);
	});

	it('closes a hole on its schedule before the roll at once', () => {
		const last: EventJSON[] = [
			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
			{ kind: 'pothole_closed', sq: 'c4', color: 'white' },
			{ kind: 'rolled_pothole', roll: 1, color: 'white' }
		];
		expect(stageAt(makeView({}, { last }), 1).potholes).toEqual([]);
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
		const v = makeView({}, { last, mamdani: '', potholes: [{ sq: 'a5', by: 'white', left: 1 }] });
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

describe('diceSteps for the cap', () => {
	it('says when the cap closes the oldest hole, and how long the new one lasts', () => {
		const last: EventJSON[] = [
			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
			{ kind: 'pothole_closed', sq: 'h3', color: 'white' },
			{ kind: 'rolled_pothole', roll: 2, color: 'white' },
			{ kind: 'target', sq: 'd4' },
			{ kind: 'pothole_closed', sq: 'c4', color: 'black' },
			{ kind: 'pothole_opened', sq: 'd4', color: 'white' }
		];
		const steps = diceSteps(makeView({}, { last }), last.length);
		expect(steps.map((s) => [s.title, s.detail])).toEqual([
			['Even. A pothole opens', 'd8 rolled 2'],
			['Square d4', 'File 4 = d, rank 4. Empty square'],
			['At most 5 potholes', 'The oldest, on c4, closes'],
			['Pothole opens on d4', 'It closes after 3 of White’s moves']
		]);
		const sum = diceSummary(makeView({}, { last }), last.length);
		expect(sum.chips.map((c) => c.text)).toEqual(['2', 'd4', 'opens', 'c4 closes']);
	});
});

describe('matedByRoll', () => {
	const mate = { winner: 'white' as const, draw: false, reason: 'checkmate' };
	it('is true when the game ended in checkmate after a roll', () => {
		const last: EventJSON[] = [
			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
			{ kind: 'rolled_pothole', roll: 2, color: 'white' }
		];
		expect(matedByRoll({ result: mate, last })).toBe(true);
	});
	it('is false for a mating move, which ends the game before any roll', () => {
		expect(matedByRoll({ result: mate, last: [{ kind: 'moved', from: 'd1', to: 'h5', piece: 'wQ', color: 'white' }] })).toBe(false);
		expect(matedByRoll({ result: { draw: true, reason: 'stalemate' }, last: [{ kind: 'rolled_pothole', roll: 2 }] })).toBe(false);
	});
});

describe('repairsShown', () => {
	const last: EventJSON[] = [
		{ kind: 'moved', from: 'c3', to: 'e5', piece: 'M', color: 'white' },
		{ kind: 'repaired', sq: 'f6' },
		{ kind: 'rolled_pothole', roll: 2, color: 'white' },
		{ kind: 'target', sq: 'd4' },
		{ kind: 'repaired', sq: 'd4' }
	];
	const v = makeView({}, { last, seq: 7 });
	it('lists repairs revealed so far: the move’s fixes an open hole, the roll’s fixes one before it opens', () => {
		expect(repairsShown(v, 2)).toEqual([{ sq: 'f6', key: '7:1', hole: true }]);
		expect(repairsShown(v, 5)).toEqual([
			{ sq: 'f6', key: '7:1', hole: true },
			{ sq: 'd4', key: '7:4', hole: false }
		]);
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

describe('diceSummary', () => {
	const sum = (last: EventJSON[], shown = last.length, extra: Partial<View> = {}, pieces: Record<string, string> = { g8: 'bN', e1: 'wK' }) => {
		const d = diceSummary(makeView(pieces, { last, ...extra }), shown);
		return { who: d.who, chips: d.chips.map((c) => `${c.text}:${c.kind}`), line: d.line, tone: d.tone };
	};
	const moved = { kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' } as const;

	it('has nothing to show before the first roll', () => {
		expect(sum([])).toEqual({ who: null, chips: [], line: 'The dice roll after every move.', tone: 'muted' });
	});

	it('shows the d8 rolling until it lands', () => {
		expect(sum(fellOnG8, 1)).toEqual({ who: 'white', chips: ['?:pending'], line: 'Rolling the d8…', tone: 'muted' });
	});

	it('says an odd roll does nothing', () => {
		expect(sum([moved, { kind: 'rolled_pothole', roll: 3, color: 'white' }])).toEqual({
			who: 'white', chips: ['3:die'], line: 'Odd: nothing happens', tone: 'muted'
		});
	});

	it('waits for the square after an even roll', () => {
		expect(sum(fellOnG8, 2)).toEqual({
			who: 'white', chips: ['2:die', '?:pending'], line: 'Even: a pothole opens · finding its square…', tone: 'normal'
		});
	});

	it('marks re-rolled squares and keeps going', () => {
		expect(sum(fellOnG8, 4).chips).toEqual(['2:die', 'e1↻:plain', '?:pending']);
		expect(sum(fellOnG8, 4).line).toBe('Re-roll: kings never fall');
	});

	it('folds two or more re-rolls into one chip, so a long turn fits a phone', () => {
		const twice: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 4, color: 'white' },
			{ kind: 'target', sq: 'e1' }, { kind: 'reroll', sq: 'e1', reason: 'king' }, { kind: 'target', sq: 'e8' }, { kind: 'reroll', sq: 'e8', reason: 'king' },
			{ kind: 'target', sq: 'g8' }, { kind: 'saving_roll', sq: 'g8', piece: 'bN', roll: 6, saved: false, color: 'black' }, { kind: 'fell', sq: 'g8', piece: 'bN' }];
		expect(sum(twice).chips).toEqual(['4:die', '↻2:plain', 'g8:square', 'save 6:bad']);
		expect(sum(twice, 6).chips).toEqual(['4:die', '↻2:plain', '?:pending']);
	});

	it('names a piece that falls, and the square', () => {
		expect(sum(fellOnG8)).toEqual({
			who: 'white', chips: ['2:die', 'e1↻:plain', 'g8:square', 'falls:bad'], line: 'Black knight falls into g8', tone: 'hazard'
		});
	});

	it('opens a pothole on an empty square', () => {
		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'd4' }, { kind: 'pothole_opened', sq: 'd4', color: 'white' }];
		expect(sum(last)).toEqual({ who: 'white', chips: ['4:die', 'd4:square', 'opens:bad'], line: 'Pothole on d4 · closes after 3 of White’s moves', tone: 'hazard' });
	});

	it('repairs a pothole next to the Mamdani at once', () => {
		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'b4' }, { kind: 'repaired', sq: 'b4' }];
		expect(sum(last)).toEqual({ who: 'white', chips: ['4:die', 'b4:square', 'repaired:good'], line: 'The Mamdani repairs b4 at once', tone: 'good' });
	});

	it('shows a saving roll, saved or lost', () => {
		const roll = (save: number, saved: boolean): EventJSON[] => [
			moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'd5' },
			{ kind: 'saving_roll', sq: 'd5', piece: 'bN', roll: save, saved, color: 'black' },
			...(saved ? [] : ([{ kind: 'fell', sq: 'd5', piece: 'bN' }, { kind: 'pothole_opened', sq: 'd5', color: 'white' }] as EventJSON[]))
		];
		expect(sum(roll(3, true))).toEqual({ who: 'white', chips: ['4:die', 'd5:square', 'save 3:good'], line: 'Black knight saved', tone: 'good' });
		expect(sum(roll(6, false))).toEqual({ who: 'white', chips: ['4:die', 'd5:square', 'save 6:bad'], line: 'Black knight falls into d5', tone: 'hazard' });
	});

	it('drops the Mamdani', () => {
		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 8, color: 'white' }, { kind: 'target', sq: 'a5' },
			{ kind: 'saving_roll', sq: 'a5', piece: 'M', roll: 2, saved: false, color: 'white' }, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5', color: 'white' }];
		expect(sum(last).line).toBe('The Mamdani falls into a5');
	});

	it('says when no square could take a pothole', () => {
		expect(sum([moved, { kind: 'rolled_pothole', roll: 2, color: 'white' }, { kind: 'no_pothole' }])).toEqual({
			who: 'white', chips: ['2:die', 'none:plain'], line: 'No pothole: no square could take one', tone: 'muted'
		});
	});

	it('says a game-ending move has no roll', () => {
		const end = makeView({}, { last: [{ kind: 'moved', from: 'd8', to: 'h4', piece: 'bQ', color: 'black' }], status: 'over', result: { winner: 'black', draw: false, reason: 'checkmate' } });
		expect(diceSummary(end, 1)).toEqual({ who: 'black', chips: [], line: 'No roll: the game is over', tone: 'muted' });
	});
});

describe('pillFor', () => {
	const v = (extra: Partial<View>) => makeView({}, { status: 'playing', turn: 'white', you: 'white', ...extra });

	it('tells you it is your move, and the other bar whose move it is', () => {
		expect(pillFor(v({}), 'white', false)).toEqual({ text: 'Your move', tone: 'turn' });
		expect(pillFor(v({ you: 'black' }), 'white', false)).toEqual({ text: 'To move', tone: 'turn' });
		expect(pillFor(v({}), 'black', false)).toEqual({ text: '', tone: 'turn' });
	});

	it('warns the side in check', () => {
		expect(pillFor(v({ check: true }), 'white', false)).toEqual({ text: 'In check', tone: 'check' });
	});

	it('shows nothing while the dice play out', () => {
		expect(pillFor(v({}), 'white', true).text).toBe('');
	});

	it('marks Black waiting to join', () => {
		expect(pillFor(v({ status: 'waiting' }), 'black', false)).toEqual({ text: 'Waiting…', tone: 'muted' });
		expect(pillFor(v({ status: 'waiting' }), 'white', false).text).toBe('');
	});

	it('marks the checkmated side at the end', () => {
		const over = v({ status: 'over', result: { winner: 'black', draw: false, reason: 'checkmate' } });
		expect(pillFor(over, 'white', false)).toEqual({ text: 'Checkmated', tone: 'check' });
		expect(pillFor(over, 'black', false).text).toBe('');
		expect(pillFor(v({ status: 'over', result: { winner: 'black', draw: false, reason: 'resignation' } }), 'white', false).text).toBe('');
	});
});

describe('the exposes re-roll', () => {
	it('says what it guards against without claiming a fall, since the cap can be the cause', () => {
		const last: EventJSON[] = [
			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
			{ kind: 'rolled_pothole', roll: 2, color: 'white' },
			{ kind: 'target', sq: 'd4' },
			{ kind: 'reroll', sq: 'd4', reason: 'exposes' }
		];
		expect(diceSteps(makeView({}, { last }), last.length).at(-1)?.detail).toBe('Would expose the roller’s king');
	});
});

describe('repair celebrations', () => {
	it('last long enough for the 👍 after the Mamdani’s longest glide', () => {
		// The 👍 starts 700 ms after the glide and runs 1.1 s (Board.svelte).
		expect(CELEBRATION_MS).toBeGreaterThanOrEqual(moveDuration('a1', 'h8') + 700 + 1100);
	});

	it('are remembered once played, so a board mounted later skips them', () => {
		markCelebrated('9:1');
		expect(celebratedSoFar().has('9:1')).toBe(true);
		expect(celebratedSoFar().has('9:4')).toBe(false);
	});

	it('remember only the latest 64', () => {
		for (let i = 0; i < 70; i++) markCelebrated(`k${i}`);
		const seen = celebratedSoFar();
		expect(seen.has('k69')).toBe(true);
		expect(seen.has('k5')).toBe(false);
		expect(seen.size).toBeLessThanOrEqual(64);
	});
});

describe('matedKing', () => {
	it("finds the mated king's square", () => {
		const v = makeView({ e8: 'bK', e1: 'wK' }, { status: 'over', result: { winner: 'white', draw: false, reason: 'checkmate' } });
		expect(matedKing(v)).toBe('e8');
	});
	it('is empty for any other ending', () => {
		const v = makeView({ e8: 'bK', e1: 'wK' }, { status: 'over', result: { winner: 'white', draw: false, reason: 'resignation' } });
		expect(matedKing(v)).toBe('');
		expect(matedKing(makeView({ e8: 'bK' }))).toBe('');
	});
});
