import { describe, expect, it } from 'vitest';
import { squareIndex } from './board.ts';
import { freeMoves, playTurn, positions, startView, type RollScript } from './sandbox.ts';

const odd: RollScript = { pothole: 1 };

describe('startView', () => {
	it('is the standard setup with the Mamdani on a5', () => {
		const v = startView();
		expect(v.board[squareIndex('e1')]).toBe('wK');
		expect(v.board[squareIndex('d8')]).toBe('bQ');
		expect(v.mamdani).toBe('a5');
		expect(v.turn).toBe('white');
		expect(v.status).toBe('playing');
		expect(v.seq).toBe(0);
	});
});

describe('freeMoves', () => {
	const tos = (v: ReturnType<typeof startView>, from: string, anySide = false) =>
		freeMoves(v, anySide)
			.filter((m) => m.from === from)
			.map((m) => m.to)
			.sort();

	it('moves pieces the way they move, ignoring check', () => {
		const v = startView();
		expect(tos(v, 'e2')).toEqual(['e3', 'e4']);
		expect(tos(v, 'g1')).toEqual(['f3', 'h3']);
		expect(tos(v, 'f1')).toEqual([]); // hemmed in by its own pawns
		expect(freeMoves(v, false).some((m) => m.from === 'e7')).toBe(false);
	});

	it('can move either side', () => {
		expect(tos(startView(), 'e7', true)).toEqual(['e5', 'e6']);
	});

	it('stops sliders at potholes, so the board can mark what they block', () => {
		const v = positions.blockedLines();
		expect(tos(v, 'a1')).toEqual(['a2', 'a3', 'b1']); // a4 is a pothole
		expect(tos(v, 'c1')).toEqual(['a3', 'b2', 'd2', 'e3']); // f4 is a pothole
	});

	it('moves the Mamdani along queen lines to empty squares only', () => {
		const v = startView();
		expect(tos(v, 'a5')).toEqual(['a3', 'a4', 'a6', 'b4', 'b5', 'b6', 'c3', 'c5', 'd5', 'e5', 'f5', 'g5', 'h5']);
	});

	it('lets pawns capture diagonally and offers all four promotions', () => {
		const v = positions.promotion();
		const promos = freeMoves(v, false).filter((m) => m.from === 'b7' && m.to === 'b8');
		expect(promos.map((m) => m.promo).sort()).toEqual(['b', 'n', 'q', 'r']);
	});

	it('offers castling when the squares between king and rook are clear', () => {
		expect(tos(positions.castling(), 'e1')).toEqual(['c1', 'd1', 'd2', 'e2', 'f1', 'f2', 'g1']);
	});
});

describe('playTurn', () => {
	it('moves, passes the turn and logs an odd roll', () => {
		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, odd);
		expect(v.board[squareIndex('e4')]).toBe('wP');
		expect(v.board[squareIndex('e2')]).toBe('');
		expect(v.turn).toBe('black');
		expect(v.seq).toBe(1);
		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole']);
		expect(v.log).toEqual([{ san: 'e2–e4', color: 'white', dice: 'd8 1' }]);
	});

	it('opens a pothole on an empty square and closes it after the roller’s next move', () => {
		let v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'd4' });
		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white' }]);
		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'pothole_opened']);
		v = playTurn(v, { from: 'e7', to: 'e5' }, odd);
		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white' }]);
		v = playTurn(v, { from: 'g1', to: 'f3' }, odd);
		expect(v.potholes).toEqual([]);
		expect(v.last[1]).toEqual({ kind: 'pothole_closed', sq: 'd4' });
	});

	it('drops a piece without a saving roll and counts it as lost', () => {
		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 4, target: 'g8' });
		expect(v.board[squareIndex('g8')]).toBe('');
		expect(v.lost.black).toEqual(['bN']);
		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'fell', 'pothole_opened']);
	});

	it('saves a piece on an odd saving roll and drops it on an even one', () => {
		const saved = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'd2', save: 5 });
		expect(saved.board[squareIndex('d2')]).toBe('wP');
		expect(saved.potholes).toEqual([]);
		expect(saved.stats).toMatchObject({ savingRolls: 1, saved: 1 });
		const lost = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'd2', save: 6 });
		expect(lost.board[squareIndex('d2')]).toBe('');
		expect(lost.last.at(-3)).toMatchObject({ kind: 'saving_roll', saved: false, color: 'white' });
	});

	it('drops the Mamdani on a failed saving roll', () => {
		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'a5', save: 2 });
		expect(v.mamdani).toBe('');
		expect(v.stats.mamdaniFell).toBe(true);
	});

	it('repairs a pothole next to the Mamdani at once', () => {
		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'b4' });
		expect(v.potholes).toEqual([]);
		expect(v.last.at(-1)).toEqual({ kind: 'repaired', sq: 'b4' });
		expect(v.stats.repaired).toBe(1);
	});

	it('never opens a second pothole on a square that already has one (re-roll, as the rules say)', () => {
		// The sandbox's default script: every even roll targets d4.
		let v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'd4' });
		v = playTurn(v, { from: 'e7', to: 'e5' }, { pothole: 2, target: 'd4' });
		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white' }]);
		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'reroll']);
		expect(v.last.at(-1)).toEqual({ kind: 'reroll', sq: 'd4', reason: 'pothole' });
	});

	it('plays re-rolls before the final target', () => {
		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, rerolls: ['e1'], target: 'h6' });
		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'reroll', 'target', 'pothole_opened']);
		expect(v.last[3]).toEqual({ kind: 'reroll', sq: 'e1', reason: 'king' });
	});

	it('captures, castles and promotes', () => {
		let v = positions.castling();
		v = playTurn(v, { from: 'e1', to: 'g1' }, odd);
		expect(v.board[squareIndex('g1')]).toBe('wK');
		expect(v.board[squareIndex('f1')]).toBe('wR');
		expect(v.board[squareIndex('h1')]).toBe('');

		v = playTurn(startView(), { from: 'd1', to: 'd7' }, odd); // playTurn itself doesn't check moves
		expect(v.last[1]).toEqual({ kind: 'captured', sq: 'd7', piece: 'bP' });

		v = playTurn(positions.promotion(), { from: 'b7', to: 'b8', promo: 'n' }, odd);
		expect(v.board[squareIndex('b8')]).toBe('wN');
		expect(v.last[0]).toMatchObject({ kind: 'moved', promo: 'n' });
	});

	it('lets the side not on turn move, then passes the turn to the other side', () => {
		const v = playTurn(startView(), { from: 'e7', to: 'e5' }, odd);
		expect(v.board[squareIndex('e5')]).toBe('bP');
		expect(v.turn).toBe('white');
	});
});
