import { beforeEach, describe, expect, it, vi } from 'vitest';
import { viewOf } from './test-boards.ts';
import { markSeen, seenTips, setTipsOn, tipFor, tipsOn, type Tip } from './tips.ts';
import type { EventJSON } from './game.ts';

const turn = (...kinds: EventJSON['kind'][]) => viewOf({ last: kinds.map((kind) => ({ kind })) });

describe('tipFor', () => {
	it('picks the rarest new moment of a turn that played out', () => {
		const v = turn('moved', 'rolled_pothole', 'target', 'saving_roll', 'fell', 'pothole_opened');
		expect(tipFor(v, new Set(), true)).toBe('saving');
		expect(tipFor(v, new Set<Tip>(['saving']), true)).toBe('fell');
		expect(tipFor(v, new Set<Tip>(['saving', 'fell', 'opened', 'roll']), true)).toBeNull();
	});

	it('says nothing about a turn shown at once (a reload)', () => {
		expect(tipFor(turn('moved', 'rolled_pothole'), new Set(), false)).toBeNull();
	});

	it('tells you the Mamdani is yours on your move', () => {
		const v = viewOf({ you: 'white', turn: 'white', mamdani: 'a5' });
		expect(tipFor(v, new Set(), false)).toBe('move');
		expect(tipFor({ ...v, you: 'black' }, new Set(), false)).toBeNull();
		expect(tipFor({ ...v, you: 'spectator' }, new Set(), false)).toBeNull();
		expect(tipFor({ ...v, mamdani: '' }, new Set(), false)).toBeNull();
	});
});

describe('tip storage', () => {
	beforeEach(() => {
		const items = new Map<string, string>();
		vi.stubGlobal('localStorage', {
			getItem: (k: string) => items.get(k) ?? null,
			setItem: (k: string, v: string) => void items.set(k, v),
			removeItem: (k: string) => void items.delete(k)
		});
	});

	it('remembers what was shown', () => {
		markSeen('roll');
		markSeen('fell');
		expect(seenTips()).toEqual(new Set(['roll', 'fell']));
	});

	it('remembers tips switched off', () => {
		expect(tipsOn()).toBe(true);
		setTipsOn(false);
		expect(tipsOn()).toBe(false);
		setTipsOn(true);
		expect(tipsOn()).toBe(true);
	});
});
