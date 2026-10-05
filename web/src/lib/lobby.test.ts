import { describe, expect, it } from 'vitest';
import { boardFromFen, formatElapsed, initials, isCode, normalizeCode } from './lobby.ts';

describe('normalizeCode', () => {
	it('upper-cases and keeps only code characters', () => {
		expect(normalizeCode('k7f3qz')).toBe('K7F3QZ');
		expect(normalizeCode(' k7-f3 qz ')).toBe('K7F3QZ');
	});
	it('drops lookalikes that are never in a code', () => {
		expect(normalizeCode('O0I1L')).toBe('');
	});
	it('stops at six characters', () => {
		expect(normalizeCode('K7F3QZAB')).toBe('K7F3QZ');
	});
	it('takes the code out of a pasted game link', () => {
		expect(normalizeCode('https://mamdani-chess.up.railway.app/game/K7F3QZ')).toBe('K7F3QZ');
		expect(normalizeCode('localhost:5173/game/k7f3qz?instant')).toBe('K7F3QZ');
	});
});

describe('isCode', () => {
	it('wants exactly six code characters', () => {
		expect(isCode('K7F3QZ')).toBe(true);
		expect(isCode('K7F3Q')).toBe(false);
		expect(isCode('k7f3qz')).toBe(false);
		expect(isCode('K7F3Q0')).toBe(false);
	});
});

describe('initials', () => {
	it('takes the first and last words', () => {
		expect(initials('pizza-rat-astoria')).toBe('PA');
		expect(initials('bagel-soho')).toBe('BS');
		expect(initials('rizzler')).toBe('R');
		expect(initials('')).toBe('');
	});
});

describe('boardFromFen', () => {
	it('puts a1 at index 0 and h8 at 63', () => {
		const b = boardFromFen('rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR');
		expect(b[0]).toBe('wR');
		expect(b[4]).toBe('wK');
		expect(b[28]).toBe('wP'); // e4
		expect(b[12]).toBe(''); // e2 is empty
		expect(b[60]).toBe('bK');
		expect(b[63]).toBe('bR');
		expect(b.filter(Boolean)).toHaveLength(32);
	});
});

describe('formatElapsed', () => {
	it('counts whole seconds up', () => {
		expect(formatElapsed(0)).toBe('0:00');
		expect(formatElapsed(7_900)).toBe('0:07');
		expect(formatElapsed(90_000)).toBe('1:30');
		expect(formatElapsed(725_000)).toBe('12:05');
	});
});
