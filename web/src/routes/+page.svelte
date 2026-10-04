<script lang="ts">
	import { goto } from '$app/navigation';
	import { createGame } from '#lib/game.ts';

	let busy = $state(false);
	let error = $state('');

	async function newGame() {
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
	<title>Pothole Chess</title>
</svelte:head>

<main>
	<h1>Pothole Chess</h1>
	<p>
		Chess, but the road is falling apart. After every move a d8 may open a pothole that swallows
		a piece. The Mamdani — a neutral piece either player can move — repairs them.
	</p>
	<button onclick={newGame} disabled={busy}>{busy ? 'Starting…' : 'Play a friend'}</button>
	{#if error}<p class="error" role="alert">{error}</p>{/if}
	<p class="hint">You play White. Send the link to a friend; they play Black.</p>
</main>

<style>
	main {
		min-height: 100dvh;
		display: grid;
		place-content: center;
		justify-items: center;
		gap: 16px;
		padding: 16px;
		max-width: 560px;
		margin: 0 auto;
		text-align: center;
	}
	h1 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 56px;
		text-transform: uppercase;
		color: var(--text);
	}
	p {
		margin: 0;
	}
	button {
		min-height: 44px;
		padding: 0 32px;
		border: 0;
		border-radius: 4px;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 24px;
		text-transform: uppercase;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.hint {
		color: var(--text-muted);
		font-size: 14px;
	}
	.error {
		color: var(--hazard);
	}
</style>
