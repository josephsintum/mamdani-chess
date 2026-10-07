import { describe, expect, it } from 'vitest';
import { squareIndex } from './board.ts';
import { applyMove, moveDuration, reconcile, settlesGuess, type PieceRef } from './pieces.ts';
import { boardOf } from './test-boards.ts';

describe('moveDuration', () => {
	it('uses One Million Chessboards’ timing: 350ms plus up to 400ms by distance', () => {
		expect(moveDuration('e2', 'e2')).toBe(0);
		expect(moveDuration('e2', 'e3')).toBeCloseTo(376.7, 1);
		expect(moveDuration('a1', 'h8')).toBeCloseTo(350 + (Math.hypot(7, 7) / 15) * 400, 1);
	});
});

describe('applyMove', () => {
	it('moves and captures', () => {
		const s = applyMove({ board: boardOf({ e4: 'wP', d5: 'bP' }), mamdani: 'a5' }, { from: 'e4', to: 'd5' });
		expect(s.board[squareIndex('d5')]).toBe('wP');
		expect(s.board[squareIndex('e4')]).toBe('');
		expect(s.mamdani).toBe('a5');
	});

	it('moves the rook when castling', () => {
		const s = applyMove({ board: boardOf({ e1: 'wK', h1: 'wR', a1: 'wR' }), mamdani: '' }, { from: 'e1', to: 'c1' });
		expect(s.board[squareIndex('c1')]).toBe('wK');
		expect(s.board[squareIndex('d1')]).toBe('wR');
		expect(s.board[squareIndex('a1')]).toBe('');
		expect(s.board[squareIndex('h1')]).toBe('wR');
	});

	it('removes the pawn taken en passant', () => {
		const s = applyMove({ board: boardOf({ d5: 'wP', e5: 'bP' }), mamdani: '' }, { from: 'd5', to: 'e6' });
		expect(s.board[squareIndex('e6')]).toBe('wP');
		expect(s.board[squareIndex('e5')]).toBe('');
	});

	it('promotes and moves the Mamdani', () => {
		expect(applyMove({ board: boardOf({ b7: 'wP' }), mamdani: '' }, { from: 'b7', to: 'b8', promo: 'n' }).board[squareIndex('b8')]).toBe('wN');
		expect(applyMove({ board: boardOf({}), mamdani: 'a5' }, { from: 'a5', to: 'h5' }).mamdani).toBe('h5');
	});
});

describe('reconcile', () => {
	let n = 0;
	const id = () => ++n;
	const start = (pieces: Record<string, string>, mamdani = '') => reconcile([], boardOf(pieces), mamdani, id);
	const byCode = (ps: PieceRef[], code: string) => ps.filter((p) => p.code === code);

	it('keeps each piece’s id when it moves', () => {
		const before = start({ e2: 'wP', d2: 'wP', e1: 'wK' });
		const pawn = before.find((p) => p.sq === 'e2')!;
		const after = reconcile(before, boardOf({ e4: 'wP', d2: 'wP', e1: 'wK' }), '', id, { from: 'e2', to: 'e4' });
		expect(after.find((p) => p.sq === 'e4')?.id).toBe(pawn.id);
		expect(after).toHaveLength(3);
	});

	it('drops a captured or fallen piece and keeps the rest', () => {
		const before = start({ e4: 'wP', d5: 'bP', g8: 'bN' });
		const after = reconcile(before, boardOf({ d5: 'wP', g8: 'bN' }), '', id, { from: 'e4', to: 'd5' });
		expect(after.map((p) => p.code).sort()).toEqual(['bN', 'wP']);
		expect(after.find((p) => p.code === 'wP')?.id).toBe(before.find((p) => p.code === 'wP')?.id);
	});

	it('tracks the rook when castling', () => {
		const before = start({ e1: 'wK', h1: 'wR', a1: 'wR' });
		const hRook = before.find((p) => p.sq === 'h1')!;
		const after = reconcile(before, boardOf({ g1: 'wK', f1: 'wR', a1: 'wR' }), '', id, { from: 'e1', to: 'g1' });
		expect(after.find((p) => p.sq === 'f1')?.id).toBe(hRook.id);
		expect(byCode(after, 'wR').find((p) => p.sq === 'a1')?.id).toBe(before.find((p) => p.sq === 'a1')?.id);
	});

	it('keeps the pawn’s id through a promotion', () => {
		const before = start({ b7: 'wP', e1: 'wK' });
		const pawn = before.find((p) => p.code === 'wP')!;
		const after = reconcile(before, boardOf({ b8: 'wQ', e1: 'wK' }), '', id, { from: 'b7', to: 'b8' });
		expect(after.find((p) => p.sq === 'b8')).toMatchObject({ id: pawn.id, code: 'wQ' });
	});

	it('treats the Mamdani as a piece', () => {
		const before = start({ e1: 'wK' }, 'a5');
		const m = before.find((p) => p.code === 'M')!;
		const after = reconcile(before, boardOf({ e1: 'wK' }), 'h5', id, { from: 'a5', to: 'h5' });
		expect(after.find((p) => p.code === 'M')).toMatchObject({ id: m.id, sq: 'h5' });
		expect(reconcile(after, boardOf({ e1: 'wK' }), '', id).some((p) => p.code === 'M')).toBe(false);
	});

	it('matches the nearest piece when there is no hint (reconnects, rollbacks)', () => {
		const before = start({ a2: 'wP', h2: 'wP' });
		const hPawn = before.find((p) => p.sq === 'h2')!;
		const after = reconcile(before, boardOf({ a2: 'wP', h4: 'wP' }), '', id);
		expect(after.find((p) => p.sq === 'h4')?.id).toBe(hPawn.id);
	});

	it('gives new pieces new ids', () => {
		const before = start({ e1: 'wK' });
		const after = reconcile(before, boardOf({ e1: 'wK', d4: 'bQ' }), '', id);
		expect(new Set(after.map((p) => p.id)).size).toBe(2);
	});
});

describe('settlesGuess', () => {
	const guess = { seq: 7, move: { from: 'e2', to: 'e4' } };

	it('keeps your instant move through an update that is not a new turn (clocks, reactions)', () => {
		expect(settlesGuess(guess, { seq: 7 })).toBe(false);
	});

	it('drops it once the server moves on to a new turn', () => {
		expect(settlesGuess(guess, { seq: 8 })).toBe(true);
	});

	it('has nothing to settle without a guess', () => {
		expect(settlesGuess(null, { seq: 8 })).toBe(true);
	});
});
