import { beforeEach, describe, expect, it, vi } from 'vitest';

// Each test loads sound.ts afresh: it keeps the page's choice in the module.
const fresh = async () => {
	vi.resetModules();
	return import('./sound.ts');
};

describe('sound setting', () => {
	let items: Map<string, string>;
	beforeEach(() => {
		items = new Map();
		vi.stubGlobal('localStorage', {
			getItem: (k: string) => items.get(k) ?? null,
			setItem: (k: string, v: string) => void items.set(k, v),
			removeItem: (k: string) => void items.delete(k)
		});
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
		vi.stubGlobal('localStorage', {
			getItem: () => {
				throw new Error('blocked');
			},
			setItem: () => {
				throw new Error('blocked');
			},
			removeItem: () => {
				throw new Error('blocked');
			}
		});
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
