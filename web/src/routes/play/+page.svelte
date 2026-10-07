<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Header from '#lib/Header.svelte';
	import { createGame, matchCard, type View } from '#lib/game.ts';
	import { reducedMotion } from '#lib/motion.ts';
	import { formatElapsed, me, type Me } from '#lib/lobby.ts';
	import { retryDelay } from '#lib/reconnect.ts';

	// Quick match: the guest is in the queue while this page's stream is
	// open. Cancel, Back or closing the tab all leave it.
	const OFFER_FRIEND_MS = 60_000;
	// How long "Opponent found" shows before going to the game. White's
	// 60 s for a first move is already running, so it stays short.
	const FOUND_MS = 2000;

	let user = $state<Me | null>(null);
	let started = Date.now();
	let now = $state(Date.now());
	let connected = $state(false);
	let error = $state('');
	let busy = $state(false);
	let elapsed = $derived(now - started);
	let found = $state<{ code: string; card: ReturnType<typeof matchCard> } | null>(null);
	// The game the guest is already playing, if any: one game at a time.
	let already = $state('');

	// Back to a game the guest is seated in: straight in if nobody has
	// moved yet (a reload of the opponent-found screen), otherwise ask.
	async function resume(code: string) {
		try {
			const res = await fetch(`/api/games/${code}`);
			const view = (await res.json()) as View;
			if (res.ok && view.seq === 0) return play(code);
		} catch {
			// fall through to asking
		}
		already = code;
	}
	let foundTimer: ReturnType<typeof setTimeout> | undefined;

	function play(code: string) {
		clearTimeout(foundTimer);
		goto(`/game/${code}`, { replace: true, state: { matched: true } });
	}

	// Who's who, from the game itself (GET shows it without taking a seat);
	// then into the game after FOUND_MS, or at once if that fails.
	async function matched(code: string) {
		try {
			const res = await fetch(`/api/games/${code}`);
			if (!res.ok) throw new Error(res.statusText);
			found = { code, card: matchCard((await res.json()) as View) };
		} catch {
			return play(code);
		}
		foundTimer = setTimeout(() => play(code), FOUND_MS);
	}

	onMount(() => {
		started = Date.now();
		let es: EventSource | undefined;
		let retry: ReturnType<typeof setTimeout> | undefined;
		let done = false; // matched, or the page has gone: open nothing more
		let failures = 0; // refused streams in a row, for the backoff
		const connect = () => {
			if (done || already) return;
			const stream = new EventSource('/api/match');
			es = stream;
			stream.addEventListener('queued', () => {
				connected = true;
				failures = 0;
				// Joining the queue gave the guest a name if they had none.
				me()
					.then((m) => (user = m))
					.catch(() => {});
			});
			stream.addEventListener('matched', (e) => {
				done = true;
				stream.close(); // before the server ends the stream, so it isn't reopened
				const { code } = JSON.parse((e as MessageEvent<string>).data);
				matched(code);
			});
			// A network error reconnects by itself. An error answer (a proxy's
			// 502 while the server restarts) closes the stream for good: open a
			// new one. The elapsed time keeps counting either way.
			stream.onerror = () => {
				connected = false;
				if (stream.readyState !== EventSource.CLOSED) return;
				// Refused (409: already in a game) or a proxy error: check which.
				retry = setTimeout(async () => {
					const m = await me().catch(() => null);
					if (m?.game) return resume(m.game);
					connect();
				}, retryDelay(failures++));
			};
		};
		me()
			.then((m) => {
				user = m;
				if (m.game) resume(m.game);
				else connect();
			})
			.catch(() => connect());
		const tick = setInterval(() => (now = Date.now()), 250);
		return () => {
			done = true;
			clearTimeout(retry);
			es?.close();
			clearInterval(tick);
			clearTimeout(foundTimer);
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
</script>

<svelte:head>
	<title>Finding an opponent · Mamdani Chess</title>
</svelte:head>

<Header bind:me={user} />

<main>
	{#if already}
		<div class="ring done" aria-hidden="true">
			<span class="outer"></span>
			<span class="middle"></span>
			<span class="inner"></span>
		</div>
		<div class="text" role="status">
			<h1>You're in a game</h1>
			<p>One game at a time: finish that one, then find another.</p>
		</div>
		<a class="cancel rejoin" href="/game/{already}">Rejoin game</a>
		<a class="cancel" href="/">Home</a>
	{:else if found}
		<div class="ring done" aria-hidden="true">
			<span class="outer"></span>
			<span class="middle"></span>
			<span class="inner"></span>
		</div>
		<div class="text" role="status">
			<h1>Opponent found</h1>
			<div class="found">
				<p class="side"><span class="swatch {found.card.you.color}" aria-hidden="true"></span><span class="who">{found.card.you.name}</span> <span class="you">(you)</span><span class="color">{found.card.you.color === 'white' ? 'White' : 'Black'}</span></p>
				<p class="vs">vs</p>
				<p class="side"><span class="swatch {found.card.them.color}" aria-hidden="true"></span><span class="who">{found.card.them.name}</span><span class="color">{found.card.them.color === 'white' ? 'White' : 'Black'}</span></p>
			</div>
		</div>
		<p class="joining">Joining game…</p>
		<div class="bar" aria-hidden="true"><span style:animation-duration="{FOUND_MS}ms" class:still={reducedMotion()}></span></div>
		<button class="cancel go" onclick={() => play(found!.code)}>Go now</button>
	{:else}
		<div class="ring" aria-hidden="true">
			<span class="outer"></span>
			<span class="middle"></span>
			<span class="inner"></span>
			<span class="time">{formatElapsed(elapsed)}</span>
		</div>
		<div class="text">
			<h1>Looking for an opponent</h1>
			<p>You'll be paired with the next player who taps Play online. Colors are picked at random.</p>
			<p class="sr-only" aria-live="polite">{connected ? 'In the queue.' : 'Connecting…'}</p>
		</div>
		<ul class="chips">
			<li>10+5</li>
			<li>Standard rules</li>
			<li>Random colors</li>
		</ul>
		<a class="cancel" href="/">Cancel</a>
		<p class="hint">{connected ? 'Keep this tab open. Closing it takes you out of the queue.' : 'Connecting…'}</p>
		{#if elapsed >= OFFER_FRIEND_MS}
			<p class="friend">
				Nobody yet.
				<button onclick={playFriend} disabled={busy}>{busy ? 'Starting…' : 'Invite a friend instead'}</button>
			</p>
		{/if}
		{#if error}<p class="error" role="alert">{error}</p>{/if}
	{/if}
</main>

<style>
	main {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 32px;
		padding: 64px 24px;
		text-align: center;
	}
	.ring {
		position: relative;
		display: grid;
		place-items: center;
		width: 240px;
		height: 240px;
	}
	.ring span {
		position: absolute;
		border-radius: 50%;
	}
	.outer {
		inset: 0;
		border: 2px dashed var(--line);
	}
	.middle {
		inset: 36px;
		border: 2px dashed var(--accent-line);
	}
	.inner {
		inset: 56px;
		border: 2px solid var(--accent);
		background: var(--surface);
	}
	.time {
		position: relative;
		color: var(--accent);
		font-family: var(--font-mono);
		font-size: 40px;
	}
	.text {
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 12px;
	}
	h1 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: clamp(40px, 7vw, 64px);
		line-height: 1;
		text-transform: uppercase;
	}
	.text p {
		margin: 0;
		max-width: 460px;
		font-size: 18px;
		line-height: 1.5;
	}
	.chips {
		display: flex;
		flex-wrap: wrap;
		justify-content: center;
		gap: 10px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.chips li {
		padding: 8px 14px;
		border: 1px solid var(--line);
		border-radius: 999px;
		font-family: var(--font-mono);
		font-size: 14px;
	}
	.cancel {
		display: flex;
		align-items: center;
		justify-content: center;
		height: 52px;
		min-width: 180px;
		padding: 0 28px;
		border: 1px solid var(--line);
		border-radius: 12px;
		color: var(--text);
		font-weight: 600;
		text-decoration: none;
	}
	.hint {
		margin: -16px 0 0;
		color: var(--text-muted);
		font-size: 14px;
	}
	.friend {
		margin: 0;
		color: var(--text-muted);
	}
	.friend button {
		padding: 0;
		border: 0;
		background: none;
		color: var(--accent);
		font: inherit;
		font-weight: 600;
		text-decoration: underline;
		cursor: pointer;
	}
	.error {
		margin: 0;
		color: var(--hazard-text);
	}
	.ring.done .outer,
	.ring.done .middle {
		border-style: solid;
	}
	.ring.done .inner {
		background: var(--accent);
	}
	.found {
		display: flex;
		flex-direction: column;
		gap: 8px;
		width: min(420px, 100%);
		margin-top: 8px;
	}
	.side {
		display: flex;
		align-items: center;
		gap: 10px;
		margin: 0;
		padding: 12px 16px;
		border: 1px solid var(--line);
		border-radius: 12px;
		background: var(--surface);
		color: var(--text);
		font-weight: 600;
		text-align: left;
	}
	.who {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.you {
		flex-shrink: 0;
		color: var(--text-muted);
		font-weight: 400;
	}
	.color {
		flex-shrink: 0;
		margin-left: auto;
		color: var(--text-muted);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.swatch {
		flex-shrink: 0;
		width: 14px;
		height: 14px;
		border-radius: 3px;
		box-shadow: 0 0 0 1px var(--line);
	}
	.swatch.white {
		background: var(--piece-light);
	}
	.swatch.black {
		background: var(--piece-dark);
	}
	.vs {
		margin: 0;
		color: var(--text-muted);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.joining {
		margin: 0;
		color: var(--text-muted);
	}
	.bar {
		width: min(240px, 100%);
		height: 4px;
		margin-top: -20px;
		overflow: hidden;
		border-radius: 2px;
		background: var(--surface-2);
	}
	.bar span {
		display: block;
		height: 100%;
		background: var(--accent);
		transform-origin: left;
		animation: fill linear both;
	}
	.bar span.still {
		animation: none;
	}
	@keyframes fill {
		from {
			transform: scaleX(0);
		}
		to {
			transform: scaleX(1);
		}
	}
	.rejoin {
		border-color: var(--accent);
		background: var(--accent);
		color: var(--accent-text);
	}
	.go {
		background: transparent;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	/* Phones: everything down to Cancel fits on an iPhone SE (320×568). */
	@media (max-width: 639px) {
		main {
			gap: 18px;
			padding: 20px 16px;
		}
		.ring {
			width: 150px;
			height: 150px;
		}
		.middle {
			inset: 22px;
		}
		.inner {
			inset: 36px;
		}
		.time {
			font-size: 26px;
		}
		h1 {
			font-size: 34px;
		}
		.text {
			gap: 8px;
		}
		.text p {
			font-size: 15px;
		}
		.chips {
			gap: 6px;
		}
		.chips li {
			padding: 6px 10px;
			font-size: 12px;
		}
		.cancel {
			height: 48px;
		}
		.hint {
			margin-top: -10px;
		}
		.bar {
			margin-top: -12px;
		}
	}
</style>
