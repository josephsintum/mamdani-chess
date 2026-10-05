<script lang="ts">
	import '#lib/theme/tokens.css';
	import Toaster from '#lib/Toaster.svelte';

	let { children } = $props();
</script>

{@render children()}
<Toaster />
