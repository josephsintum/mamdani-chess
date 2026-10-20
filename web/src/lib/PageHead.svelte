<script lang="ts">
	// A prerendered page's title and canonical link (path: "/rules"). Here,
	// not from the server, so the link goes when the app navigates on.
	let { title, path }: { title: string; path: string } = $props();
</script>

<svelte:head>
	<title>{title}</title>
	<link rel="canonical" href="https://mamdanichess.com{path}" />
</svelte:head>
