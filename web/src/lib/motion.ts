// Motion settings shared by the board and dice.

/** True when the player asked their system for less motion. */
export function reducedMotion(): boolean {
	return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}
