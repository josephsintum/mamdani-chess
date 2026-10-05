import { afterEach, describe, expect, it, vi } from 'vitest';
import { followsRematch, isStale, joinNotice, matchCard, pieceName, showsOffline, trySendMove, type View } from './game.ts';

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

describe('joinNotice', () => {
	const at = (you: View['you'], status: View['status'], seq = 0) =>
		({ you, status, seq, players: { white: 'pizza-rat-astoria', black: status === 'waiting' ? '' : 'bagel-soho' } }) as View;

	it('tells White when their friend sits down', () => {
		expect(joinNotice(at('white', 'waiting'), at('white', 'playing'), false)).toBe("bagel-soho joined · You're White, your move");
	});

	it('tells Black whose game they joined on arrival', () => {
		expect(joinNotice(null, at('black', 'playing'), false)).toBe("You joined pizza-rat-astoria · You're Black");
	});

	it("tells White their color when they arrive at a started game (a rematch swaps colors)", () => {
		expect(joinNotice(null, at('white', 'playing'), false)).toBe("Playing bagel-soho · You're White, your move");
	});

	it('says nothing after a quick match: the opponent-found screen did', () => {
		expect(joinNotice(null, at('black', 'playing'), true)).toBe('');
		expect(joinNotice(null, at('white', 'playing'), true)).toBe('');
	});

	it('says nothing to spectators, while waiting, or once moves are made', () => {
		expect(joinNotice(null, at('spectator', 'playing'), false)).toBe('');
		expect(joinNotice(null, at('white', 'waiting'), false)).toBe('');
		expect(joinNotice(null, at('black', 'playing', 3), false)).toBe('');
		expect(joinNotice(at('black', 'playing'), at('black', 'playing'), false)).toBe('');
	});

	it('falls back when a name is missing', () => {
		const nameless = { you: 'white', status: 'playing', seq: 0, players: { white: '', black: '' } } as View;
		expect(joinNotice({ ...nameless, status: 'waiting' } as View, nameless, false)).toBe("Your friend joined · You're White, your move");
	});
});

describe('matchCard', () => {
	it("puts you first, with each side's color", () => {
		const view = { you: 'black', players: { white: 'pizza-rat-astoria', black: 'bagel-soho' } } as View;
		expect(matchCard(view)).toEqual({ you: { name: 'bagel-soho', color: 'black' }, them: { name: 'pizza-rat-astoria', color: 'white' } });
	});
});
