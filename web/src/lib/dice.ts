// The dice's pacing and faces (spec: board feel, section 9). Each step of a
// roll plays for its time from dice-timing.json, the table the server's
// clock pause uses too. A die tumbles through faces from a seeded schedule,
// so the tray's dice and the board's scan, drawn apart, show the same faces
// at the same moment.

import TIMING from './dice-timing.json';
import { pieceName, type EventJSON, type View } from './game.ts';

/** How long the move shows before the dice roll. */
export const MOVE_MS = TIMING.moveMs;
/** A thrown die flies in, bounces twice and settles. */
export const THROW_MS = 450;
/** The file die tumbles this long; the rank die is thrown RANK_DELAY_MS later and tumbles RANK_MS. */
export const FILE_MS = 700;
export const RANK_DELAY_MS = 150;
export const RANK_MS = 850;
/** The scan runs until the rank die lands. */
export const SCAN_MS = RANK_DELAY_MS + RANK_MS;
/** A saving die lands, then the pill says whether the piece is saved. */
export const SAVE_MS = THROW_MS;
/** A re-roll's target blinks twice, one blink each half of its step. */
export const BLINK_MS = TIMING.playMs.reroll / 2;

/** Index of the pothole roll in a turn's events; the end if none was rolled (as board.ts's firstDiceStep). */
function firstDiceStep(last: EventJSON[]): number {
	const i = last.findIndex((e) => e.kind === 'rolled_pothole');
	return i < 0 ? last.length : i;
}

/** How long the browser plays one dice step, from the shared table. */
export function playMs(e: EventJSON): number {
	const t = TIMING.playMs as Record<string, number>;
	if (e.kind === 'rolled_pothole') return (e.roll ?? 1) % 2 === 1 ? t.rolled_pothole_odd : t.rolled_pothole_even;
	return t[e.kind] ?? t.other;
}

/** The Animator's wait before showing the next step: the move first, then the step just shown. */
export function dicePace(shownLast: EventJSON | undefined): number {
	return shownLast ? playMs(shownLast) : MOVE_MS;
}

/** How long a whole turn takes to play, from the move to the last step's end (0 with no roll). */
export function turnMs(last: EventJSON[]): number {
	const first = firstDiceStep(last);
	if (first === last.length) return 0;
	return MOVE_MS + last.slice(first).reduce((sum, e) => sum + playMs(e), 0);
}

/**
 * A die's faces from `seed`: random faces that flick fast, then slower, and
 * never show `final` until the last frame, at `ms`.
 */
export function tumbleFaces(final: number, ms: number, seed: number): { at: number; face: number }[] {
	let s = (seed * 9301 + 49297) % 233280 || 1;
	const rand = () => (s = (s * 16807) % 2147483647) / 2147483647;
	const frames: { at: number; face: number }[] = [];
	let at = 0;
	let prev = 0;
	while (at < ms) {
		let face = 1 + Math.floor(rand() * 8);
		while (face === final || face === prev) face = (face % 8) + 1;
		frames.push({ at, face });
		prev = face;
		at += Math.round(55 + 140 * (at / ms));
	}
	frames.push({ at: ms, face: final });
	return frames;
}

/** A die's face at `t` ms after it was thrown (its first face before then). */
function faceAt(frames: { at: number; face: number }[], t: number): number {
	let face = frames[0].face;
	for (const f of frames) if (f.at <= t) face = f.face;
	return face;
}

const FILES = 'abcdefgh';

/**
 * Where the orange scan stands while the file and rank dice tumble for a
 * target `sq`: the square their current faces point to. The file die lands
 * first and locks the column; the scan reaches `sq` only when the rank die
 * lands, at SCAN_MS.
 */
export function scanFrames(sq: string, seed: number): { at: number; sq: string }[] {
	const file = tumbleFaces(FILES.indexOf(sq[0]) + 1, FILE_MS, seed + 1);
	const rank = tumbleFaces(Number(sq[1]), RANK_MS, seed + 2).map((f) => ({ at: f.at + RANK_DELAY_MS, face: f.face }));
	const times = [...new Set([...file.map((f) => f.at), ...rank.map((f) => f.at)])].sort((a, b) => a - b);
	const frames: { at: number; sq: string }[] = [];
	for (const at of times) {
		const here = FILES[faceAt(file, at) - 1] + faceAt(rank, at);
		if (frames.at(-1)?.sq !== here) frames.push({ at, sq: here });
	}
	return frames;
}

/** The seed for the dice of step `index` of turn `seq`: the tray and the board use the same one. */
export function diceSeed(seq: number, index: number): number {
	return seq * 97 + index * 2;
}

