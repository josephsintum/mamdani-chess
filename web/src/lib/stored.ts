// A setting that is on unless this browser switched it off (sound, tips).

/**
 * An on/off switch stored as `key` (set while off). The choice made on this
 * page holds even when storage is blocked, for as long as the page is open.
 */
export function offSwitch(key: string): { on(): boolean; set(on: boolean): void } {
	let chosen: boolean | undefined;
	return {
		on() {
			if (chosen !== undefined) return chosen;
			try {
				return localStorage.getItem(key) === null;
			} catch {
				return true;
			}
		},
		set(on) {
			chosen = on;
			try {
				if (on) localStorage.removeItem(key);
				else localStorage.setItem(key, '1');
			} catch {
				// Blocked storage: the choice lasts while the page is open.
			}
		}
	};
}
