// What the board says at big moments: a speech bubble from the square, with
// an emoji and a line (spec: board feel, section 8). The lines are the game
// piece's playful voice, never presented as real quotes.

import { FALL_MS } from './feel.ts';
import type { EventJSON } from './game.ts';

export type QuipKind = 'mamdaniFell' | 'repaired' | 'queenFell' | 'saved';

export const QUIPS: Record<QuipKind, { emoji: string; lines: string[] }> = {
	mamdaniFell: {
		emoji: '😢',
		lines: ['Sorry, my wife is calling.', 'Got paperwork to do.', 'Ask Batman for help.', 'Your friendly neighborhood Spidey can handle this.']
	},
	repaired: { emoji: '👍', lines: ['Filled. Next!', 'Another one off the list.'] },
	queenFell: { emoji: '😱', lines: ['Not the queen!', 'Mind the gap.'] },
	saved: { emoji: '😅', lines: ['That was close.'] }
};

/**
 * The latest big moment among the first `shown` events of a turn: the
 * Mamdani or a queen falling in, the Mamdani repairing a hole, or a saving
 * roll saving a queen or rook. Smaller moments (a pawn saved) say nothing.
 */
export function quipFor(events: EventJSON[], shown: number): { kind: QuipKind; sq: string; index: number } | null {
	for (let i = Math.min(shown, events.length) - 1; i >= 0; i--) {
		const e = events[i];
		if (!e.sq) continue;
		const kind = e.piece?.[1];
		if (e.kind === 'fell' && e.piece === 'M') return { kind: 'mamdaniFell', sq: e.sq, index: i };
		if (e.kind === 'fell' && kind === 'Q') return { kind: 'queenFell', sq: e.sq, index: i };
		if (e.kind === 'repaired') return { kind: 'repaired', sq: e.sq, index: i };
		if (e.kind === 'saving_roll' && e.saved && (kind === 'Q' || kind === 'R')) return { kind: 'saved', sq: e.sq, index: i };
	}
	return null;
}

/** A line for `kind`, picked at random but never `previous` again when there's another. */
export function pickLine(kind: QuipKind, previous: string | undefined, random: () => number = Math.random): string {
	const lines = QUIPS[kind].lines;
	const choices = lines.length > 1 ? lines.filter((l) => l !== previous) : lines;
	return choices[Math.min(choices.length - 1, Math.floor(random() * choices.length))];
}

/**
 * Remembers the line picked for each moment, so it stays the same while the
 * board redraws, and the line said last, so the next moment says another.
 * Returns the bubble for the first `shown` events of turn `seq`, or null.
 * After a fall it waits `delay` ms, the fall's length, so the piece drops first.
 */
export function quipper(random: () => number = Math.random) {
	const said = new Map<string, string>();
	let last: string | undefined;
	return (events: EventJSON[], seq: number, shown: number): { sq: string; emoji: string; line: string; key: string; delay: number } | null => {
		const q = quipFor(events, shown);
		if (!q) return null;
		const key = `${seq}:${q.index}`;
		let line = said.get(key);
		if (!line) {
			line = pickLine(q.kind, last, random);
			said.set(key, line);
			last = line;
		}
		const delay = q.kind === 'mamdaniFell' || q.kind === 'queenFell' ? FALL_MS : 0;
		return { sq: q.sq, emoji: QUIPS[q.kind].emoji, line, key, delay };
	};
}
