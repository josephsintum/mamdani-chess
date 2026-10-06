// Pure board logic for the game page: blocked lines, the board while the
// dice play out, and the dice tray's lines. No DOM, so it is unit-tested.

import { pieceName, type Color, type EventJSON, type View } from './game.ts';

export function squareIndex(name: string): number {
	return (Number(name[1]) - 1) * 8 + (name.charCodeAt(0) - 97);
}

function squareAt(file: number, rank: number): string {
	return String.fromCharCode(97 + file) + (rank + 1);
}

const rookDirs = [
	[1, 0],
	[-1, 0],
	[0, 1],
	[0, -1]
];
const bishopDirs = [
	[1, 1],
	[1, -1],
	[-1, 1],
	[-1, -1]
];
const slides: Record<string, number[][]> = {
	B: bishopDirs,
	R: rookDirs,
	Q: [...rookDirs, ...bishopDirs],
	M: [...rookDirs, ...bishopDirs]
};

/**
 * The squares the piece on `from` could reach if the potholes in its way
 * were not there. Sliders (bishop, rook, queen) and the Mamdani only: they
 * can't cross a pothole. Knights jump and kings and pawns step, so a pothole
 * never blocks a line for them.
 */
export function blockedSquares(view: Pick<View, 'board' | 'potholes' | 'mamdani'>, from: string): string[] {
	const isMamdani = from === view.mamdani;
	const piece = isMamdani ? 'M' : view.board[squareIndex(from)];
	const dirs = slides[isMamdani ? 'M' : (piece?.[1] ?? '')];
	if (!piece || !dirs) return [];
	const holes = new Set(view.potholes.map((p) => p.sq));
	const blocked: string[] = [];
	const f0 = from.charCodeAt(0) - 97;
	const r0 = Number(from[1]) - 1;
	for (const [df, dr] of dirs) {
		let pastHole = false;
		for (let f = f0 + df, r = r0 + dr; f >= 0 && f < 8 && r >= 0 && r < 8; f += df, r += dr) {
			const sq = squareAt(f, r);
			if (holes.has(sq)) {
				pastHole = true;
				continue;
			}
			const occupant = sq === view.mamdani ? 'M' : view.board[squareIndex(sq)];
			if (occupant) {
				// It could have captured this piece, but never the Mamdani, and the
				// Mamdani never captures.
				if (pastHole && !isMamdani && occupant !== 'M' && occupant[0] !== piece[0]) blocked.push(sq);
				break;
			}
			if (pastHole) blocked.push(sq);
		}
	}
	return blocked;
}

/** How many of its roller's moves a pothole lasts, and the most open at once (rules/position.go). */
export const HOLE_ROUNDS = 3;
export const HOLE_CAP = 5;

/** Index of the first dice event (the pothole roll); the end if none was rolled. */
export function firstDiceStep(last: EventJSON[]): number {
	const i = last.findIndex((e) => e.kind === 'rolled_pothole');
	return i < 0 ? last.length : i;
}

/**
 * The square of the king in check, to glow, or "". It waits while the dice
 * play out, and while your own move is in flight: until the server answers,
 * view.check still describes the position before it.
 */
export function checkSquare(view: View, stage: Stage, { animating, guessing }: { animating: boolean; guessing: boolean }): string {
	if (!view.check || animating || guessing) return '';
	const i = stage.board.indexOf(view.turn === 'white' ? 'wK' : 'bK');
	return i < 0 ? '' : 'abcdefgh'[i % 8] + (Math.floor(i / 8) + 1);
}

export interface Stage {
	board: string[];
	potholes: View['potholes'];
	mamdani: string;
	target: string; // square ringed while the dice play out, or ""
	lost: View['lost']; // pieces lost to potholes, without falls not yet revealed
}

/**
 * The board after the first `shown` events of the latest turn have been
 * revealed. The view holds the final position, so effects not yet revealed
 * are undone: a fallen piece is put back and a new pothole is hidden.
 */
