// The game clocks. The server sends each side's remaining time and when the
// running clock started; the browser counts down from those locally.

import type { Color } from './game.ts';

export interface ClockJSON {
	whiteMs: number;
	blackMs: number;
	/** The side whose clock is counting, if any. */
	running?: Color;
	/** When the running clock starts or started (server ms); after `now` during the dice pause. */
	since?: number;
	/** The server's time when it sent the view, in ms. */
	now: number;
	/** When the side to move must make its first move by (server ms). */
	firstMoveDeadline?: number;
}

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

/** Time left to make a first move, in ms, or null when there is no deadline. */
export function firstMoveLeft(clock: ClockJSON, serverNow: number): number | null {
	if (!clock.firstMoveDeadline) return null;
	return Math.max(0, clock.firstMoveDeadline - serverNow);
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
