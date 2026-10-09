<script lang="ts">
	import '#lib/theme/tokens.css';
	import Toaster from '#lib/Toaster.svelte';
	import { reportError, reportVisit, visitPayload } from '#lib/visit.ts';
	import { afterNavigate } from '$app/navigation';
	import { onMount } from 'svelte';
	import type { LayoutProps } from './$types';

	let { children }: LayoutProps = $props();

	// One visit per page the person lands on; the first carries the referrer.
	let first = true;
	afterNavigate(({ to }) => {
		if (!to) return;
		reportVisit(visitPayload(to.url, first, document.referrer, window.screen, navigator.maxTouchPoints > 0));
		first = false;
	});

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
