import { beforeEach, describe, expect, it, vi } from 'vitest';
import { blockedStorage, memoryStorage } from './test-boards.ts';

// Each test loads sound.ts afresh: it keeps the page's choice in the module.
const fresh = async () => {
	vi.resetModules();
	return import('./sound.ts');
};

describe('sound setting', () => {
	beforeEach(() => {
		vi.stubGlobal('localStorage', memoryStorage());
	});

	it('is on by default', async () => {
		expect((await fresh()).soundOn()).toBe(true);
	});

	it('remembers sound switched off, and back on', async () => {
		(await fresh()).setSoundOn(false);
		const next = await fresh();
		expect(next.soundOn()).toBe(false);
		next.setSoundOn(true);
		expect((await fresh()).soundOn()).toBe(true);
	});

	it('keeps the choice for the page when storage is blocked', async () => {
		vi.stubGlobal('localStorage', blockedStorage());
		const s = await fresh();
		expect(s.soundOn()).toBe(true);
		s.setSoundOn(false);
		expect(s.soundOn()).toBe(false);
	});

	it('plays nothing, and fails nothing, without audio (a server, a test)', async () => {
		const s = await fresh();
		expect(() => s.play('move')).not.toThrow();
		expect(() => s.load(s.CORE)).not.toThrow();
		expect(() => s.loop('waiting')()).not.toThrow();
	});
});
