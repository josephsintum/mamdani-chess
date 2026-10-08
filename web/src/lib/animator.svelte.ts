// Plays a turn's dice one step at a time. Shared by the game page and the
// /dev/board sandbox, so the sandbox shows exactly what players see.

import { dicePace, firstDiceStep } from './dice.ts';
import type { EventJSON, View } from './game.ts';

/**
 * How long to wait before showing the next dice step: a fixed number of ms,
 * or a function of the step just shown (undefined before the roll). The
 * game uses dicePace, from the table the server's clock pause shares, so a
 * clock never starts while the dice still play.
 */
export type Pace = number | ((shownLast: EventJSON | undefined) => number);

export class Animator {
	// Raw: a view is only ever replaced, never changed in place.
	view = $state.raw<View | null>(null);
	/** Events of view.last revealed so far. */
	shown = $state(0);
	/**
	 * Whether the current turn is playing out step by step, rather than shown
	 * at once (first load, reconnect, a hidden tab). One-off effects such as a
	 * repair celebration play only for an animated turn, so a reload never
	 * replays them.
	 */
	animated = $state(false);
	/** The wait before each step; applies from the next step on. */
	pace: Pace;
	/**
	 * Called with the events of view.last just revealed, [from, to), on a turn
	 * that is playing out; never for one shown at once. The game's sounds hang
	 * on it, so a reload or a hidden tab never replays them.
	 */
	onreveal: ((view: View, from: number, to: number) => void) | undefined;
	#timer: ReturnType<typeof setTimeout> | undefined;

	constructor(pace: Pace = dicePace) {
		this.pace = pace;
	}

	#wait(): number {
		if (typeof this.pace === 'number') return this.pace;
		const last = this.view?.last ?? [];
		return this.pace(this.shown > firstDiceStep(last) ? last[this.shown - 1] : undefined);
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
		this.animated = animate;
		clearTimeout(this.#timer);
		this.shown = animate ? firstDiceStep(next.last) : next.last.length;
		if (animate) this.onreveal?.(next, 0, this.shown);
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
			if (this.view) this.onreveal?.(this.view, this.shown - 1, this.shown);
			this.#tick();
		}, this.#wait());
	}
}
