import { afterEach, describe, expect, it } from 'vitest';
import { reducedMotion, setInstant } from './motion.ts';

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
