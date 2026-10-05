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

/** The caller's name, or null if they haven't played yet. */
export async function myName(): Promise<string | null> {
	const res = await fetch('/api/me');
	if (!res.ok) throw new Error(`could not load your name (${res.status})`);
	return (await res.json()).name;
}

/** Draws a new name for the caller and returns it. */
export async function rerollName(): Promise<string> {
	const res = await fetch('/api/me/name', { method: 'POST' });
	if (!res.ok) throw new Error(`could not change your name (${res.status})`);
	return (await res.json()).name;
}

export async function liveGames(): Promise<LiveGames> {
	const res = await fetch('/api/games');
	if (!res.ok) throw new Error(`could not load live games (${res.status})`);
	return res.json();
}

/** Game codes use these characters: no lookalikes (0 O 1 I L). See game/dice.go. */
export const CODE_ALPHABET = 'ABCDEFGHJKMNPQRSTUVWXYZ23456789';

/** What a typed code becomes: upper case, only code characters, at most 6. */
export function normalizeCode(input: string): string {
	return [...input.toUpperCase()]
		.filter((c) => CODE_ALPHABET.includes(c))
		.join('')
		.slice(0, 6);
}

export function isCode(code: string): boolean {
	return code.length === 6 && normalizeCode(code) === code;
}

/** Two letters for a name's badge: its first and last words. "pizza-rat-astoria" → "PA". */
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
