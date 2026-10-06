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

/** How long a fall into a pothole plays: a teeter, then the drop. It fits the fall's dice step (dice-timing.json). */
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

/**
 * The shards of a checkmate burst: an angle all the way round (degrees), how
 * far each flies (squares), its spin, and whether it's orange. Seeded, so the
 * burst is the same every time.
 */
export function burstShards(count = 18, seed = 7): { angle: number; dist: number; spin: number; hazard: boolean }[] {
	let s = seed;
	const rand = () => ((s = (s * 16807) % 2147483647) - 1) / 2147483646;
	return Array.from({ length: count }, (_, i) => ({
		angle: Math.round(((i + rand() * 0.6) / count) * 360),
		dist: Math.round((2 + rand() * 3) * 10) / 10,
		spin: Math.round(rand() * 540 - 270),
		hazard: i % 3 === 0
	}));
}

/**
 * How long a checkmate burst holds the board, in ms, before it dims and the
 * result card shows (with the tally and the winner's confetti).
 */
export const BURST_HOLD_MS = 1800;

/** How long the winner's confetti falls, in ms. */
export const CONFETTI_MS = 2500;

export type Confetto = {
	/** Where it starts across the screen, 0 to 1. */
	x: number;
	/** When it starts falling, and how long it takes to cross the screen (ms). */
	delay: number;
	fall: number;
	/** How far it sways side to side (px), and how fast it spins (degrees a second). */
	sway: number;
	spin: number;
	/** Its size (px); a cone is drawn in a box this size. */
	w: number;
	h: number;
	tone: 'accent' | 'hazard' | 'cream';
	cone: boolean;
};

/**
 * The winner's confetti: strips in road-works yellow, hazard orange and
 * cream, one in sixteen a tiny traffic cone. Seeded, so it is the same every
 * time; every piece is through the screen before CONFETTI_MS.
 */
export function confetti(count = 80, seed = 11): Confetto[] {
	let s = seed;
	const rand = () => ((s = (s * 16807) % 2147483647) - 1) / 2147483646;
	const tones = ['accent', 'hazard', 'cream'] as const;
	return Array.from({ length: count }, (_, i) => {
		const cone = i % 16 === 15;
		const delay = Math.round(rand() * 500);
		return {
			x: Math.round(rand() * 1000) / 1000,
			delay,
			fall: Math.round(1400 + rand() * (CONFETTI_MS - 1400 - delay)),
			sway: Math.round(10 + rand() * 30),
			spin: Math.round(rand() * 720 - 360),
			w: cone ? 12 : Math.round(6 + rand() * 4),
			h: cone ? 14 : Math.round(10 + rand() * 6),
			tone: tones[i % 3],
			cone
		};
	});
}

/** A count-up's value at `t` (0 to 1 of the way through), easing out. */
export function countAt(value: number, t: number): number {
	const p = Math.min(1, Math.max(0, t));
	return Math.round(value * (1 - (1 - p) ** 3));
}

/**
 * How far to slide a bubble centred at `center`, `width` wide, so it stays
 * 4 px inside the board's edges `lo` and `hi` (all in px). A bubble wider
 * than the board keeps its left edge in.
 */
export function fitShift(center: number, width: number, lo: number, hi: number): number {
	const left = center - width / 2;
	return Math.max(lo + 4 - left, Math.min(0, hi - 4 - (left + width)));
}
