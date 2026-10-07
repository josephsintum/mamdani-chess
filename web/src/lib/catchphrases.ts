// What the board says at big moments: a speech bubble from the square, with
// an emoji and a line (spec: board feel, section 8). The lines are the game
// piece's playful voice, never presented as real quotes.

import { kingSquare, matedByRoll, stageAt } from './board.ts';
import { FALL_MS } from './feel.ts';
import { opponent, type EventJSON, type View } from './game.ts';

export type EndKind =
	| 'castleMate'
	| 'promoMate'
	| 'speedrun'
	| 'twoLeft'
	| 'byAHair'
	| 'queenless'
	| 'mateByRoll'
	| 'mateByMove'
	| 'timeWin'
	| 'resigned'
	| 'draw';
/** A finished game's line, on the king at `sq`; `afterBurst`: it waits for the mate's burst. */
export type EndQuip = { kind: EndKind; sq: string; afterBurst?: true };
export type QuipKind = 'mamdaniFell' | 'repaired' | 'queenFell' | 'saved' | 'kingSpared' | 'sixSeven' | 'potholeSeason' | 'fifthRepair' | 'wipedOut' | EndKind;

// New York sayings, subway announcements, the city's pothole work, and Gen Z
// and Gen Alpha slang (the shortlist, 2026-10-06).
export const QUIPS: Record<QuipKind, { emoji: string; lines: string[] }> = {
	mamdaniFell: {
		emoji: '😢',
		lines: [
			'Sorry, my wife is calling.',
			'Got paperwork to do.',
			'Ask ~bat~ Bruce Wayne for help.',
			'Someone call 311.',
			"I'm fallin' here!",
			'Stand clear of the closing hole.',
			"This one's not on the list.",
			"Tell DOT I'll be late.",
			"Not for nothin', but ow.",
			'Did anybody see that?',
			"It's so over.",
			'Mogged by a pothole.',
			'−1000 aura.'
		]
	},
	repaired: {
		emoji: '👍',
		lines: [
			'Filled. Next!',
			'Another one off the list.',
			'Pothole Blitz!',
			"That's 200,001.",
			'Smooth as a fresh schmear.',
			'Fuhgeddaboudit.',
			"We're so back.",
			'+1000 aura.',
			'Ate. No crumbs.'
		]
	},
	queenFell: { emoji: '😱', lines: ['Not the queen!', 'Watch the gap!', 'Gone to Queens.', 'Finally, some action!', "Chat, we're cooked.", 'Massive L.'] },
	saved: { emoji: '😅', lines: ['That was close.', "It's showtime!", "I'm walkin' here!", 'Deadass close.', 'Clutch.', 'No cap, that was close.'] },
	kingSpared: { emoji: '🛡️', lines: ['Not on my block.', 'Fuhgeddaboudit.', 'Main character energy.'] },
	// The pothole's file and rank dice landed on 6 and 7: the "six-seven" meme.
	sixSeven: { emoji: '🤷', lines: ['6-7'] },
	timeWin: { emoji: '⏱️', lines: ['Missed the train.', 'This is the last stop.', 'Get a move on!', 'Skill issue.'] },
	mateByRoll: { emoji: '🗽', lines: ['Only in New York.', 'Somebody call the papers.', 'Chat, is this real?'] },
	mateByMove: { emoji: '🏁', lines: ['Fuhgeddaboudit.', 'Ate. No crumbs.', "That's a W.", 'Gotta be quicker than that. This is New York.'] },
	resigned: { emoji: '🏳️', lines: ['Somebody call a cab.', "It's so over.", 'Touched grass.'] },
	draw: { emoji: '🤝', lines: ['Mid.', '6-7 🤷', 'Same time tomorrow?'] },
	// Rare: the winner has just two pieces left, its king and one other (a lone
	// king can't win: it can never give check, and on time it's a draw).
	twoLeft: { emoji: '👑', lines: ["Everybody want to know what I would do if I didn't win… I guess we'll never know."] },
	// More rare ones, each for one very specific moment.
	speedrun: { emoji: '🚇', lines: ['Fastest commute in New York.'] }, // mate within two moves each
	promoMate: { emoji: '🥯', lines: ['From bodega to boardroom.'] }, // mate by promoting a pawn
	castleMate: { emoji: '🏰', lines: ['Moved in and took over.'] }, // mate by castling
	queenless: { emoji: '💅', lines: ['Who needs a queen? Not me.'] }, // a win after your queen fell in
	byAHair: { emoji: '😮‍💨', lines: ['By a hair. Deadass.'] }, // a win with under a second left
	potholeSeason: { emoji: '🚧', lines: ['Pothole season.'] }, // the fifth hole open at once
	fifthRepair: { emoji: '🏆', lines: ['Employee of the month.'] }, // the Mamdani's fifth repair of the game
	wipedOut: { emoji: '🕳️', lines: ['Gone. All of them.'] } // a side's last piece but its king falls in
};

