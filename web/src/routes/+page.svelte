<script lang="ts">
	// Walking-skeleton demo: one shared counter over SSE.
	// Throwaway: replaced by the home page in a later plan.
	import { onMount } from 'svelte';

	let count = $state<number | null>(null);
	let connected = $state(false);

	onMount(() => {
		const source = new EventSource('/api/honk/stream');
		source.onopen = () => (connected = true);
		source.onerror = () => (connected = false);
		source.addEventListener('honk', (e) => {
			count = JSON.parse((e as MessageEvent<string>).data).count;
		});
		return () => source.close();
	});

	async function honk() {
		await fetch('/api/honk', { method: 'POST' });
	}
</script>

<svelte:head>
	<title>Pothole Chess</title>
</svelte:head>

<main>
	<h1>Pothole Chess</h1>
	<p class="count" aria-live="polite">{count ?? '–'}</p>
	<button onclick={honk} disabled={!connected}>Honk</button>
	<p class="status">{connected ? 'Live' : 'Reconnecting…'}</p>
</main>

<style>
	main {
		min-height: 100dvh;
		display: grid;
		place-content: center;
		justify-items: center;
		gap: 16px;
		padding: 16px;
	}
	h1 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 48px;
		text-transform: uppercase;
		color: var(--text);
	}
	.count {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 72px;
		color: var(--accent);
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
		opacity: 0.5;
		cursor: default;
	}
	.status {
		margin: 0;
		color: var(--text-muted);
		font-size: 14px;
	}
</style>