export function stageAt(view: View, shown: number): Stage {
	const board = [...view.board];
	let potholes = [...view.potholes];
	let mamdani = view.mamdani;
	let target = '';
	const lost = { white: [...view.lost.white], black: [...view.lost.black] };
	const roll = firstDiceStep(view.last);
	view.last.forEach((e, i) => {
		const revealed = i < shown;
		if (e.kind === 'target' && revealed) target = e.sq ?? '';
		if (e.kind === 'fell' && !revealed && e.sq) {
			if (e.piece === 'M') {
				mamdani = e.sq;
			} else {
				board[squareIndex(e.sq)] = e.piece ?? '';
				// The fall is the newest entry in its side's list.
				const list = e.piece?.[0] === 'w' ? lost.white : lost.black;
				const at = list.lastIndexOf(e.piece ?? '');
				if (at >= 0) list.splice(at, 1);
			}
		}
		if (e.kind === 'pothole_opened' && !revealed) potholes = potholes.filter((p) => p.sq !== e.sq);
		// The cap closes the oldest hole as a new one opens: until then it stays.
		if (e.kind === 'pothole_closed' && !revealed && i > roll && e.sq && e.color) potholes = [...potholes, { sq: e.sq, by: e.color, left: 1 }];
	});
	return { board, potholes, mamdani, target: shown < view.last.length ? target : '', lost };
}

export interface DiceStep {
	title: string;
	detail: string;
	dice: number[]; // d8 values shown as diamonds
	tone: 'normal' | 'muted' | 'good' | 'hazard';
}

const rerollReasons: Record<string, string> = {
	king: 'Kings never fall',
	pothole: 'Already a pothole',
	exposes: 'Would expose the roller’s king'
};

function capitalize(s: string): string {
	return s.charAt(0).toUpperCase() + s.slice(1);
}

function colorTitle(c: Color | undefined): string {
	return c === 'black' ? 'Black' : 'White';
}

/** The dice tray's lines for the first `shown` events of the latest turn. */
export function diceSteps(view: View, shown: number): DiceStep[] {
	const steps: DiceStep[] = [];
	let rolled = false;
	view.last.slice(0, shown).forEach((e, i) => {
		switch (e.kind) {
			case 'rolled_pothole': {
				rolled = true;
				const even = (e.roll ?? 1) % 2 === 0;
				steps.push({
					title: even ? 'Even. A pothole opens' : 'Odd. No pothole',
					detail: `d8 rolled ${e.roll}`,
					dice: [e.roll ?? 0],
					tone: even ? 'normal' : 'muted'
				});
				break;
			}
			case 'target': {
				const sq = e.sq ?? '';
				const file = sq.charCodeAt(0) - 96;
				const rank = Number(sq[1]);
				const before = stageAt(view, i);
				const occupant = sq === before.mamdani ? 'M' : before.board[squareIndex(sq)];
				const there = occupant ? `${capitalize(pieceName(occupant))} is there` : 'Empty square';
				steps.push({ title: `Square ${sq}`, detail: `File ${file} = ${sq[0]}, rank ${rank}. ${there}`, dice: [file, rank], tone: 'normal' });
				break;
			}
			case 'reroll':
				steps.push({ title: 'Re-roll', detail: rerollReasons[e.reason ?? ''] ?? e.reason ?? '', dice: [], tone: 'muted' });
				break;
			case 'saving_roll': {
				const who = capitalize(pieceName(e.piece));
				steps.push({
					title: e.saved ? `Odd. ${who} is saved` : `Even. No save`,
					detail:
						e.piece === 'M'
							? 'The Mamdani always gets a saving roll.'
							: `The Mamdani on ${view.mamdani || stageAt(view, i).mamdani} has a clear line to ${e.sq}.`,
					dice: [e.roll ?? 0],
					tone: e.saved ? 'good' : 'hazard'
				});
				break;
			}
			case 'fell':
				steps.push({ title: `${capitalize(pieceName(e.piece))} falls in`, detail: `Lost on ${e.sq}`, dice: [], tone: 'hazard' });
				break;
			case 'pothole_opened':
				steps.push({
					title: `Pothole opens on ${e.sq}`,
					detail: `It closes after ${HOLE_ROUNDS} of ${colorTitle(e.color)}’s moves`,
					dice: [],
					tone: 'hazard'
				});
				break;
			case 'repaired':
				if (rolled) steps.push({ title: 'Repaired at once', detail: `${e.sq} is next to the Mamdani`, dice: [], tone: 'good' });
				break;
			case 'pothole_closed':
				// Before the roll a hole closes on its own schedule (the tray notes it);
				// during the roll only the cap closes one.
				if (rolled) steps.push({ title: `At most ${HOLE_CAP} potholes`, detail: `The oldest, on ${e.sq}, closes`, dice: [], tone: 'muted' });
				break;
			case 'no_pothole':
				steps.push({ title: 'No pothole this turn', detail: '64 re-rolls found no valid square', dice: [], tone: 'muted' });
				break;
		}
	});
	return steps;
}

