// Which sound each moment of a game plays. Pure, so it is tested without
// audio; the pages hand the result to sound.ts.

import { fallsWith, squareIndex } from './board.ts';
import { firstDiceStep, SAVE_MS } from './dice.ts';
import { BURST_HOLD_MS, TEETER_MS } from './feel.ts';
import type { Color, EventJSON, MoveJSON, View } from './game.ts';
import { reducedMotion } from './motion.ts';
import { moveDuration } from './pieces.ts';
import type { Sound } from './sound.ts';

export interface Cue {
	sound: Sound;
	delayMs: number;
}

// The steps that throw dice, and the sound of each throw (dice.ts, stepDice).
const THROWS: Readonly<Record<string, Sound>> = { rolled_pothole: 'die', saving_roll: 'die', target: 'dice' };

/**
 * The sounds for a turn's events [from, to), just revealed, in order.
 * `skipMove` skips the move itself (and its capture) when it already
 * sounded as the player made it. Each throw rattles: one die for the
 * pothole roll and a saving roll, two for the file and rank dice. A
 * re-roll buzzes as its target blinks, and a saved piece chimes as its die
 * lands and it hops out. A new
 * pothole cracks open, and one that closes on its own, its rounds run out,
 * hisses (once, however many close together); one the cap pushes out
 * closes quietly. A piece falling in cracks its hole open (the hole opens
 * in the fall's step, before the server's pothole_opened) and explodes as
 * it drops, after its teeter; the Mamdani gets only the crowd's gasp. A repair sounds with its celebration: once the Mamdani has
 * arrived when its move fixed the hole, or at once when the dice land
 * beside it.
 */
export function revealCues(last: readonly EventJSON[], { from = 0, to = last.length, skipMove = false } = {}): Cue[] {
	const cues: Cue[] = [];
	// Holes close on their own before the roll; during it, only the cap closes one.
	const roll = firstDiceStep(last);
	let hissed = false;
	for (let i = from; i < to; i++) {
		const e = last[i];
		if (e.kind === 'moved' && !skipMove) cues.push({ sound: last[i + 1]?.kind === 'captured' ? 'capture' : 'move', delayMs: 0 });
		const thrown = THROWS[e.kind];
		if (thrown) cues.push({ sound: thrown, delayMs: 0 });
		if (e.kind === 'reroll') cues.push({ sound: 'reroll', delayMs: 0 });
		if (e.kind === 'saving_roll' && e.saved) cues.push({ sound: 'saved', delayMs: SAVE_MS });
		if (e.kind === 'pothole_opened' && fallsWith(last, i) === Infinity) cues.push({ sound: 'pothole', delayMs: 0 });
		if (e.kind === 'pothole_closed' && i < roll && !hissed) {
			hissed = true;
			cues.push({ sound: 'closed', delayMs: 0 });
		}
		if (e.kind === 'repaired') cues.push({ sound: 'repair', delayMs: mamdaniGlide(last.slice(0, i)) });
		if (e.kind === 'fell') {
			const hole = holeWithFall(last, i);
			if (e.piece === 'M') cues.push({ sound: 'mamdani-fell', delayMs: TEETER_MS });
			else {
				if (hole >= 0) cues.push({ sound: 'pothole', delayMs: 0 });
				cues.push({ sound: 'fell', delayMs: TEETER_MS });
			}
		}
	}
	return cues;
}

// The index of the hole the fall at i opens in the same step, or -1
// (board.ts, stageAt).
function holeWithFall(last: readonly EventJSON[], i: number): number {
	let j = i + 1;
	while (last[j]?.kind === 'pothole_closed') j++;
	return last[j]?.kind === 'pothole_opened' && fallsWith(last, j) === i ? j : -1;
}

// How long the Mamdani glides if one of these events moved it: Board waits
// that long before celebrating a repair its move made.
function mamdaniGlide(events: readonly EventJSON[]): number {
	const m = events.find((e) => e.kind === 'moved' && e.piece === 'M');
	return m?.from && m.to && !reducedMotion() ? moveDuration(m.from, m.to) : 0;
}

/**
 * Everything a reveal of view.last's events [from, to) plays. Once the turn
 * has played out: check or checkmate, then, with `end` and when the turn
 * ended the game, the player's end sound, after the checkmate burst.
 */
export function turnCues(view: Pick<View, 'last' | 'check' | 'result' | 'you' | 'board'>, from: number, to: number, { skipMove = false, end = false } = {}): Cue[] {
	const cues = revealCues(view.last, { from, to, skipMove });
	if (to < view.last.length) return cues;
	const last = turnEndCue(view);
	if (last) cues.push({ sound: last, delayMs: 0 });
	const ending = end ? endCue(view) : null;
	if (ending) cues.push({ sound: ending, delayMs: last === 'checkmate' ? BURST_HOLD_MS : 0 });
	return cues;
}

/** The sound once a turn has played out: checkmate, or check. */
export function turnEndCue(view: Pick<View, 'check' | 'result'>): Sound | null {
	if (view.result?.reason === 'checkmate') return 'checkmate';
	return view.check && !view.result ? 'check' : null;
}

/**
 * The sound a player hears as their game ends: a win (flawless when they lost
 * no piece at all), a loss or a draw. Spectators hear none, and an aborted or
 * expired game, which nobody wins, plays nothing.
 */
export function endCue(view: Pick<View, 'result' | 'you' | 'board'>): Sound | null {
	const r = view.result;
	if (!r || view.you === 'spectator') return null;
	if (r.draw) return 'draw';
	if (!r.winner) return null;
	if (r.winner !== view.you) return 'loss';
	return flawless(view.board, view.you) ? 'flawless' : 'victory';
}

/**
 * Whether a side still has all 16 pieces: nothing captured, nothing lost to a
 * pothole. A promotion keeps the count.
 */
export function flawless(board: readonly string[], color: Color): boolean {
	const prefix = color === 'white' ? 'w' : 'b';
	return board.filter((p) => p.startsWith(prefix)).length === 16;
}

/** The sound of a move the player makes on the board: a capture (en passant too) or a move. */
export function moveSound(board: readonly string[], m: MoveJSON): Sound {
	const target = board[squareIndex(m.to)];
	const piece = board[squareIndex(m.from)] ?? '';
	const enPassant = piece.endsWith('P') && m.from[0] !== m.to[0];
	return target || enPassant ? 'capture' : 'move';
}
