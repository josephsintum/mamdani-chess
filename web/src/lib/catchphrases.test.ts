import { describe, expect, it } from 'vitest';
import { contextOf, endQuip, lineParts, pickLine, quipper, QUIPS, quipFor, spoken } from './catchphrases.ts';
import type { EventJSON, View } from './game.ts';

const roll: EventJSON[] = [{ kind: 'moved', from: 'e2', to: 'e4' }, { kind: 'rolled_pothole', roll: 4 }, { kind: 'target', sq: 'a5' }];

describe('quipFor', () => {
	it('finds the Mamdani falling in, once that step is revealed', () => {
		const events = [...roll, { kind: 'saving_roll', sq: 'a5', piece: 'M', roll: 2, saved: false }, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }];
		expect(quipFor(events, 4)).toBeNull();
		expect(quipFor(events, 5)).toEqual({ kind: 'mamdaniFell', sq: 'a5', index: 4 });
		expect(quipFor(events, 6)).toEqual({ kind: 'mamdaniFell', sq: 'a5', index: 4 });
	});
	it('finds a queen falling in and the Mamdani repairing a hole', () => {
		expect(quipFor([...roll, { kind: 'fell', sq: 'd8', piece: 'bQ' }], 4)).toMatchObject({ kind: 'queenFell', sq: 'd8' });
		expect(quipFor([{ kind: 'moved', from: 'a5', to: 'b6' }, { kind: 'repaired', sq: 'c7' }], 2)).toMatchObject({ kind: 'repaired', sq: 'c7' });
	});
	it('finds a saving roll that saves a queen or a rook, not a pawn', () => {
		expect(quipFor([...roll, { kind: 'saving_roll', sq: 'a1', piece: 'wR', roll: 3, saved: true }], 4)).toMatchObject({ kind: 'saved', sq: 'a1' });
		expect(quipFor([...roll, { kind: 'saving_roll', sq: 'a2', piece: 'wP', roll: 3, saved: true }], 4)).toBeNull();
		expect(quipFor([...roll, { kind: 'fell', sq: 'a2', piece: 'wP' }], 4)).toBeNull();
	});
});

describe('pickLine', () => {
	it('never picks the same line twice in a row', () => {
		const lines = QUIPS.mamdaniFell.lines;
		for (const previous of lines) {
			for (const r of [0, 0.3, 0.6, 0.99]) expect(pickLine('mamdaniFell', previous, () => r)).not.toBe(previous);
		}
	});
	it('picks from the event\'s lines, the only one when there is one', () => {
		expect(QUIPS.mamdaniFell.lines).toContain(pickLine('mamdaniFell', undefined, () => 0.5));
		expect(pickLine('sixSeven', '6-7', () => 0)).toBe('6-7');
	});
});

describe('quipper', () => {
	const fall: EventJSON[] = [...roll, { kind: 'fell', sq: 'a5', piece: 'M' }, { kind: 'pothole_opened', sq: 'a5' }];
	it('keeps the same line for a moment while the board redraws', () => {
		const say = quipper(() => 0.5);
		const first = say(fall, 7, 4);
		expect(first).toMatchObject({ sq: 'a5', emoji: '😢', key: '7:3', delay: 550 });
		expect(say(fall, 7, 5)).toEqual(first);
	});
	it('waits for a fall to finish, but not for a repair', () => {
		const repair: EventJSON[] = [{ kind: 'moved', from: 'a5', to: 'b6' }, { kind: 'repaired', sq: 'c7' }];
		expect(quipper()(repair, 3, 2)).toMatchObject({ delay: 0 });
	});
	it('says nothing before the moment is revealed', () => {
		expect(quipper()(fall, 7, 3)).toBeNull();
	});
	it('never repeats the last line on the next moment', () => {
		const say = quipper(() => 0);
		const a = say(fall, 7, 4)!.line;
		const b = say(fall, 9, 4)!.line;
		expect(b).not.toBe(a);
	});
});

describe('struck-through words', () => {
	it('splits a line into plain and struck-through parts', () => {
		expect(lineParts('Ask ~bat~ Bruce Wayne for help.')).toEqual([
			{ text: 'Ask ', struck: false },
			{ text: 'bat', struck: true },
			{ text: ' Bruce Wayne for help.', struck: false }
		]);
		expect(lineParts('Filled. Next!')).toEqual([{ text: 'Filled. Next!', struck: false }]);
	});
	it('reads a line aloud without its struck-through words', () => {
		expect(spoken('Ask ~bat~ Bruce Wayne for help.')).toBe('Ask Bruce Wayne for help.');
		expect(spoken('Got paperwork to do.')).toBe('Got paperwork to do.');
	});
	it('says the Batman line with Bruce Wayne', () => {
		expect(QUIPS.mamdaniFell.lines).toContain('Ask ~bat~ Bruce Wayne for help.');
		expect(QUIPS.mamdaniFell.lines).not.toContain('Ask Batman for help.');
	});
});

