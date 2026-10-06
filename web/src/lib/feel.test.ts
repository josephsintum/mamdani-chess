import { describe, expect, it } from 'vitest';
import { cellOf, coordinates } from './feel.ts';

describe('coordinates', () => {
	it('reads 8 to 1 down and a to h across for White', () => {
		expect(coordinates(false)).toEqual({ ranks: ['8', '7', '6', '5', '4', '3', '2', '1'], files: ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'] });
	});
	it('reads 1 to 8 down and h to a across for Black', () => {
		expect(coordinates(true)).toEqual({ ranks: ['1', '2', '3', '4', '5', '6', '7', '8'], files: ['h', 'g', 'f', 'e', 'd', 'c', 'b', 'a'] });
	});
});

describe('cellOf', () => {
	it('puts a1 bottom left for White and top right for Black', () => {
		expect(cellOf('a1', false)).toEqual({ col: 0, row: 7 });
		expect(cellOf('a1', true)).toEqual({ col: 7, row: 0 });
		expect(cellOf('e4', false)).toEqual({ col: 4, row: 4 });
	});
});
