// Plays whole games through the real UI, as two guests per game, with random
// legal moves, and reports how each one ended. It fails (exit 1) on any page
// error, console error, move that never lands, board that gets stuck, or
// pothole left drawn after it closed.
//
// Needs the Go server (:8080) and the dev server (:5173) running; it uses the
// game page's dev-only ?instant mode, so a whole game takes seconds.
//
// The dev server reloads every open page when svelte-kit sync rewrites its
// generated files (after a rebase, new imports, or `pnpm check`/`build` that
// change them), which breaks a game mid-move. The script reports that as
// "page reloaded mid-game": run check and build before a play-test, not during
// one, or point --base at a production server (no reloads).
//
//   pnpm --dir web playtest                       # 6 games in Chromium
//   pnpm --dir web playtest --browser webkit      # Safari's engine
//   pnpm --dir web playtest --games 12 --drag 0.5 # half the moves by dragging
//   pnpm --dir web playtest --phone               # as iPhones: taps, the phone layout
//   pnpm --dir web playtest --match               # both guests tap Play online (quick match)
//   pnpm --dir web playtest --sound               # with sound on (each guest starts with it off)
//
// Against a production build (no ?instant: the dice play out in full), allow
// each turn longer:
//
//   pnpm --dir web playtest --base https://<domain> --games 2 --max-plies 30 --turn-ms 15000

import { chromium, devices, webkit } from 'playwright';
import { parseArgs } from 'node:util';

const { values: opts } = parseArgs({
	options: {
		games: { type: 'string', default: '6' },
		browser: { type: 'string' }, // chromium; webkit with --phone
		drag: { type: 'string', default: '0.3' }, // share of moves made by dragging
		base: { type: 'string', default: 'http://localhost:5173' },
		'max-plies': { type: 'string', default: '1000' },
		'turn-ms': { type: 'string', default: '3000' }, // how long a move may take to land
		headed: { type: 'boolean', default: false },
		phone: { type: 'boolean', default: false }, // play as an iPhone 15, in the phone layout
		match: { type: 'boolean', default: false }, // find each other through quick match, not a link
		sound: { type: 'boolean', default: false } // leave sound on (sound.ts)
	}
});
const GAMES = Number(opts.games);
const DRAG = Number(opts.drag);
const MAX_PLIES = Number(opts['max-plies']);
const TURN_MS = Number(opts['turn-ms']);
const engines = { chromium, webkit };
opts.browser ??= opts.phone ? 'webkit' : 'chromium';
if (!engines[opts.browser]) throw new Error(`--browser must be one of ${Object.keys(engines).join(', ')}`);

// The result card: beside the board on desktop, in the card slot on a phone.
const RESULT = '.result, section[aria-label="Game over"]';
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
const pick = (xs) => xs[Math.floor(Math.random() * xs.length)];

/** Clicks a random movable piece, then a random target; returns what it did. */
async function clickMove(page) {
	return page.evaluate(async () => {
		const pick = (xs) => xs[Math.floor(Math.random() * xs.length)];
		const tick = () => new Promise((r) => setTimeout(r, 0));
		const from = pick([...document.querySelectorAll('.square.movable')]);
		from.click();
		await tick();
		const tos = [...document.querySelectorAll('.square.legal')];
		if (!tos.length) return { err: `no targets for ${from.getAttribute('aria-label')}` };
		const to = pick(tos);
		to.click();
		await tick();
		return { from: from.getAttribute('aria-label'), to: to.getAttribute('aria-label') };
	});
}

/** Taps a random movable piece, then a random target, as a finger would. */
async function tapMove(page) {
	const from = pick(await page.locator('.square.movable').all());
	const fromLabel = await from.getAttribute('aria-label');
	await from.tap();
	const tos = await page.locator('.square.legal').all();
	if (!tos.length) return { err: `no targets for ${fromLabel}` };
	const to = pick(tos);
	const toLabel = await to.getAttribute('aria-label');
	await to.tap();
	return { from: fromLabel, to: toLabel };
}

/** The phone layout's rules: one screen, no scrolling, a board near full width. */
async function phoneLayout(page) {
	return page.evaluate(() => {
		if (!document.querySelector('.phone')) return 'not in the phone layout';
		if (document.documentElement.scrollHeight > innerHeight + 1) return `the page scrolls (${document.documentElement.scrollHeight} > ${innerHeight})`;
		const board = document.querySelector('.board')?.getBoundingClientRect().width ?? 0;
		// Near full width: the coordinates outside the board take 14 px of it
		// (12 px labels and a 2 px gap), so on a short screen it's about 89%.
		if (board < innerWidth * 0.88) return `the board is ${Math.round(board)}px on a ${innerWidth}px screen`;
		return '';
	});
}