describe('new moments', () => {
	it('finds a king saved by a re-roll', () => {
		const events: EventJSON[] = [...roll.slice(0, 2), { kind: 'target', sq: 'e1' }, { kind: 'reroll', sq: 'e1', reason: 'king' }, { kind: 'target', sq: 'g8' }];
		expect(quipFor(events, 4)).toEqual({ kind: 'kingSpared', sq: 'e1', index: 3 });
	});
	it('says 6-7 when the dice land on f7, and nothing for another square', () => {
		expect(quipFor([...roll.slice(0, 2), { kind: 'target', sq: 'f7' }], 3)).toEqual({ kind: 'sixSeven', sq: 'f7', index: 2 });
		expect(quipFor(roll, 3)).toBeNull();
	});
});

describe('pairs and the winner-only line', () => {
	it("answers \"It's so over\" with \"We're so back\" on the next repair", () => {
		expect(pickLine('repaired', "It's so over.", () => 0)).toBe("We're so back.");
		expect(pickLine('repaired', '−1000 aura.', () => 0.99)).toBe('+1000 aura.');
	});
	it('keeps "Skill issue." for the winner', () => {
		const lines = QUIPS.timeWin.lines.filter((l) => l !== 'Skill issue.');
		for (const r of [0, 0.3, 0.6, 0.99]) expect(lines).toContain(pickLine('timeWin', undefined, () => r, { winner: false }));
		expect(QUIPS.timeWin.lines).toContain('Skill issue.');
	});
	it('replaces London\'s "Mind the gap" with New York\'s "Watch the gap"', () => {
		expect(QUIPS.queenFell.lines).toContain('Watch the gap!');
		expect(QUIPS.queenFell.lines).not.toContain('Mind the gap.');
	});
});

describe('endQuip', () => {
	const view = (extra: Partial<View>) =>
		// Kings on e1 and e8, two pawns each (a2, b2 and a7, b7).
		({ board: Array.from({ length: 64 }, (_, i) => ({ 4: 'wK', 8: 'wP', 9: 'wP', 48: 'bP', 49: 'bP', 60: 'bK' })[i] ?? ''), last: [], result: null, ...extra }) as unknown as View;
	it("puts a win on time on the loser's king", () => {
		expect(endQuip(view({ result: { winner: 'black', draw: false, reason: 'timeout' } }))).toEqual({ kind: 'timeWin', sq: 'e1' });
	});
	it('puts a mate by a pothole roll on the mated king', () => {
		const last: EventJSON[] = [{ kind: 'moved', from: 'a2', to: 'a3' }, { kind: 'rolled_pothole', roll: 4 }];
		expect(endQuip(view({ last, result: { winner: 'white', draw: false, reason: 'checkmate' } }))).toEqual({ kind: 'mateByRoll', sq: 'e8', afterBurst: true });
	});
	it('puts a mate by a move on the mated king', () => {
		expect(endQuip(view({ result: { winner: 'white', draw: false, reason: 'checkmate' } }))).toEqual({ kind: 'mateByMove', sq: 'e8', afterBurst: true });
	});
	it("puts a resignation on the resigning side's king", () => {
		expect(endQuip(view({ result: { winner: 'white', draw: false, reason: 'resignation' } }))).toEqual({ kind: 'resigned', sq: 'e8' });
	});
	it("puts a draw on the king of the side to move", () => {
		expect(endQuip(view({ turn: 'black', result: { draw: true, reason: 'stalemate' } }))).toEqual({ kind: 'draw', sq: 'e8' });
		expect(endQuip(view({ turn: 'white', result: { draw: true, reason: 'repetition' } }))).toEqual({ kind: 'draw', sq: 'e1' });
	});
	it('says nothing for a game nobody started, or one in play', () => {
		expect(endQuip(view({ result: { winner: 'black', draw: false, reason: 'aborted' } }))).toBeNull();
		expect(endQuip(view({ result: { draw: true, reason: 'expired' } }))).toBeNull();
		expect(endQuip(view({}))).toBeNull();
	});
	it("brags from the winner's king when it won with just two pieces left", () => {
		// Black has its king (e8) and one rook (h8); White still has a rook and a pawn.
		const board = Array.from({ length: 64 }, (_, i) => ({ 0: 'wR', 4: 'wK', 8: 'wP', 60: 'bK', 63: 'bR' })[i] ?? '');
		const two = view({ board, result: { winner: 'black', draw: false, reason: 'checkmate' } });
		expect(endQuip(two)).toEqual({ kind: 'twoLeft', sq: 'e8', afterBurst: true });
		expect(QUIPS.twoLeft.lines).toEqual(["Everybody want to know what I would do if I didn't win… I guess we'll never know."]);
		// White wins with three pieces: an ordinary end line.
		expect(endQuip(view({ board, result: { winner: 'white', draw: false, reason: 'resignation' } }))).toEqual({ kind: 'resigned', sq: 'e8' });
	});
	it('needs the king and exactly one other piece', () => {
		const lone = Array.from({ length: 64 }, (_, i) => ({ 4: 'wK', 8: 'wP', 9: 'wP', 60: 'bK' })[i] ?? '');
		expect(endQuip(view({ board: lone, result: { winner: 'black', draw: false, reason: 'resignation' } }))).toEqual({ kind: 'resigned', sq: 'e1' });
	});
	it('has the new end lines', () => {
		expect(QUIPS.mateByMove.lines).toContain('Gotta be quicker than that. This is New York.');
		expect(QUIPS.resigned.lines).toContain('Somebody call a cab.');
		expect(QUIPS.draw.lines).toEqual(['Mid.', '6-7 🤷', 'Same time tomorrow?']);
	});
	it('gives the end its own key, and waits for the burst after a mate', () => {
		const say = quipper(() => 0);
		const b = say([], 9, 0, { end: { kind: 'mateByRoll', sq: 'e8', afterBurst: true }, key: 'G1:end', winner: true });
		expect(b).toMatchObject({ sq: 'e8', emoji: '🗽', key: 'G1:end' });
		expect(b!.delay).toBeGreaterThan(0);
		expect(say([], 9, 0, { end: { kind: 'mateByMove', sq: 'e8', afterBurst: true }, key: 'G2:end', winner: true })!.delay).toBeGreaterThan(0);
		expect(say([], 9, 0, { end: { kind: 'resigned', sq: 'e8' }, key: 'G3:end', winner: true })!.delay).toBe(0);
	});
});

