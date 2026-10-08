import { describe, expect, it } from 'vitest';
import { BURST_HOLD_MS, TEETER_MS } from './feel.ts';
import type { EventJSON } from './game.ts';
import { SAVE_MS } from './dice.ts';
import { moveDuration } from './pieces.ts';
import { endCue, flawless, moveSound, revealCues, turnCues, turnEndCue } from './soundCues.ts';
import { boardOf, viewOf } from './test-boards.ts';

const moved: EventJSON = { kind: 'moved', from: 'e2', to: 'e4', color: 'white' };
const captured: EventJSON = { kind: 'captured', sq: 'e4', piece: 'bP' };
const roll: EventJSON = { kind: 'rolled_pothole', roll: 3 };

// A side with all 16 pieces, on any squares.
const full = (prefix: 'w' | 'b') => Object.fromEntries(['P', 'P', 'P', 'P', 'P', 'P', 'P', 'P', 'R', 'N', 'B', 'Q', 'K', 'B', 'N', 'R'].map((p, i) => [`${'abcdefgh'[i % 8]}${prefix === 'w' ? 1 + Math.floor(i / 8) : 8 - Math.floor(i / 8)}`, prefix + p]));

describe('revealCues', () => {
	it('plays a move, or a capture when one follows it', () => {
		expect(revealCues([moved])).toEqual([{ sound: 'move', delayMs: 0 }]);
		expect(revealCues([moved, captured])).toEqual([{ sound: 'capture', delayMs: 0 }]);
	});

	it('skips a move that sounded as the player made it', () => {
		expect(revealCues([moved, captured], { skipMove: true })).toEqual([]);
	});

	it('builds as the Mamdani repairs: once its move arrives, at once beside the dice', () => {
		const mamdani: EventJSON = { kind: 'moved', from: 'a5', to: 'c3', piece: 'M', color: 'white' };
		expect(revealCues([mamdani, { kind: 'repaired', sq: 'd4' }], { skipMove: true })).toEqual([{ sound: 'repair', delayMs: moveDuration('a5', 'c3') }]);
		expect(revealCues([roll, { kind: 'target', sq: 'b4' }, { kind: 'repaired', sq: 'b4' }])).toEqual([
			{ sound: 'die', delayMs: 0 },
			{ sound: 'dice', delayMs: 0 },
			{ sound: 'repair', delayMs: 0 }
		]);
	});

	it('explodes as a piece drops into a pothole, and only gasps for the Mamdani', () => {
		expect(revealCues([{ kind: 'fell', sq: 'd4', piece: 'wN' }])).toEqual([{ sound: 'fell', delayMs: TEETER_MS }]);
		expect(revealCues([{ kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }])).toEqual([{ sound: 'mamdani-fell', delayMs: TEETER_MS }]);
	});

	it('buzzes for a re-roll, and chimes when a saving roll saves the piece', () => {
		const die = { sound: 'die', delayMs: 0 };
		expect(revealCues([{ kind: 'target', sq: 'e1' }, { kind: 'reroll', sq: 'e1', reason: 'king' }])).toEqual([{ sound: 'dice', delayMs: 0 }, { sound: 'reroll', delayMs: 0 }]);
		expect(revealCues([{ kind: 'saving_roll', sq: 'b8', roll: 3, saved: true }])).toEqual([die, { sound: 'saved', delayMs: SAVE_MS }]);
		expect(revealCues([{ kind: 'saving_roll', sq: 'b8', roll: 6, saved: false }])).toEqual([die]);
	});

	it('hisses once as holes close on their own, and not when the cap pushes one out', () => {
		const hiss = { sound: 'closed', delayMs: 0 };
		const closes: EventJSON[] = [moved, { kind: 'pothole_closed', sq: 'a3', color: 'white' }, { kind: 'pothole_closed', sq: 'h6', color: 'white' }, roll];
		expect(revealCues(closes, { from: 0, to: 3, skipMove: true })).toEqual([hiss]);
		const cap: EventJSON[] = [roll, { kind: 'target', sq: 'c3' }, { kind: 'pothole_closed', sq: 'a3', color: 'black' }, { kind: 'pothole_opened', sq: 'c3' }];
		expect(revealCues(cap, { from: 2, to: 3 })).toEqual([]);
		expect(revealCues(cap, { from: 3, to: 4 })).toEqual([{ sound: 'pothole', delayMs: 0 }]);
	});

	it('cracks a pothole open as it appears: with the fall, when a piece falls in', () => {
		const crack = { sound: 'pothole', delayMs: 0 };
		const boom = { sound: 'fell', delayMs: TEETER_MS };
		const turn: EventJSON[] = [roll, { kind: 'target', sq: 'd4' }, { kind: 'fell', sq: 'd4', piece: 'wN' }, { kind: 'pothole_closed', sq: 'a3', color: 'black' }, { kind: 'pothole_opened', sq: 'd4' }];
		expect(revealCues(turn, { from: 2, to: 3 })).toEqual([crack, boom]);
		expect(revealCues(turn, { from: 3, to: 5 })).toEqual([]);
	});

	it('rattles one die or two for each throw, and nothing else of the roll', () => {
		const die = { sound: 'die', delayMs: 0 };
		const dice = { sound: 'dice', delayMs: 0 };
		expect(revealCues([roll, { kind: 'target', sq: 'c3' }, { kind: 'pothole_opened', sq: 'c3' }])).toEqual([die, dice, { sound: 'pothole', delayMs: 0 }]);
		expect(revealCues([{ kind: 'saving_roll', roll: 4 }])).toEqual([die]);
		expect(revealCues([{ kind: 'rolled_pothole', roll: 3 }, { kind: 'no_pothole' }])).toEqual([die]);
	});
});

