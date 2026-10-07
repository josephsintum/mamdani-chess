import { describe, expect, it, vi } from 'vitest';
import { canShare, shareLink } from './share.ts';

const url = 'https://mamdanichess.com/game/K7F3QZ';

describe('canShare', () => {
	it('is false without a share sheet', () => {
		expect(canShare(url, {})).toBe(false);
	});
	it('is true with one that takes the link', () => {
		expect(canShare(url, { share: vi.fn(), canShare: () => true })).toBe(true);
	});
	it('is false when the browser says it cannot share the link', () => {
		expect(canShare(url, { share: vi.fn(), canShare: () => false })).toBe(false);
	});
	it('trusts share alone when canShare is missing', () => {
		expect(canShare(url, { share: vi.fn() })).toBe(true);
	});
});

describe('shareLink', () => {
	it('hands the sheet the link and a title, nothing else', async () => {
		const share = vi.fn().mockResolvedValue(undefined);
		expect(await shareLink(url, { share })).toBe('shared');
		expect(share).toHaveBeenCalledWith({ title: 'Mamdani Chess', url });
	});
	it('reports a backed-out sheet as cancelled', async () => {
		const share = vi.fn().mockRejectedValue(new DOMException('Share canceled', 'AbortError'));
		expect(await shareLink(url, { share })).toBe('cancelled');
	});
	it('reports a refused sheet as failed, so the caller can copy instead', async () => {
		const share = vi.fn().mockRejectedValue(new DOMException('Not allowed', 'NotAllowedError'));
		expect(await shareLink(url, { share })).toBe('failed');
	});
	it('fails without a share sheet', async () => {
		expect(await shareLink(url, {})).toBe('failed');
	});
});
