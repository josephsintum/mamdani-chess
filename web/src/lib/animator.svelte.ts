// Plays a turn's dice one step at a time, and remembers when each step
// played, so a one-off effect (a die's throw, a scan, a repair's 👍, a
// bubble) plays once: an effect mounted late catches up by its age, and one
// shown at once has none and shows settled. Shared by the game page and the
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

/** Animator.age: how far in a one-off effect of the current turn is, in ms. */
export type Age = (at: number | 'end') => number;

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
	 * that arrived live: step by step as it plays out, or all at once
	 * (`jumped`) when it arrived in a hidden tab or the rest of a roll was
	 * skipped. Never for a turn shown at once on load or reconnect, so a
	 * reload never replays the game's sounds, which hang on it.
	 */
	onreveal: ((view: View, from: number, to: number, jumped?: boolean) => void) | undefined;
	#timer: ReturnType<typeof setTimeout> | undefined;
	#now: () => number;
	// When each event of view.last was revealed as the turn played out; null
	// for one shown at once.
	#revealedAt: (number | null)[] = [];
	// When the game's result arrived: a time, 'turn' (when the turn that
	// brought it finishes playing), or null (shown at once).
	#endedAt: number | 'turn' | null = null;

	constructor(pace: Pace = dicePace, now: () => number = () => performance.now()) {
		this.pace = pace;
		this.#now = now;
	}

	/**
	 * How long ago, in ms, event `at` of the current turn was revealed, or
	 * the game ended ('end'). Infinity for anything shown at once: on load,
	 * on a reconnect, in a hidden tab, or skipped by finish(). An effect reads
	 * it when it mounts and starts that far in, so a reload, a board drawn
	 * again or a sheet opened later never plays it from the start.
	 */
	age: Age = (at) => {
		const t = at !== 'end' ? this.#revealedAt[at] : this.#endedAt === 'turn' ? this.#revealedAt[(this.view?.last.length ?? 0) - 1] : this.#endedAt;
		return t === null || t === undefined ? Infinity : this.#now() - t;
	};

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
		const prev = this.view;
		if (prev !== null && next.seq === prev.seq) {
			if (next.result && !prev.result) this.#endedAt = this.#now();
			this.view = next;
			this.shown = Math.min(this.shown, next.last.length);
			return;
		}
		const live = prev !== null && next.seq === prev.seq + 1 && next.last.length > 0;
		const animate = live && !hidden;
		this.view = next;
		this.animated = animate;
		clearTimeout(this.#timer);
		this.shown = animate ? firstDiceStep(next.last) : next.last.length;
		const now = this.#now();
		this.#revealedAt = next.last.map((_, i) => (animate && i < this.shown ? now : null));
		this.#endedAt = animate && next.result && !prev?.result ? 'turn' : null;
		if (animate) this.onreveal?.(next, 0, this.shown);
		else if (live) this.onreveal?.(next, 0, this.shown, true);
		this.#tick();
	}

	/** Jumps to the end of the current roll. */
	finish() {
		clearTimeout(this.#timer);
		if (!this.view) return;
		const from = this.shown;
		this.shown = this.view.last.length;
		if (from < this.shown) this.onreveal?.(this.view, from, this.shown, true);
	}

	/** Stops the timer; call when the page goes away. */
	stop() {
		clearTimeout(this.#timer);
	}

	#tick() {
		if (!this.animating) return;
		this.#timer = setTimeout(() => {
			this.shown += 1;
			this.#revealedAt[this.shown - 1] = this.#now();
			if (this.view) this.onreveal?.(this.view, this.shown - 1, this.shown);
			this.#tick();
		}, this.#wait());
	}
}
