import { afterEach, describe, expect, it, vi } from 'vitest';
import { followsRematch, isStale, pieceName, showsOffline, trySendMove, type View } from './game.ts';

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

describe('isStale', () => {
	const at = (now: number) => ({ clock: { whiteMs: 0, blackMs: 0, now } }) as View;

	it('drops a view the server built before the one on screen (a slow 409 reply)', () => {
		expect(isStale(at(2000), at(1000))).toBe(true);
	});

	it('keeps newer views, views from the same moment, and the first one', () => {
		expect(isStale(at(1000), at(2000))).toBe(false);
		expect(isStale(at(1000), at(1000))).toBe(false);
		expect(isStale(null, at(1000))).toBe(false);
	});
});

describe('showsOffline', () => {
	const at = (status: View['status'], white: boolean) => ({ status, online: { white, black: true } }) as View;

	it('marks a player who left a game in progress', () => {
		expect(showsOffline(at('playing', false), 'white')).toBe(true);
		expect(showsOffline(at('playing', true), 'white')).toBe(false);
	});

	it('says nothing before the game starts or once it is over', () => {
		expect(showsOffline(at('waiting', false), 'white')).toBe(false);
		expect(showsOffline(at('over', false), 'white')).toBe(false); // gone to the rematch, say
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
		vi.stubGlobal('fetch', async () => Response.json({ error: 'not your turn' }, { status: 403 }));
		expect(await trySendMove('ABC123', move, 0)).toEqual({ refused: 'not your turn' });
	});

	it('counts a gateway error (a deploy in progress) as unsent, so the move is sent again', async () => {
		for (const status of [502, 503, 504]) {
			vi.stubGlobal('fetch', async () => new Response('Bad Gateway', { status }));
			expect(await trySendMove('ABC123', move, 0)).toBe('unsent');
		}
	});

	it('gives a refusal a message even when the response has no status text (HTTP/2)', async () => {
		vi.stubGlobal('fetch', async () => new Response('', { status: 400 }));
		const sent = await trySendMove('ABC123', move, 0);
		expect(typeof sent === 'object' && sent.refused.length > 0).toBe(true);
	});

	it('hands back the current state a conflict carries, so the page can resync at once', async () => {
		const state = { seq: 5, code: 'ABC123' };
		vi.stubGlobal('fetch', async () => Response.json({ error: 'the game has moved on', state }, { status: 409 }));
		expect(await trySendMove('ABC123', move, 4)).toEqual({ refused: 'the game has moved on', state });
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
