<script lang="ts">
	import { fly } from 'svelte/transition';
	import { initials, rerollName } from './lobby.ts';
	import { reducedMotion } from './motion.ts';

	// The site header on the home and quick-match pages: logo, Play, and the
	// guest's name with a button that draws a new one. A guest who hasn't
	// played yet has no name, so no name shows.
	let { name = $bindable(null) }: { name?: string | null } = $props();

	let busy = $state(false);
	let error = $state('');

	async function reroll() {
		busy = true;
		error = '';
		try {
			name = await rerollName();
		} catch {
			error = 'Could not change your name. Try again.';
		} finally {
			busy = false;
		}
	}

	const flip = (node: Element) => fly(node, { y: reducedMotion() ? 0 : -10, duration: reducedMotion() ? 0 : 180 });
</script>

<header>
	<div class="inner">
		<a class="brand" href="/">
			<svg viewBox="0 0 40 40" width="30" height="30" aria-hidden="true">
				<rect x="4" y="33" width="32" height="5" rx="2" fill="var(--accent)" />
				<polygon points="20,3 33,34 7,34" fill="var(--accent)" />
				<polygon points="16.2,13 23.8,13 25.5,17 14.5,17" fill="var(--bg)" />
				<polygon points="12.4,22 27.6,22 29.3,26 10.7,26" fill="var(--bg)" />
			</svg>
			<span class="title">Pothole Chess</span>
			<span class="edition">Mamdani Edition</span>
		</a>
		{#if name}
			<div class="me">
				<span class="badge" aria-hidden="true">{initials(name)}</span>
				{#key name}
					<span class="name" title={name} in:flip>{name}</span>
				{/key}
				<button
					type="button"
					class="reroll"
					onclick={reroll}
					disabled={busy}
					aria-label="New name"
					title="New name"
				>
					<svg viewBox="0 0 24 24" width="20" height="20" aria-hidden="true">
						<rect x="3" y="3" width="18" height="18" rx="4" fill="none" stroke="currentColor" stroke-width="2" />
						<circle cx="8" cy="8" r="1.6" fill="currentColor" />
						<circle cx="16" cy="8" r="1.6" fill="currentColor" />
						<circle cx="12" cy="12" r="1.6" fill="currentColor" />
						<circle cx="8" cy="16" r="1.6" fill="currentColor" />
						<circle cx="16" cy="16" r="1.6" fill="currentColor" />
					</svg>
				</button>
			</div>
		{/if}
	</div>
	<p class="sr-only" aria-live="polite">{name ? `Your name is ${name}` : ''}</p>
	{#if error}<p class="error" role="alert">{error}</p>{/if}
</header>

<style>
	header {
		border-bottom: 1px solid var(--surface-2);
	}
	.inner {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		max-width: 1280px;
		height: 72px;
		margin: 0 auto;
		padding: 0 32px;
	}
	.brand {
		display: flex;
		align-items: center;
		gap: 12px;
		min-width: 0;
		color: var(--text);
		text-decoration: none;
	}
	.brand svg {
		flex-shrink: 0;
	}
	.title {
		overflow: hidden;
		text-overflow: ellipsis;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 26px;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		white-space: nowrap;
	}
	.edition {
		padding: 3px 6px;
		border: 1px solid var(--accent-line);
		border-radius: 4px;
		color: var(--accent);
		font-family: var(--font-mono);
		font-size: 11px;
		letter-spacing: 0.1em;
		text-transform: uppercase;
		white-space: nowrap;
	}
	.me {
		display: flex;
		align-items: center;
		gap: 10px;
		min-width: 0;
		height: 44px;
		padding: 0 4px 0 6px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 22px;
	}
	.badge {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		width: 32px;
		height: 32px;
		border-radius: 50%;
		background: var(--line);
		color: var(--accent);
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.name {
		overflow: hidden;
		max-width: 24ch;
		color: var(--text);
		font-size: 15px;
		font-weight: 600;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.reroll {
		display: grid;
		place-items: center;
		flex-shrink: 0;
		width: 36px;
		height: 36px;
		padding: 0;
		border: 0;
		border-radius: 50%;
		background: transparent;
		color: var(--text-muted);
		cursor: pointer;
	}
	.reroll:hover,
	.reroll:focus-visible {
		background: var(--surface-2);
		color: var(--accent);
	}
	.reroll:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.error {
		max-width: 1280px;
		margin: 0 auto;
		padding: 0 32px 8px;
		color: var(--hazard-text);
		font-size: 14px;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	/* Phones (canvas "Home (phone)"): a smaller mark, no edition tag, and the
	   name as its initials; screen readers still hear the whole name. */
	@media (max-width: 639px) {
		.inner {
			height: 60px;
			padding: 0 16px;
			gap: 8px;
		}
		.brand {
			gap: 8px;
		}
		.brand svg {
			width: 24px;
			height: 24px;
		}
		.title {
			font-size: 22px;
		}
		.edition {
			display: none;
		}
		.me {
			flex-shrink: 0;
			gap: 6px;
		}
		.name {
			position: absolute;
			width: 1px;
			height: 1px;
			overflow: hidden;
			clip-path: inset(50%);
		}
	}
</style>