/** Opens "Moves and rolls" and closes it again. */
async function sheetOpensAndCloses(page) {
	await page.getByRole('button', { name: 'Moves and rolls' }).tap();
	if (!(await page.locator('dialog.sheet[open]').count())) return 'the moves sheet did not open';
	await page.locator('dialog.sheet').getByRole('button', { name: 'Close' }).tap();
	if (await page.locator('dialog.sheet[open]').count()) return 'the moves sheet did not close';
	return '';
}

/** Drags a random movable piece to a random target with the real mouse. */
async function dragMove(page) {
	const from = pick(await page.locator('.square.movable').all());
	const fromLabel = await from.getAttribute('aria-label');
	await from.click(); // select it to learn its targets; dragging a selected piece works the same
	const tos = await page.locator('.square.legal').all();
	if (!tos.length) return { err: `no targets for ${fromLabel}` };
	// Locators are lazy: measure the target while it is still highlighted.
	const to = pick(tos);
	const toLabel = await to.getAttribute('aria-label');
	const a = await from.boundingBox();
	const b = await to.boundingBox();
	await page.mouse.move(a.x + a.width / 2, a.y + a.height / 2);
	await page.mouse.down();
	await page.mouse.move(b.x + b.width / 2, b.y + b.height / 2, { steps: 12 });
	await page.mouse.up();
	return { from: fromLabel, to: toLabel, dragged: true };
}

/**
 * Whether the potholes drawn match the board's own model (each pothole
 * square's label says so). A hole can still be shrinking when the turn
 * lands, so a mismatch is checked again after its exit has had time to end.
 */
async function holesMatch(page) {
	const count = () =>
		page.evaluate(() => [
			// Not counted: a repaired hole, or the one a saving roll flashes open.
			document.querySelectorAll('.layer .slot > .hole:not(.patched):not(.briefly)').length,
			document.querySelectorAll('.square[aria-label*="pothole"]').length
		]);
	let [drawn, model] = await count();
	if (drawn !== model) {
		await sleep(600);
		[drawn, model] = await count();
	}
	return drawn === model ? '' : `the board draws ${drawn} potholes but has ${model}`;
}

let pairing = Promise.resolve();

