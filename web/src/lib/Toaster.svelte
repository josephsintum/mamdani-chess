<script lang="ts">
	import { Toaster } from 'svelte-sonner';
	import { MediaQuery } from 'svelte/reactivity';

	// The one place notices appear: top centre, up to 3 stacked. svelte-sonner
	// handles the queue, pausing on hover and while the tab is hidden, the ✕
	// button, swipe to dismiss and the Alt+T hotkey; the look is ours
	// (unstyled + these classes). On a phone a toast covers the top player
	// bar, so it goes sooner there.
	const phone = new MediaQuery('max-width: 639px');
</script>

<Toaster
	position="top-center"
	visibleToasts={3}
	duration={phone.current ? 3000 : 4000}
	pauseWhenPageIsHidden
	closeButton
	closeButtonAriaLabel="Dismiss"
	offset="16px"
	mobileOffset="12px"
	toastOptions={{
		unstyled: true,
		classes: {
			toast: 'mc-toast',
			title: 'mc-toast-title',
			description: 'mc-toast-description',
			actionButton: 'mc-toast-action',
			icon: 'mc-toast-icon',
			closeButton: 'mc-toast-close',
			success: 'mc-toast-success',
			error: 'mc-toast-error'
		}
	}}
/>

<style>
	:global(.mc-toast) {
		display: flex;
		align-items: center;
		gap: 10px;
		width: min(380px, calc(100vw - 24px));
		padding: 12px 16px;
		border: 1px solid var(--accent-line);
		border-radius: 12px;
		background: var(--surface);
		box-shadow: 0 14px 36px var(--hole);
		color: var(--text);
		font-family: var(--font-body);
		font-size: 15px;
	}
	/* A dot in the notice's tone; types with their own icon hide it. */
	:global(.mc-toast:not(:has(.mc-toast-icon))::before) {
		content: '';
		flex-shrink: 0;
		width: 10px;
		height: 10px;
		border-radius: 50%;
		background: var(--accent);
	}
	:global(.mc-toast-icon) {
		display: flex;
		flex-shrink: 0;
		color: var(--accent);
	}
	:global(.mc-toast-success) {
		border-color: var(--accent);
	}
	:global(.mc-toast-error) {
		border-color: var(--hazard);
	}
	:global(.mc-toast-error .mc-toast-icon) {
		color: var(--hazard);
	}
	:global(.mc-toast-title) {
		font-weight: 600;
	}
	:global(.mc-toast-description) {
		color: var(--text-muted);
		font-size: 13px;
	}
	/* The ✕ sits at the end of the row. */
	:global(.mc-toast-close) {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		order: 10;
		width: 32px;
		height: 32px;
		margin: -6px -8px -6px auto;
		padding: 0;
		border: 0;
		border-radius: 8px;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}
	:global(.mc-toast-close:hover),
	:global(.mc-toast-close:focus-visible) {
		background: var(--surface-2);
		color: var(--text);
	}
	:global(.mc-toast-close svg) {
		width: 14px;
		height: 14px;
	}
	:global(.mc-toast-action) {
		flex-shrink: 0;
		margin-left: auto;
		min-height: 36px;
		padding: 0 14px;
		border: 0;
		border-radius: 8px;
		background: var(--accent);
		color: var(--accent-text);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
</style>