/** One chip in the phone's dice card: a die, a square, or an outcome. */
export interface DiceChip {
	text: string;
	kind: 'die' | 'square' | 'pending' | 'good' | 'bad' | 'plain';
}

/** The phone's one-line version of a turn's dice (the steps are in the moves sheet). */
export interface DiceSummary {
	who: Color | null; // whose roll; null before the first one
	chips: DiceChip[]; // as far as the dice have played
	line: string;
	tone: 'normal' | 'good' | 'hazard' | 'muted';
}

/**
 * Sums up the turn's dice as far as they have played (shown events of
 * view.last), for the phone's dice card: chips that fill in step by step
 * and one outcome line.
 */
export function diceSummary(view: View, shown: number): DiceSummary {
	const mover = view.last.find((e) => e.kind === 'moved')?.color ?? null;
	if (!view.last.some((e) => e.kind === 'rolled_pothole')) {
		if (mover && view.result) return { who: mover, chips: [], line: 'No roll: the game is over', tone: 'muted' };
		return { who: null, chips: [], line: 'The dice roll after every move.', tone: 'muted' };
	}
	const chips: DiceChip[] = [];
	let line = 'Rolling the d8…';
	let tone: DiceSummary['tone'] = 'muted';
	let pending = true; // the next die or square is still to come
	let square = '';
	let rolled = false;
	let capped = ''; // a hole the cap closes, told after the new one opens
	for (const e of view.last.slice(0, shown)) {
		switch (e.kind) {
			case 'rolled_pothole': {
				rolled = true;
				const even = (e.roll ?? 1) % 2 === 0;
				chips.push({ text: String(e.roll), kind: 'die' });
				[line, tone, pending] = even ? ['Even: a pothole opens · finding its square…', 'normal', true] : ['Odd: nothing happens', 'muted', false];
				break;
			}
			case 'target':
				square = e.sq ?? '';
				chips.push({ text: square, kind: 'square' });
				pending = false;
				break;
			case 'reroll': {
				// The re-rolled square becomes "e1↻"; a second re-roll folds both into "↻2".
				chips.pop();
				const prev = chips[chips.length - 1];
				const folded = prev?.text.endsWith('↻') ? 2 : prev?.text.startsWith('↻') ? Number(prev.text.slice(1)) + 1 : 0;
				if (folded) chips[chips.length - 1] = { text: `↻${folded}`, kind: 'plain' };
				else chips.push({ text: `${e.sq}↻`, kind: 'plain' });
				const why = rerollReasons[e.reason ?? ''] ?? e.reason ?? '';
				[line, tone, pending] = [`Re-roll: ${why.charAt(0).toLowerCase()}${why.slice(1)}`, 'muted', true];
				break;
			}
			case 'saving_roll':
				chips.push({ text: `save ${e.roll}`, kind: e.saved ? 'good' : 'bad' });
				if (e.saved) [line, tone] = [`${capitalize(pieceName(e.piece))} saved`, 'good'];
				break;
			case 'fell':
				if (chips[chips.length - 1]?.kind === 'square') chips.push({ text: 'falls', kind: 'bad' });
				[line, tone] = [`${capitalize(pieceName(e.piece))} falls into ${e.sq}`, 'hazard'];
				break;
			case 'pothole_opened':
				// After a fall the fall is the news; on an empty square the hole is.
				if (chips[chips.length - 1]?.kind === 'square') {
					chips.push({ text: 'opens', kind: 'bad' });
					[line, tone] = [`Pothole on ${e.sq} · closes after ${HOLE_ROUNDS} of ${colorTitle(e.color)}’s moves`, 'hazard'];
				}
				if (capped) chips.push({ text: `${capped} closes`, kind: 'plain' });
				break;
			case 'repaired':
				if (square === e.sq) {
					chips.push({ text: 'repaired', kind: 'good' });
					[line, tone] = [`The Mamdani repairs ${e.sq} at once`, 'good'];
				}
				break;
			case 'pothole_closed':
				// Only the cap closes a hole during the roll.
				if (rolled) capped = e.sq ?? '';
				break;
			case 'no_pothole':
				chips.push({ text: 'none', kind: 'plain' });
				[line, tone, pending] = ['No pothole: no square could take one', 'muted', false];
				break;
		}
	}
	if (pending) chips.push({ text: '?', kind: 'pending' });
	const roll = view.last.find((e) => e.kind === 'rolled_pothole');
	return { who: roll?.color ?? mover, chips, line, tone };
}

