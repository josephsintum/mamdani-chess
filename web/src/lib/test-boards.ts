// Boards, views and storage for unit tests, with pieces written by square.

import type { View } from './game.ts';
import { emptyView } from './sandbox.ts';

export { boardOf } from './board.ts';

/** A whole view, an empty board with nothing played, with `extra` over it. */
export function viewOf(extra: Partial<View> = {}): View {
	return { ...emptyView(), ...extra };
}

/** A localStorage kept in `items`, for vi.stubGlobal('localStorage', …). */
export function memoryStorage(items = new Map<string, string>()) {
	return {
		getItem: (k: string) => items.get(k) ?? null,
		setItem: (k: string, v: string) => void items.set(k, v),
		removeItem: (k: string) => void items.delete(k)
	};
}

/** A localStorage the browser blocks (private mode, cookies off): every call throws. */
export function blockedStorage() {
	const blocked = () => {
		throw new Error('blocked');
	};
	return { getItem: blocked, setItem: blocked, removeItem: blocked };
}
