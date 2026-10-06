// Calls for finding games: the guest's name, the live games list and game
// codes. Mirrors server/me.go and game/live.go.

import type { MoveJSON, View } from './game.ts';

/** A game being played, as the home page lists it. */
export interface LiveGame {
	code: string;
	white: string;
	black: string;
	move: number;
	board: string[]; // 64 entries, index 0 = a1
	mamdani: string;
	potholes: View['potholes'];
	last: MoveJSON | null;
	watching: number;
}

export interface LiveGames {
	games: LiveGame[];
	/** Guests waiting in quick match. */
	looking: number;
}

/**
 * The caller's name (null until they first play) and their name changes:
 * 3 in any 24 hours. changesResetAt (Unix ms) is when they get 3 again, or
 * null while none are used. Mirrors server/me.go.
 */
export interface Me {
	name: string | null;
	changesLeft: number;
	changesResetAt: number | null;
	/** The code of the game the caller is playing right now, if any. */
	game?: string;
}

/** A refused API call, with its HTTP status (409 stale or taken, 429 none left…). */
export class ApiError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
	}
}

/** Name changes a guest gets in any 24 hours (store.NameChanges on the server). */
export const NAME_CHANGES = 3;

// The name the server last gave this browser, kept so the header can show it
// before /api/me answers. The server stays the truth: every answer replaces it.
const NAME_KEY = 'name';

/** This browser's name from its last visit, or null (none yet, or storage blocked). */
export function rememberedName(): string | null {
	try {
		return localStorage.getItem(NAME_KEY);
	} catch {
		return null;
	}
}

function remember(name: string | null) {
	try {
		if (name) localStorage.setItem(NAME_KEY, name);
		else localStorage.removeItem(NAME_KEY);
	} catch {
		// Private mode or blocked storage: the header waits for the server.
	}
}

export async function me(): Promise<Me> {
	const res = await fetch('/api/me');
	if (!res.ok) throw new Error(`could not load your name (${res.status})`);
	const m: Me = await res.json();
	remember(m.name);
	return m;
}

/** The names on offer, or none left today (with when they come back). */
export type Offers = { names: string[]; changesLeft: number } | { resetAt: number };

/** The names the caller may change to; the same ones until one is chosen. */
export async function nameOffers(): Promise<Offers> {
	const res = await fetch('/api/me/names');
	const body = await res.json().catch(() => ({}));
	if (res.status === 429) return { resetAt: body.changesResetAt };
	if (!res.ok) throw new Error(body.error ?? `could not load names (${res.status})`);
	return { names: body.names, changesLeft: body.changesLeft };
}

/** Changes the caller's name to one of their offers; returns the new state. */
export async function chooseName(name: string): Promise<Me> {
	const res = await fetch('/api/me/name', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ name })
	});
	const body = await res.json().catch(() => ({}));
	if (!res.ok) throw new ApiError(body.error ?? `could not change your name (${res.status})`, res.status);
	remember(body.name);
	return body;
}

/** How long until resetAt, for "New names again in 5 h": hours or minutes, rounded up. */
export function untilText(resetAt: number, now: number): string {
	const mins = Math.max(1, Math.ceil((resetAt - now) / 60_000));
	return mins >= 60 ? `${Math.ceil(mins / 60)} h` : `${mins} min`;
}

export async function liveGames(): Promise<LiveGames> {
	const res = await fetch('/api/games');
	if (!res.ok) throw new Error(`could not load live games (${res.status})`);
	return res.json();
}

/** Game codes use these characters: no lookalikes (0 O 1 I L). See game/dice.go. */
export const CODE_ALPHABET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';

/**
 * What a typed or pasted code becomes: upper case, only code characters, at
 * most 6. A pasted game link ("https://…/game/K7F3QZ") gives its code.
 */
export function normalizeCode(input: string): string {
	const link = input.match(/\/game\/([^/?#\s]*)/i);
	return [...(link ? link[1] : input).toUpperCase()]
		.filter((c) => CODE_ALPHABET.includes(c))
		.join('')
		.slice(0, 6);
}

export function isCode(code: string): boolean {
	return code.length === 6 && normalizeCode(code) === code;
}

/** Two letters for a name's badge: its first and last words. "astoria-pizza-rat" → "AR". */
export function initials(name: string): string {
	const words = name.split('-').filter(Boolean);
	if (words.length === 0) return '';
	const first = words[0][0];
	return (words.length > 1 ? first + words[words.length - 1][0] : first).toUpperCase();
}

/** A board from a FEN's piece placement ("rnbqkbnr/pppppppp/8/…"), index 0 = a1. */
export function boardFromFen(placement: string): string[] {
	const board: string[] = Array(64).fill('');
	placement.split('/').forEach((row, i) => {
		let file = 0;
		for (const ch of row) {
			if (/\d/.test(ch)) {
				file += Number(ch);
				continue;
			}
			const color = ch === ch.toUpperCase() ? 'w' : 'b';
			board[(7 - i) * 8 + file] = color + ch.toUpperCase();
			file++;
		}
	});
	return board;
}

/** Time spent waiting, counting up: "0:07", "1:30", "12:05". */
export function formatElapsed(ms: number): string {
	const secs = Math.max(0, Math.floor(ms / 1000));
	return `${Math.floor(secs / 60)}:${String(secs % 60).padStart(2, '0')}`;
}