/** Whether the dice, not the move, delivered checkmate: a mating move ends the game before any roll. */
export function matedByRoll(view: Pick<View, 'result' | 'last'>): boolean {
	return view.result?.reason === 'checkmate' && view.last.some((e) => e.kind === 'rolled_pothole');
}

/**
 * The repairs to celebrate on the board: repaired events revealed so far.
 * One made by the Mamdani's move (before the roll) fixes an open hole; one
 * during the roll fixes a hole before it opens. Each key plays once.
 */
export function repairsShown(view: View, shown: number): { sq: string; key: string; hole: boolean }[] {
	const roll = firstDiceStep(view.last);
	return view.last.flatMap((e, i) => (e.kind === 'repaired' && e.sq && i < shown ? [{ sq: e.sq, key: `${view.seq}:${i}`, hole: i < roll }] : []));
}

/**
 * How long a repair celebration stays on the board, in ms: long enough for
 * the 👍 (it starts 700 ms after the Mamdani's glide and runs 1.1 s) after
 * the longest glide there is, corner to corner, 614 ms.
 */
export const CELEBRATION_MS = 2500;

const celebrated: string[] = [];

/**
 * Remembers that a repair (by its `repairsShown` key) has been celebrated,
 * so a board mounted later, as when the game page switches layouts at
 * 640 px, doesn't play it again. Keeps the latest 64.
 */
export function markCelebrated(key: string) {
	if (celebrated.includes(key)) return;
	celebrated.push(key);
	if (celebrated.length > 64) celebrated.shift();
}

/** The repairs celebrated so far, as a set to check keys against. */
export function celebratedSoFar(): Set<string> {
	return new Set(celebrated);
}

/** The pill on a player's bar: whose move, check, waiting, checkmated. */
export function pillFor(view: View, color: Color, animating: boolean): { text: string; tone: 'turn' | 'check' | 'muted' } {
	if (view.status === 'waiting') return color === 'black' ? { text: 'Waiting…', tone: 'muted' } : { text: '', tone: 'muted' };
	if (view.result) {
		const mated = view.result.reason === 'checkmate' && !view.result.draw && view.result.winner !== color;
		return mated ? { text: 'Checkmated', tone: 'check' } : { text: '', tone: 'turn' };
	}
	if (animating || view.turn !== color) return { text: '', tone: 'turn' };
	if (view.check) return { text: 'In check', tone: 'check' };
	return { text: view.you === color ? 'Your move' : 'To move', tone: 'turn' };
}
