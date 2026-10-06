import { afterEach, describe, expect, it } from 'vitest';
import { exitMs, reducedMotion, setInstant } from './motion.ts';

describe('instant mode', () => {
	afterEach(() => setInstant(false));

	it('turns every animation off, whatever the system setting', () => {
		expect(reducedMotion()).toBe(false);
		setInstant(true);
		expect(reducedMotion()).toBe(true);
		setInstant(false);
		expect(reducedMotion()).toBe(false);
	});
});

describe('exitMs', () => {
	it('gives what is left of an exit, but never 0 ms', () => {
		// A 0 ms exit beside timed ones left a whole batch of leaving nodes in the DOM (Svelte 5.57).
		expect(exitMs(400, 2400)).toBe(2000);
		expect(exitMs(2400, 2400)).toBe(1);
		expect(exitMs(3000, 2400)).toBe(1);
	});
});
