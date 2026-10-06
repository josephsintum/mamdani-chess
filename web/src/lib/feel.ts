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

/** The glide's easing: a piece leaves fast and settles gently (easeOutQuart). */
export const GLIDE_EASE = 'cubic-bezier(0.25, 1, 0.5, 1)';
/** How long a trail takes to fade once its piece lands. */
export const TRAIL_FADE_MS = 180;
/** How long the whiplash runs on after the glide, settling the piece. */
export const WHIP_TAIL_MS = 160;

/**
 * A trail from the centre of `from` toward `to` on screen: where it starts
 * (x, y) and its length, in squares, and its angle in degrees (0 points
 * right, 90 down).
 */
export function trailOf(from: string, to: string, flipped: boolean): { x: number; y: number; length: number; angle: number } {
	const a = cellOf(from, flipped);
	const b = cellOf(to, flipped);
	const dx = b.col - a.col;
	const dy = b.row - a.row;
	return { x: a.col + 0.5, y: a.row + 0.5, length: Math.hypot(dx, dy), angle: (Math.atan2(dy, dx) * 180) / Math.PI };
}

/**
 * The whiplash's peak in degrees: 5 times the sideways share of the move on
 * screen, positive moving right. A move straight up or down barely tilts.
 */
export function whiplash(from: string, to: string, flipped: boolean): number {
	const a = cellOf(from, flipped);
	const b = cellOf(to, flipped);
	const dx = b.col - a.col;
	const dist = Math.hypot(dx, b.row - a.row);
	return dist === 0 ? 0 : (5 * dx) / dist;
}

/**
 * The whiplash for peak `w`: the piece leans back as it launches, whips
 * forward along the move as it brakes, swings back a touch and settles.
 */
export function whipFrames(w: number): Keyframe[] {
	const r = (deg: number) => `rotate(${Math.round(deg * 100) / 100 || 0}deg)`;
	return [
		{ transform: r(0) },
		{ transform: r(-w * 0.6), offset: 0.12 },
		{ transform: r(w), offset: 0.62 },
		{ transform: r(-w * 0.3), offset: 0.84 },
		{ transform: r(0) }
	];
}

/** A trail's color, from the piece that leaves it ("wN", "bB", "M"). */
export function trailColor(code: string): string {
	if (code === 'M') return 'var(--trail-mamdani)';
	return code[0] === 'w' ? 'var(--trail-white)' : 'var(--trail-black)';
}