/** A line that answers the one said before it, when its moment comes next. */
const PAIRS: Record<string, string> = { "It's so over.": "We're so back.", '−1000 aura.': '+1000 aura.' };

/** Lines only the winner sees: they rub it in. */
const WINNER_ONLY = ['Skill issue.'];

/** How long a mate-by-roll bubble waits, so the burst's shards fly first. */
const AFTER_BURST_MS = 900;

/**
 * What the board looks like just after event `i` of a turn (its pieces, and
 * how many potholes are open), and how many repairs the game has had in all:
 * the rare moments need them.
 */
export interface TurnContext {
	after?: (i: number) => { board: string[]; holes: number };
	repaired?: number;
}

/** The rare moments' context for a turn of `view`: the board after each event, and the game's repairs. */
export function contextOf(view: View): TurnContext {
	return {
		after: (i) => {
			const stage = stageAt(view, i + 1);
			return { board: stage.board, holes: stage.potholes.length };
		},
		repaired: view.stats.repaired
	};
}

/**
 * The latest big moment among the first `shown` events of a turn: the
 * Mamdani or a queen falling in, the Mamdani repairing a hole, a saving roll
 * saving a queen or rook, a king saved by a re-roll, or the dice landing on
 * f7 (6 and 7). Smaller moments (a pawn saved) say nothing. With `ctx`, rare
 * ones win: a side's last piece but its king falling in, the fifth pothole
 * open at once, and the Mamdani's fifth repair of the game.
 */
export function quipFor(events: EventJSON[], shown: number, ctx: TurnContext = {}): { kind: QuipKind; sq: string; index: number } | null {
	for (let i = Math.min(shown, events.length) - 1; i >= 0; i--) {
		const e = events[i];
		if (!e.sq) continue;
		const kind = e.piece?.[1];
		if (e.kind === 'fell' && e.piece) {
			if (e.piece !== 'M' && ctx.after) {
				const side = e.piece[0];
				const left = ctx.after(i).board.filter((code) => code.startsWith(side));
				if (left.length === 1 && left[0][1] === 'K') return { kind: 'wipedOut', sq: e.sq, index: i };
			}
			if (e.piece === 'M') return { kind: 'mamdaniFell', sq: e.sq, index: i };
			if (kind === 'Q') return { kind: 'queenFell', sq: e.sq, index: i };
		}
		if (e.kind === 'repaired') {
			// This repair's place in the game: the game's total, less the ones after it this turn.
			const nth = ctx.repaired === undefined ? 0 : ctx.repaired - events.slice(i + 1).filter((x) => x.kind === 'repaired').length;
			return { kind: nth === 5 ? 'fifthRepair' : 'repaired', sq: e.sq, index: i };
		}
		// The fifth hole open at once; not the cap closing the oldest for a new one.
		if (e.kind === 'pothole_opened' && ctx.after && events[i - 1]?.kind !== 'pothole_closed' && ctx.after(i).holes === 5)
			return { kind: 'potholeSeason', sq: e.sq, index: i };
		if (e.kind === 'saving_roll' && e.saved && (kind === 'Q' || kind === 'R')) return { kind: 'saved', sq: e.sq, index: i };
		if (e.kind === 'reroll' && e.reason === 'king') return { kind: 'kingSpared', sq: e.sq, index: i };
		if (e.kind === 'target' && e.sq === 'f7') return { kind: 'sixSeven', sq: e.sq, index: i };
	}
	return null;
}

/**
 * The line a finished game earns, and where it shows. The rare ones come
 * first: a mate by castling, by promoting a pawn, or within two moves each
 * (on the mated king); a win with just the king and one other piece, with
 * under a second left before the final move's +5, or after your queen fell
 * into a pothole (on the winning king). Then a mate by a pothole roll or by a
 * move (on the mated king), a win on time or a resignation (on the losing
 * king), or a draw (on the king of the side to move). A game nobody started,
 * or one still in play, says nothing. `afterBurst`: wait for the mate's burst.
 */
