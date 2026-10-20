<script lang="ts">
	import type { Snippet } from 'svelte';

	// A card saying a page or game isn't there, with a way back home: the
	// game page's "Game not found" and the error page.
	let { title, children }: { title: string; children: Snippet } = $props();
</script>

<section class="missing" role="alert">
	<h1>{title}</h1>
	<p>{@render children()}</p>
	<a class="home-link" href="/">Back to the home page</a>
</section>

<style>
	.missing {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 14px;
		max-width: 520px;
		margin-top: 48px;
		padding: 24px;
		border: 1px solid var(--line);
		border-radius: 14px;
		background: var(--surface);
	}
	h1 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 36px;
		text-transform: uppercase;
	}
	p {
		margin: 0;
		line-height: 1.5;
	}
	.home-link {
		display: inline-flex;
		align-items: center;
		min-height: 44px;
		padding: 0 18px;
		border-radius: 10px;
		background: var(--accent);
		color: var(--accent-text);
		font-weight: 600;
		text-decoration: none;
	}
</style>
