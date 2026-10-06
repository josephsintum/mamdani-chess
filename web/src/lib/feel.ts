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

/**
 * How long a legal square waits before popping in when a piece is picked
 * up: 15 ms per square of distance, so the moves ripple out from the piece.
 */
export function rippleDelay(from: string, to: string): number {
	const df = to.charCodeAt(0) - from.charCodeAt(0);
	const dr = Number(to[1]) - Number(from[1]);
	return Math.round(15 * Math.hypot(df, dr));
}

/** How long a fall into a pothole plays: a teeter, then the drop. It fits one dice step (STEP_MS). */
export const FALL_MS = 550;

/** A board shake: how far it moves and for how long. */
export type Shake = { px: number; ms: number };

/** The shakes by size of moment: the bigger the moment, the bigger the shake. */
export const SHAKE = {
	minor: { px: 1.5, ms: 140 }, // a pawn, knight or bishop taken
	major: { px: 3, ms: 200 }, // a queen or rook taken
	fall: { px: 4.5, ms: 280 }, // a piece falls into a hole
	mate: { px: 7, ms: 420 } // checkmate
} as const satisfies Record<string, Shake>;

/** The shake for taking the piece `code` ("bQ", "wP"). */
export function captureShake(code: string): Shake {
	return code[1] === 'Q' || code[1] === 'R' ? SHAKE.major : SHAKE.minor;
}

/** One shake at a time: the bigger wins. */
export function biggerShake(current: Shake | null, next: Shake): Shake {
	return current && current.px >= next.px ? current : next;
}

/** A decaying zigzag of `px`, ending still. */
export function shakeFrames(px: number): Keyframe[] {
	const frames: Keyframe[] = [];
	for (let i = 0; i < 8; i++) {
		const f = 1 - i / 8;
		const x = Math.round((i % 2 ? -1 : 1) * px * f * 100) / 100;
		const y = Math.round((i % 3 === 1 ? 1 : i % 3 === 2 ? -1 : 0) * px * 0.5 * f * 100) / 100;
		frames.push({ transform: `translate(${x}px, ${y}px)` });
	}
	frames.push({ transform: 'translate(0px, 0px)' });
	return frames;
}

/** Where a taken piece is knocked: a third of a square along the move, in % of a square. */
export function knockOffset(from: string, to: string, flipped: boolean): { x: number; y: number } {
	const a = cellOf(from, flipped);
	const b = cellOf(to, flipped);
	const dx = b.col - a.col;
	const dy = b.row - a.row;
	const d = Math.hypot(dx, dy) || 1;
	const r = (v: number) => Math.round(v * 10) / 10 || 0;
	return { x: r((33 * dx) / d), y: r((33 * dy) / d) };
}
