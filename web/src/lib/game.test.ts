import { afterEach, describe, expect, it, vi } from 'vitest';
import { followsRematch, pieceName, trySendMove, type View } from './game.ts';

function view(you: View['you'], code?: string): View {
	return { you, rematch: code ? { code } : {} } as View;
}

describe('followsRematch', () => {
	it('follows a rematch that starts while the page is open', () => {
		expect(followsRematch(view('white'), view('white', 'NEW123'))).toBe(true);
	});

	it('stays on a finished game opened after its rematch began (Back, or the old link)', () => {
		expect(followsRematch(null, view('white', 'NEW123'))).toBe(false);
		expect(followsRematch(view('black', 'NEW123'), view('black', 'NEW123'))).toBe(false);
	});

	it('never moves a spectator', () => {
		expect(followsRematch(view('spectator'), view('spectator', 'NEW123'))).toBe(false);
	});
});

describe('trySendMove', () => {
	afterEach(() => vi.unstubAllGlobals());
	const move = { from: 'e2', to: 'e4' };

	it('reports a move the server took', async () => {
		vi.stubGlobal('fetch', async () => new Response(null, { status: 204 }));
		expect(await trySendMove('ABC123', move, 0)).toBe('sent');
	});

	it('reports a refusal with the server’s reason', async () => {
		vi.stubGlobal('fetch', async () => Response.json({ error: 'not your turn' }, { status: 409 }));
		expect(await trySendMove('ABC123', move, 0)).toEqual({ refused: 'not your turn' });
	});

	// The page decides from this outcome alone, never from an error left on
	// screen by an earlier refusal, so a move sent offline is always kept.
	it('reports a move that never reached the server as unsent', async () => {
		vi.stubGlobal('fetch', async () => {
			throw new TypeError('Failed to fetch');
		});
		expect(await trySendMove('ABC123', move, 0)).toBe('unsent');
	});
});

describe('pieceName', () => {
	it('names a piece for screen readers', () => {
		expect(pieceName('bN')).toBe('black knight');
		expect(pieceName('wQ')).toBe('white queen');
		expect(pieceName('M')).toBe('the Mamdani');
	});
});
