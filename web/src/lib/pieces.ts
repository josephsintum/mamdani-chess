// Pieces with identities, so the board can glide each one from where it was
// to where it is, the way One Million Chessboards does. Pure: no DOM.

import { squareIndex } from './board.ts';
import type { MoveJSON } from './game.ts';

/** One piece on the board: a stable id, what it is ("wN", "M"), where it is. */
export interface PieceRef {
	id: number;
	code: string;
	sq: string;
}

const files = 'abcdefgh';

const kinds: Record<string, string> = { K: 'king', Q: 'queen', R: 'rook', B: 'bishop', N: 'knight', P: 'pawn' };

/** A piece's name for screen readers: "black knight", or "the Mamdani". */
export function pieceName(code: string): string {
	if (code === 'M') return 'the Mamdani';
	return `${code[0] === 'w' ? 'white' : 'black'} ${kinds[code[1]]}`;
}

function squareName(index: number): string {
	return files[index % 8] + (Math.floor(index / 8) + 1);
}

function distance(a: string, b: string): number {
	return Math.hypot(a.charCodeAt(0) - b.charCodeAt(0), Number(a[1]) - Number(b[1]));
}

/**
 * How long a piece takes to glide from a to b, in ms: One Million
 * Chessboards' formula, 350ms plus up to 400ms more as the distance grows
 * to 15 squares. 0 when it doesn't move.
 */
export function moveDuration(a: string, b: string): number {
	if (a === b) return 0;
	return 350 + Math.min(distance(a, b) / 15, 1) * 400;
}

/**
 * Plays a move's mechanics on a board: the piece moves, a capture goes,
 * castling brings the rook, en passant removes the pawn behind, a pawn
 * promotes, or the Mamdani moves. No legality check: callers pass moves the
 * server listed as legal, or the sandbox's moves.
 */
export function applyMove(state: { board: string[]; mamdani: string }, move: MoveJSON): { board: string[]; mamdani: string } {
	const board = [...state.board];
	if (move.from === state.mamdani) return { board, mamdani: move.to };
	const piece = board[squareIndex(move.from)];
	const df = move.to.charCodeAt(0) - move.from.charCodeAt(0);
	// En passant: a pawn moving diagonally onto an empty square.
	if (piece[1] === 'P' && df !== 0 && !board[squareIndex(move.to)]) {
		board[squareIndex(move.to[0] + move.from[1])] = '';
	}
	board[squareIndex(move.from)] = '';
	board[squareIndex(move.to)] = move.promo ? piece[0] + move.promo.toUpperCase() : piece;
	if (piece[1] === 'K' && Math.abs(df) === 2) {
		const rank = move.from[1];
		const [rookFrom, rookTo] = df > 0 ? ['h' + rank, 'f' + rank] : ['a' + rank, 'd' + rank];
		board[squareIndex(rookTo)] = board[squareIndex(rookFrom)];
		board[squareIndex(rookFrom)] = '';
	}
	return { board, mamdani: state.mamdani };
}

/**
 * Whether a view from the server settles your instant move (the guess made
 * at seq). Only a new turn does: an update for the same turn (a clock, a
 * reaction) keeps the guess, or the piece would glide back and forth.
 */
export function settlesGuess(guess: { seq: number } | null, next: { seq: number }): boolean {
	return guess === null || next.seq !== guess.seq;
}

/**
 * Works out which piece is which in a new position, so ids carry over:
 * 1. a piece of the same kind still on its square keeps its id;
 * 2. the hinted move (the last move, if known) carries its piece over, even
 *    if it promoted;
 * 3. other pieces match the nearest unmatched piece of the same kind (this
 *    handles castling rooks, reconnects and rollbacks);
 * 4. anything left is new and gets a new id.
 * Pieces in prev with no match have left the board (captured or fallen).
 */
export function reconcile(
	prev: PieceRef[],
	board: string[],
	mamdani: string,
	nextId: () => number,
	hint?: { from: string; to: string }
): PieceRef[] {
	const targets: { code: string; sq: string }[] = [];
	board.forEach((code, i) => {
		if (code) targets.push({ code, sq: squareName(i) });
	});
	if (mamdani) targets.push({ code: 'M', sq: mamdani });

	const free = new Set(prev);
	const out: PieceRef[] = [];
	const unmatched: { code: string; sq: string }[] = [];
	const take = (p: PieceRef, t: { code: string; sq: string }) => {
		free.delete(p);
		out.push({ id: p.id, code: t.code, sq: t.sq });
	};

	// 1. Unmoved pieces.
	for (const t of targets) {
		const same = [...free].find((p) => p.code === t.code && p.sq === t.sq);
		if (same) take(same, t);
		else unmatched.push(t);
	}

	// 2. The hinted move, promotion included (same colour, or the Mamdani).
	let rest = unmatched;
	if (hint) {
		const t = rest.find((u) => u.sq === hint.to);
		const p = [...free].find((q) => q.sq === hint.from);
		if (t && p && (t.code === p.code || (t.code[0] === p.code[0] && p.code !== 'M'))) {
			take(p, t);
			rest = rest.filter((u) => u !== t);
		}
	}

	// 3. Nearest of the same kind, closest pairs first.
	const pairs: { p: PieceRef; t: { code: string; sq: string }; d: number }[] = [];
	for (const t of rest) {
		for (const p of free) if (p.code === t.code) pairs.push({ p, t, d: distance(p.sq, t.sq) });
	}
	pairs.sort((a, b) => a.d - b.d);
	const placed = new Set<{ code: string; sq: string }>();
	for (const { p, t } of pairs) {
		if (free.has(p) && !placed.has(t)) {
			take(p, t);
			placed.add(t);
		}
	}

	// 4. New pieces.
	for (const t of rest) if (!placed.has(t) && !out.some((o) => o.sq === t.sq)) out.push({ id: nextId(), ...t });

	return out.sort((a, b) => a.id - b.id);
}
