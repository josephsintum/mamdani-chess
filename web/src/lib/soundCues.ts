// Which sound each moment of a game plays. Pure, so it is tested without
// audio; the pages hand the result to sound.ts.

import { fallsWith } from './board.ts';
import { SAVE_MS, withRoll } from './dice.ts';
import { BURST_HOLD_MS, TEETER_MS } from './feel.ts';
import type { Color, EventJSON, MoveJSON, View } from './game.ts';
import { reducedMotion } from './motion.ts';
import { capturedSquare, moveDuration } from './pieces.ts';
import type { Sound } from './sound.ts';

export interface Cue {
	sound: Sound;
	delayMs: number;
}

// The steps that throw dice, and the sound of each throw (dice.ts, stepDice).
const THROWS: Readonly<Record<string, Sound>> = { rolled_pothole: 'die', saving_roll: 'die', target: 'dice' };

/**
 * The sounds for a turn's events [from, to), just revealed, in order.
 * `skipMove`: the move (and its capture) already sounded as the player made
 * it. A saved piece chimes as its die lands. Holes closing on their own hiss
 * once per turn; the cap's close is silent. A fall cracks its hole open at
 * once (the hole opens in the fall's step, before the server's
 * pothole_opened) and lands after its teeter; the Mamdani gets only the
 * crowd's gasp. A repair by the Mamdani's move waits for its glide.
 */
export function revealCues(last: readonly EventJSON[], { from = 0, to = last.length, skipMove = false } = {}): Cue[] {
	const cues: Cue[] = [];
	// With reduced motion a piece drops at once, without its teeter.
	const teeter = reducedMotion() ? 0 : TEETER_MS;
	let hissed = false;
	// A resignation or a timeout mid-roll empties last while the roll's timer
	// still reveals one more step.
	for (let i = from; i < Math.min(to, last.length); i++) {
		const e = last[i];
		if (e.kind === 'moved' && !skipMove) cues.push({ sound: last[i + 1]?.kind === 'captured' ? 'capture' : 'move', delayMs: 0 });
		const thrown = THROWS[e.kind];
		if (thrown) cues.push({ sound: thrown, delayMs: 0 });
		if (e.kind === 'reroll') cues.push({ sound: 'reroll', delayMs: 0 });
		if (e.kind === 'saving_roll' && e.saved) cues.push({ sound: 'saved', delayMs: SAVE_MS });
		if (e.kind === 'pothole_opened' && fallsWith(last, i) === Infinity) cues.push({ sound: 'pothole', delayMs: 0 });
		if (e.kind === 'pothole_closed' && !withRoll(last, i) && !hissed) {
			hissed = true;
			cues.push({ sound: 'closed', delayMs: 0 });
		}
		if (e.kind === 'repaired') cues.push({ sound: 'repair', delayMs: withRoll(last, i) ? 0 : mamdaniGlide(last.slice(0, i)) });
		if (e.kind === 'fell') {
			if (e.piece === 'M') cues.push({ sound: 'mamdani-fell', delayMs: teeter });
			else {
				if (opensWithFall(last, i)) cues.push({ sound: 'pothole', delayMs: 0 });
				cues.push({ sound: 'fell', delayMs: teeter });
			}
		}
	}
	return cues;
}

// Whether the fall at i opens its hole in the same step (board.ts, stageAt).
function opensWithFall(last: readonly EventJSON[], i: number): boolean {
	let j = i + 1;
	while (last[j]?.kind === 'pothole_closed') j++;
	return last[j]?.kind === 'pothole_opened' && fallsWith(last, j) === i;
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
 * ended the game, the player's end sound, after the checkmate burst. A
 * turn `jumped` to its end (it arrived in a hidden tab, or the player came
 * back mid-roll) plays only its move and that ending, not every die at once.
 */
export function turnCues(view: Pick<View, 'last' | 'check' | 'result' | 'you' | 'board'>, from: number, to: number, { skipMove = false, end = false, jumped = false } = {}): Cue[] {
	const revealed = revealCues(view.last, { from, to, skipMove });
	const cues = jumped ? revealed.filter((c) => c.sound === 'move' || c.sound === 'capture') : revealed;
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
	return capturedSquare(board, m) ? 'capture' : 'move';
}
