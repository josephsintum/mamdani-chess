import { describe, expect, it } from 'vitest';
import { firstMoveLeft, formatClock, paused, timeLeft, type ClockJSON } from './clock.ts';

const running: ClockJSON = { whiteMs: 300_000, blackMs: 600_000, running: 'white', since: 10_000, now: 9_000 };

describe('timeLeft', () => {
	it('counts down the running side from since', () => {
		expect(timeLeft(running, 'white', 25_000)).toBe(285_000);
		expect(timeLeft(running, 'black', 25_000)).toBe(600_000);
	});

	it('charges nothing during the dice pause', () => {
		expect(timeLeft(running, 'white', 9_500)).toBe(300_000);
		expect(paused(running, 9_500)).toBe(true);
		expect(paused(running, 10_000)).toBe(false);
	});

	it('never goes below zero', () => {
		expect(timeLeft(running, 'white', 10_000_000)).toBe(0);
	});

	it('shows stopped clocks as they are', () => {
		const stopped: ClockJSON = { whiteMs: 1_000, blackMs: 2_000, now: 0 };
		expect(timeLeft(stopped, 'white', 99_999)).toBe(1_000);
		expect(paused(stopped, 0)).toBe(false);
	});
});

describe('firstMoveLeft', () => {
	it('counts down to the deadline, and is null without one', () => {
		const c: ClockJSON = { whiteMs: 600_000, blackMs: 600_000, now: 0, firstMoveDeadline: 60_000 };
		expect(firstMoveLeft(c, 15_000)).toBe(45_000);
		expect(firstMoveLeft(c, 70_000)).toBe(0);
		expect(firstMoveLeft(running, 0)).toBeNull();
	});
});

describe('formatClock', () => {
	it('shows minutes and seconds, rounding up', () => {
		expect(formatClock(600_000)).toBe('10:00');
		expect(formatClock(599_001)).toBe('10:00');
		expect(formatClock(599_000)).toBe('9:59');
		expect(formatClock(65_000)).toBe('1:05');
	});

	it('shows tenths in the last ten seconds', () => {
		expect(formatClock(9_950)).toBe('0:09.9');
		expect(formatClock(420)).toBe('0:00.4');
		expect(formatClock(0)).toBe('0:00.0');
	});
});
