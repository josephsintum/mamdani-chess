// Motion settings shared by the board and dice.

let instant = false;

/** Dev only: turns every animation off, for fast play-testing. */
export function setInstant(on: boolean) {
	instant = on;
}

/** True when the player asked their system for less motion, or instant mode is on. */
export function reducedMotion(): boolean {
	return instant || (typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
}
