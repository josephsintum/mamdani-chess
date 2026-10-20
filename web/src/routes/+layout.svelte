<script lang="ts">
	import '#lib/theme/tokens.css';
	import '#lib/theme/content.css';
	import Toaster from '#lib/Toaster.svelte';
	import { reportError } from '#lib/report-error.ts';
	import { onMount } from 'svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	onMount(() => {
		const onError = (e: ErrorEvent) => reportError(location.pathname, String(e.message ?? e.error ?? 'error'));
		const onRejection = (e: PromiseRejectionEvent) => reportError(location.pathname, String(e.reason ?? 'rejection'));
		window.addEventListener('error', onError);
		window.addEventListener('unhandledrejection', onRejection);
		return () => {
			window.removeEventListener('error', onError);
			window.removeEventListener('unhandledrejection', onRejection);
		};
	});
</script>

{@render children()}
<Toaster />
