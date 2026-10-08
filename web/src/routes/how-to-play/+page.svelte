<script lang="ts">
	import Header from '#lib/Header.svelte';
	import Scene from '#lib/Scene.svelte';
	import SiteFooter from '#lib/SiteFooter.svelte';
	import { scenes, type Scene as SceneData } from '#lib/scenes.ts';
	import { HOLE_CAP, HOLE_ROUNDS } from '#lib/wire.gen.ts';

	// How this chess differs, one scene at a time on the real board. The
	// rules page has the same rules as a reference.

	interface Chapter {
		scene: SceneData;
		title: string;
		text: string;
		/** What a screen reader hears for the board. */
		label: string;
		rules: string; // the rules page's section
	}

	const chapters: Chapter[] = [
		{
			scene: scenes.dice,
			title: 'Every move rolls the dice',
			text: 'After every move, the mover rolls a d8. Odd: nothing happens. Even: two more d8s pick a file and a rank, a pothole opens there, and whatever stands on it falls in. Kings never fall: if the dice land on one, they roll again.',
			label: 'White moves and rolls a 3: nothing. Black moves and rolls a 6: the dice pick f3 and the white knight falls in. White moves and rolls a 4: the dice land on the black king, roll again, and open a pothole on h6.',
			rules: 'turn'
		},
		{
			scene: scenes.roads,
			title: 'Potholes block the road',
			text: "Nothing can land on a pothole. Bishops, rooks, queens and the Mamdani can't slide across one, so it cuts lines and checks like a piece would. Knights jump, so a pothole never stops them.",
			label: 'A rook on a3 can go no further along the rank than c3: a pothole on d3 is in the way. The knight on d2 jumps over the pothole to e4.',
			rules: 'potholes'
		},
		{
			scene: scenes.rounds,
			title: `${HOLE_ROUNDS} rounds, ${HOLE_CAP} at most`,
			text: `A pothole stays open for ${HOLE_ROUNDS} of its roller's moves, and the cones on its edge count them down. At most ${HOLE_CAP} are open at once: when another opens, the oldest closes. If the dice land on an open pothole, it resets: it's the roller's now, with ${HOLE_ROUNDS} rounds again.`,
			label: 'Each move takes a cone off the mover’s potholes, and the one on c4 closes. Black opens a pothole on a3, making five. White opens one on h5, and the oldest, on e5, closes. Then Black’s dice land on White’s f3, down to its last cone: it resets to Black’s, with three cones.',
			rules: 'potholes'
		},
		{
			scene: scenes.blocks,
			title: 'The Mamdani blocks',
			text: "The Mamdani belongs to neither player. On your turn you can move it instead of a piece, like a queen, onto empty squares. It never captures and can't be captured, but it blocks lines like any piece. Here it steps in front of a bishop's check.",
			label: 'A black bishop on b4 checks the white king on e1. White moves the Mamdani from f4 to d2, and the bishop can now go no further than c3.',
			rules: 'mamdani'
		},
		{
			scene: scenes.repairs,
			title: 'The Mamdani repairs',
			text: 'Any pothole next to the Mamdani is repaired at once: one it moves beside, and one the dice try to open there. Either player can send it, so it is the best answer to a bad roll.',
			label: 'Black moves the Mamdani from c3 to e5, next to the pothole on f6, which is repaired. White rolls a pothole on d4, next to the Mamdani: it is repaired before it opens.',
			rules: 'mamdani'
		},
		{
			scene: scenes.saving,
			title: 'Saving rolls',
			text: "When the dice pick a piece the Mamdani has a clear line to, its owner rolls one more d8. Odd: saved, and the pothole never opens. Even: it falls in.",
			label: 'The dice pick the white bishop on c3, which the Mamdani on a5 can see. White rolls a 5: saved. Then the dice pick the black knight on d8. Black rolls a 4: it falls in.',
			rules: 'saving'
		},
		{
			scene: scenes.mate,
			title: 'A roll can checkmate',
			text: "Kings never fall, but a pothole can take a king's last way out. Here the rook checks along the back rank, and the dice open a pothole on g7, the king's only escape: checkmate.",
			label: 'White’s rook moves to a8 and checks the black king on h8. The dice open a pothole on g7, its only escape: checkmate.',
			rules: 'winning'
		}
	];
</script>

<svelte:head>
	<title>How to play · Mamdani Chess</title>
</svelte:head>

<Header />

