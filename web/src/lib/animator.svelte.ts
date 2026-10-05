// Plays a turn's dice one step at a time. Shared by the game page and the
// /dev/board sandbox, so the sandbox shows exactly what players see.

import { firstDiceStep } from './board.ts';
import type { View } from './game.ts';

/** Time between dice steps; a whole roll takes about two seconds. */
export const STEP_MS = 550;

export class Animator {
	view = $state<View | null>(null);
	/** Events of view.last revealed so far. */
	shown = $state(0);
	/** Delay between steps, in ms; applies from the next step on. */
	stepMs: number;
	#timer: ReturnType<typeof setTimeout> | undefined;

	constructor(stepMs = STEP_MS) {
		this.stepMs = stepMs;
	}

	get animating(): boolean {
		return this.view !== null && this.shown < this.view.last.length;
	}

	/**
	 * Shows a new view. It plays out its dice one step at a time when it is
	 * the next turn; anything else (first load, reconnect, resignation)
	 * shows at once. A hidden tab skips the animation: browsers throttle its
	 * timers, so a player coming back would otherwise wait through slow dice.
	 * An update for the same turn (a player going offline, a rematch offer)
	 * keeps a roll that is playing where it is.
	 */
	receive(next: View, { hidden = false }: { hidden?: boolean } = {}) {
		if (this.view !== null && next.seq === this.view.seq) {
			this.view = next;
			this.shown = Math.min(this.shown, next.last.length);
			return;
		}
		const animate = this.view !== null && next.seq === this.view.seq + 1 && next.last.length > 0 && !hidden;
		this.view = next;
		clearTimeout(this.#timer);
		this.shown = animate ? firstDiceStep(next.last) : next.last.length;
		this.#tick();
	}

	/** Jumps to the end of the current roll. */
	finish() {
		clearTimeout(this.#timer);
		if (this.view) this.shown = this.view.last.length;
	}

	/** Stops the timer; call when the page goes away. */
	stop() {
		clearTimeout(this.#timer);
	}

	#tick() {
		if (!this.animating) return;
		this.#timer = setTimeout(() => {
			this.shown += 1;
			this.#tick();
		}, this.stepMs);
	}
}
