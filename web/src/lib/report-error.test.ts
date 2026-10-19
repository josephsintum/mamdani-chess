import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { reportError } from './report-error.ts';

describe('reportError', () => {
	// Each test starts a minute after the last, past the 10 s gap.
	let clock = Date.now();
	beforeEach(() => {
		vi.useFakeTimers();
		clock += 60_000;
		vi.setSystemTime(clock);
	});
	afterEach(() => {
		vi.useRealTimers();
		vi.unstubAllGlobals();
	});

	it('uses sendBeacon when there is one', () => {
		const sendBeacon = vi.fn(() => true);
		vi.stubGlobal('navigator', { sendBeacon });
		vi.stubGlobal('fetch', vi.fn());
		reportError('/game/K7F3QZ', 'TypeError: x');
		expect(sendBeacon).toHaveBeenCalledTimes(1);
		const [url, body] = sendBeacon.mock.calls[0] as unknown as [string, Blob];
		expect(url).toBe('/api/error');
		expect(body.type).toBe('application/json');
		expect(fetch).not.toHaveBeenCalled();
	});

	it('falls back to fetch, sends the path and the message cut to 300 characters, and swallows a failure', async () => {
		vi.stubGlobal('navigator', {});
		const fetch = vi.fn(() => Promise.reject(new Error('down')));
		vi.stubGlobal('fetch', fetch);
		expect(() => reportError('/rules', 'x'.repeat(500))).not.toThrow();
		await Promise.resolve();
		const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
		expect(url).toBe('/api/error');
		expect(init).toMatchObject({ method: 'POST', keepalive: true });
		expect(JSON.parse(init.body as string)).toEqual({ path: '/rules', message: 'x'.repeat(300) });
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
