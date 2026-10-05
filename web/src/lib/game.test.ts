import { describe, expect, it } from 'vitest';
import { pieceName } from './game.ts';

describe('pieceName', () => {
	it('names a piece for screen readers', () => {
		expect(pieceName('bN')).toBe('black knight');
		expect(pieceName('wQ')).toBe('white queen');
		expect(pieceName('M')).toBe('the Mamdani');
	});
});
