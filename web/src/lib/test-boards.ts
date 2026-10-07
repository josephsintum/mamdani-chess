// Boards and views for unit tests, with pieces written by square.

import { squareIndex } from './board.ts';
import type { View } from './game.ts';
import { emptyView } from './sandbox.ts';

/** A board with the given pieces, e.g. { e1: 'wK' }. */
export function boardOf(pieces: Record<string, string>): string[] {
	const b = Array<string>(64).fill('');
	for (const [sq, p] of Object.entries(pieces)) b[squareIndex(sq)] = p;
	return b;
}

/** A whole view, an empty board with nothing played, with `extra` over it. */
export function viewOf(extra: Partial<View> = {}): View {
	return { ...emptyView(), ...extra };
}
