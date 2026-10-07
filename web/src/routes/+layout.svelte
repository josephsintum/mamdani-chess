<script lang="ts">
	import '#lib/theme/tokens.css';
	import Toaster from '#lib/Toaster.svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();
</script>

{@render children()}
<Toaster />
