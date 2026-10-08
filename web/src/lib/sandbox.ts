// A local game for testing the board UI at /dev/board. Moves follow how
// each piece moves but ignore check, and the dice are scripted. Every turn
// still produces the same events the server sends, so it plays through the
// real board, dice tray and animation code. This is not the rules engine:
// the Go server is the only source of truth for play.

import { HOLE_CAP, HOLE_ROUNDS } from './wire.gen.ts';
import { squareAt, squareIndex } from './board.ts';
import { applyMove } from './pieces.ts';
import { rerollReasons, type Color, type EventJSON, type LogEntry, type MoveJSON, type View } from './game.ts';

/** What the dice do on the next turn. */
export interface RollScript {
	/** The pothole d8: odd means nothing happens. */
	pothole: number;
	/** Squares re-rolled before the final target: a bare square is a king ("kings never fall"). */
	rerolls?: (string | { sq: string; reason: 'king' | 'pothole' })[];
	/** The square the placement dice pick. */
	target?: string;
	/** A saving roll for a piece or the Mamdani on the target: odd saves. */
	save?: number;
}

/** A game in play with an empty board and nothing played yet. */
export function emptyView(): View {
	return {
		code: 'SANDBOX',
		status: 'playing',
		you: 'white',
		board: Array<string>(64).fill(''),
		mamdani: '',
		potholes: [],
		turn: 'white',
		check: false,
		legal: [],
		last: [],
		log: [],
		lost: { white: [], black: [] },
		taken: { white: [], black: [] },
		stats: { savingRolls: 0, saved: 0, repaired: 0, mamdaniFell: false },
		result: null,
		seq: 0,
		clock: { whiteMs: 600_000, blackMs: 600_000, now: 0 },
		online: { white: true, black: true },
		players: { white: '', black: '' },
		rematch: {}
	};
}

function place(v: View, pieces: Record<string, string>): View {
	for (const [s, p] of Object.entries(pieces)) v.board[squareIndex(s)] = p;
	return v;
}

/** The standard setup with the Mamdani on a5. */
export function startView(): View {
	const v = emptyView();
	const back = 'RNBQKBNR';
	for (let f = 0; f < 8; f++) {
		v.board[squareIndex(squareAt(f, 0))] = 'w' + back[f];
		v.board[squareIndex(squareAt(f, 1))] = 'wP';
		v.board[squareIndex(squareAt(f, 6))] = 'bP';
		v.board[squareIndex(squareAt(f, 7))] = 'b' + back[f];
	}
	v.mamdani = 'a5';
	return v;
}

/** Preset positions for testing particular parts of the UI. */
export const positions = {
	start: startView,
	/** Sliders and the Mamdani with potholes in their lines: shows the × marks. */
	blockedLines: (): View => {
		const v = place(emptyView(), { e1: 'wK', a1: 'wR', c1: 'wB', d1: 'wQ', e8: 'bK', h8: 'bR', f8: 'bB' });
		v.mamdani = 'h4';
		v.potholes = [
			{ sq: 'a4', by: 'black', left: 2 },
			{ sq: 'f4', by: 'white', left: 1 }
		];
		return v;
	},
	/** A white pawn on b7 with b8 empty: shows the promotion picker. */
	promotion: (): View => {
		const v = place(emptyView(), { e1: 'wK', b7: 'wP', e8: 'bK', h7: 'bP' });
		v.mamdani = 'a5';
		return v;
	},
	/**
	 * The Mamdani's repairs. Move it c3–e5 to fix the pothole on f6, or roll
	 * "Pothole opens" on d4 (next to it) to see one fixed before it opens.
	 */
	repair: (): View => {
		const v = place(emptyView(), { e1: 'wK', b2: 'wP', g2: 'wP', e8: 'bK', b7: 'bP', g7: 'bP' });
		v.mamdani = 'c3';
		v.potholes = [{ sq: 'f6', by: 'black', left: 3 }];
		return v;
	},
	/** Black has just its king and a rook: press Resignation (Black wins) for the two-pieces line. */
	twoLeft: (): View => place(emptyView(), { e1: 'wK', a1: 'wR', h2: 'wP', e8: 'bK', h8: 'bR' }),
	/** King and rooks on their squares: castle with e1–g1 or e1–c1. */
	castling: (): View => {
		const v = place(emptyView(), { e1: 'wK', a1: 'wR', h1: 'wR', e8: 'bK', a8: 'bR', h8: 'bR' });
		v.mamdani = 'a5';
		return v;
	}
};

function colorOf(piece: string): Color {
	return piece.startsWith('w') ? 'white' : 'black';
}