<main>
	<header class="intro">
		<p class="eyebrow">How to play</p>
		<h1>Chess, but the road fights back</h1>
		<p class="lede">
			Everything you know about chess still holds. Two things are new: potholes that open under your pieces, and the
			Mamdani, a piece both players share. Each scene below plays on the real board.
		</p>
		<a class="more" href="/rules">Read the full rules →</a>
	</header>

	<section class="chapter get" id="get-a-game" aria-labelledby="get-heading">
		<div class="text">
			<span class="num">01</span>
			<h2 id="get-heading">Get a game</h2>
			<p>No sign-up. Play the next person looking for a game, or send a friend a link: the first to open it plays Black.</p>
			<a class="more" href="/play">Play online now →</a>
		</div>
		<div class="cards" aria-hidden="true">
			<div class="card">
				<span class="card-title">Play online</span>
				<span class="swap">
					<span class="first">Looking for a player<span class="dots"><i>.</i><i>.</i><i>.</i></span></span>
					<span class="second found">Opponent found · You're Black</span>
				</span>
			</div>
			<div class="card">
				<span class="card-title">Invite a friend</span>
				<span class="link"><span class="url">mamdanichess.com/game/K7F3QZ</span><span class="copy">Copy</span></span>
				<span class="swap">
					<span class="first muted">Waiting for your friend…</span>
					<span class="second found">Friend joined · You're White, your move</span>
				</span>
			</div>
		</div>
	</section>

	{#each chapters as c, i (c.scene.id)}
		<section class="chapter" class:flip={i % 2 === 0} id={c.scene.id} aria-labelledby="{c.scene.id}-heading">
			<div class="text">
				<span class="num">{String(i + 2).padStart(2, '0')}</span>
				<h2 id="{c.scene.id}-heading">{c.title}</h2>
				<p>{c.text}</p>
				<a class="more" href="/rules#{c.rules}">The rules →</a>
			</div>
			<div class="stage">
				<Scene scene={c.scene} label={c.label} />
			</div>
		</section>
	{/each}

	<div class="cta">
		<a class="action" href="/play">Play online</a>
		<a class="more" href="/practice">Try it yourself →</a>
		<a class="more" href="/rules">Read the full rules →</a>
	</div>
	<SiteFooter />
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		max-width: 1120px;
		margin: 0 auto;
		padding: 56px 32px 0;
	}
	.intro {
		display: flex;
		flex-direction: column;
		gap: 18px;
		max-width: 760px;
		padding-bottom: 56px;
	}
	.eyebrow {
		margin: 0;
		color: var(--accent);
		font-family: var(--font-mono);
		font-size: 13px;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	h1 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: clamp(48px, 8vw, 88px);
		line-height: 0.92;
		text-transform: uppercase;
	}
	.lede {
		margin: 0;
		font-size: 19px;
		line-height: 1.55;
	}
	.more {
		align-self: flex-start;
		padding: 10px 0;
		margin: -10px 0;
		color: var(--accent);
		font-weight: 600;
		text-decoration: none;
	}
	.more:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.chapter {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(0, 440px);
		gap: 64px;
		align-items: center;
		padding: 56px 0;
		border-top: 1px solid var(--surface-2);
		scroll-margin-top: 16px;
	}
	.chapter.flip {
		grid-template-columns: minmax(0, 440px) minmax(0, 1fr);
	}
	.chapter.flip .text {
		order: 2;
	}
	.text {
		display: flex;
		flex-direction: column;
		gap: 14px;
		max-width: 520px;
	}
	.num {
		color: var(--accent);
		font-family: var(--font-mono);
		font-weight: 600;
	}
	h2 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 44px;
		line-height: 1;
		text-transform: uppercase;
	}
	p {
		margin: 0;
		font-size: 17px;
		line-height: 1.6;
	}
	.stage {
		min-width: 0;
	}

	/* "Get a game": two cards that loop between before and after. */
	.cards {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	.card {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 20px;
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		background: var(--surface);
	}
	.card-title {
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 24px;
		text-transform: uppercase;
	}
	.link {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 6px 6px 6px 12px;
		border: 1px solid var(--line);
		border-radius: 10px;
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.url {
		flex: 1;
		min-width: 0;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}
	.copy {
		padding: 6px 12px;
		border-radius: 8px;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-body);
		font-weight: 600;
		animation: copied 6s infinite;
	}
	.swap {
		display: grid;
	}
	.swap > span {
		grid-area: 1 / 1;
	}
	.first {
		animation: before 6s infinite;
	}
	.second {
		opacity: 0;
		animation: after 6s infinite;
	}
	.found {
		color: var(--accent);
		font-weight: 600;
	}
	.muted {
		color: var(--text-muted);
	}
	.dots i {
		font-style: normal;
		animation: blink 1.2s infinite;
	}
	.dots i:nth-child(2) {
		animation-delay: 0.2s;
	}
	.dots i:nth-child(3) {
		animation-delay: 0.4s;
	}
	@keyframes before {
		0%,
		50% {
			opacity: 1;
		}
		55%,
		95% {
			opacity: 0;
		}
		100% {
			opacity: 1;
		}
	}
	@keyframes after {
		0%,
		50% {
			opacity: 0;
		}
		55%,
		95% {
			opacity: 1;
		}
		100% {
			opacity: 0;
		}
	}
	@keyframes copied {
		0%,
		20%,
		32%,
		100% {
			transform: none;
		}
		25% {
			transform: scale(0.92);
		}
	}
	@keyframes blink {
		0%,
		100% {
			opacity: 0.2;
		}
		50% {
			opacity: 1;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.first,
		.copy,
		.dots i {
			animation: none;
		}
		.first {
			opacity: 0;
		}
		.second {
			animation: none;
			opacity: 1;
		}
	}

	.cta {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 24px;
		padding: 56px 0;
		border-top: 1px solid var(--surface-2);
	}
	.action {
		padding: 14px 28px;
		border-radius: 12px;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 24px;
		text-decoration: none;
		text-transform: uppercase;
	}

	/* A phone on its side: a scene as tall as the screen, so it plays and
	   shows whole. */
	@media (orientation: landscape) and (max-height: 500px) {
		.stage {
			justify-self: center;
			width: min(100%, calc(100dvh - 140px));
		}
	}
	/* Phones and narrow windows: the words, then the board. */
	@media (max-width: 799px) {
		main {
			padding: 32px 16px 0;
		}
		.intro {
			padding-bottom: 32px;
		}
		.chapter,
		.chapter.flip {
			grid-template-columns: minmax(0, 1fr);
			gap: 24px;
			padding: 40px 0;
		}
		.chapter.flip .text {
			order: 0;
		}
		h2 {
			font-size: 36px;
		}
	}
</style>
