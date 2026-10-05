<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Header from '#lib/Header.svelte';
	import MiniBoard from '#lib/MiniBoard.svelte';
	import { createGame } from '#lib/game.ts';
	import { boardFromFen, isCode, liveGames, myName, normalizeCode, type LiveGame } from '#lib/lobby.ts';

	// The canvas's hero position: a pothole open on f4, the Mamdani on a5.
	const demo = boardFromFen('r1bqkb1r/1p3ppp/p1n1pn2/3N4/3P4/5N2/PP2PPPP/R1BQKB1R');
	const REFRESH_MS = 10_000;

	let name = $state<string | null>(null);
	let games = $state<LiveGame[] | null>(null); // null until the first answer
	let looking = $state(0);
	let code = $state('');
	let busy = $state(false);
	let error = $state('');

	async function refresh() {
		try {
			({ games, looking } = await liveGames());
		} catch {
			// Keep what's shown; the next refresh tries again.
		}
	}

	onMount(() => {
		myName()
			.then((n) => (name = n))
			.catch(() => {});
		refresh();
		// Every 10 s while the tab is visible, and at once when it comes back.
		const timer = setInterval(() => {
			if (!document.hidden) refresh();
		}, REFRESH_MS);
		const onVisible = () => {
			if (!document.hidden) refresh();
		};
		document.addEventListener('visibilitychange', onVisible);
		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisible);
		};
	});

	async function playFriend() {
		busy = true;
		error = '';
		try {
			await goto(`/game/${await createGame()}`);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			busy = false;
		}
	}

	function join(e: SubmitEvent) {
		e.preventDefault();
		if (isCode(code)) goto(`/game/${code}`);
	}

	const side = (name: string, fallback: string) => name || fallback;
</script>

<svelte:head>
	<title>Pothole Chess: Mamdani Edition</title>
</svelte:head>

<Header bind:name />

<main>
	<section class="hero">
		<div class="pitch">
			<p class="eyebrow">A chess variant · 10+5 · No sign-up</p>
			<h1>Chess.<br />With potholes.</h1>
			<p class="lede">
				Every move rolls the dice. Potholes open and swallow pieces. The Mamdani, a neutral piece either player can
				move, blocks lines and fixes the road.
			</p>
			<div class="actions">
				<a class="action primary" href="/play">
					<span class="label">Play online</span>
					<span class="sub">Quick match · 10+5{#if looking > 0}{' · '}<strong>{looking} looking</strong>{/if}</span>
				</a>
				<button class="action" onclick={playFriend} disabled={busy}>
					<span class="label">{busy ? 'Starting…' : 'Play a friend'}</span>
					<span class="sub">Get a link to share</span>
				</button>
			</div>
			{#if error}<p class="error" role="alert">{error}</p>{/if}
			<form class="join" onsubmit={join}>
				<label for="join-code">Have a game code?</label>
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
					/>
					<button type="submit" disabled={!isCode(code)}>Join</button>
				</div>
			</form>
		</div>
		<div class="demo">
			<MiniBoard
				board={demo}
				potholes={[{ sq: 'f4', by: 'white' }]}
				mamdani="a5"
				last={{ from: 'e7', to: 'e6' }}
				label="A game in progress: a pothole is open on f4 and the Mamdani stands on a5."
			/>
			<p class="callout" aria-hidden="true"><span class="dot"></span>e6 · d8 4 → pothole at f4</p>
		</div>
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
					<li>
						<a class="card" href="/game/{g.code}">
							<MiniBoard
								board={g.board}
								potholes={g.potholes}
								mamdani={g.mamdani}
								last={g.last}
								label="{side(g.white, 'White')} against {side(g.black, 'Black')}, move {g.move}"
							/>
							<span class="players">
								<span class="player" title={side(g.white, 'White')}
									><span class="swatch white" aria-hidden="true"></span><span class="pname">{side(g.white, 'White')}</span></span
								>
								<span class="player" title={side(g.black, 'Black')}
									><span class="swatch black" aria-hidden="true"></span><span class="pname">{side(g.black, 'Black')}</span></span
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

	<section class="how" aria-labelledby="how-heading">
		<h2 id="how-heading">How it works</h2>
		<div class="steps">
			<div>
				<span class="num hazard">01</span>
				<h3>Potholes open</h3>
				<p>
					After every move, roll a d8. Even, and two more d8s pick a square: whatever stands there falls in. Kings
					never do.
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
	</section>

	<footer>Based on Pot-Hole Chess by Peter Spicer and Michael Chamberlain (2001).</footer>
</main>

<style>
	main {
		max-width: 1280px;
		margin: 0 auto;
		padding: 0 32px;
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
	.join label {
		color: var(--text-muted);
		font-size: 14px;
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
		padding: 8px 0 88px;
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
	.how {
		padding: 56px 0 72px;
		border-top: 1px solid var(--surface-2);
	}
	.how h2 {
		margin-bottom: 32px;
	}
	.steps {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		gap: 40px;
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
		.live {
			padding-bottom: 40px;
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
		.steps {
			grid-template-columns: minmax(0, 1fr);
			gap: 24px;
		}
	}
	/* The narrowest phones: the line under each button goes below it. */
	@media (max-width: 379px) {
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
