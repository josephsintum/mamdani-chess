import { describe, expect, it } from 'vitest';
import { pieceOn } from './board.ts';
import { freeMoves } from './sandbox.ts';
import { framesOf, scenes, type Scene } from './scenes.ts';

const holes = (s: Scene) => framesOf(s).at(-1)!.view.potholes.map((h) => h.sq);
const end = (s: Scene) => framesOf(s).at(-1)!.view;

describe('scenes', () => {
	it.each<Scene>(Object.values(scenes))('$id plays only moves the pieces can make, and shows only pieces', (scene) => {
		const frames = framesOf(scene);
		for (const [i, step] of scene.steps.entries()) {
			const before = frames[i].view;
			if ('show' in step) {
				expect(pieceOn(before, step.show), step.show).not.toBe('');
			} else {
				expect(freeMoves(before, true), `${step.move.from}-${step.move.to}`).toContainEqual(step.move);
			}
		}
	});

	it('numbers seq from where it is told to, so each run is new', () => {
		const frames = framesOf(scenes.dice, 3000);
		expect(frames.map((f) => f.view.seq)).toEqual([3000, 3001, 3002, 3003]);
	});

	it('dice: an odd roll does nothing, the knight falls, a king is re-rolled', () => {
		const v = end(scenes.dice);
		expect(v.board[21]).toBe(''); // f3
		expect(v.lost.white).toEqual(['wN']);
		expect(v.last.some((e) => e.kind === 'reroll' && e.sq === 'e8')).toBe(true);
		expect(holes(scenes.dice)).toEqual(['f3', 'h6']);
	});

	it('roads: the rook stops at the pothole and the knight jumps it', () => {
		const start = framesOf(scenes.roads)[0].view;
		const rook = freeMoves(start, true).filter((m) => m.from === 'a3').map((m) => m.to);
		expect(rook).toContain('c3');
		expect(rook).not.toContain('e3');
		expect(pieceOn(end(scenes.roads), 'e4')).toBe('wN');
	});

	it('rounds: c4 runs out, then the cap closes the oldest', () => {
		expect(holes(scenes.rounds)).toEqual(['f3', 'b6', 'd7', 'a3', 'h5']);
		expect(end(scenes.rounds).last.some((e) => e.kind === 'pothole_closed' && e.sq === 'e5')).toBe(true);
	});

	it('blocks: the Mamdani on d2 cuts the bishop off from the king', () => {
		const v = end(scenes.blocks);
		expect(v.mamdani).toBe('d2');
		expect(v.check).toBe(false);
		expect(freeMoves(v, true).filter((m) => m.from === 'b4').map((m) => m.to)).not.toContain('e1');
	});

	it('repairs: both potholes are repaired', () => {
		const v = end(scenes.repairs);
		expect(v.potholes).toEqual([]);
		expect(v.stats.repaired).toBe(2);
	});

	it('saving: the bishop is saved and the knight falls', () => {
		const v = end(scenes.saving);
		expect(pieceOn(v, 'c3')).toBe('wB');
		expect(pieceOn(v, 'd8')).toBe('');
		expect(v.stats).toMatchObject({ savingRolls: 2, saved: 1 });
	});

	it('mate: a pothole on g7 mates the king on h8', () => {
		const v = end(scenes.mate);
		expect(holes(scenes.mate)).toEqual(['g7']);
		expect(v.result).toEqual({ winner: 'white', draw: false, reason: 'checkmate' });
		// Its one step, g8, is on the checking rook's line, and the Mamdani is boxed in.
		const moves = freeMoves(v, true);
		expect(moves.filter((m) => m.from === 'h8').map((m) => m.to)).toEqual(['g8']);
		expect(moves.filter((m) => m.from === 'a8').map((m) => m.to)).toContain('g8');
		expect(moves.filter((m) => m.from === 'h1')).toEqual([]);
	});
});