/** The latest target revealed, unless a re-roll followed it: the scan the board runs. */
export function scanOf(view: View, shown: number): { sq: string; key: string; seed: number } | null {
	for (let i = Math.min(shown, view.last.length) - 1; i >= firstDiceStep(view.last); i--) {
		const e = view.last[i];
		if (e.kind === 'reroll') return null;
		if (e.kind === 'target' && e.sq) return { sq: e.sq, key: `${view.seq}:${i}`, seed: diceSeed(view.seq, i) };
	}
	return null;
}

export type PillTone = 'odd' | 'pot' | 'where' | 'save';
export interface DicePill {
	key: string;
	text: string;
	tone: PillTone;
	/** When it shows, in ms after its step: a roll's pill waits for its die to land. */
	at?: number;
	/** What it changes to `at` ms after it shows: the square once the dice land, a saving roll's result. */
	then?: { text: string; tone: PillTone; at: number };
	/** The turn's last pill: it fades on its own. */
	last: boolean;
}

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
const PILL_KINDS = ['rolled_pothole', 'target', 'reroll', 'saving_roll', 'no_pothole'];

/** The pill for one dice event, or null for events the pill doesn't name. */
function pillFor(view: View, i: number): Omit<DicePill, 'last'> | null {
	const e = view.last[i];
	const key = `${view.seq}:${i}`;
	const roll = view.last.find((x) => x.kind === 'rolled_pothole')?.roll ?? 0;
	switch (e.kind) {
		case 'rolled_pothole':
			return (e.roll ?? 1) % 2 === 1
				? { key, text: `d8 ${e.roll} · no pothole`, tone: 'odd', at: THROW_MS }
				: { key, text: `d8 ${e.roll} · pothole`, tone: 'pot', at: THROW_MS };
		case 'target': {
			// Straight after the roll, the roll's pill stays (same key) and turns
			// into the square; after a re-roll it is a new pill.
			const sameKey = view.last[i - 1]?.kind === 'rolled_pothole' ? `${view.seq}:${i - 1}` : key;
			return { key: sameKey, text: `d8 ${roll} · pothole`, tone: 'pot', then: { text: e.sq ?? '', tone: 'where', at: SCAN_MS } };
		}
		case 'reroll':
			return { key, text: `${e.sq} · re-roll`, tone: 'pot' };
		case 'saving_roll': {
			const who = `${capitalize(pieceName(e.piece))} ${e.sq}`;
			return { key, text: `${who} · saving roll`, tone: 'save', then: { text: `${who} · ${e.saved ? 'saved' : 'falls'}`, tone: 'save', at: SAVE_MS } };
		}
		case 'no_pothole':
			return { key, text: 'no pothole this turn', tone: 'odd' };
	}
	return null;
}

/** The pill on the board for the first `shown` events of a turn: the latest dice event's, or null before the roll. */
export function dicePill(view: View, shown: number): DicePill | null {
	const first = firstDiceStep(view.last);
	for (let i = Math.min(shown, view.last.length) - 1; i >= first; i--) {
		const pill = pillFor(view, i);
		if (!pill) continue;
		const last = !view.last.slice(i + 1).some((e) => PILL_KINDS.includes(e.kind));
		return { ...pill, last };
	}
	return null;
}

/**
 * A die to throw: its value and color (grey for an odd pothole roll,
 * yellow for even, orange for the file and rank dice, cream for a saving
 * roll), when it is thrown and how long it tumbles, and its seed.
 */
export type DieTone = 'dull' | 'pot' | 'where' | 'save';
export interface DieSpec {
	value: number;
	tone: DieTone;
	delay: number;
	ms: number;
	seed: number;
}

/** The dice thrown for event `i` of a turn: one for a roll, two (file and rank) for a target, none otherwise. */
export function stepDice(view: View, i: number): DieSpec[] {
	const e = view.last[i];
	const seed = diceSeed(view.seq, i);
	switch (e.kind) {
		case 'rolled_pothole':
			return [{ value: e.roll ?? 0, tone: (e.roll ?? 1) % 2 === 1 ? 'dull' : 'pot', delay: 0, ms: THROW_MS, seed }];
		case 'target': {
			const sq = e.sq ?? 'a1';
			return [
				{ value: FILES.indexOf(sq[0]) + 1, tone: 'where', delay: 0, ms: FILE_MS, seed: seed + 1 },
				{ value: Number(sq[1]), tone: 'where', delay: RANK_DELAY_MS, ms: RANK_MS, seed: seed + 2 }
			];
		}
		case 'saving_roll':
			return [{ value: e.roll ?? 0, tone: 'save', delay: 0, ms: THROW_MS, seed }];
	}
	return [];
}
