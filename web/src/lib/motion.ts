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

const caught = new WeakSet<Element>();

/**
 * An attachment that starts a one-off effect's CSS animations `age` ms in:
 * its age from Animator.age, read once, when it mounts. So an effect drawn
 * late (on a reload, a board drawn again, a sheet opened) carries on where
 * it should be rather than playing from the start, and one with no age
 * (Infinity) shows how it ends. Script animations (a die's throw) take
 * their age themselves.
 */
export function catchUp(age: number) {
	return (node: Element) => {
		if (!(age > 0) || caught.has(node)) return;
		caught.add(node);
		for (const a of node.getAnimations({ subtree: true })) {
			if (!('animationName' in a) || a.effect?.getComputedTiming().iterations === Infinity) continue;
			if (age === Infinity) a.finish();
			else a.currentTime = Number(a.currentTime ?? 0) + age;
		}
	};
}

/**
 * What is left of an exit that started `elapsed` ms ago and lasts `total`,
 * never under 1 ms: with Svelte 5.57, an update whose leaving elements mixed
 * a 0 ms exit with a timed one left them all on the page.
 */
export function exitMs(elapsed: number, total: number): number {
	return Math.max(1, total - elapsed);
}
