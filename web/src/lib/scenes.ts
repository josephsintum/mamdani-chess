// The how-to-play page's scenes: short scripted games played on the real
// board (ScriptedBoard), each showing one way this chess differs. A scene
// is a position and a few steps; sandbox.ts plays each turn as the server
// would send it, so every effect is the one players see in a game.

import { boardFromFen } from './lobby.ts';
import { emptyView, playTurn, type RollScript } from './sandbox.ts';
import type { Color, MoveJSON, Pothole, View } from './wire.gen.ts';

export type Step =
	/** A turn: the move, then the dice (odd by default). `set` changes the view after it, for what the sandbox doesn't work out: check and checkmate. */
	| { move: MoveJSON; roll?: RollScript; set?: Partial<View>; hold?: number }
	/** A piece's moves shown as if picked up. */
	| { show: string; hold?: number };

export interface Scene {
	id: string;
	start: () => View;
	steps: Step[];
}

/** One frame of a scene: the view on the board, a piece shown picked up, and how long to stay after the dice settle. */
export interface Frame {
	view: View;
	show: string;
	hold: number;
}

const START_HOLD = 1200;
const TURN_HOLD = 1000;
const SHOW_HOLD = 1800;

function position(placement: string, { mamdani = '', potholes = [], turn = 'white' }: { mamdani?: string; potholes?: Pothole[]; turn?: Color } = {}): View {
	return { ...emptyView(), you: 'spectator', board: boardFromFen(placement), mamdani, potholes, turn };
}

const hole = (sq: string, by: Color, left: number): Pothole => ({ sq, by, left });
const m = (from: string, to: string): MoveJSON => ({ from, to });

export const scenes = {
	/** Odd does nothing; even opens a pothole and the piece there falls in; kings never fall. */
	dice: {
		id: 'dice',
		start: () => position('r1bqkbnr/pppp1ppp/2n5/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R', { mamdani: 'a5' }),
		steps: [
			{ move: m('f1', 'c4'), roll: { pothole: 3 } },
			{ move: m('g8', 'f6'), roll: { pothole: 6, target: 'f3' }, hold: 1600 },
			{ move: m('d2', 'd3'), roll: { pothole: 4, rerolls: ['e8'], target: 'h6' }, hold: 1600 }
		]
	},
	/** Sliders stop at a pothole; a knight jumps over it. */
	roads: {
		id: 'roads',
		start: () => position('6k1/5ppp/8/8/8/R7/3N1PPP/6K1', { mamdani: 'h4', potholes: [hole('d3', 'black', 3)] }),
		steps: [{ show: 'a3' }, { show: 'd2' }, { move: m('d2', 'e4'), hold: 1400 }]
	},
	/** Cones count a pothole's rounds down; a sixth closes the oldest; a roll onto a hole resets it. */
	rounds: {
		id: 'rounds',
		start: () =>
			position('6k1/6pp/8/8/8/8/6PP/6K1', {
				mamdani: 'a8',
				potholes: [hole('c4', 'white', 1), hole('e5', 'black', 3), hole('f3', 'white', 3), hole('b6', 'black', 3), hole('d7', 'white', 3)]
			}),
		steps: [
			{ move: m('h2', 'h3'), roll: { pothole: 3 } },
			{ move: m('h7', 'h6'), roll: { pothole: 6, target: 'a3' } },
			{ move: m('g2', 'g3'), roll: { pothole: 4, target: 'h5' }, hold: 1600 },
			{ move: m('g7', 'g5'), roll: { pothole: 2, target: 'f3' }, hold: 1600 }
		]
	},
	/** The Mamdani blocks lines like a piece: here a bishop's check. */
	blocks: {
		id: 'blocks',
		start: () => ({ ...position('4k3/8/8/8/1b6/8/5PPP/4K2R', { mamdani: 'f4' }), check: true }),
		// The Mamdani's own reach isn't shown here: while in check, only the
		// moves that answer it are legal, and the sandbox doesn't know that.
		steps: [{ show: 'b4' }, { move: m('f4', 'd2'), set: { check: false } }, { show: 'b4' }]
	},
	/** The Mamdani repairs a pothole next to it, and one the dice open there. */
	repairs: {
		id: 'repairs',
		start: () => position('4k3/1p4p1/8/8/8/8/1P4P1/4K3', { mamdani: 'c3', potholes: [hole('f6', 'black', 3)], turn: 'black' }),
		steps: [
			{ move: m('c3', 'e5'), hold: 2600 },
			{ move: m('b2', 'b3'), roll: { pothole: 4, target: 'd4' }, hold: 2600 }
		]
	},
	/** A piece the Mamdani has a clear line to gets a saving roll: odd saves it. */
	saving: {
		id: 'saving',
		start: () => position('3n2k1/5ppp/8/8/8/2B5/5PPP/6K1', { mamdani: 'a5' }),
		steps: [
			{ move: m('h2', 'h3'), roll: { pothole: 6, target: 'c3', save: 5 }, hold: 1600 },
			{ move: m('h7', 'h6'), roll: { pothole: 2, target: 'd8', save: 4 }, hold: 1600 }
		]
	},
	/** A pothole on a checked king's last way out is checkmate. */
	mate: {
		id: 'mate',
		start: () => position('7k/7p/8/8/8/8/6PP/R5K1', { mamdani: 'h1' }),
		steps: [
			{
				move: m('a1', 'a8'),
				roll: { pothole: 4, target: 'g7' },
				set: { check: true, status: 'over', result: { winner: 'white', draw: false, reason: 'checkmate' } },
				hold: 3000
			}
		]
	}
} satisfies Record<string, Scene>;

/**
 * A scene's frames, its seq numbers starting at `seq` (so its repair
 * celebrations, keyed by seq, play again on every run).
 */
export function framesOf(scene: Scene, seq = 0): Frame[] {
	let view: View = { ...scene.start(), seq };
	const frames: Frame[] = [{ view, show: '', hold: START_HOLD }];
	for (const step of scene.steps) {
		if ('show' in step) {
			frames.push({ view, show: step.show, hold: step.hold ?? SHOW_HOLD });
			continue;
		}
		view = { ...playTurn(view, step.move, step.roll ?? { pothole: 1 }), ...step.set };
		frames.push({ view, show: '', hold: step.hold ?? TURN_HOLD });
	}
	return frames;
}
