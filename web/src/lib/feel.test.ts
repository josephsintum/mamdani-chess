import { describe, expect, it } from 'vitest';
import { biggerShake, burstShards, captureShake, cellOf, confetti, CONFETTI_MS, countAt, fitShift, coordinates, knockOffset, rippleDelay, SHAKE, shakeFrames, trailColor, trailOf, whipFrames, whiplash } from './feel.ts';

describe('coordinates', () => {
	it('reads 8 to 1 down and a to h across for White', () => {
		expect(coordinates(false)).toEqual({ ranks: ['8', '7', '6', '5', '4', '3', '2', '1'], files: ['a', 'b', 'c', 'd', 'e', 'f', 'g', 'h'] });
	});
	it('reads 1 to 8 down and h to a across for Black', () => {
		expect(coordinates(true)).toEqual({ ranks: ['1', '2', '3', '4', '5', '6', '7', '8'], files: ['h', 'g', 'f', 'e', 'd', 'c', 'b', 'a'] });
	});
});

describe('cellOf', () => {
	it('puts a1 bottom left for White and top right for Black', () => {
		expect(cellOf('a1', false)).toEqual({ col: 0, row: 7 });
		expect(cellOf('a1', true)).toEqual({ col: 7, row: 0 });
		expect(cellOf('e4', false)).toEqual({ col: 4, row: 4 });
	});
});

describe('trailOf', () => {
	it("runs from the start square's centre, in squares and degrees", () => {
		expect(trailOf('a1', 'h1', false)).toEqual({ x: 0.5, y: 7.5, length: 7, angle: 0 });
		const up = trailOf('e2', 'e4', false);
		expect(up).toMatchObject({ x: 4.5, y: 6.5, length: 2 });
		expect(up.angle).toBeCloseTo(-90);
	});
	it('follows the board when it is flipped', () => {
		const t = trailOf('e2', 'e4', true);
		expect(t).toMatchObject({ x: 3.5, y: 1.5, length: 2 });
		expect(t.angle).toBeCloseTo(90);
	});
	it('measures a knight move along its diagonal', () => {
		expect(trailOf('g1', 'f3', false).length).toBeCloseTo(Math.sqrt(5));
	});
});

describe('whiplash', () => {
	it('swings 5 degrees on a move straight across', () => {
		expect(whiplash('a1', 'h1', false)).toBeCloseTo(5);
		expect(whiplash('h1', 'a1', false)).toBeCloseTo(-5);
	});
	it('barely tilts a move straight up or down', () => {
		expect(whiplash('e2', 'e4', false)).toBeCloseTo(0);
	});
	it('scales with how sideways the move is, and flips with the board', () => {
		expect(whiplash('a1', 'h8', false)).toBeCloseTo(5 / Math.SQRT2);
		expect(whiplash('a1', 'h1', true)).toBeCloseTo(-5);
	});
	it('is zero for a piece that did not move', () => {
		expect(whiplash('e4', 'e4', false)).toBe(0);
	});
});

describe('whipFrames', () => {
	it('leans back, whips forward, swings back and settles upright', () => {
		const angles = whipFrames(5).map((k) => k.transform);
		expect(angles).toEqual(['rotate(0deg)', 'rotate(-3deg)', 'rotate(5deg)', 'rotate(-1.5deg)', 'rotate(0deg)']);
	});
});

describe('trailColor', () => {
	it('tints the trail by the piece', () => {
		expect(trailColor('wN')).toBe('var(--trail-white)');
		expect(trailColor('bB')).toBe('var(--trail-black)');
		expect(trailColor('M')).toBe('var(--trail-mamdani)');
	});
});

describe('rippleDelay', () => {
	it('waits 15 ms per square of distance from the picked-up piece', () => {
		expect(rippleDelay('e2', 'e3')).toBe(15);
		expect(rippleDelay('e2', 'e4')).toBe(30);
		expect(rippleDelay('a1', 'h8')).toBe(Math.round(15 * 7 * Math.SQRT2));
	});
	it('pops the nearer squares first, whichever way the board faces', () => {
		const near = rippleDelay('d4', 'e5'),
			far = rippleDelay('d4', 'h8');
		expect(near).toBeLessThan(far);
	});
});

