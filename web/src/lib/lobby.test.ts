import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	boardFromFen,
	chooseName,
	formatElapsed,
	initials,
	isCode,
	me,
	normalizeCode,
	rememberedName,
	untilText
} from './lobby.ts';

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

describe('untilText', () => {
	const now = 1_000_000_000;
	it('says hours, rounding up, for an hour or more', () => {
		expect(untilText(now + 5 * 3_600_000, now)).toBe('5 h');
		expect(untilText(now + 4 * 3_600_000 + 1, now)).toBe('5 h');
		expect(untilText(now + 3_600_000, now)).toBe('1 h');
	});
	it('says minutes, rounding up, under an hour', () => {
		expect(untilText(now + 40 * 60_000, now)).toBe('40 min');
		expect(untilText(now + 1, now)).toBe('1 min');
	});
	it('never goes below a minute', () => {
		expect(untilText(now - 5_000, now)).toBe('1 min');
	});
});

describe('rememberedName', () => {
	let store: Map<string, string>;
	const answer = (body: unknown, status = 200) =>
		vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(body), { status })));

	beforeEach(() => {
		store = new Map();
		vi.stubGlobal('localStorage', {
			getItem: (k: string) => store.get(k) ?? null,
			setItem: (k: string, v: string) => void store.set(k, v),
			removeItem: (k: string) => void store.delete(k)
		});
	});
	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('is null before the server has named this browser', () => {
		expect(rememberedName()).toBeNull();
	});
	it('keeps the name /api/me gave', async () => {
		answer({ name: 'Pothole Pete', changesLeft: 3, changesResetAt: null });
		await me();
		expect(rememberedName()).toBe('Pothole Pete');
	});
	it('forgets it when /api/me says there is no name', async () => {
		store.set('name', 'Pothole Pete');
		answer({ name: null, changesLeft: 3, changesResetAt: null });
		await me();
		expect(rememberedName()).toBeNull();
	});
	it('keeps a newly chosen name', async () => {
		answer({ name: 'Asphalt Annie', changesLeft: 2, changesResetAt: 1 });
		await chooseName('Asphalt Annie');
		expect(rememberedName()).toBe('Asphalt Annie');
	});
	it('keeps the old name when a change is refused', async () => {
		store.set('name', 'Pothole Pete');
		answer({ error: 'taken' }, 409);
		await expect(chooseName('Asphalt Annie')).rejects.toThrow();
		expect(rememberedName()).toBe('Pothole Pete');
	});
	it('is null when storage is blocked', () => {
		vi.stubGlobal('localStorage', {
			getItem: () => {
				throw new Error('blocked');
			}
		});
		expect(rememberedName()).toBeNull();
	});
	it('still answers when storage is blocked', async () => {
		vi.stubGlobal('localStorage', undefined);
		answer({ name: 'Pothole Pete', changesLeft: 3, changesResetAt: null });
		await expect(me()).resolves.toMatchObject({ name: 'Pothole Pete' });
	});
});