describe('turnEndCue', () => {
	it('plays check, or checkmate when the game ended so', () => {
		expect(turnEndCue({ check: true, result: null })).toBe('check');
		expect(turnEndCue({ check: false, result: null })).toBeNull();
		expect(turnEndCue({ check: true, result: { winner: 'white', draw: false, reason: 'checkmate' } })).toBe('checkmate');
		expect(turnEndCue({ check: false, result: { draw: true, reason: 'stalemate' } })).toBeNull();
	});
});

describe('endCue', () => {
	const won = { winner: 'white' as const, draw: false, reason: 'checkmate' };

	it('tells the winner, the loser and a draw apart', () => {
		expect(endCue(viewOf({ you: 'white', result: won }))).toBe('victory');
		expect(endCue(viewOf({ you: 'black', result: won }))).toBe('loss');
		expect(endCue(viewOf({ you: 'white', result: { draw: true, reason: 'stalemate' } }))).toBe('draw');
	});

	it('is flawless when the winner lost no piece', () => {
		expect(endCue(viewOf({ you: 'white', result: won, board: boardOf(full('w')) }))).toBe('flawless');
	});

	it('plays nothing for spectators, or a game nobody won', () => {
		expect(endCue(viewOf({ you: 'spectator', result: won }))).toBeNull();
		expect(endCue(viewOf({ you: 'white', result: { draw: false, reason: 'aborted' } }))).toBeNull();
		expect(endCue(viewOf({ you: 'white' }))).toBeNull();
	});
});

describe('flawless', () => {
	it('counts every piece of the side, promoted or not', () => {
		const board = boardOf(full('w'));
		expect(flawless(board, 'white')).toBe(true);
		expect(flawless(board, 'black')).toBe(false);
		board[board.indexOf('wP')] = 'wQ'; // a promotion
		expect(flawless(board, 'white')).toBe(true);
		board[board.indexOf('wN')] = ''; // a piece lost
		expect(flawless(board, 'white')).toBe(false);
	});
});

describe('turnCues', () => {
	const mate = viewOf({
		you: 'white',
		last: [moved, captured],
		check: true,
		result: { winner: 'white', draw: false, reason: 'checkmate' }
	});

	it('adds check once the turn has played out', () => {
		const v = viewOf({ last: [moved, roll], check: true });
		expect(turnCues(v, 0, 1)).toEqual([{ sound: 'move', delayMs: 0 }]);
		expect(turnCues(v, 1, 2)).toEqual([
			{ sound: 'die', delayMs: 0 },
			{ sound: 'check', delayMs: 0 }
		]);
	});

	it('plays checkmate, then the end sound after the burst', () => {
		expect(turnCues(mate, 0, 2, { end: true })).toEqual([
			{ sound: 'capture', delayMs: 0 },
			{ sound: 'checkmate', delayMs: 0 },
			{ sound: 'victory', delayMs: BURST_HOLD_MS }
		]);
	});

	it('leaves the end sound out unless asked (practice)', () => {
		expect(turnCues(mate, 0, 2, { skipMove: true })).toEqual([{ sound: 'checkmate', delayMs: 0 }]);
	});
});

describe('moveSound', () => {
	const board = boardOf({ e4: 'wP', d5: 'bP', a2: 'wP', h5: 'bP' });

	it('hears a capture, en passant too', () => {
		expect(moveSound(board, { from: 'e4', to: 'd5' })).toBe('capture');
		expect(moveSound(boardOf({ e5: 'wP', d5: 'bP' }), { from: 'e5', to: 'd6' })).toBe('capture');
	});

	it('hears a quiet move, and a Mamdani move (it never captures)', () => {
		expect(moveSound(board, { from: 'a2', to: 'a4' })).toBe('move');
		expect(moveSound(board, { from: 'a5', to: 'b6' })).toBe('move');
	});
});
