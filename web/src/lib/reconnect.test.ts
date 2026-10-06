import { describe, expect, it } from 'vitest';
import { retryDelay } from './reconnect.ts';

describe('retryDelay', () => {
	it('waits 1 to 3 s on the first retry', () => {
		expect(retryDelay(0, () => 0)).toBe(1000);
		expect(retryDelay(0, () => 0.999)).toBeCloseTo(3000, -1);
	});
	it('waits 2 to 6 s after that', () => {
		expect(retryDelay(1, () => 0)).toBe(2000);
		expect(retryDelay(1, () => 0.999)).toBeCloseTo(6000, -1);
	});
	it('stays inside the 10 s a restored game waits, however long the outage', () => {
		expect(retryDelay(50, () => 0.999)).toBeLessThan(10000);
	});
	it('spreads tabs out at every attempt, never in lock-step', () => {
		for (const attempt of [0, 1, 5, 50]) {
			const delays = new Set([0.1, 0.4, 0.7].map((r) => retryDelay(attempt, () => r)));
			expect(delays.size).toBe(3);
		}
	});
});