describe('rare moments at the end', () => {
	// Kings on e1 and e8, two pawns each; White wins unless said otherwise.
	const base = Array.from({ length: 64 }, (_, i) => ({ 4: 'wK', 8: 'wP', 9: 'wP', 48: 'bP', 49: 'bP', 60: 'bK' })[i] ?? '');
	const log = (...sans: string[]) => sans.map((san, i) => ({ san, color: i % 2 ? 'black' : 'white', dice: '' }));
	const end = (extra: Record<string, unknown>) =>
		({ board: base, last: [], log: log('e4', 'e5', 'Nf3', 'Nc6', 'Bc4'), lost: { white: [], black: [] }, clock: { whiteMs: 60_000, blackMs: 60_000, now: 0 }, turn: 'black', result: { winner: 'white', draw: false, reason: 'checkmate' }, ...extra }) as unknown as View;
	it("calls a mate in two moves a speedrun", () => {
		const v = end({ log: log('f3', 'e5', 'g4', 'Qh4#'), result: { winner: 'black', draw: false, reason: 'checkmate' } });
		expect(endQuip(v)).toEqual({ kind: 'speedrun', sq: 'e1', afterBurst: true });
		expect(QUIPS.speedrun.lines).toEqual(['Fastest commute in New York.']);
	});
	it('notices a mate by a promoted pawn, and by castling', () => {
		expect(endQuip(end({ log: log('e4', 'e5', 'Nf3', 'Nc6', 'a8=Q#') }))).toEqual({ kind: 'promoMate', sq: 'e8', afterBurst: true });
		expect(endQuip(end({ log: log('e4', 'e5', 'Nf3', 'Nc6', 'O-O#') }))).toEqual({ kind: 'castleMate', sq: 'e8', afterBurst: true });
		expect(QUIPS.promoMate.lines).toEqual(['From bodega to boardroom.']);
		expect(QUIPS.castleMate.lines).toEqual(['Moved in and took over.']);
	});
	it("brags when the winner's queen fell into a pothole", () => {
		const v = end({ lost: { white: ['wQ'], black: [] }, result: { winner: 'white', draw: false, reason: 'resignation' } });
		expect(endQuip(v)).toEqual({ kind: 'queenless', sq: 'e1' });
		expect(QUIPS.queenless.lines).toEqual(['Who needs a queen? Not me.']);
	});
	it("notices a win with under a second left, before the final move's +5", () => {
		// White mated on its own move: 5.8 s now is 0.8 s before the increment.
		expect(endQuip(end({ clock: { whiteMs: 5_800, blackMs: 60_000, now: 0 } }))).toEqual({ kind: 'byAHair', sq: 'e1', afterBurst: true });
		// Black resigned on its own turn, White's last move long done: 0.9 s.
		const resigned = end({ log: log('e4', 'e5', 'Nf3', 'Nc6', 'Bc4', 'Nf6'), clock: { whiteMs: 900, blackMs: 60_000, now: 0 }, result: { winner: 'white', draw: false, reason: 'resignation' } });
		expect(endQuip(resigned)).toEqual({ kind: 'byAHair', sq: 'e1' });
		expect(endQuip(end({ clock: { whiteMs: 7_000, blackMs: 60_000, now: 0 } }))).toEqual({ kind: 'mateByMove', sq: 'e8', afterBurst: true });
		expect(QUIPS.byAHair.lines).toEqual(['By a hair. Deadass.']);
	});
});

