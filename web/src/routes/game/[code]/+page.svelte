<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { reducedMotion, setInstant } from '#lib/motion.ts';
	import { dev } from '$app/env';
	import { page } from '$app/state';
	import Board from '#lib/Board.svelte';
	import DiceTray from '#lib/DiceTray.svelte';
	import MoveLog from '#lib/MoveLog.svelte';
	import PlayerBar from '#lib/PlayerBar.svelte';
	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
	import { stageAt } from '#lib/board.ts';
	import { applyMove } from '#lib/pieces.ts';
	import { createGame, reasons, resign, sendMove, type Color, type MoveJSON, type View } from '#lib/game.ts';

	const code = page.params.code ?? '';

	// Dev only: /game/CODE?instant turns every animation off, for fast play-testing.
	const instant = dev && page.url.searchParams.has('instant');
	setInstant(instant);
	const anim = new Animator(instant ? 0 : STEP_MS);
	let view = $derived(anim.view);
	let shown = $derived(anim.shown);
	// Your move, shown before the server confirms it (One Million Chessboards
	// style): the piece glides at once. If the server refuses the move, the
	// guess is dropped and the piece glides back.
	let optimistic = $state<{ seq: number; move: MoveJSON } | null>(null);
	let connected = $state(false);
	let notFound = $state(false);
	let lost = $state(false);
	let error = $state('');
	let busy = $state(false); // a move or resignation is on its way
	let confirmResign = $state(false);
	let copyHint = $state('');

	function receive(next: View) {
		error = '';
		optimistic = null;
		anim.receive(next, { hidden: document.hidden });
	}

	onMount(() => {
		const source = new EventSource(`/api/games/${code}/stream`);
		source.onopen = () => (connected = true);
		source.onerror = () => {
			connected = false;
			// A 404 or other error status closes the stream for good; network
			// errors reconnect by themselves. Games live in memory for now, so a
			// server restart is the usual way a game in progress disappears.
			if (source.readyState === EventSource.CLOSED) {
				if (view) lost = true;
				else notFound = true;
			}
		};
		source.addEventListener('state', (e) => receive(JSON.parse((e as MessageEvent<string>).data)));
		// Coming back to a tab mid-animation jumps to the end of the roll.
		const finishOnReturn = () => {
			if (!document.hidden) anim.finish();
		};
		document.addEventListener('visibilitychange', finishOnReturn);
		return () => {
			source.close();
			anim.stop();
			document.removeEventListener('visibilitychange', finishOnReturn);
		};
	});

	let animating = $derived(anim.animating);
	let stage = $derived.by(() => {
		if (!view) return null;
		const base = stageAt(view, shown);
		if (optimistic?.seq !== view.seq) return base;
		return { ...base, ...applyMove(base, optimistic.move) };
	});
	let checkSquare = $derived.by(() => {
		if (!view?.check || !stage || animating) return '';
		const i = stage.board.indexOf(view.turn === 'white' ? 'wK' : 'bK');
		return i < 0 ? '' : 'abcdefgh'[i % 8] + (Math.floor(i / 8) + 1);
	});
	let savedSquare = $derived(view?.last.find((e, i) => e.kind === 'saving_roll' && e.saved && i < shown)?.sq ?? '');
	let you = $derived(view?.you ?? 'spectator');
	let bottom = $derived<Color>(you === 'black' ? 'black' : 'white');
	let top = $derived<Color>(bottom === 'white' ? 'black' : 'white');
	let lastMove = $derived.by(() => {
		if (optimistic && optimistic.seq === view?.seq) return { from: optimistic.move.from, to: optimistic.move.to };
		const m = view?.last.find((e) => e.kind === 'moved');
		return m?.from && m?.to ? { from: m.from, to: m.to } : null;
	});
	let playing = $derived(view?.status === 'playing');
	let isPlayer = $derived(you === 'white' || you === 'black');

	async function move(m: MoveJSON) {
		if (!view || busy) return;
		busy = true;
		optimistic = { seq: view.seq, move: m };
		try {
			error = (await sendMove(code, m, view.seq)) ?? '';
		} catch {
			error = 'Could not reach the server. Try again.';
		} finally {
			busy = false;
		}
		if (error) optimistic = null; // refused or unsent: glide back
	}

	async function doResign() {
		busy = true;
		error = (await resign(code)) ?? '';
		busy = false;
		confirmResign = false;
	}

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(page.url.href);
			copyHint = 'Link copied';
		} catch {
			// No clipboard on plain-http addresses: select the link instead.
			(document.getElementById('link') as HTMLInputElement | null)?.select();
			copyHint = 'Press Ctrl+C (⌘C on a Mac) to copy';
		}
	}

	async function newGame() {
		busy = true;
		try {
			// A full page load gives the new game a fresh stream.
			location.href = `/game/${await createGame()}`;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			busy = false;
		}
	}

	let status = $derived.by(() => {
		if (!view) return '';
		if (view.status === 'waiting')
			return you === 'white' ? 'Waiting for your friend to open the link…' : 'Waiting for White’s friend to join…';
		if (view.result) return animating ? 'Last move played…' : 'Game over';
		if (animating) return 'Dice are rolling…';
		const side = view.turn === 'white' ? 'White' : 'Black';
		const check = view.check ? ' — check!' : '';
		return (view.turn === you ? 'Your move' : `${side} to move`) + check;
	});

	let resultCard = $derived.by(() => {
		const r = view?.result;
		if (!view || !r || animating) return null;
		const why = reasons[r.reason] ?? r.reason;
		const winner = r.winner === 'white' ? 'White' : 'Black';
		const loser = r.winner === 'white' ? 'Black' : 'White';
		const lastSan = view.log.at(-1)?.san ?? '';
		let detail = `By ${why}.`;
		if (r.reason === 'resignation') detail = `${loser} resigned.`;
		if (r.reason === 'checkmate') detail = `${winner} mated with ${lastSan}.`;
		return {
			kicker: `${why} · Move ${Math.max(1, Math.ceil(view.seq / 2))}`,
			title: r.draw ? 'Draw' : `${winner} wins`,
			detail
		};
	});