/** Both guests tap Play online, the second once the first is queued. */
async function quickMatch(first, second) {
	for (const p of [first, second]) {
		await p.goto(`${opts.base}/`);
		await p.getByRole('link', { name: /Play online/ }).click();
		if (p === first) await p.getByText('Keep this tab open').waitFor();
	}
	await Promise.all([first.waitForURL(/\/game\//), second.waitForURL(/\/game\//)]);
}

async function playGame(browser, n) {
	const device = opts.phone ? { ...devices['iPhone 15'], defaultBrowserType: undefined } : {};
	const contexts = [await browser.newContext(device), await browser.newContext(device)];
	// The server doesn't count playtest visits. The header goes on our own
	// requests only: on a cross-origin one (the fonts) it fails CORS.
	for (const c of contexts) {
		await c.route(`${opts.base}/**`, (route) => route.continue({ headers: { ...route.request().headers(), 'x-playtest': '1' } }));
	}
	const [w, b] = await Promise.all(contexts.map((c) => c.newPage()));
	const errors = [];
	const sides = [
		[w, 'White'],
		[b, 'Black']
	];
	// Sound off: ?instant plays none anyway, and White's game page, which opens
	// once before ?instant, would start audio and then be left at once. WebKit
	// keeps such a page a moment and reports its cut-off requests as errors.
	if (!opts.sound) for (const c of contexts) await c.addInitScript(() => localStorage.setItem('sound-off', '1'));
	for (const [p, who] of sides) {
		p.on('pageerror', (e) => errors.push(`${who} page error: ${e.message.slice(0, 240)}`));
		p.on('console', (m) => m.type() === 'error' && errors.push(`${who} console: ${m.text().slice(0, 240)}`));
		// The last few console lines and any reload after the game started, so a failure explains itself.
		p.recent = [];
		p.on('console', (m) => p.recent.push(`${m.type()}: ${m.text().slice(0, 160)}`) > 5 && p.recent.shift());
		// A real reload fires load again (framenavigated also fires for history updates, which are fine).
		p.on('load', () => p.started && errors.push(`${who} page reloaded mid-game (${p.url()})`));
	}
	const started = Date.now();
	if (opts.match) {
		// One pair queues at a time, so each game's two guests match each other.
		await (pairing = pairing.then(() => quickMatch(w, b)));
		if (w.url() !== b.url()) errors.push(`quick match sent the guests to different games: ${w.url()} / ${b.url()}`);
	} else {
		await w.goto(`${opts.base}/`);
		await w.getByRole('button', { name: 'Invite a friend' }).click();
		await w.waitForURL(/\/game\//);
	}
	const url = w.url().split('?')[0] + '?instant';
	await w.goto(url);
	await b.goto(url);
	w.started = b.started = true;

	const movable = (p) => p.locator('.square.movable').count();
	const over = async () => (await w.locator(RESULT).count()) > 0 && (await b.locator(RESULT).count()) > 0;
	let plies = 0;
	let drags = 0;
	let idle = 0;
	let resigned = false;
	while (!(await over())) {
		if (plies >= MAX_PLIES) {
			await w.getByRole('button', { name: 'Resign' }).click();
			await w.getByRole('button', { name: 'Yes, resign' }).click();
			resigned = true;
			await sleep(300);
			break;
		}
		let mover = null;
		let other = null;
		if (await movable(w)) [mover, other] = [w, b];
		else if (await movable(b)) [mover, other] = [b, w];
		if (!mover) {
			if (++idle > 200) {
				errors.push(`stuck: nobody could move for 10s at ply ${plies}`);
				break;
			}
			await sleep(50);
			continue;
		}
		idle = 0;
		const move = opts.phone ? await tapMove(mover) : Math.random() < DRAG ? await dragMove(mover) : await clickMove(mover);
		if (move.err) {
			errors.push(`ply ${plies}: ${move.err}`);
			break;
		}
		const promo = mover.locator('[role=dialog] button[aria-label]');
		if (await promo.count()) await pick(await promo.all()).click();
		// The move landed when the other side gets the turn or the game ends.
		let landed = false;
		for (let i = 0; i < TURN_MS / 30 && !landed; i++) {
			if ((await movable(other)) > 0 || (await over())) landed = true;
			else await sleep(30);
		}
		if (!landed) {
			errors.push(`ply ${plies}: ${move.from} → ${move.to}${move.dragged ? ' (drag)' : ''} never landed`);
			if (errors.length > 5) break;
			continue;
		}
		if (move.dragged) drags++;
		plies++;
		const holes = (await holesMatch(w)) || (await holesMatch(b));
		if (holes) {
			errors.push(`ply ${plies}: ${holes}`);
			break;
		}
		if (opts.phone) {
			const bad = (await phoneLayout(w)) || (await phoneLayout(b)) || (plies === 10 ? await sheetOpensAndCloses(other) : '');
			if (bad) {
				errors.push(`ply ${plies}: ${bad}`);
				break;
			}
		}
	}
	if (errors.length) for (const [p, who] of sides) errors.push(`${who} last console: ${p.recent.join(' | ') || '(none)'}`);
	// The result, without the phone card's tally, which may still be counting up.
	const resultText = (p) =>
		p
			.locator(RESULT)
			.evaluate((el) => [...el.children].filter((c) => !c.classList.contains('ph-tally')).map((c) => c.innerText).join('\n'))
			.then((text) => text.replace(/\s*\n+\s*/g, ' · '))
			.catch(() => '');
	const result = await resultText(w);
	const resultBlack = await resultText(b);
	// Each player reads the result as themselves ("You win" / "You lost"), so
	// the two cards agree on how and when the game ended, and on who won.
	const outcome = (p) =>
		p.evaluate(() => {
			const text = (sel) => document.querySelector(sel)?.textContent?.trim() ?? '';
			return { title: text('.ph-headline, .result h1'), kicker: text('.ph-kicker, .result .kicker') };
		});
	const [ow, ob] = [await outcome(w), await outcome(b)];
	const pair = new Set([ow.title.toLowerCase(), ob.title.toLowerCase()]);
	const agree = ow.kicker === ob.kicker && (ow.title === ob.title ? !/^you /i.test(ow.title) : pair.has('you win') && pair.has('you lost'));
	if (!agree) errors.push(`the two sides disagree: "${result}" / "${resultBlack}"`);
	// Each bar's pieces lost to potholes: icons on a computer, a count on a phone.
	const lost = await w.locator('.bar').evaluateAll((bars) => bars.map((b) => b.querySelectorAll('.glyphs img').length || Number(b.querySelector('.pothole')?.textContent || 0)));
	await Promise.all(contexts.map((c) => c.close()));
	return { game: n, url, plies, drags, resigned, result, lost, seconds: Math.round((Date.now() - started) / 1000), errors };
}

const browser = await engines[opts.browser].launch({ headless: !opts.headed });
const games = await Promise.all(Array.from({ length: GAMES }, (_, i) => playGame(browser, i + 1)));
await browser.close();

for (const g of games) {
	const status = g.errors.length ? 'FAIL' : 'ok  ';
	console.log(`${status} game ${g.game}: ${g.plies} plies (${g.drags} dragged), ${g.seconds}s, lost to potholes ${g.lost.join('/')} — ${g.result || 'no result'}${g.resigned ? ' (resigned at the ply limit)' : ''}`);
	for (const e of g.errors) console.log(`       ${e}  [${g.url}]`);
}
const failed = games.filter((g) => g.errors.length).length;
console.log(`\n${opts.browser}: ${games.length - failed}/${games.length} games finished cleanly`);
process.exit(failed ? 1 : 0);