describe('shakes', () => {
	it('shakes harder for a queen or rook than for a minor piece', () => {
		expect(captureShake('bP')).toEqual(SHAKE.minor);
		expect(captureShake('wN')).toEqual(SHAKE.minor);
		expect(captureShake('bQ')).toEqual(SHAKE.major);
		expect(captureShake('wR')).toEqual(SHAKE.major);
	});
	it('lets the bigger shake win', () => {
		expect(biggerShake(SHAKE.minor, SHAKE.fall)).toEqual(SHAKE.fall);
		expect(biggerShake(SHAKE.mate, SHAKE.minor)).toEqual(SHAKE.mate);
		expect(biggerShake(null, SHAKE.major)).toEqual(SHAKE.major);
	});
	it('zigzags, dies away and ends still', () => {
		const frames = shakeFrames(4);
		const xs = frames.map((k) => Number(/translate\((-?[\d.]+)px/.exec(String(k.transform))![1]));
		expect(Math.abs(xs[0])).toBe(4);
		expect(xs.at(-1)).toBe(0);
		for (let i = 1; i < xs.length - 1; i++) expect(Math.sign(xs[i])).toBe(-Math.sign(xs[i - 1]));
		for (let i = 1; i < xs.length; i++) expect(Math.abs(xs[i])).toBeLessThanOrEqual(Math.abs(xs[i - 1]));
	});
});

describe('knockOffset', () => {
	it('knocks the taken piece a third of a square along the move', () => {
		expect(knockOffset('a1', 'h1', false)).toEqual({ x: 33, y: 0 });
		expect(knockOffset('e2', 'e4', false)).toEqual({ x: 0, y: -33 });
		const k = knockOffset('g1', 'f3', false);
		expect(Math.hypot(k.x, k.y)).toBeCloseTo(33, 0);
	});
	it('follows the board when it is flipped', () => {
		expect(knockOffset('a1', 'h1', true)).toEqual({ x: -33, y: 0 });
	});
});

describe('burstShards', () => {
	it('throws 18 shards all the way round, one in three orange', () => {
		const shards = burstShards();
		expect(shards).toHaveLength(18);
		expect(shards.filter((s) => s.hazard)).toHaveLength(6);
		for (const s of shards) {
			expect(s.dist).toBeGreaterThanOrEqual(2);
			expect(s.dist).toBeLessThanOrEqual(5);
		}
		const angles = shards.map((s) => s.angle).sort((a, b) => a - b);
		expect(angles[0]).toBeLessThan(40);
		expect(angles.at(-1)).toBeGreaterThan(320);
	});
	it('is the same burst every time, so a test can pin it', () => {
		expect(burstShards()).toEqual(burstShards());
	});
});

describe('confetti', () => {
	it('throws 80 strips in three colors, a few of them traffic cones', () => {
		const pieces = confetti();
		expect(pieces).toHaveLength(80);
		expect(pieces.filter((c) => c.cone)).toHaveLength(5);
		expect(new Set(pieces.map((c) => c.tone))).toEqual(new Set(['accent', 'hazard', 'cream']));
		expect(confetti()).toEqual(pieces); // seeded: the same every time
	});
	it('lands every piece before it is cleared away', () => {
		for (const c of confetti()) {
			expect(c.x).toBeGreaterThanOrEqual(0);
			expect(c.x).toBeLessThanOrEqual(1);
			expect(c.delay + c.fall).toBeLessThanOrEqual(CONFETTI_MS);
		}
	});
});

describe('countAt', () => {
	it('counts from 0 up to the value, never past it', () => {
		expect(countAt(7, 0)).toBe(0);
		expect(countAt(7, 1)).toBe(7);
		expect(countAt(7, 2)).toBe(7);
		const steps = [0, 0.2, 0.4, 0.6, 0.8, 1].map((t) => countAt(7, t));
		expect(steps).toEqual([...steps].sort((a, b) => a - b));
		expect(countAt(0, 0.5)).toBe(0);
	});
});

describe('fitShift', () => {
	it('leaves a bubble that fits where it is', () => {
		expect(fitShift(180, 100, 0, 360)).toBe(0);
	});
	it('slides a bubble back inside the board, 4 px from the edge', () => {
		// Centred on b-file's square (x = 67) a 240 px bubble would start at -53.
		expect(fitShift(67, 240, 0, 360)).toBe(57);
		expect(fitShift(293, 240, 0, 360)).toBe(-57);
	});
	it('keeps the left edge in when the bubble is wider than the board', () => {
		expect(fitShift(180, 400, 0, 360)).toBe(24);
	});
});
