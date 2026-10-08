// Tips during your first games: one line the first time each part of the
// game happens in front of you. Each shows once per browser, with a few
// quiet turns after each (TIP_GAP), and the help dialog can switch them off.

import type { EventJSON, View } from './game.ts';
import { HOLE_ROUNDS } from './wire.gen.ts';

export type Tip = 'move' | 'roll' | 'opened' | 'fell' | 'saving' | 'repaired';

export const TIPS: Record<Tip, string> = {
	move: 'Your move. You may move the Mamdani instead of a piece.',
	roll: 'Every move ends with a d8. Even opens a pothole.',
	opened: `A pothole stays ${HOLE_ROUNDS} of its roller's moves. The cones count down.`,
	fell: 'A piece fell in. A clear line from the Mamdani gives a saving roll.',
	saving: 'A saving roll: odd saves the piece.',
	repaired: 'The Mamdani repairs any pothole next to it.'
};

const byEvent: Partial<Record<EventJSON['kind'], Tip>> = {
	rolled_pothole: 'roll',
	pothole_opened: 'opened',
	fell: 'fell',
	saving_roll: 'saving',
	repaired: 'repaired'
};

/** Turns after a tip before the next one may show. */
export const TIP_GAP = 4;

// The rarer moment first: a turn that saves a piece also rolled the dice.
const rarestFirst: Tip[] = ['repaired', 'saving', 'fell', 'opened', 'roll'];

/**
 * The tip for a view, or null: something the turn did that you haven't had
 * a tip for (only for a turn that played out in front of you, `live`),
 * else, on your move, that the Mamdani is yours to move too. None while
 * the last tip (shown at turn `lastSeq`) is under TIP_GAP turns old.
 */
export function tipFor(view: View, seen: ReadonlySet<Tip>, live: boolean, lastSeq?: number): Tip | null {
	if (lastSeq !== undefined && view.seq - lastSeq < TIP_GAP) return null;
	if (live) {
		const happened = new Set(view.last.map((e) => byEvent[e.kind]));
		const tip = rarestFirst.find((t) => happened.has(t) && !seen.has(t));
		if (tip) return tip;
	}
	const yourMove = view.status === 'playing' && view.you === view.turn && !!view.mamdani;
	return yourMove && !seen.has('move') ? 'move' : null;
}

const SEEN_KEY = 'tips-seen';
const OFF_KEY = 'tips-off';

/** The tips this browser has shown. Storage can be blocked: then none. */
export function seenTips(): Set<Tip> {
	try {
		return new Set(JSON.parse(localStorage.getItem(SEEN_KEY) ?? '[]') as Tip[]);
	} catch {
		return new Set();
	}
}

export function markSeen(tip: Tip) {
	try {
		localStorage.setItem(SEEN_KEY, JSON.stringify([...seenTips(), tip]));
	} catch {
		// Blocked storage: the tip may show again next visit.
	}
}

// The choice made on this page, which holds even when storage is blocked.
let chosen: boolean | undefined;

export function tipsOn(): boolean {
	if (chosen !== undefined) return chosen;
	try {
		return localStorage.getItem(OFF_KEY) === null;
	} catch {
		return true;
	}
}

export function setTipsOn(on: boolean) {
	chosen = on;
	try {
		if (on) localStorage.removeItem(OFF_KEY);
		else localStorage.setItem(OFF_KEY, '1');
	} catch {
		// Blocked storage: the choice lasts while the page is open.
	}
}