</script>

<svelte:head>
	<title>Game {code} · Pothole Chess</title>
</svelte:head>

<main>
	<header>
		<a href="/" class="logo">Pothole Chess</a>
		<span class="code">{code}</span>
		{#if view && !connected && !lost}<span class="warn">Reconnecting…</span>{/if}
	</header>

	{#if lost}
		<p class="error" role="alert">
			Lost the connection to this game. If the server restarted, the game is gone.
			<a href={`/game/${code}`} data-sveltekit-reload>Reload</a> or <a href="/">start a new one</a>.
		</p>
	{/if}

	{#if notFound}
		<p>Game not found. <a href="/">Start a new one</a>.</p>
	{:else if !view || !stage}
		<p>Connecting…</p>
	{:else}
		<p class="status" aria-live="polite">
			{status}
			{#if you === 'spectator'}<span class="muted">(watching)</span>{/if}
		</p>

		{#if view.status === 'waiting' && you === 'white'}
			<div class="share">
				<label for="link">Send this link to your friend</label>
				<div class="share-row">
					<input id="link" readonly value={page.url.href} />
					<button class="primary" onclick={copyLink}>Copy link</button>
				</div>
				{#if copyHint}<span class="muted">{copyHint}</span>{/if}
			</div>
		{/if}

		<div class="layout">
			<div class="log-col">
				<MoveLog log={view.log} rolling={animating} />
			</div>

			<div class="board-col">
				<PlayerBar color={top} you={you === top} lost={stage.lost[top]} toMove={playing && view.turn === top && !animating} />
				<div class="board-wrap">
					<Board
						{stage}
						legal={view.legal}
						{lastMove}
						flipped={bottom === 'black'}
						interactive={!animating && !busy && !optimistic && playing}
						dim={!!resultCard}
						check={checkSquare}
						saved={savedSquare}
						onmove={move}
					/>
					{#if resultCard}
						<div class="result" role="status" in:fly={{ y: -24, duration: reducedMotion() ? 0 : 500 }}>
							<span class="kicker">{resultCard.kicker}</span>
							<h1>{resultCard.title}</h1>
							<p>{resultCard.detail}</p>
						</div>
					{/if}
				</div>
				<PlayerBar
					color={bottom}
					you={you === bottom}
					lost={stage.lost[bottom]}
					toMove={playing && view.turn === bottom && !animating}
				/>
			</div>

			<div class="side-col">
				<DiceTray {view} {shown} />
				{#if error}<p class="error" role="alert">{error}</p>{/if}

				{#if resultCard}
					<dl class="stats">
						<div><dt>Lost to potholes</dt><dd>White {view.lost.white.length} · Black {view.lost.black.length}</dd></div>
						<div><dt>Saving rolls</dt><dd>{view.stats.saved} of {view.stats.savingRolls} saved</dd></div>
						<div>
							<dt>Repaired by the Mamdani</dt>
							<dd>{view.stats.repaired} {view.stats.repaired === 1 ? 'pothole' : 'potholes'}</dd>
						</div>
						{#if view.stats.mamdaniFell}<div><dt>The Mamdani</dt><dd>fell in</dd></div>{/if}
					</dl>
					<div class="actions">
						<button class="primary" onclick={newGame} disabled={busy}>New game</button>
						<a class="outline" href="/">Home</a>
					</div>
				{:else if isPlayer && playing}
					{#if confirmResign}
						<div class="confirm" role="group" aria-label="Confirm resignation">
							<span>Resign this game?</span>
							<button class="danger" onclick={doResign} disabled={busy}>Yes, resign</button>
							<button class="outline" onclick={() => (confirmResign = false)}>Keep playing</button>
						</div>
					{:else}
						<button class="outline resign" onclick={() => (confirmResign = true)} disabled={busy}>
							<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 22V4"></path><path d="M4 4h12l-2 4 2 4H4"></path></svg>
							Resign
						</button>
					{/if}
				{/if}
			</div>
		</div>
	{/if}
</main>

<style>
	main {
		max-width: 1400px;
		margin: 0 auto;
		padding: 16px 24px 40px;
		display: grid;
		gap: 12px;
	}
	header {
		display: flex;
		align-items: baseline;
		gap: 12px;
	}
	.logo {
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 28px;
		text-transform: uppercase;
		color: var(--text);
		text-decoration: none;
	}
	.code {
		font-family: var(--font-mono);
		color: var(--text-muted);
	}
	.warn {
		color: var(--hazard);
	}
	p {
		margin: 0;
	}
	.status {
		font-size: 20px;
		color: var(--text);
	}
	.muted {
		color: var(--text-muted);
	}
	.share {
		display: grid;
		gap: 6px;
		max-width: 560px;
	}
	.share label {
		color: var(--text-body);
	}
	.share-row {
		display: flex;
		gap: 8px;
	}
	.share input {
		flex-grow: 1;
		min-width: 0;
		min-height: 44px;
		padding: 0 12px;
		border: 1px solid var(--line);
		border-radius: 8px;
		background: var(--surface-2);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 14px;
	}
	.layout {
		display: grid;
		grid-template-columns: 260px minmax(0, 600px) minmax(260px, 340px);
		gap: 24px;
		align-items: start;
	}
	@media (max-width: 1100px) {
		.layout {
			grid-template-columns: minmax(0, 600px);
		}
		.log-col {
			order: 3;
		}
	}
	.board-col,
	.side-col {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.board-wrap {
		position: relative;
	}
	.result {
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		width: min(380px, 86%);
		box-sizing: border-box;
		padding: 28px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		text-align: center;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 18px;
		box-shadow: 0 24px 60px var(--hole);
	}
	.kicker {
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.result h1 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 52px;
		line-height: 0.95;
		text-transform: uppercase;
		color: var(--text);
	}
	.result p {
		color: var(--text-body);
	}
	.stats {
		margin: 0;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		overflow: hidden;
	}
	.stats div {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		padding: 14px 18px;
		border-bottom: 1px solid var(--surface-2);
	}
	.stats div:last-child {
		border-bottom: 0;
	}
	.stats dt {
		color: var(--text-body);
	}
	.stats dd {
		margin: 0;
		font-family: var(--font-mono);
		font-weight: 600;
		color: var(--text);
	}
	.actions,
	.confirm {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px;
	}
	.confirm span {
		width: 100%;
		color: var(--text);
	}
	button,
	.outline {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 48px;
		padding: 0 22px;
		border-radius: 12px;
		font: inherit;
		font-weight: 600;
		text-decoration: none;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.primary {
		flex-grow: 1;
		border: 0;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 22px;
		text-transform: uppercase;
	}
	.share .primary {
		flex-grow: 0;
	}
	.outline {
		border: 1px solid var(--line);
		background: none;
		color: var(--text);
	}
	.resign {
		width: 100%;
	}
	.danger {
		border: 0;
		background: var(--hazard);
		color: var(--accent-text);
	}
	.error {
		color: var(--hazard);
	}
	a {
		color: var(--accent);
	}
</style>
