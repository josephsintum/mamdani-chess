<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Header from '#lib/Header.svelte';
	import MiniBoard from '#lib/MiniBoard.svelte';
	import { FriendGame } from '#lib/friend.svelte.ts';
	import { boardFromFen, isCode, liveGames, me, normalizeCode, type LiveGame, type Me } from '#lib/lobby.ts';

	// The canvas's hero position: a pothole open on f4, the Mamdani on a5.
	const demo = boardFromFen('r1bqkb1r/1p3ppp/p1n1pn2/3N4/3P4/5N2/PP2PPPP/R1BQKB1R');
	const REFRESH_MS = 10_000;

	let user = $state<Me | null>(null);
	let games = $state<LiveGame[] | null>(null); // null until the first answer
	let looking = $state(0);
	let code = $state('');
	// Friends arrive by link, so the code field waits behind "Have a game code?".
	let codeOpen = $state(false);
	const friend = new FriendGame();

	async function refresh() {
		try {
			({ games, looking } = await liveGames());
		} catch {
			// Keep what's shown; the next refresh tries again.
		}
	}

	// Your name, changes and any game you're in (the Rejoin banner).
	function refreshMe() {
		me()
			.then((m) => (user = m))
			.catch(() => {}); // keep what's shown, as refresh does
	}

	// Every 10 s while the tab is visible, and at once when it comes back.
	function update() {
		if (!document.hidden) {
			void refresh();
			refreshMe();
		}
	}

	onMount(() => {
		refreshMe();
		void refresh();
		const timer = setInterval(update, REFRESH_MS);
		return () => clearInterval(timer);
	});

	function join(e: SubmitEvent) {
		e.preventDefault();
		if (isCode(code)) void goto(`/game/${code}`);
	}
</script>

<svelte:head>
	<title>Mamdani Chess</title>
</svelte:head>

<svelte:document onvisibilitychange={update} />

<Header bind:me={user} />

