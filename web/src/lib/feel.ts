// How the board looks and moves: pure helpers for the board's frame and
// effects (docs/superpowers/specs/2026-10-06-board-feel-design.md).

const RANKS = ['8', '7', '6', '5', '4', '3', '2', '1'];
const FILES = ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'];

/** The labels beside the board in screen order: ranks top to bottom, files left to right. */
export function coordinates(flipped: boolean): { ranks: string[]; files: string[] } {
	return flipped ? { ranks: [...RANKS].reverse(), files: [...FILES].reverse() } : { ranks: RANKS, files: FILES };
}

/** A square's column and row on screen, 0 to 7 from the top left. */
export function cellOf(sq: string, flipped: boolean): { col: number; row: number } {
	const file = sq.charCodeAt(0) - 97;
	const rank = Number(sq[1]) - 1;
	return flipped ? { col: 7 - file, row: rank } : { col: file, row: 7 - rank };
}
