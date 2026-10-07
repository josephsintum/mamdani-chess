// Calls for the game API, and what the page says about a game. The JSON
// types are generated from the server's Go types (wire.gen.ts, written by
// go run ./cmd/wiregen), so they can't drift from it.

export type { Color, EventJSON, LogEntry, MoveJSON, View } from './wire.gen.ts';
import type { Color, MoveJSON, View } from './wire.gen.ts';
import { jsonOf, post } from './api.ts';

export function squareName(index: number): string {
	return 'abcdefgh'[index % 8] + (Math.floor(index / 8) + 1);
}

/** "White" or "Black". */
export const sideName = (c: Color) => (c === 'white' ? 'White' : 'Black');

export const opponent = (c: Color): Color => (c === 'white' ? 'black' : 'white');

/** Whether the viewer has a seat (rather than watching). */
export const isPlayer = (you: View['you']): you is Color => you === 'white' || you === 'black';

export const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);

/** Starts a friend game with the caller as White; returns its code. */
export async function createGame(): Promise<string> {
	const res = await fetch('/api/games', { method: 'POST' });
	if (!res.ok) throw new Error(`could not create a game (${res.status})`);
	return ((await res.json()) as { code: string }).code;
}

/** Starts a practice game, the caller playing both sides; returns its code. */
export async function createPractice(): Promise<string> {
	const res = await fetch('/api/practice', { method: 'POST' });
	if (!res.ok) throw new Error(`could not start a practice game (${res.status})`);
	return ((await res.json()) as { code: string }).code;
}

/** A game as anyone sees it (GET never takes a seat); throws if it can't be had. */
export async function fetchView(code: string): Promise<View> {
	const res = await fetch(`/api/games/${code}`);
	if (!res.ok) throw new Error(`could not load the game (${res.status})`);
	return (await res.json()) as View;
}

/**
 * What became of a move: the server took it, refused it (why, and the
 * game's current state when it sends one, as a 409 does), or never got it.
 * A move that never got there can be sent again with the same seq: the
 * server refuses a copy of a move it already has.
 */
export type SendOutcome = 'sent' | 'unsent' | { refused: string; state?: View };

/** Sends a move and says what became of it, without throwing. */
export async function trySendMove(code: string, move: MoveJSON, seq: number): Promise<SendOutcome> {
	let res: Response;
	try {
		res = await fetch(`/api/games/${code}/move`, {
			method: 'POST',
			headers: { 'content-type': 'application/json' },
			body: JSON.stringify({ ...move, seq })
		});
	} catch {
		return 'unsent';
	}
	if (res.ok) return 'sent';
	// A gateway error means the server is restarting (a deploy): it never
	// saw the move, so send it again. A 500 is the game's own error: not that.
	if (res.status >= 502 && res.status <= 504) return 'unsent';
	const body = await jsonOf<{ error: string; state: View }>(res);
	const refused = body.error || res.statusText || 'The server refused the move.';
	return body.state ? { refused, state: body.state } : { refused };
}

/**
 * Whether a player's page should go to the rematch: only when it starts
 * while the page is open. A finished game opened later (Back, or the old
 * link) stays put and offers a link instead.
 */
export function followsRematch(prev: View | null, next: View): boolean {
	return isPlayer(next.you) && !!next.rematch.code && prev !== null && !prev.rematch.code;
}

/**
 * Whether a view is older than the one on screen: the server built it
 * earlier (clock.now), as a 409's state can be when a newer stream update
 * overtakes the HTTP reply. Showing it would roll the board back.
 */
export function isStale(current: View | null, next: View): boolean {
	return current !== null && next.clock.now < current.clock.now;
}

/**
 * Whether a player's bar says they're disconnected: only while the game is
 * on. Before it starts there's no one to wait for, and after it ends the
 * players have usually moved on (to the rematch, say).
 */
export function showsOffline(view: View, color: Color): boolean {
	return view.status === 'playing' && !view.online[color];
}

/** Offers or accepts a rematch, or declines one. Returns null or the error. */
export async function rematch(code: string, decline = false): Promise<string | null> {
	return post(`/api/games/${code}/rematch`, { decline });
}

/** Whether the game still exists: false only when the server says it's gone. */
export async function gameExists(code: string): Promise<boolean> {
	try {
		return (await fetch(`/api/games/${code}`)).status !== 404;
	} catch {
		return true; // can't tell: the server may be restarting
	}
}

/** Ends the game; the caller's opponent wins. Returns null or the error. */
export async function resign(code: string): Promise<string | null> {
	return post(`/api/games/${code}/resign`);
}

const pieceNames: Record<string, string> = {
	P: 'pawn',
	N: 'knight',
	B: 'bishop',
	R: 'rook',
	Q: 'queen',
	K: 'king'
};

/** "white knight", "the Mamdani". */
export function pieceName(code = ''): string {
	if (code === 'M') return 'the Mamdani';
	const color = code[0] === 'w' ? 'white' : 'black';
	return `${color} ${pieceNames[code[1]] ?? 'piece'}`;
}

/** Why the dice picked again (a reroll event's reason). */
export const rerollReasons: Record<string, string> = {
	king: 'kings never fall',
	pothole: 'already a pothole',
	exposes: 'would expose the roller’s king'
};

export const reasons: Record<string, string> = {
	checkmate: 'checkmate',
	resignation: 'resignation',
	timeout: 'timeout',
	timeout_vs_insufficient: 'timeout, no mate possible',
	aborted: 'no first move',
	expired: 'nobody joining',
	stalemate: 'stalemate',
	fifty_moves: 'the 50-move rule',
	repetition: 'threefold repetition',
	insufficient_material: 'insufficient material'
};

/**
 * What to tell a player as a game starts, or "" for nothing: White hears
 * that their friend sat down; a player arriving at a game not yet moved in
 * by a friend's link hears their color and opponent. A rematch (fromRematch)
 * says so, with the swapped color. After a quick match the opponent-found
 * screen already said it, so fromMatch silences the arrival notice.
 */
export function joinNotice(prev: View | null, next: View, fromMatch: boolean, fromRematch = false): string {
	if (next.you === 'spectator' || next.status !== 'playing' || next.seq !== 0) return '';
	// A rematch swaps colours: say so, rather than "You joined".
	if (fromRematch && prev === null) return next.you === 'black' ? "Rematch · You're Black now" : "Rematch · You're White, your move";
	const them = next.players[opponent(next.you)];
	if (prev?.status === 'waiting' && next.you === 'white') {
		return `${them || 'Your friend'} joined · You're White, your move`;
	}
	if (prev !== null || fromMatch) return '';
	return next.you === 'black'
		? `You joined ${them || 'the game'} · You're Black`
		: `Playing ${them || 'your opponent'} · You're White, your move`;
}

export interface MatchSide {
	name: string;
	color: Color;
}

export interface MatchCard {
	you: MatchSide;
	them: MatchSide;
}

/** The two sides of a matched game for the opponent-found screen, you first. */
export function matchCard(view: View): MatchCard {
	const mine: Color = view.you === 'black' ? 'black' : 'white';
	const theirs = opponent(mine);
	return {
		you: { name: view.players[mine] || sideName(mine), color: mine },
		them: { name: view.players[theirs] || sideName(theirs), color: theirs }
	};
}
