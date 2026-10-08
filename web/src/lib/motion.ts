// Motion settings shared by the board and dice.

let instant = false;
let reduceQuery: MediaQueryList | undefined;

/** Dev only: turns every animation off, for fast play-testing. */
export function setInstant(on: boolean) {
	instant = on;
}

/** Whether instant mode is on: no animations and no sound. */
export function instantMode(): boolean {
	return instant;
}

/** True when the player asked their system for less motion, or instant mode is on. */
export function reducedMotion(): boolean {
	if (instant) return true;
	if (typeof window === 'undefined') return false;
	reduceQuery ??= window.matchMedia('(prefers-reduced-motion: reduce)');
	return reduceQuery.matches;
}

/**
 * What is left of an exit that started `elapsed` ms ago and lasts `total`,
 * never under 1 ms: with Svelte 5.57, an update whose leaving elements mixed
 * a 0 ms exit with a timed one left them all on the page.
 */
export function exitMs(elapsed: number, total: number): number {
	return Math.max(1, total - elapsed);
}