<main>
	<section class="hero">
		<div class="pitch">
			<p class="eyebrow">A chess variant · 10+5 · No sign-up</p>
			<h1>Chess.<br />With potholes.</h1>
			<p class="lede">
				Every move rolls the dice. Potholes open and swallow pieces. The Mamdani, a neutral piece either player can
				move, blocks lines and fixes the road.
			</p>
			{#if user?.game}
				<!-- One game at a time: while you're in one, Rejoin takes the place of the ways to start another. -->
				<div class="rejoin">
					<p class="rejoin-head"><span class="dot" aria-hidden="true"></span>You're in a game</p>
					<p class="rejoin-text">Your opponent is waiting.</p>
					<a class="action primary" href="/game/{user.game}"><span class="label">Rejoin game</span></a>
				</div>
				<p class="note">One game at a time. Finish this one to start another.</p>
			{:else}
				<div class="actions">
					<a class="action primary" href="/play">
						<span class="label">Play online</span>
						<span class="sub"><span class="wide">Quick match · </span>10+5{#if looking > 0}{' · '}<strong>{looking} looking</strong>{/if}</span>
					</a>
					<button class="action" onclick={friend.start} disabled={friend.busy}>
						<span class="label">{friend.busy ? 'Starting…' : 'Invite a friend'}</span>
						<span class="sub"><span class="wide">Send a link to your group chat</span><span class="narrow">Share a link</span></span>
					</button>
				</div>
				{#if friend.error}<p class="error" role="alert">{friend.error}</p>{/if}
				{#if codeOpen}
					<form class="join" onsubmit={join}>
						<label for="join-code" class="sr-only">Game code</label>
						<div class="row">
							<input
								id="join-code"
								type="text"
								placeholder="K7F3QZ"
								autocomplete="off"
								autocapitalize="characters"
								spellcheck="false"
								value={code}
								oninput={(e) => {
									code = normalizeCode(e.currentTarget.value);
									e.currentTarget.value = code;
								}}
								{@attach (input) => input.focus()}
							/>
							<button type="submit" disabled={!isCode(code)}>Join</button>
						</div>
					</form>
				{:else}
					<button class="code-link" type="button" onclick={() => (codeOpen = true)}>Have a game code?</button>
				{/if}
			{/if}
		</div>
		<div class="demo">
			<MiniBoard
				board={demo}
				potholes={[{ sq: 'f4', by: 'white', left: 3 }]}
				mamdani="a5"
				last={{ from: 'e7', to: 'e6' }}
				label="A game in progress: a pothole is open on f4 and the Mamdani stands on a5."
			/>
			<p class="callout" aria-hidden="true"><span class="dot"></span>e6 · d8 4 → pothole at f4</p>
		</div>
	</section>

	<section class="how" aria-labelledby="how-heading">
		<div class="how-text">
			<h2 id="how-heading">How it works</h2>
			<div class="steps">
				<div>
					<span class="num hazard">01</span>
					<h3>Potholes open</h3>
					<p>
						After every move, roll a d8. Even, and two more d8s pick a square: whatever stands there falls in. Kings
						never do. A pothole stays open for three of its roller's moves, and at most five are open at once.
					</p>
				</div>
				<div>
					<span class="num accent">02</span>
					<h3>The Mamdani</h3>
					<p>
						A neutral piece that moves like a queen. Either player can spend a turn moving it. It never captures, but
						it blocks lines and repairs any pothole next to it.
					</p>
				</div>
				<div>
					<span class="num">03</span>
					<h3>Saving rolls</h3>
					<p>If the Mamdani has a clear line to a doomed piece, roll a d8. Odd, and the piece is saved.</p>
				</div>
			</div>
		</div>
		<figure class="reel">
			<iframe
				src="https://www.instagram.com/reel/Dd8tV_MxEQL/embed/"
				title="The Mamdani Patch, a reel by @bardelo_bardalini on Instagram"
				loading="lazy"
				allowfullscreen
			></iframe>
			<figcaption>
				Credit: the Mamdani comes from
				<a href="https://www.instagram.com/reel/Dd8tV_MxEQL/" target="_blank" rel="noopener">The Mamdani Patch</a>
				by
				<a href="https://www.instagram.com/bardelo_bardalini/" target="_blank" rel="noopener">@bardelo_bardalini</a>. All
				credit to the original creator.
			</figcaption>
		</figure>
	</section>

	<section class="live" aria-labelledby="live-heading">
		<div class="live-head">
			<h2 id="live-heading">Live now</h2>
			{#if games && games.length > 0}
				<span class="count"><span class="dot"></span>{games.length} {games.length === 1 ? 'game' : 'games'}</span>
			{/if}
		</div>
		{#if games === null}
			<p class="quiet">Loading games…</p>
		{:else if games.length === 0}
			<p class="quiet">No games right now. Start one: play online or invite a friend.</p>
		{:else}
			<ul class="cards">
				{#each games as g (g.code)}
					{@const white = g.white || 'White'}
					{@const black = g.black || 'Black'}
					<li>
						<a class="card" href="/game/{g.code}">
							<MiniBoard
								board={g.board}
								potholes={g.potholes}
								mamdani={g.mamdani}
								last={g.last}
								label="{white} against {black}, move {g.move}"
							/>
							<span class="players">
								<span class="player" title={white}
									><span class="swatch white" aria-hidden="true"></span><span class="pname">{white}</span></span
								>
								<span class="player" title={black}
									><span class="swatch black" aria-hidden="true"></span><span class="pname">{black}</span></span
								>
							</span>
							<span class="meta">
								<span>Move {g.move}</span>
								<span>{g.watching} watching</span>
							</span>
						</a>
					</li>
				{/each}
			</ul>
		{/if}
	</section>

	<footer>Based on Pot-Hole Chess by Peter Spicer and Michael Chamberlain (2001).</footer>
</main>

<style>
	main {
		max-width: 1280px;
		margin: 0 auto;
		padding: 0 32px;
	}
	.rejoin {
		display: flex;
		flex-direction: column;
		gap: 12px;
		max-width: 460px;
		padding: 16px;
		border: 1px solid var(--accent);
		border-radius: 12px;
		background: var(--accent-wash);
	}
	.rejoin-head {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 24px;
		letter-spacing: 0.02em;
		text-transform: uppercase;
	}
	.rejoin .dot {
		background: var(--accent);
	}
	.rejoin-text {
		margin: 0;
	}
	.note {
		margin: -12px 0 0;
		color: var(--text-muted);
		font-size: 14px;
	}
	.hero {
		display: grid;
		grid-template-columns: minmax(0, 1.05fr) minmax(0, 1fr);
		gap: 64px;
		align-items: center;
		padding: 72px 0 88px;
	}
	.pitch {
		display: flex;
		flex-direction: column;
		gap: 28px;
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
		font-size: clamp(56px, 8vw, 104px);
		line-height: 0.9;
		text-transform: uppercase;
	}
	.lede {
		margin: 0;
		max-width: 520px;
		font-size: 20px;
		line-height: 1.5;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 16px;
	}
	.action {
		display: flex;
		flex-direction: column;
		gap: 2px;
		min-width: 220px;
		padding: 15px 23px;
		border: 1px solid var(--line);
		border-radius: 12px;
		background: transparent;
		color: var(--text);
		font: inherit;
		text-align: left;
		text-decoration: none;
		cursor: pointer;
	}
	.action.primary {
		border-color: var(--accent);
		background: var(--accent);
		color: var(--accent-text);
	}
	.action:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.label {
		white-space: nowrap;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 28px;
		letter-spacing: 0.02em;
		text-transform: uppercase;
	}
	.sub {
		color: var(--text-muted);
		font-size: 14px;
		font-weight: 600;
	}
	.primary .sub {
		color: var(--accent-text);
	}
	.join {
		display: flex;
		flex-direction: column;
		gap: 8px;
		max-width: 460px;
	}
	.code-link {
		align-self: flex-start;
		height: 44px;
		margin-top: -12px;
		padding: 0 2px;
		border: 0;
		background: none;
		color: var(--text-muted);
		font: inherit;
		text-decoration: underline;
		text-underline-offset: 3px;
		cursor: pointer;
	}
	.code-link:hover {
		color: var(--text);
	}
	.narrow {
		display: none;
	}
	.row {
		display: flex;
		gap: 8px;
	}
	.join input {
		flex-grow: 1;
		min-width: 0;
		height: 48px;
		padding: 0 16px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 18px;
		letter-spacing: 0.18em;
	}
	.join button {
		height: 48px;
		padding: 0 22px;
		border: 0;
		border-radius: 10px;
		background: var(--surface-2);
		color: var(--text);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.join button:disabled {
		color: var(--text-muted);
		cursor: default;
	}
	.demo {
		position: relative;
		max-width: 520px;
		width: 100%;
		justify-self: center;
	}
	.callout {
		position: absolute;
		left: -12px;
		bottom: 28px;
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0;
		padding: 10px 14px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--surface);
		box-shadow: 0 10px 24px var(--hole);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 14px;
	}
	.dot {
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--hazard);
	}
	.live {
		padding: 56px 0 88px;
		border-top: 1px solid var(--surface-2);
	}
	.live-head {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 16px;
		margin-bottom: 24px;
	}
	h2 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 44px;
		text-transform: uppercase;
	}
	.count {
		display: flex;
		align-items: center;
		gap: 8px;
		color: var(--text-muted);
		font-family: var(--font-mono);
		font-size: 14px;
	}
	.count .dot {
		width: 8px;
		height: 8px;
		background: var(--accent);
	}
	.quiet {
		margin: 0;
		color: var(--text-muted);
	}
	.cards {
		display: grid;
		grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
		gap: 24px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.card {
		display: flex;
		flex-direction: column;
		gap: 14px;
		padding: 16px;
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		background: var(--surface);
		color: var(--text);
		text-decoration: none;
	}
	.card:hover,
	.card:focus-visible {
		border-color: var(--accent-line);
	}
	.players {
		display: flex;
		flex-direction: column;
		gap: 6px;
		min-width: 0;
	}
	.player {
		display: flex;
		align-items: center;
		gap: 8px;
		min-width: 0;
		font-weight: 600;
		font-size: 15px;
	}
	.pname {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.swatch {
		flex-shrink: 0;
		width: 10px;
		height: 10px;
		border-radius: 2px;
		box-shadow: 0 0 0 1px var(--line);
	}
	.swatch.white {
		background: var(--piece-light);
	}
	.swatch.black {
		background: var(--piece-dark);
	}
	.meta {
		display: flex;
		justify-content: space-between;
		color: var(--text-muted);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	/* The steps on the left, the reel the Mamdani comes from on the right. */
	.how {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 400px;
		gap: 64px;
		align-items: start;
		padding: 56px 0 72px;
		border-top: 1px solid var(--surface-2);
	}
	.how h2 {
		margin-bottom: 32px;
	}
	.steps {
		display: flex;
		flex-direction: column;
		gap: 32px;
		max-width: 560px;
	}
	.reel {
		display: flex;
		flex-direction: column;
		gap: 14px;
		width: 100%;
		max-width: 400px;
		margin: 0;
	}
	/* Instagram's embed doesn't size itself without its script: a 4:5 video
	   plus 210px of header and footer. */
	.reel iframe {
		width: 100%;
		height: calc(min(400px, 100vw - 32px) * 1.25 + 210px);
		border: 0;
		border-radius: 12px;
		background: var(--surface);
	}
	figcaption {
		color: var(--text-muted);
		font-size: 14px;
		line-height: 1.5;
	}
	figcaption a {
		color: var(--accent);
	}
	/* Too narrow for the reel beside the steps: the reel goes under them. */
	@media (max-width: 959px) {
		.how {
			grid-template-columns: minmax(0, 1fr);
			gap: 32px;
		}
	}
	.steps div {
		display: flex;
		flex-direction: column;
		gap: 10px;
	}
	.num {
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 14px;
	}
	.num.hazard {
		color: var(--hazard);
	}
	.num.accent {
		color: var(--accent);
	}
	h3 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 700;
		font-size: 28px;
		text-transform: uppercase;
	}
	.steps p {
		margin: 0;
		font-size: 16px;
		line-height: 1.55;
	}
	footer {
		padding: 24px 0 40px;
		border-top: 1px solid var(--surface-2);
		color: var(--text-muted);
		font-size: 13px;
	}
	.error {
		margin: 0;
		color: var(--hazard-text);
	}
	/* Phones (canvas "Home (phone)"): one column, no demo board, live games
	   in a row that scrolls sideways. */
	@media (max-width: 639px) {
		main {
			padding: 0 16px;
		}
		.hero {
			grid-template-columns: minmax(0, 1fr);
			gap: 18px;
			padding: 32px 0 40px;
		}
		.pitch {
			gap: 18px;
		}
		.lede {
			font-size: 16px;
		}
		.demo {
			display: none;
		}
		.actions {
			flex-direction: column;
			gap: 10px;
		}
		.action {
			flex-direction: row;
			align-items: center;
			justify-content: space-between;
			min-width: 0;
			height: 60px;
			padding: 0 20px;
		}
		.label {
			font-size: 26px;
		}
		/* The short lines under the buttons, so each fits beside its label. */
		.wide {
			display: none;
		}
		.narrow {
			display: inline;
		}
		.code-link {
			margin-top: -8px;
		}
		.note {
			margin-top: -8px;
		}
		h2 {
			font-size: 34px;
		}
		.cards {
			display: flex;
			gap: 12px;
			margin-right: -16px;
			padding-right: 16px;
			overflow-x: auto;
			scroll-snap-type: x mandatory;
		}
		.cards li {
			flex-shrink: 0;
			width: 164px;
			scroll-snap-align: start;
		}
		.card {
			gap: 10px;
			padding: 12px;
			border-radius: 12px;
		}
		.player {
			font-size: 13px;
		}
		.meta {
			font-size: 12px;
		}
		.how {
			padding: 32px 0 40px;
		}
		.live {
			padding: 32px 0 40px;
		}
		.steps {
			gap: 24px;
		}
	}
	/* The narrowest phones: a smaller headline so both buttons sit above
	   the fold at 320 x 568, and the line under each button goes below it. */
	@media (max-width: 379px) {
		.hero {
			padding-top: 24px;
		}
		.pitch {
			gap: 16px;
		}
		.eyebrow {
			font-size: 11px;
			letter-spacing: 0.1em;
		}
		h1 {
			font-size: 44px;
		}
		.action {
			flex-direction: column;
			align-items: flex-start;
			justify-content: center;
			height: auto;
			min-height: 60px;
			padding: 10px 20px;
		}
	}
</style>
