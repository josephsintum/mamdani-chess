import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { reportError, reportVisit, visitPayload } from './visit.ts';

const screen = { width: 390, height: 844 };

describe('visitPayload', () => {
	it('sends the path, the screen and the referrer of a first page', () => {
		expect(visitPayload(new URL('https://mamdanichess.com/game/K7F3QZ'), true, 'https://l.instagram.com/', screen, true)).toEqual({
			path: '/game/K7F3QZ',
			w: 390,
			h: 844,
			touch: true,
			referrer: 'https://l.instagram.com/'
		});
	});

	it('sends no referrer after the first page', () => {
		expect(visitPayload(new URL('https://mamdanichess.com/rules'), false, 'https://mamdanichess.com/', screen, false).referrer).toBe('');
	});

	it('turns a ?ref= tag into ref:<tag>', () => {
		expect(visitPayload(new URL('https://mamdanichess.com/?ref=ig'), true, '', screen, false).referrer).toBe('ref:ig');
	});
});

describe('reportVisit', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('uses sendBeacon when there is one', () => {
		const sendBeacon = vi.fn(() => true);
		vi.stubGlobal('navigator', { sendBeacon });
		vi.stubGlobal('fetch', vi.fn());
		reportVisit({ path: '/', w: 1, h: 1, touch: false, referrer: '' });
		expect(sendBeacon).toHaveBeenCalledTimes(1);
		const [url, body] = sendBeacon.mock.calls[0] as unknown as [string, Blob];
		expect(url).toBe('/api/visit');
		expect(body.type).toBe('application/json');
		expect(fetch).not.toHaveBeenCalled();
	});

	it('falls back to fetch and swallows a failure', async () => {
		vi.stubGlobal('navigator', {});
		const fetch = vi.fn(() => Promise.reject(new Error('down')));
		vi.stubGlobal('fetch', fetch);
		expect(() => reportVisit({ path: '/', w: 1, h: 1, touch: false, referrer: '' })).not.toThrow();
		await Promise.resolve();
		expect(fetch).toHaveBeenCalledWith('/api/visit', expect.objectContaining({ method: 'POST', keepalive: true }));
	});
});

describe('reportError', () => {
	beforeEach(() => vi.useFakeTimers());
	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('sends at most one report every 10 seconds', () => {
		const sendBeacon = vi.fn(() => true);
		vi.stubGlobal('navigator', { sendBeacon });
		reportError('/game/K7F3QZ', 'TypeError: x');
		reportError('/game/K7F3QZ', 'TypeError: y');
		expect(sendBeacon).toHaveBeenCalledTimes(1);
		vi.advanceTimersByTime(10_001);
		reportError('/game/K7F3QZ', 'TypeError: z');
		expect(sendBeacon).toHaveBeenCalledTimes(2);
	});
});
