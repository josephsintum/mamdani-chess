<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import Board from '#lib/Board.svelte';
	import { eventText, resultText, sendMove, type MoveJSON, type View } from '#lib/game.ts';

	const code = page.params.code ?? '';

	let view = $state<View | null>(null);
	let connected = $state(false);
	let notFound = $state(false);
	let error = $state('');
	let copied = $state(false);

	onMount(() => {
		const source = new EventSource(`/api/games/${code}/stream`);
		source.onopen = () => (connected = true);
		source.onerror = () => {
			connected = false;
			// A 404 closes the stream for good; other errors reconnect by themselves.
			if (source.readyState === EventSource.CLOSED && !view) notFound = true;
		};
		source.addEventListener('state', (e) => {
			view = JSON.parse((e as MessageEvent<string>).data);
			error = '';
		});
		return () => source.close();
	});

	async function move(m: MoveJSON) {
		if (!view) return;
		error = (await sendMove(code, m, view.seq)) ?? '';
	}

	async function copyLink() {
		await navigator.clipboard.writeText(location.href);
		copied = true;
	}

	let status = $derived.by(() => {
		if (!view) return '';
		if (view.result) return resultText(view.result);
		if (view.status === 'waiting') return 'Waiting for your friend to open the link…';
		const side = view.turn === 'white' ? 'White' : 'Black';
		const check = view.check ? ' — check!' : '';
		if (view.you === view.turn) return `Your move${check}`;
		return `${side} to move${check}`;
	});
</script>

<svelte:head>
	<title>Game {code} · Pothole Chess</title>
</svelte:head>

<main>
	<header>
		<a href="/" class="logo">Pothole Chess</a>
		<span class="code">{code}</span>
		{#if view && !connected}<span class="warn">Reconnecting…</span>{/if}
	</header>

	{#if notFound}
		<p>Game not found. <a href="/">Start a new one</a>.</p>
	{:else if !view}
		<p>Connecting…</p>
	{:else}
		<p class="status" aria-live="polite">
			{status}
			{#if view.you === 'spectator'}<span class="muted">(watching)</span>{/if}
		</p>

		{#if view.status === 'waiting' && view.you === 'white'}
			<button class="share" onclick={copyLink}>{copied ? 'Link copied' : 'Copy link for your friend'}</button>
		{/if}

		<div class="layout">
			<Board {view} onmove={move} />
			<aside>
				{#if error}<p class="error" role="alert">{error}</p>{/if}
				<h2>Last turn</h2>
				{#if view.last.length === 0}
					<p class="muted">No moves yet.</p>
				{:else}
					<ul>
						{#each view.last as e, i (i)}
							<li class:hazard={['pothole_opened', 'fell'].includes(e.kind)}>{eventText(e)}</li>
						{/each}
					</ul>
				{/if}
				<h2>Moves</h2>
				<ol class="log">
					{#each view.log as line, i (i)}
						<li>{line}</li>
					{/each}
				</ol>
			</aside>
		</div>
	{/if}
</main>

<style>
	main {
		max-width: 960px;
		margin: 0 auto;
		padding: 16px;
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
		justify-self: start;
		min-height: 44px;
		padding: 0 20px;
		border: 0;
		border-radius: 4px;
		background: var(--accent);
		color: var(--accent-text);
		font-weight: 600;
		cursor: pointer;
	}
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 560px) 1fr;
		gap: 16px;
		align-items: start;
	}
	@media (max-width: 760px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
	aside {
		display: grid;
		gap: 8px;
	}
	h2 {
		margin: 8px 0 0;
		font-family: var(--font-display);
		text-transform: uppercase;
		font-size: 18px;
		color: var(--text);
	}
	ul,
	ol {
		margin: 0;
		padding-left: 20px;
	}
	.hazard {
		color: var(--hazard);
	}
	.log {
		padding-left: 36px; /* room for two-digit move numbers */
		font-family: var(--font-mono);
		font-size: 14px;
		max-height: 320px;
		overflow-y: auto;
	}
	.error {
		color: var(--hazard);
	}
	a {
		color: var(--accent);
	}
</style>
