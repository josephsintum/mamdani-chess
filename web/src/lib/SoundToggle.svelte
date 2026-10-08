<script lang="ts">
	import { setSoundOn, soundOn } from './sound.ts';

	// The speaker button: all sound on or off, remembered in this browser.
	let { compact = false }: { compact?: boolean } = $props();

	let on = $state(soundOn());

	function toggle() {
		on = !on;
		setSoundOn(on);
	}
</script>

<button type="button" class={['sound', { compact }]} onclick={toggle} aria-pressed={on} aria-label="Sound" title={on ? 'Sound on' : 'Sound off'}>
	<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
		<path d="M4 9h4l5-4v14l-5-4H4z" fill="currentColor" />
		{#if on}
			<path d="M16 9a4 4 0 0 1 0 6M18.5 6.5a7.5 7.5 0 0 1 0 11" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
		{:else}
			<path d="M16 9l5 6M21 9l-5 6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" />
		{/if}
	</svg>
</button>

<style>
	.sound {
		display: grid;
		place-items: center;
		width: 40px;
		height: 40px;
		padding: 0;
		border: 1px solid var(--line);
		border-radius: 50%;
		background: var(--surface);
		color: var(--text-body);
		cursor: pointer;
	}
	.sound:hover {
		color: var(--text);
		border-color: var(--text-muted);
	}
	.sound[aria-pressed='false'] {
		color: var(--text-muted);
	}
	.sound.compact {
		width: 44px;
		height: 44px;
		border: 0;
		background: none;
	}
</style>
