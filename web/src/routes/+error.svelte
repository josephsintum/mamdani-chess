<script lang="ts">
	import { page } from '$app/state';
	import Header from '#lib/Header.svelte';
	import MissingCard from '#lib/MissingCard.svelte';

	// An address with no page (a mistyped link, say), or a page that failed.
	let missing = $derived(page.status === 404);
	let title = $derived(missing ? 'Page not found' : 'Something went wrong');
</script>

<svelte:head>
	<title>{title} · Mamdani Chess</title>
</svelte:head>

<Header />

<main>
	<MissingCard {title}>
		{#if missing}
			There's nothing at <span class="path">{page.url.pathname}</span>. Check the link, or start from the home page.
		{:else}
			{page.error?.message ?? 'The page could not be shown.'} Try again, or start from the home page.
		{/if}
	</MissingCard>
</main>

<style>
	main {
		max-width: 1280px;
		margin: 0 auto;
		padding: 0 32px;
	}
	.path {
		overflow-wrap: anywhere;
		font-family: var(--font-mono);
		color: var(--text);
	}
	@media (max-width: 639px) {
		main {
			padding: 0 16px;
		}
		main :global(.missing) {
			margin-top: 32px;
		}
	}
</style>
