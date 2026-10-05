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
	exposes: 'The fall would expose the roller’s king',
	checkmate: 'The result would checkmate: a roll never wins on its own'
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
					detail: `It closes when ${colorTitle(e.color)} finishes their next move`,
					dice: [],
					tone: 'hazard'
				});
				break;
			case 'repaired':
				if (rolled) steps.push({ title: 'Repaired at once', detail: `${e.sq} is next to the Mamdani`, dice: [], tone: 'good' });
				break;
			case 'no_pothole':
				steps.push({ title: 'No pothole this turn', detail: '64 re-rolls found no valid square', dice: [], tone: 'muted' });
				break;
		}
	});
	return steps;
}