export function endQuip(view: View): EndQuip | null {
	const r = view.result;
	if (!r || r.reason === 'aborted' || r.reason === 'expired') return null;
	if (r.draw) {
		const sq = kingSquare(view.board, view.turn);
		return sq ? { kind: 'draw', sq } : null;
	}
	if (!r.winner) return null;
	const winner = r.winner;
	const mate = r.reason === 'checkmate';
	const byRoll = mate && matedByRoll(view);
	const burst = mate ? ({ afterBurst: true } as const) : {};
	const loserKing = kingSquare(view.board, opponent(winner));
	const winnerKing = kingSquare(view.board, winner);
	const last = view.log.at(-1);
	const san = last?.san ?? '';
	if (mate && !byRoll && loserKing) {
		if (san.startsWith('O-O')) return { kind: 'castleMate', sq: loserKing, ...burst };
		if (san.includes('=')) return { kind: 'promoMate', sq: loserKing, ...burst };
		if (view.log.length <= 4) return { kind: 'speedrun', sq: loserKing, ...burst };
	}
	if (winnerKing) {
		if (view.board.filter((code) => code.startsWith(winner[0])).length === 2) return { kind: 'twoLeft', sq: winnerKing, ...burst };
		// The winner's time before the final move's +5, if the game ended on its move.
		const ms = winner === 'white' ? view.clock.whiteMs : view.clock.blackMs;
		if (ms - (last?.color === winner ? 5_000 : 0) < 1_000) return { kind: 'byAHair', sq: winnerKing, ...burst };
		if (view.lost[winner].some((code) => code[1] === 'Q')) return { kind: 'queenless', sq: winnerKing, ...burst };
	}
	if (!loserKing) return null;
	if (r.reason === 'timeout') return { kind: 'timeWin', sq: loserKing };
	if (r.reason === 'resignation') return { kind: 'resigned', sq: loserKing };
	if (mate) return { kind: byRoll ? 'mateByRoll' : 'mateByMove', sq: loserKing, ...burst };
	return null;
}

/**
 * A line for `kind`, picked at random but never `previous` again when there's
 * another. A line that pairs with `previous` ("It's so over." → "We're so
 * back.") comes next when it can. Winner-only lines need `winner`.
 */
export function pickLine(kind: QuipKind, previous: string | undefined, random: () => number = Math.random, { winner = true }: { winner?: boolean } = {}): string {
	const lines = QUIPS[kind].lines.filter((l) => winner || !WINNER_ONLY.includes(l));
	const answer = previous ? PAIRS[previous] : undefined;
	if (answer && lines.includes(answer)) return answer;
	const choices = lines.length > 1 ? lines.filter((l) => l !== previous) : lines;
	return choices[Math.min(choices.length - 1, Math.floor(random() * choices.length))];
}

/**
 * Remembers the line picked for each moment, so it stays the same while the
 * board redraws, and the line said last, so the next moment says another.
 * Returns the bubble for the first `shown` events of turn `seq`, or null; or,
 * given `end` (from endQuip), the bubble for the game's end, under `key`.
 * After a fall it waits `delay` ms, the fall's length, so the piece drops
 * first; after a mate by a roll, until the burst's shards have flown.
 */
export function quipper(random: () => number = Math.random) {
	const said = new Map<string, string>();
	let last: string | undefined;
	return (
		events: EventJSON[],
		seq: number,
		shown: number,
		ending?: { end: EndQuip; key: string; winner: boolean },
		ctx?: TurnContext
	): { sq: string; emoji: string; line: string; key: string; delay: number } | null => {
		const found = quipFor(events, shown, ctx);
		const q = ending ? { ...ending.end, key: ending.key } : found && { ...found, key: `${seq}:${found.index}` };
		if (!q) return null;
		let line = said.get(q.key);
		if (!line) {
			line = pickLine(q.kind, last, random, { winner: ending?.winner ?? true });
			said.set(q.key, line);
			last = line;
		}
		const fell = q.kind === 'mamdaniFell' || q.kind === 'queenFell' || q.kind === 'wipedOut';
		const delay = fell ? FALL_MS : ending?.end.afterBurst ? AFTER_BURST_MS : 0;
		return { sq: q.sq, emoji: QUIPS[q.kind].emoji, line, key: q.key, delay };
	};
}

/** A line's parts: text between ~tildes~ is drawn struck through. */
export function lineParts(line: string): { text: string; struck: boolean }[] {
	return line
		.split('~')
		.map((text, i) => ({ text, struck: i % 2 === 1 }))
		.filter((p) => p.text !== '');
}

/** A line as a screen reader should hear it: struck-through words left out. */
export function spoken(line: string): string {
	return lineParts(line)
		.filter((p) => !p.struck)
		.map((p) => p.text)
		.join('')
		.replace(/\s+/g, ' ')
		.trim();
}