const steps: Record<string, number[][]> = {
	N: [
		[1, 2],
		[2, 1],
		[2, -1],
		[1, -2],
		[-1, -2],
		[-2, -1],
		[-2, 1],
		[-1, 2]
	],
	K: [
		[1, 0],
		[-1, 0],
		[0, 1],
		[0, -1],
		[1, 1],
		[1, -1],
		[-1, 1],
		[-1, -1]
	]
};
const lines: Record<string, number[][]> = {
	R: steps.K.slice(0, 4),
	B: steps.K.slice(4),
	Q: steps.K,
	M: steps.K
};

/**
 * Moves for the side to move (or both sides when anySide is set), shaped
 * like the real thing so the board looks as it does in a game: sliders and
 * the Mamdani stop at pieces and potholes, knights jump, kings step and
 * castle past clear squares, pawns push and capture diagonally, and pawns
 * on the last rank get the four promotions. Check is ignored.
 */
export function freeMoves(v: View, anySide: boolean): MoveJSON[] {
	const holes = new Set(v.potholes.map((p) => p.sq));
	const at = (f: number, r: number) => {
		const s = squareAt(f, r);
		return s === v.mamdani ? 'M' : v.board[squareIndex(s)];
	};
	const onBoard = (f: number, r: number) => f >= 0 && f < 8 && r >= 0 && r < 8;
	const free = (f: number, r: number) => onBoard(f, r) && !at(f, r) && !holes.has(squareAt(f, r));
	const moves: MoveJSON[] = [];
	const add = (from: string, f: number, r: number, pawn: boolean) => {
		const to = squareAt(f, r);
		if (pawn && (r === 0 || r === 7)) {
			for (const promo of ['q', 'r', 'b', 'n']) moves.push({ from, to, promo });
		} else {
			moves.push({ from, to });
		}
	};

	for (let i = 0; i < 64; i++) {
		const f0 = i % 8;
		const r0 = Math.floor(i / 8);
		const from = squareAt(f0, r0);
		const isMamdani = from === v.mamdani;
		const piece = isMamdani ? 'M' : v.board[i];
		if (!piece || (!isMamdani && !anySide && colorOf(piece) !== v.turn)) continue;
		const kind = isMamdani ? 'M' : piece[1];
		const enemy = (f: number, r: number) => {
			const t = at(f, r);
			return !isMamdani && !!t && t !== 'M' && colorOf(t) !== colorOf(piece);
		};

		if (lines[kind]) {
			for (const [df, dr] of lines[kind]) {
				for (let f = f0 + df, r = r0 + dr; onBoard(f, r); f += df, r += dr) {
					if (holes.has(squareAt(f, r))) break;
					if (at(f, r)) {
						if (enemy(f, r)) add(from, f, r, false);
						break;
					}
					add(from, f, r, false);
				}
			}
		} else if (kind === 'N' || kind === 'K') {
			for (const [df, dr] of steps[kind]) {
				const f = f0 + df;
				const r = r0 + dr;
				if (onBoard(f, r) && !holes.has(squareAt(f, r)) && (!at(f, r) || enemy(f, r))) add(from, f, r, false);
			}
			if (kind === 'K' && f0 === 4 && (r0 === 0 || r0 === 7)) {
				const rook = piece[0] + 'R';
				if (at(7, r0) === rook && free(5, r0) && free(6, r0)) add(from, 6, r0, false);
				if (at(0, r0) === rook && free(1, r0) && free(2, r0) && free(3, r0)) add(from, 2, r0, false);
			}
		} else if (kind === 'P') {
			const dir = piece.startsWith('w') ? 1 : -1;
			if (free(f0, r0 + dir)) {
				add(from, f0, r0 + dir, true);
				if (r0 === (dir === 1 ? 1 : 6) && free(f0, r0 + 2 * dir)) add(from, f0, r0 + 2 * dir, true);
			}
			for (const df of [-1, 1]) {
				if (onBoard(f0 + df, r0 + dir) && enemy(f0 + df, r0 + dir)) add(from, f0 + df, r0 + dir, true);
			}
		}
	}
	return moves;
}

function adjacent(a: string, b: string): boolean {
	const df = Math.abs(a.charCodeAt(0) - b.charCodeAt(0));
	const dr = Math.abs(Number(a[1]) - Number(b[1]));
	return Math.max(df, dr) === 1;
}

/**
 * Plays one free move and a scripted roll, returning the next view with
 * the same events the server would send: moved, captured, pothole_closed,
 * repaired, then the dice.
 */
