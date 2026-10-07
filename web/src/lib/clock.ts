// The game clocks. The server sends each side's remaining time and when the
// running clock started; the browser counts down from those locally.

import type { ClockJSON, Color } from './wire.gen.ts';

export type { ClockJSON };

/** What color has left at serverNow, in ms. */
export function timeLeft(clock: ClockJSON, color: Color, serverNow: number): number {
	const base = color === 'white' ? clock.whiteMs : clock.blackMs;
	if (clock.running !== color || clock.since === undefined) return base;
	return Math.max(0, base - Math.max(0, serverNow - clock.since));
}

/** Whether the running clock is waiting for the dice to finish. */
export function paused(clock: ClockJSON, serverNow: number): boolean {
	return clock.running !== undefined && clock.since !== undefined && serverNow < clock.since;
}

/** How long each side has for its first move (FirstMoveTime in game/clock.go). */
const FIRST_MOVE_MS = 60_000;

/**
 * Time left to make a first move, in ms, or null when there is no deadline.
 * During the dice pause the deadline is more than a minute away; the
 * countdown shows a full minute until the pause ends.
 */
export function firstMoveLeft(clock: ClockJSON, serverNow: number): number | null {
	if (!clock.firstMoveDeadline) return null;
	return Math.min(FIRST_MOVE_MS, Math.max(0, clock.firstMoveDeadline - serverNow));
}

/** "9:59"; tenths in the last ten seconds: "0:07.3". */
export function formatClock(ms: number): string {
	if (ms < 10_000) {
		const tenths = Math.floor(ms / 100);
		return `0:0${Math.floor(tenths / 10)}.${tenths % 10}`;
	}
	const secs = Math.ceil(ms / 1000);
	return `${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, '0')}`;
}

/**
 * The time a player just gained from the 5 s increment, from their clock's
 * last reading and this one, or 0: a move landed (`moved`), and the clock
 * jumped up by about the increment (less what the move's trip to the server
 * took) and stopped, since it's now the other side's turn. A deploy that
 * restores a clock moves nothing, so it never counts.
 */
export function bonusOf(prev: number | undefined, next: number, { ticking, moved }: { ticking: boolean; moved: boolean }): number {
	if (prev === undefined || ticking || !moved) return 0;
	const gain = next - prev;
	return gain >= 3_000 && gain <= 6_000 ? gain : 0;
}
