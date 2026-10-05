// Types and calls for the game API. Mirrors game/view.go on the server.

import type { ClockJSON } from './clock.ts';

export type Color = 'white' | 'black';

export interface MoveJSON {
	from: string;
	to: string;
	promo?: string;
}

export interface EventJSON {
	kind: string;
	sq?: string;
	from?: string;
	to?: string;
	promo?: string;
	piece?: string;
	color?: Color;
	roll?: number;
	saved?: boolean;
	reason?: string;
}

export interface View {
	code: string;
	status: 'waiting' | 'playing' | 'over';
	you: Color | 'spectator';
	board: string[]; // 64 entries, index 0 = a1; "" or "wP", "bQ", ...
	mamdani: string; // "" once it has fallen
	potholes: { sq: string; by: Color }[];
	turn: Color;
	check: boolean;
	legal: MoveJSON[];
	last: EventJSON[];
	log: LogEntry[];
	lost: { white: string[]; black: string[] };
	stats: { savingRolls: number; saved: number; repaired: number; mamdaniFell: boolean };
	result: { winner?: Color; draw: boolean; reason: string } | null;
	seq: number;
	clock: ClockJSON;
	/** Which players have the game open. */
	online: { white: boolean; black: boolean };
	/** Once the game is over: an offer waiting, a declined offer, or the new game's code. */
	rematch: { offer?: Color; declined?: boolean; code?: string };
}

export interface LogEntry {
	san: string;
	color: Color;
	dice: string;
}

export function squareName(index: number): string {
	return 'abcdefgh'[index % 8] + (Math.floor(index / 8) + 1);
}

/** Starts a friend game with the caller as White; returns its code. */
export async function createGame(): Promise<string> {
	const res = await fetch('/api/games', { method: 'POST' });
	if (!res.ok) throw new Error(`could not create a game (${res.status})`);
	return (await res.json()).code;
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
	const body: { error?: string; state?: View } = await res.json().catch(() => ({}));
	const refused: { refused: string; state?: View } = {
		refused: body.error || res.statusText || 'The server refused the move.'
	};
	if (body.state) refused.state = body.state;
	return refused;
}

/**
 * Whether a player's page should go to the rematch: only when it starts
 * while the page is open. A finished game opened later (Back, or the old
 * link) stays put and offers a link instead.
 */
export function followsRematch(prev: View | null, next: View): boolean {
	const player = next.you === 'white' || next.you === 'black';
	return player && !!next.rematch.code && prev !== null && !prev.rematch.code;
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

async function post(path: string, body: unknown): Promise<string | null> {
	const res = await fetch(path, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(body)
	});
	if (res.ok) return null;
	try {
		return (await res.json()).error ?? res.statusText;
	} catch {
		return res.statusText;
	}
}

const pieceNames: Record<string, string> = {
	P: 'pawn',
	N: 'knight',
	B: 'bishop',
	R: 'rook',
	Q: 'queen',
	K: 'king'
};

/** Ends the game; the caller's opponent wins. Returns null or the error. */
export async function resign(code: string): Promise<string | null> {
	const res = await fetch(`/api/games/${code}/resign`, { method: 'POST' });
	if (res.ok) return null;
	try {
		return (await res.json()).error ?? res.statusText;
	} catch {
		return res.statusText;
	}
}

/** "white knight", "the Mamdani". */
export function pieceName(code = ''): string {
	if (code === 'M') return 'the Mamdani';
	const color = code[0] === 'w' ? 'white' : 'black';
	return `${color} ${pieceNames[code[1]] ?? 'piece'}`;
}

const rerollReasons: Record<string, string> = {
	king: 'kings never fall',
	pothole: 'already a pothole',
	exposes: 'would expose the roller’s king',
	checkmate: 'would decide the game'
};

/** One plain-English line per turn event. */
export function eventText(e: EventJSON): string {
	switch (e.kind) {
		case 'moved':
			return `${e.color} moves ${pieceName(e.piece)} ${e.from}–${e.to}`;
		case 'captured':
			return `${pieceName(e.piece)} captured`;
		case 'pothole_closed':
			return `pothole on ${e.sq} closes`;
		case 'repaired':
			return `the Mamdani repairs ${e.sq}`;
		case 'rolled_pothole':
			return `pothole roll: ${e.roll} — ${(e.roll ?? 1) % 2 === 0 ? 'a pothole opens' : 'nothing happens'}`;
		case 'target':
			return `the dice pick ${e.sq}`;
		case 'reroll':
			return `re-roll: ${rerollReasons[e.reason ?? ''] ?? e.reason}`;
		case 'saving_roll':
			return `saving roll for ${pieceName(e.piece)}: ${e.roll} — ${e.saved ? 'saved' : 'lost'}`;
		case 'fell':
			return `${pieceName(e.piece)} falls into ${e.sq}`;
		case 'pothole_opened':
			return `pothole opens on ${e.sq}`;
		case 'no_pothole':
			return 'no valid square: no pothole this turn';
	}
	return e.kind;
}

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

export function resultText(r: NonNullable<View['result']>): string {
	const why = reasons[r.reason] ?? r.reason;
	return r.draw ? `Draw by ${why}` : `${r.winner === 'white' ? 'White' : 'Black'} wins by ${why}`;
}