export function playTurn(prev: View, move: MoveJSON, roll: RollScript): View {
	// A JSON copy also works when prev is a Svelte state proxy.
	const v: View = JSON.parse(JSON.stringify(prev));
	const ev: EventJSON[] = [];
	const isMamdani = move.from === v.mamdani;
	const piece = isMamdani ? 'M' : v.board[squareIndex(move.from)];
	const mover: Color = isMamdani ? v.turn : colorOf(piece);

	// The move.
	ev.push({ kind: 'moved', from: move.from, to: move.to, piece, color: mover, ...(move.promo ? { promo: move.promo } : {}) });
	if (!isMamdani) {
		const captured = v.board[squareIndex(move.to)];
		if (captured) {
			ev.push({ kind: 'captured', sq: move.to, piece: captured });
			v.taken[mover].push(captured);
		}
	}
	Object.assign(v, applyMove(v, move));

	// Count down the mover's potholes (one closes after HOLE_ROUNDS of its
	// roller's moves), then repair any next to the Mamdani.
	for (const h of v.potholes) if (h.by === mover) h.left--;
	for (const p of v.potholes.filter((h) => h.left <= 0)) ev.push({ kind: 'pothole_closed', sq: p.sq, color: p.by });
	v.potholes = v.potholes.filter((h) => h.left > 0);
	const dice: string[] = []; // the move log's text, as the server's describe writes it
	const repairs = v.potholes.filter((h) => v.mamdani && adjacent(h.sq, v.mamdani));
	for (const p of repairs) {
		ev.push({ kind: 'repaired', sq: p.sq });
		v.stats.repaired++;
		dice.push(`repairs ${p.sq}`);
	}
	v.potholes = v.potholes.filter((h) => !repairs.includes(h));

	// The dice.
	dice.push(`d8 ${roll.pothole}`);
	const rollPart = dice.length - 1;
	ev.push({ kind: 'rolled_pothole', roll: roll.pothole, color: mover });
	if (roll.pothole % 2 === 0 && roll.target) {
		for (const r of roll.rerolls ?? []) {
			const { sq: rs, reason } = typeof r === 'string' ? { sq: r, reason: 'king' } : r;
			ev.push({ kind: 'target', sq: rs }, { kind: 'reroll', sq: rs, reason });
		}
		const t = roll.target;
		ev.push({ kind: 'target', sq: t });
		let text = `→ ${t}`;
		const occupant = t === v.mamdani ? 'M' : v.board[squareIndex(t)];
		let opens = true;
		const reroll = occupant?.[1] === 'K' ? 'king' : v.potholes.some((h) => h.sq === t) ? 'pothole' : null;
		if (reroll) {
			// Kings never fall, and a square holds one pothole: the server
			// re-rolls both d8s. A scripted target would land here again, so the
			// sandbox stops at the re-roll.
			ev.push({ kind: 'reroll', sq: t, reason: reroll });
			opens = false;
			text += ` re-roll (${rerollReasons[reroll]})`;
		} else if (v.mamdani && t !== v.mamdani && adjacent(t, v.mamdani)) {
			ev.push({ kind: 'repaired', sq: t });
			v.stats.repaired++;
			opens = false;
			text += ' repaired';
		} else if (occupant) {
			if (roll.save !== undefined) {
				const saved = roll.save % 2 === 1;
				const roller = occupant === 'M' ? mover : colorOf(occupant);
				ev.push({ kind: 'saving_roll', sq: t, piece: occupant, roll: roll.save, saved, color: roller });
				v.stats.savingRolls++;
				text += ` · save ${roll.save} ${saved ? '✓' : '✗'}`;
				if (saved) {
					v.stats.saved++;
					opens = false;
				}
			}
			if (opens) {
				ev.push({ kind: 'fell', sq: t, piece: occupant });
				text += ` · ${occupant} falls`;
				if (occupant === 'M') {
					v.mamdani = '';
					v.stats.mamdaniFell = true;
				} else {
					v.board[squareIndex(t)] = '';
					v.lost[colorOf(occupant)].push(occupant);
				}
			}
		}
		dice[rollPart] += ` ${text}`;
		if (opens) {
			// Past the cap the oldest hole closes as the new one opens.
			if (v.potholes.length >= HOLE_CAP) {
				const oldest = v.potholes.shift();
				if (oldest) {
					ev.push({ kind: 'pothole_closed', sq: oldest.sq, color: oldest.by });
					dice.push(`${oldest.sq} closes`);
				}
			}
			ev.push({ kind: 'pothole_opened', sq: t, color: mover });
			v.potholes.push({ sq: t, by: mover, left: HOLE_ROUNDS });
		}
	}

	v.last = ev;
	// What the log marks, picked out of the events as the server's logEntry does.
	const entry: LogEntry = { san: `${move.from}–${move.to}`, color: mover, piece, dice: dice.join(' · ') };
	for (const e of ev) {
		if (e.kind === 'pothole_opened') entry.opened = e.sq;
		if (e.kind === 'fell' && e.piece) (entry.fell ??= []).push(e.piece);
		if (e.kind === 'repaired') entry.repaired = true;
	}
	v.log.push(entry);
	v.turn = mover === 'white' ? 'black' : 'white';
	v.seq++;
	return v;
}
