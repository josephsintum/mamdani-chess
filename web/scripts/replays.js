// Checks that a turn shown at once never replays its one-off effects: plays a
// friend game through the API until a turn rolls a pothole square, lets it
// play out, then reloads the page (desktop and phone) and opens the phone's
// moves sheet, and lists every animation still running. It exits 1 if any
// one-off effect plays again.
//
// Needs the Go server (:8080) and the dev server (:5173) running. It plays
// the dice in full (no ?instant), so it takes about half a minute.
//
//   pnpm --dir web replays
//   pnpm --dir web replays --browser webkit

import { chromium, devices, webkit } from 'playwright';
import { parseArgs } from 'node:util';

const { values: opts } = parseArgs({
	options: {
		browser: { type: 'string', default: 'chromium' },
		base: { type: 'string', default: 'http://localhost:5173' },
		headed: { type: 'boolean', default: false }
	}
});
const engines = { chromium, webkit };
if (!engines[opts.browser]) throw new Error(`--browser must be one of ${Object.keys(engines).join(', ')}`);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/**
 * Animations running now inside `within`, by element class and name. Endless
 * ones (a spinner) aren't one-off effects, nor is `within`'s own (the sheet
 * sliding up).
 */
function running(page, within = 'body') {
	return page.evaluate((sel) => {
		const root = document.querySelector(sel);
		if (!root) return [`no ${sel}`];
		return root
			.getAnimations({ subtree: true })
			.filter((a) => a.playState === 'running' && a.effect?.getComputedTiming().iterations !== Infinity && (sel === 'body' || a.effect?.target !== root))
			.map((a) => {
				const t = a.effect?.target;
				const cls = typeof t?.className === 'string' ? t.className : (t?.className?.baseVal ?? '');
				return `${cls.split(' ').filter((c) => !c.startsWith('s-svelte')).join('.') || t?.tagName}: ${a.animationName ?? 'script'}`;
			});
	}, within);
}

/** The game as `page`'s guest sees it. */
const viewOf = (page, code) => page.evaluate((c) => fetch(`/api/games/${c}`).then((r) => r.json()), code);

async function main() {
	const browser = await engines[opts.browser].launch({ headless: !opts.headed });
	const newContext = async (device = {}) => {
		const c = await browser.newContext({ viewport: { width: 1280, height: 900 }, ...device });
		await c.route(`${opts.base}/**`, (route) => route.continue({ headers: { ...route.request().headers(), 'x-playtest': '1' } }));
		await c.addInitScript(() => localStorage.setItem('sound-off', '1'));
		return c;
	};
	const [wc, bc] = [await newContext(), await newContext()];
	const [w, b] = [await wc.newPage(), await bc.newPage()];
	await w.goto(`${opts.base}/`, { waitUntil: 'networkidle' });
	await w.getByRole('button', { name: 'Invite a friend' }).click();
	await w.waitForURL(/\/game\//);
	const url = w.url().split('?')[0];
	const code = url.split('/').pop();
	await b.goto(url);
	await w.waitForSelector('.board');
	await b.waitForSelector('.board');

	// Random moves through the API until a turn's dice pick a square.
	let view;
	for (let ply = 0; ply < 60; ply++) {
		const [vw, vb] = [await viewOf(w, code), await viewOf(b, code)];
		const [mover, v] = vw.legal?.length ? [w, vw] : [b, vb];
		if (v.result) throw new Error(`the game ended at ply ${ply} before a pothole square was rolled`);
		const m = v.legal[Math.floor(Math.random() * v.legal.length)];
		const res = await mover.evaluate(
			([c, body]) => fetch(`/api/games/${c}/move`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }).then((r) => r.status),
			[code, { from: m.from, to: m.to, promotion: m.promotion, seq: v.seq }]
		);
		if (res >= 300) throw new Error(`move ${m.from}${m.to} answered ${res}`);
		view = await viewOf(w, code);
		if (ply >= 1 && view.last.some((e) => e.kind === 'target')) break;
	}
	const kinds = view.last.map((e) => e.kind).join(', ');
	console.log(`turn ${view.seq}: ${kinds}`);
	await sleep(8000); // the turn plays out on the open pages

	const results = [];
	const check = async (name, page, within) => {
		const left = await running(page, within);
		results.push([name, left]);
		console.log(`${left.length ? 'FAIL' : 'ok  '} ${name}${left.length ? `: ${left.length} running\n       ${[...new Set(left)].join('\n       ')}` : ''}`);
	};

	// A reload shows the last turn at once.
	await w.reload();
	await w.waitForSelector('.board .piece');
	await sleep(150);
	await check('desktop reload: board and tray', w);

	// The phone layout: its dice card on load, then the moves sheet once the page has settled.
	const pc = await newContext({ ...devices['iPhone 15'], defaultBrowserType: undefined });
	await pc.addCookies(await wc.cookies());
	const p = await pc.newPage();
	await p.goto(url);
	await p.waitForSelector('.board .piece');
	await sleep(150);
	await check('phone load: board and dice card', p);
	await sleep(3000);
	await p.getByRole('button', { name: 'Moves and rolls' }).tap();
	await p.waitForSelector('dialog.sheet[open]');
	await sleep(150);
	await check('phone: moves sheet opened', p, 'dialog.sheet');

	// A turn that arrives live still plays out: the next move's dice are thrown on the open desktop page.
	const next = await viewOf(w, code);
	if (!next.result) {
		const [vw, vb] = [next, await viewOf(b, code)];
		const [mover, v] = vw.legal?.length ? [w, vw] : [b, vb];
		const m = v.legal[Math.floor(Math.random() * v.legal.length)];
		await mover.evaluate(
			([c, body]) => fetch(`/api/games/${c}/move`, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(body) }),
			[code, { from: m.from, to: m.to, promotion: m.promotion, seq: v.seq }]
		);
		const seen = new Set();
		for (let i = 0; i < 40; i++) {
			for (const a of await running(w)) seen.add(a);
			await sleep(50);
		}
		const thrown = [...seen].some((a) => a.startsWith('die'));
		results.push(['live turn', thrown ? [] : ['no die thrown']]);
		console.log(`${thrown ? 'ok  ' : 'FAIL'} live turn: the dice are thrown${thrown ? '' : ` (saw ${[...seen].join(', ') || 'nothing'})`}`);
	}

	await browser.close();
	process.exit(results.some(([, left]) => left.length) ? 1 : 0);
}

main().catch((e) => {
	console.error(e);
	process.exit(1);
});
