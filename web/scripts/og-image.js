// Draws the link preview card (static/og.png, 1200×630) and the app icon
// (static/apple-touch-icon.png, 180×180) from HTML, in the game's colors,
// fonts, pieces and pothole, and screenshots them with Playwright. Rerun it
// when the look changes:
//
//   pnpm --dir web og-image
//
// The fonts come from Google Fonts, so it needs the network.

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = fileURLToPath(new URL('..', import.meta.url));
const tokens = readFileSync(`${root}src/lib/theme/tokens.css`, 'utf8').match(/:root\s*{[^}]*}/)[0];
const piece = (code) =>
	`data:image/svg+xml;base64,${readFileSync(`${root}static/pieces/${code}.svg`).toString('base64')}`;

// The board's traffic cone (Board.svelte's coneShape).
const cone = `<svg class="cone" viewBox="0 0 40 40" aria-hidden="true">
	<path class="cone-body" d="M17.5 4h5l9.5 29h-24z" />
	<path class="cone-band" d="M14.6 13h10.8l1.7 5H12.9zM11.6 22h16.8l1.7 5H9.9z" />
	<rect class="cone-base" x="4" y="32" width="32" height="5" rx="1.5" />
</svg>`;
const hole = (cones) => `<div class="hole"><div class="cones">${cone.repeat(cones)}</div></div>`;

const style = `
${tokens}
* { box-sizing: border-box; margin: 0; }
body { background: var(--bg); }
.cone { width: 32%; aspect-ratio: 1; overflow: visible; }
.cone-body, .cone-base { fill: var(--hazard); }
.cone-band { fill: var(--piece-light); }
.cone path, .cone rect { stroke: var(--piece-dark); stroke-width: 2.5; stroke-linejoin: round; }
.hole {
	position: relative; width: 76%; height: 76%;
	border-radius: 46% 54% 42% 58% / 55% 45% 55% 45%;
	background: var(--hole);
	box-shadow: 0 0 0 6px var(--hazard), inset 0 8px 20px var(--bg);
}
.cones { position: absolute; left: 50%; bottom: -16%; display: flex; justify-content: center; gap: 2%; width: 116%; transform: translateX(-50%); }
.sq { display: grid; place-items: center; }
.sq img { width: 92%; height: 92%; }
.l { background: var(--board-light); }
.d { background: var(--board-dark); }
`;

// Rows from the top, as White sees the board; "h" is a pothole.
const corner = [
	['', 'bQ', '', 'bP', ''],
	['bP', '', 'h3', '', 'bN'],
	['', 'wN', '', '', 'wP'],
	['wP', '', 'wP', 'h1', ''],
	['', 'wR', '', '', 'wK']
];
const squares = corner
	.flatMap((row, r) =>
		row.map((c, f) => {
			const shade = (r + f) % 2 ? 'd' : 'l';
			const inner = c.startsWith('h') ? hole(Number(c[1])) : c ? `<img src="${piece(c)}" alt="" />` : '';
			return `<div class="sq ${shade}">${inner}</div>`;
		})
	)
	.join('');

const card = `<!doctype html><html><head>
<link rel="stylesheet" href="https://fonts.googleapis.com/css2?family=Barlow+Condensed:wght@800&family=IBM+Plex+Mono:wght@500&family=IBM+Plex+Sans:wght@400&display=swap">
<style>${style}
body { width: 1200px; height: 630px; overflow: hidden; position: relative; font-family: var(--font-body); }
.text { position: absolute; left: 72px; top: 92px; width: 560px; display: grid; gap: 22px; }
.eyebrow { font: 500 26px/1 var(--font-mono); letter-spacing: .16em; color: var(--accent); }
h1 { font: 800 148px/.86 var(--font-display); color: var(--text); text-transform: uppercase; letter-spacing: .01em; }
p { font-size: 32px; line-height: 1.35; color: var(--text-body); max-width: 500px; }
.board { position: absolute; left: 650px; top: 40px; display: grid; grid-template-columns: repeat(5, 124px); grid-template-rows: repeat(5, 124px); border-radius: 10px; overflow: hidden; transform: rotate(-4deg); box-shadow: 0 0 0 4px var(--line); }
.stripe { position: absolute; left: 0; right: 0; bottom: 0; height: 22px; background: repeating-linear-gradient(-45deg, var(--accent) 0 22px, var(--bg) 22px 44px); }
</style></head><body>
<div class="text">
	<span class="eyebrow">A POTHOLE CHESS VARIANT</span>
	<h1>Mamdani<br>Chess</h1>
	<p>Potholes open under the pieces. Play a friend, no sign-up.</p>
</div>
<div class="board">${squares}</div>
<div class="stripe"></div>
</body></html>`;

const icon = `<!doctype html><html><head><style>${style}
body { width: 180px; height: 180px; overflow: hidden; display: grid; grid-template-columns: 90px 90px; grid-template-rows: 90px 90px; }
.center { position: absolute; inset: 40px 36px 50px; }
.center .hole { width: 100%; height: 100%; box-shadow: 0 0 0 6px var(--hazard), inset 0 6px 14px var(--bg); }
</style></head><body>
<div class="l"></div><div class="d"></div><div class="d"></div><div class="l"></div>
<div class="center">${hole(3)}</div>
</body></html>`;

const browser = await chromium.launch();
for (const [html, width, height, out] of [
	[card, 1200, 630, 'og.png'],
	[icon, 180, 180, 'apple-touch-icon.png']
]) {
	const page = await browser.newPage({ viewport: { width, height } });
	await page.setContent(html, { waitUntil: 'networkidle' });
	await page.evaluate(() => document.fonts.ready);
	await page.screenshot({ path: `${root}static/${out}` });
	console.log(`wrote static/${out}`);
	await page.close();
}
await browser.close();