describe('rare moments during a turn', () => {
	const at = (holes: number, board: string[] = []) => () => ({ board, holes });
	it('calls the fifth open pothole a season, but not the cap swapping one for another', () => {
		const opened: EventJSON[] = [{ kind: 'moved', from: 'e2', to: 'e4' }, { kind: 'rolled_pothole', roll: 2 }, { kind: 'target', sq: 'c5' }, { kind: 'pothole_opened', sq: 'c5' }];
		expect(quipFor(opened, 4, { after: at(5) })).toEqual({ kind: 'potholeSeason', sq: 'c5', index: 3 });
		expect(quipFor(opened, 4, { after: at(4) })).toBeNull();
		const swapped: EventJSON[] = [...opened.slice(0, 3), { kind: 'pothole_closed', sq: 'a3' }, { kind: 'pothole_opened', sq: 'c5' }];
		expect(quipFor(swapped, 5, { after: at(5) })).toBeNull();
		expect(QUIPS.potholeSeason.lines).toEqual(['Pothole season.']);
	});
	it("celebrates the Mamdani's fifth repair of the game", () => {
		const two: EventJSON[] = [{ kind: 'moved', from: 'c3', to: 'd4' }, { kind: 'repaired', sq: 'e5' }, { kind: 'repaired', sq: 'c5' }];
		// Six repairs this game, two of them this turn: e5 is the fifth, c5 the sixth.
		expect(quipFor(two, 2, { repaired: 6 })).toEqual({ kind: 'fifthRepair', sq: 'e5', index: 1 });
		expect(quipFor(two, 3, { repaired: 6 })).toEqual({ kind: 'repaired', sq: 'c5', index: 2 });
		expect(QUIPS.fifthRepair.lines).toEqual(['Employee of the month.']);
	});
	it("notices a pothole swallowing a side's last piece but its king", () => {
		const fell: EventJSON[] = [{ kind: 'moved', from: 'e2', to: 'e4' }, { kind: 'rolled_pothole', roll: 2 }, { kind: 'target', sq: 'd8' }, { kind: 'fell', sq: 'd8', piece: 'bQ' }];
		const onlyKing = Array.from({ length: 64 }, (_, i) => ({ 4: 'wK', 8: 'wP', 60: 'bK' })[i] ?? '');
		expect(quipFor(fell, 4, { after: at(1, onlyKing) })).toEqual({ kind: 'wipedOut', sq: 'd8', index: 3 });
		const more = Array.from({ length: 64 }, (_, i) => ({ 4: 'wK', 8: 'wP', 48: 'bP', 60: 'bK' })[i] ?? '');
		expect(quipFor(fell, 4, { after: at(1, more) })).toEqual({ kind: 'queenFell', sq: 'd8', index: 3 });
		expect(QUIPS.wipedOut.lines).toEqual(['Gone. All of them.']);
	});
});

describe('contextOf', () => {
	it("reads the board and open holes after each event, and the game's repairs", () => {
		const last: EventJSON[] = [{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' }, { kind: 'rolled_pothole', roll: 2 }, { kind: 'target', sq: 'c5' }, { kind: 'pothole_opened', sq: 'c5', color: 'white' }];
		const board = Array.from({ length: 64 }, (_, i) => ({ 4: 'wK', 28: 'wP', 60: 'bK' })[i] ?? '');
		const v = { board, last, mamdani: 'a5', potholes: [{ sq: 'c5', by: 'white', left: 3 }], lost: { white: [], black: [] }, stats: { savingRolls: 0, saved: 0, repaired: 4, mamdaniFell: false } } as unknown as View;
		const ctx = contextOf(v);
		expect(ctx.repaired).toBe(4);
		expect(ctx.after!(3).holes).toBe(1);
		expect(ctx.after!(2).holes).toBe(0);
	});
});
