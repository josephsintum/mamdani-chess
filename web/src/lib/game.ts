// Types and calls for the game API. Mirrors game/view.go on the server.

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
	log: string[];
	result: { winner?: Color; draw: boolean; reason: string } | null;
	seq: number;
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

/** Sends a move. Returns null on success, or the server's error message. */
export async function sendMove(code: string, move: MoveJSON, seq: number): Promise<string | null> {
	const res = await fetch(`/api/games/${code}/move`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ ...move, seq })
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

const reasons: Record<string, string> = {
	checkmate: 'checkmate',
	stalemate: 'stalemate',
	fifty_moves: 'the 50-move rule',
	repetition: 'threefold repetition',
	insufficient_material: 'insufficient material'
};

export function resultText(r: NonNullable<View['result']>): string {
	const why = reasons[r.reason] ?? r.reason;
	return r.draw ? `Draw by ${why}` : `${r.winner === 'white' ? 'White' : 'Black'} wins by ${why}`;
}
