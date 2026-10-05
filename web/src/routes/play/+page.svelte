<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Header from '#lib/Header.svelte';
	import { createGame } from '#lib/game.ts';
	import { formatElapsed, myName } from '#lib/lobby.ts';

	// Quick match: the guest is in the queue while this page's stream is
	// open. Cancel, Back or closing the tab all leave it.
	const OFFER_FRIEND_MS = 60_000;

	let name = $state<string | null>(null);
	let started = Date.now();
	let now = $state(Date.now());
	let connected = $state(false);
	let error = $state('');
	let busy = $state(false);
	let elapsed = $derived(now - started);

	onMount(() => {
		started = Date.now();
		let es: EventSource | undefined;
		let retry: ReturnType<typeof setTimeout> | undefined;
		let done = false; // matched, or the page has gone: open nothing more
		const connect = () => {
			if (done) return;
			const stream = new EventSource('/api/match');
			es = stream;
			stream.addEventListener('queued', () => {
				connected = true;
				// Joining the queue gave the guest a name if they had none.
				myName()
					.then((n) => (name = n))
					.catch(() => {});
			});
			stream.addEventListener('matched', (e) => {
				done = true;
				stream.close(); // before the server ends the stream, so it isn't reopened
				const { code } = JSON.parse((e as MessageEvent<string>).data);
				goto(`/game/${code}`, { replace: true });
			});
			// A network error reconnects by itself. An error answer (a proxy's
			// 502 while the server restarts) closes the stream for good: open a
			// new one. The elapsed time keeps counting either way.
			stream.onerror = () => {
				connected = false;
				if (stream.readyState === EventSource.CLOSED) retry = setTimeout(connect, 2000);
			};
		};
		connect();
		const tick = setInterval(() => (now = Date.now()), 250);
		return () => {
			done = true;
			clearTimeout(retry);
			es?.close();
			clearInterval(tick);
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
	<title>Finding an opponent · Pothole Chess</title>
</svelte:head>

<Header bind:name />

<main>
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
			<button onclick={playFriend} disabled={busy}>{busy ? 'Starting…' : 'Play a friend instead'}</button>
		</p>
	{/if}
	{#if error}<p class="error" role="alert">{error}</p>{/if}
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
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	@media (max-width: 639px) {
		main {
			gap: 24px;
			padding: 32px 16px;
		}
		.ring {
			width: 200px;
			height: 200px;
		}
		.middle {
			inset: 30px;
		}
		.inner {
			inset: 46px;
		}
		.time {
			font-size: 34px;
		}
		.text p {
			font-size: 16px;
		}
	}
</style>
