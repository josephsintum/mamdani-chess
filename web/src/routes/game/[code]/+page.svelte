<script lang="ts">
	import { onMount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { fly } from 'svelte/transition';
	import { reducedMotion, setInstant } from '#lib/motion.ts';
	import { dev } from '$app/env';
	import { page } from '$app/state';
	import Board from '#lib/Board.svelte';
	import DiceSummary from '#lib/DiceSummary.svelte';
	import DiceTray from '#lib/DiceTray.svelte';
	import MoveLog from '#lib/MoveLog.svelte';
	import MovesSheet from '#lib/MovesSheet.svelte';
	import PlayerBar from '#lib/PlayerBar.svelte';
	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
	import { checkSquare, pillFor, stageAt } from '#lib/board.ts';
	import { applyMove, settlesGuess } from '#lib/pieces.ts';
	import { firstMoveLeft, paused, timeLeft } from '#lib/clock.ts';
	import {
		createGame,
		followsRematch,
		gameExists,
		showsOffline,
		reasons,
		rematch,
		resign,
		trySendMove,
		type Color,
		type MoveJSON,
		type View
	} from '#lib/game.ts';

	const code = page.params.code ?? '';

	// Dev only: /game/CODE?instant turns every animation off, for fast play-testing.
	const instant = dev && page.url.searchParams.has('instant');
	setInstant(instant);
	const anim = new Animator(instant ? 0 : STEP_MS);
	// Phones in portrait get their own layout (canvas row "Phone game: playtest build").
	const phone = new MediaQuery('max-width: 639px');
	let sheet: MovesSheet | undefined = $state();
	let view = $derived(anim.view);
	let shown = $derived(anim.shown);
	// Your move, shown before the server confirms it (One Million Chessboards
	// style): the piece glides at once. If the server refuses the move, the
	// guess is dropped and the piece glides back.
	let optimistic = $state<{ seq: number; move: MoveJSON } | null>(null);
	// The guess couldn't reach the server; it is sent again once it can.
	let unsent = $state(false);
	let connected = $state(false);
	// The server's clock minus ours, and the server's time now: the clocks
	// count down from it.
	let offset = 0;
	let serverNow = $state(Date.now());
	let notFound = $state(false);
	let lost = $state(false);
	let error = $state('');
	let busy = $state(false); // a move or resignation is on its way
	let confirmResign = $state(false);
	let copyHint = $state('');

	function receive(next: View) {
		error = '';
		offset = next.clock.now - Date.now();
		serverNow = next.clock.now;
		if (settlesGuess(optimistic, next)) {
			optimistic = null;
			unsent = false;
		}
		if (next.result) confirmResign = false; // the game ended before you chose
		const prev = anim.view;
		anim.receive(next, { hidden: document.hidden });
		if (unsent) resend();
		// A rematch accepted while this page is open: players go to it. Replace,
		// so Back returns to where they were before, not to a page that would
		// forward them again. A full load gives the new game a fresh stream.
		if (followsRematch(prev, next)) location.replace(`/game/${next.rematch.code}`);
	}

	onMount(() => {
		let source: EventSource | null = null;
		let retry: ReturnType<typeof setTimeout> | undefined;
		let disposed = false; // the page has gone: open nothing more
		const connect = () => {
			if (disposed) return;
			source = new EventSource(`/api/games/${code}/stream`);
			source.onopen = () => (connected = true);
			source.onerror = () => {
				connected = false;
				// Network errors reconnect by themselves. An error status (a
				// proxy's 502 while the server restarts, or a 404) closes the
				// stream for good: ask whether the game still exists, and if it
				// might, open a new stream.
				if (source?.readyState !== EventSource.CLOSED) return;
				retry = setTimeout(async () => {
					if (await gameExists(code)) connect();
					else if (view) lost = true;
					else notFound = true;
				}, 2000);
			};
			source.addEventListener('state', (e) => receive(JSON.parse((e as MessageEvent<string>).data)));
		};
		connect();
		// The browser knows at once when the device loses its network; the
		// stream can take much longer to notice. Show it, and when the network
		// is back, start a fresh stream (the old one may be dead without
		// knowing it): its first state resends a move made meanwhile.
		const goneOffline = () => (connected = false);
		const backOnline = () => {
			source?.close();
			connect();
		};
		window.addEventListener('offline', goneOffline);
		window.addEventListener('online', backOnline);
		const tick = setInterval(() => (serverNow = Date.now() + offset), 100);
		// Coming back to a tab mid-animation jumps to the end of the roll.
		const finishOnReturn = () => {
			if (!document.hidden) anim.finish();
		};
		document.addEventListener('visibilitychange', finishOnReturn);
		return () => {
			disposed = true;
			source?.close();
			window.removeEventListener('offline', goneOffline);
			window.removeEventListener('online', backOnline);
			clearTimeout(retry);
			clearTimeout(resendTimer);
			clearInterval(tick);
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
	let checked = $derived(view && stage ? checkSquare(view, stage, { animating, guessing: optimistic?.seq === view.seq }) : '');
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
	let topPill = $derived(view ? pillFor(view, top, animating) : { text: '', tone: 'turn' as const });
	let bottomPill = $derived(view ? pillFor(view, bottom, animating) : { text: '', tone: 'turn' as const });

	async function move(m: MoveJSON) {
		if (!view || busy) return;
		busy = true;
		optimistic = { seq: view.seq, move: m };
		const sent = await trySendMove(code, m, view.seq);
		busy = false;
		if (sent === 'unsent') {
			// No connection (a train in a tunnel, say): keep the move on the
			// board and send it again when the server can be reached.
			unsent = true;
			retryLater();
		} else if (sent !== 'sent') {
			optimistic = null; // refused: glide back
			if (sent.state) receive(sent.state); // resync at once (this clears error)
			error = sent.refused;
		}
	}

	let resendTimer: ReturnType<typeof setTimeout> | undefined;
	function retryLater() {
		clearTimeout(resendTimer);
		resendTimer = setTimeout(resend, 1500);
	}

	// Sends an unsent move again. The same seq means the server refuses a
	// copy of a move it already has; then the stream's state settles it.
	async function resend() {
		const guess = optimistic;
		if (!unsent || !guess || view?.seq !== guess.seq) return;
		if (!connected) return retryLater();
		unsent = false;
		const sent = await trySendMove(code, guess.move, guess.seq);
		if (sent === 'unsent') {
			unsent = true;
			retryLater();
		} else if (sent !== 'sent') {
			// Usually the server already has the move; its state settles it.
			if (optimistic === guess) optimistic = null;
			if (sent.state) receive(sent.state);
		}
	}

	async function offerRematch(decline = false) {
		busy = true;
		error = (await rematch(code, decline).catch(() => 'Could not reach the server. Try again.')) ?? '';
		busy = false;
	}

	async function doResign() {
		busy = true;
		error = (await resign(code)) ?? '';
		busy = false;
		confirmResign = false;
	}

	let hintTimer: ReturnType<typeof setTimeout> | undefined;

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(page.url.href);
			copyHint = 'Link copied';
		} catch {
			// No clipboard on plain-http addresses: select the link instead.
			const link = document.getElementById('link') as HTMLInputElement | null;
			link?.select();
			if (!phone.current) copyHint = 'Press Ctrl+C (⌘C on a Mac) to copy';
			else copyHint = link ? 'Tap and hold the link to copy it' : 'Copy the address bar to share';
		}
		// The phone's header shows the hint in place of the code, briefly.
		clearTimeout(hintTimer);
		hintTimer = setTimeout(() => (copyHint = ''), 2500);
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

	let clockFor = (c: Color) => (view ? timeLeft(view.clock, c, serverNow) : 0);
	let pausedForDice = $derived(!!view && paused(view.clock, serverNow));
	let firstMove = $derived(view ? firstMoveLeft(view.clock, serverNow) : null);

	let status = $derived.by(() => {
		if (!view) return '';
		if (view.status === 'waiting')
			return you === 'white' ? 'Waiting for your friend to open the link…' : 'Waiting for White’s friend to join…';
		if (view.result) return animating ? 'Last move played…' : 'Game over';
		if (unsent) return connected ? 'Sending your move…' : 'Reconnecting — your move will be sent';
		if (animating) return 'Dice are rolling…';
		const side = view.turn === 'white' ? 'White' : 'Black';
		const yours = view.turn === you;
		let text = yours ? 'Your move' : `${side} to move`;
		// Name who is in check: a bare "check!" was read as the reader's own king.
		if (view.check) text += yours ? ' — you’re in check' : ` — ${side} is in check`;
		if (firstMove !== null) text += ` · ${Math.ceil(firstMove / 1000)}s to make the first move`;
		return text;
	});

	// Phones have no visible status line, so the first-move countdown rides
	// on the pill of the side that has to move.
	function phonePill(p: { text: string }, c: Color): string {
		if (!p.text || firstMove === null || view?.turn !== c) return p.text;
		return `${p.text} · ${Math.ceil(firstMove / 1000)}s`;
	}

	// The phone's game-over bar: what its rematch button does, if anything.
	let rematchAction = $derived.by((): { kind: 'go' | 'offer' | 'offered' | 'answer'; label: string } | null => {
		const r = view?.rematch;
		if (!view || !r || !resultCard || view.result?.reason === 'expired') return null;
		if (r.code) return { kind: 'go', label: isPlayer ? 'Go to rematch' : 'Watch rematch' };
		if (!isPlayer) return null;
		if (r.offer && r.offer !== you) return { kind: 'answer', label: '' };
		if (r.offer === you) return { kind: 'offered', label: 'Offered…' };
		return { kind: 'offer', label: 'Rematch' };
	});
	// On phones a rematch offer or decline replaces the result's detail line,
	// so the card keeps its height and the page never scrolls.
	let rematchNote = $derived.by(() => {
		const r = view?.rematch;
		if (!r || r.code || !isPlayer) return '';
		if (r.offer && r.offer !== you) return 'Your opponent wants a rematch.';
		if (r.declined) return 'Rematch declined.';
		return '';
	});

	let resultCard = $derived.by(() => {
		const r = view?.result;
		if (!view || !r || animating) return null;
		const why = reasons[r.reason] ?? r.reason;
		const winner = r.winner === 'white' ? 'White' : 'Black';
		const loser = r.winner === 'white' ? 'Black' : 'White';
		const lastSan = view.log.at(-1)?.san ?? '';
		const toMove = view.turn === 'white' ? 'White' : 'Black';
		let detail = `By ${why}.`;
		let title = r.draw ? 'Draw' : `${winner} wins`;
		if (r.reason === 'resignation') detail = `${loser} resigned.`;
		if (r.reason === 'checkmate') detail = `${winner} mated with ${lastSan}.`;
		if (r.reason === 'timeout') detail = `${loser} ran out of time.`;
		if (r.reason === 'timeout_vs_insufficient')
			detail = `${toMove} ran out of time, but ${toMove === 'White' ? 'Black' : 'White'} couldn’t have mated.`;
		if (r.reason === 'aborted') {
			title = 'Aborted';
			detail = `${toMove} didn’t make a first move in time. Nobody wins.`;
		}
		if (r.reason === 'expired') {
			title = 'Expired';
			detail = 'Nobody joined within a day.';
		}
		return {
			kicker: `${why} · Move ${Math.max(1, Math.ceil(view.seq / 2))}`,
			title,
			detail
		};
	});
</script>

<svelte:head>
	<title>Game {code} · Pothole Chess</title>
</svelte:head>

{#if phone.current && view && stage && !notFound}
	<div class="phone">
		<header class="ph-head">
			<a href="/" class="ph-logo">Pothole Chess</a>
			<span class="ph-meta">
				{#if !connected && !lost}<span class="ph-chip warn">Reconnecting…</span>{/if}
				{#if you === 'spectator'}<span class="ph-chip">Watching</span>{/if}
				<span class="ph-code">{copyHint && view.status !== 'waiting' ? copyHint : code}</span>
				<button type="button" class="ph-icon" aria-label="Copy game link" onclick={copyLink}>
					<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="12" height="12" rx="2"></rect><path d="M5 15V5a2 2 0 0 1 2-2h10"></path></svg>
				</button>
			</span>
		</header>
		{#if lost}
			<p class="ph-lost" role="alert">
				This game is no longer on the server. <a href="/">Start a new one</a>.
			</p>
		{/if}
		<!-- The pills and the dice card show this; screen readers hear it. -->
		<p class="sr-only" aria-live="polite">{status}</p>

		<div class="ph-play">
			<PlayerBar
				compact
				color={top}
				you={you === top}
				lost={stage.lost[top]}
				pill={phonePill(topPill, top)}
				pillTone={topPill.tone}
				toMove={playing && view.turn === top && !animating}
				clockMs={clockFor(top)}
				ticking={view.clock.running === top && !pausedForDice}
				offline={showsOffline(view, top)}
			/>
			<div class="ph-board">
				<Board
					{stage}
					legal={view.legal}
					{lastMove}
					flipped={bottom === 'black'}
					interactive={!animating && !busy && !optimistic && playing}
					dim={!!resultCard || view.status === 'waiting'}
					check={checked}
					saved={savedSquare}
					onmove={move}
				/>
			</div>
			<PlayerBar
				compact
				color={bottom}
				you={you === bottom}
				lost={stage.lost[bottom]}
				pill={phonePill(bottomPill, bottom)}
				pillTone={bottomPill.tone}
				toMove={playing && view.turn === bottom && !animating}
				clockMs={clockFor(bottom)}
				ticking={view.clock.running === bottom && !pausedForDice}
				offline={showsOffline(view, bottom)}
			/>
		</div>

		<!-- The card and the bottom bar keep one height whatever shows, so the
		     board never moves when a card changes. -->
		<div class="ph-bottom">
		{#if view.status === 'waiting' && you === 'white'}
			<section class="ph-card" aria-label="Invite a friend">
				<label for="link" class="ph-title">Send this link to your friend</label>
				<div class="ph-row">
					<input id="link" readonly value={page.url.href} />
					<button type="button" class="primary" onclick={copyLink}>Copy link</button>
				</div>
				{#if copyHint}<span class="muted">{copyHint}</span>{/if}
			</section>
		{:else if confirmResign}
			<section class="ph-card danger-line" aria-label="Resign">
				<span class="ph-title">Resign this game? <span class="muted">{you === 'white' ? 'Black' : 'White'} wins.</span></span>
				<div class="ph-two">
					<button type="button" class="outline" onclick={() => (confirmResign = false)}>Keep playing</button>
					<button type="button" class="danger" onclick={doResign} disabled={busy}>Yes, resign</button>
				</div>
			</section>
		{:else if resultCard}
			<section class="ph-card accent-line" role="status" aria-label="Game over" in:fly={{ y: 24, duration: reducedMotion() ? 0 : 400 }}>
				<span class="ph-result"><span class="ph-headline">{resultCard.title}</span><span class="ph-kicker">{resultCard.kicker}</span></span>
				<span class="ph-detail">{rematchNote || resultCard.detail}</span>
			</section>
		{:else if error}
			<p class="ph-card ph-error" role="alert">{error}</p>
		{:else if unsent}
			<p class="ph-card ph-detail" role="status">{status}</p>
		{:else}
			<DiceSummary {view} {shown} />
		{/if}

		{#if view.status !== 'waiting' && !confirmResign}
			<nav class="ph-nav" aria-label="Game actions" class:single={!resultCard && !(isPlayer && playing)} class:three={!!rematchAction}>
				{#if rematchAction}
					{#if rematchAction.kind === 'answer'}
						<button type="button" class="primary" onclick={() => offerRematch()} disabled={busy}>Accept</button>
						<button type="button" class="outline" onclick={() => offerRematch(true)} disabled={busy}>Decline</button>
					{:else}
						{#if rematchAction.kind === 'go'}
							<button type="button" class="primary" onclick={() => location.assign(`/game/${view.rematch.code}`)}>{rematchAction.label}</button>
						{:else}
							<button type="button" class="primary" onclick={() => offerRematch()} disabled={busy || rematchAction.kind === 'offered'}
								>{rematchAction.label}</button
							>
						{/if}
						<button type="button" class="outline" onclick={newGame} disabled={busy}>New game</button>
					{/if}
				{:else if resultCard}<button type="button" class="primary" onclick={newGame} disabled={busy}>New game</button>{/if}
				<button type="button" class="solid" onclick={(e) => sheet?.open(e.currentTarget)}>
					<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M8 6h13"></path><path d="M8 12h13"></path><path d="M8 18h13"></path><path d="M3 6h.01"></path><path d="M3 12h.01"></path><path d="M3 18h.01"></path></svg>
					{rematchAction ? 'Moves' : 'Moves and rolls'}
				</button>
				{#if !resultCard && isPlayer && playing}
					<button type="button" class="outline" onclick={() => (confirmResign = true)} disabled={busy}>
						<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 22V4"></path><path d="M4 4h12l-2 4 2 4H4"></path></svg>
						Resign
					</button>
				{/if}
			</nav>
		{/if}
		</div>
	</div>
	<MovesSheet bind:this={sheet} {view} {shown} rolling={animating} />
{:else}
<main>
	<header>
		<a href="/" class="logo">Pothole Chess</a>
		<span class="code">{code}</span>
		{#if view && !connected && !lost}<span class="warn">Reconnecting…</span>{/if}
	</header>

	{#if lost}
		<p class="error" role="alert">
			This game is no longer on the server. <a href="/">Start a new one</a>.
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
				<PlayerBar
					color={top}
					you={you === top}
					lost={stage.lost[top]}
					pill={topPill.text}
					pillTone={topPill.tone}
					toMove={playing && view.turn === top && !animating}
					clockMs={clockFor(top)}
					ticking={view.clock.running === top && !pausedForDice}
					pausedForDice={playing && view.turn === top && pausedForDice}
					offline={showsOffline(view, top)}
				/>
				<div class="board-wrap">
					<Board
						{stage}
						legal={view.legal}
						{lastMove}
						flipped={bottom === 'black'}
						interactive={!animating && !busy && !optimistic && playing}
						dim={!!resultCard}
						check={checked}
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
					pill={bottomPill.text}
					pillTone={bottomPill.tone}
					toMove={playing && view.turn === bottom && !animating}
					clockMs={clockFor(bottom)}
					ticking={view.clock.running === bottom && !pausedForDice}
					pausedForDice={playing && view.turn === bottom && pausedForDice}
					offline={showsOffline(view, bottom)}
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
					{#if isPlayer && view.result?.reason !== 'expired'}
						{@const offer = view.rematch.offer}
						<div class="actions" aria-live="polite">
							{#if view.rematch.code}
								<a class="primary" href={`/game/${view.rematch.code}`} data-sveltekit-reload>Go to the rematch</a>
							{:else if offer && offer !== you}
								<span class="note">Your opponent wants a rematch.</span>
								<button class="primary" onclick={() => offerRematch()} disabled={busy}>Accept</button>
								<button class="outline" onclick={() => offerRematch(true)} disabled={busy}>Decline</button>
							{:else if offer === you}
								<button class="primary" disabled>Rematch offered…</button>
							{:else}
								{#if view.rematch.declined}<span class="note">Rematch declined.</span>{/if}
								<button class="primary" onclick={() => offerRematch()} disabled={busy}>Rematch</button>
							{/if}
						</div>
					{:else if view.rematch.code}
						<a class="primary" href={`/game/${view.rematch.code}`} data-sveltekit-reload>Watch rematch</a>
					{/if}
					<div class="actions">
						<button class={isPlayer ? 'outline' : 'primary'} onclick={newGame} disabled={busy}>New game</button>
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
{/if}

<style>
	/* Phone layout (under 640px): one screen, no scrolling. */
	.phone {
		display: flex;
		flex-direction: column;
		height: 100vh;
		height: 100dvh;
		overflow: hidden;
	}
	.ph-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-shrink: 0;
		height: 48px;
		padding: 0 4px 0 12px;
	}
	.ph-logo {
		font-family: var(--font-display);
		font-size: 20px;
		font-weight: 800;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--text);
		text-decoration: none;
	}
	.ph-meta {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}
	.ph-code {
		overflow: hidden;
		font-family: var(--font-mono);
		font-size: 14px;
		font-weight: 600;
		white-space: nowrap;
		text-overflow: ellipsis;
		color: var(--text-muted);
	}
	.ph-chip {
		padding: 2px 8px;
		border: 1px solid var(--line);
		border-radius: 999px;
		font-size: 12px;
		color: var(--text-muted);
	}
	.ph-chip.warn {
		border-color: var(--hazard);
		color: var(--hazard-text);
	}
	.ph-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		padding: 0;
		border: 0;
		background: none;
		color: var(--text-body);
		cursor: pointer;
	}
	.ph-lost {
		margin: 0 8px 4px;
		font-size: 14px;
		color: var(--hazard-text);
	}
	/* Bars and board take the space left over; the board is as big as fits. */
	.ph-play {
		display: flex;
		flex-direction: column;
		/* Spare height on tall phones gathers at the top, under the header,
		   so the board sits just above the card and buttons, in thumb reach. */
		justify-content: flex-end;
		flex: 1 1 auto;
		min-height: 0;
		container-type: size;
	}
	.ph-board {
		width: min(calc(100cqw - 16px), calc(100cqh - 92px));
		margin: 2px auto;
	}
	.ph-card {
		display: flex;
		flex-direction: column;
		gap: 10px;
		flex-shrink: 0;
		margin: 6px 8px 0;
		padding: 12px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 12px;
	}
	.ph-card.accent-line {
		gap: 4px;
		border-color: var(--accent-line);
	}
	.ph-card.danger-line {
		border-color: var(--hazard);
	}
	.ph-title {
		font-weight: 600;
		color: var(--text);
	}
	.ph-row {
		display: flex;
		gap: 8px;
	}
	.ph-row input {
		flex: 1;
		min-width: 0;
		height: 44px;
		box-sizing: border-box;
		padding: 0 10px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--bg);
		color: var(--text-body);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.ph-row .primary {
		flex-grow: 0;
	}
	.ph-two {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
	}
	.ph-result {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 8px;
	}
	.ph-headline {
		font-family: var(--font-display);
		font-size: 28px;
		font-weight: 800;
		line-height: 1;
		text-transform: uppercase;
		color: var(--accent);
	}
	.ph-kicker {
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--text-muted);
	}
	.ph-detail {
		font-size: 14px;
	}
	.ph-error {
		margin-bottom: 0;
		font-size: 14px;
		color: var(--hazard-text);
	}
	.ph-bottom {
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
		gap: 8px;
		/* The height of the playing state (dice card and bottom bar), which
		   every other state fits in. */
		min-height: calc(143px + max(12px, env(safe-area-inset-bottom)));
	}
	.ph-nav {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
		flex-shrink: 0;
		margin-top: auto;
		padding: 8px 8px max(12px, env(safe-area-inset-bottom));
		border-top: 1px solid var(--surface-2);
	}
	.ph-nav.single {
		grid-template-columns: 1fr;
	}
	.ph-nav.three {
		grid-template-columns: repeat(3, minmax(0, 1fr));
	}
	.phone button {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 44px;
		padding: 0 14px;
		border-radius: 10px;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.phone .solid {
		border: 0;
		background: var(--surface-2);
		color: var(--text);
	}
	.phone .outline {
		border: 1px solid var(--line);
		background: none;
		color: var(--text);
	}
	.phone .primary {
		border: 0;
		background: var(--accent);
		color: var(--accent-text);
	}
	.phone .danger {
		border: 0;
		background: var(--hazard);
		color: var(--accent-text);
	}
	.phone .ph-icon {
		padding: 0;
		border: 0;
		background: none;
		color: var(--text-body);
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
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
		/* The board shrinks with the window's height, so both player bars
		   (and your clock) fit without scrolling: about 240 px go to the page
		   padding, header, status line, bars and gaps. */
		--board: clamp(320px, calc(100dvh - 240px), 600px);
		display: grid;
		grid-template-columns: 260px minmax(0, var(--board)) minmax(260px, 340px);
		gap: 24px;
		align-items: start;
	}
	@media (max-width: 1100px) {
		.layout {
			grid-template-columns: minmax(0, var(--board));
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
	.confirm span,
	.actions .note {
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
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 48px;
		border-radius: 12px;
		text-decoration: none;
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
