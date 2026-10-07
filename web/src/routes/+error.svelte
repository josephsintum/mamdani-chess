<script lang="ts">
	import { page } from '$app/state';
	import Header from '#lib/Header.svelte';

	// An address with no page (a mistyped link, say), or a page that failed.
	let missing = $derived(page.status === 404);
</script>

<svelte:head>
	<title>{missing ? 'Page not found' : 'Something went wrong'} · Mamdani Chess</title>
</svelte:head>

<Header />

<main>
	<section class="missing" role="alert">
		<h1>{missing ? 'Page not found' : 'Something went wrong'}</h1>
		{#if missing}
			<p>There's nothing at <span class="path">{page.url.pathname}</span>. Check the link, or start from the home page.</p>
		{:else}
			<p>{page.error?.message ?? 'The page could not be shown.'} Try again, or start from the home page.</p>
		{/if}
		<a class="home-link" href="/">Back to the home page</a>
	</section>
</main>

<style>
	main {
		max-width: 1280px;
		margin: 0 auto;
		padding: 0 32px;
	}
	/* The game page's "Game not found" card. */
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
	.path {
		overflow-wrap: anywhere;
		font-family: var(--font-mono);
		color: var(--text);
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
	@media (max-width: 639px) {
		main {
			padding: 0 16px;
		}
		.missing {
			margin-top: 32px;
		}
	}
</style>
