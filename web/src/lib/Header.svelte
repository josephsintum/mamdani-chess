<script lang="ts">
	import { fly } from 'svelte/transition';
	import {
		ApiError,
		chooseName,
		initials,
		me as fetchMe,
		NAME_CHANGES,
		nameOffers,
		rememberedName,
		untilText,
		type Me
	} from './lobby.ts';
	import { reducedMotion } from './motion.ts';

	// The site header on the home and quick-match pages: the logo, and the
	// guest's name with a die that offers three new names to pick from (3
	// changes in any 24 hours). A guest who hasn't played yet has no name,
	// so no name shows. Until the page's /api/me answers, the name from this
	// browser's last visit shows, so it doesn't pop in a moment later.
	let { me = $bindable(null) }: { me?: Me | null } = $props();
	const remembered = rememberedName();

	let open = $state(false);
	let offers = $state<string[] | null>(null);
	let busy = $state(false);
	let choosing = $state<string | null>(null); // the offer being saved
	let error = $state('');
	let loadFailed = $state(false); // offers didn't load: show a retry
	let note = $state(''); // e.g. the offers were out of date
	let now = $state(Date.now());
	let announce = $state(''); // read out after a change, not on every load
	let pill: HTMLDivElement | undefined = $state();
	let die: HTMLButtonElement | undefined = $state();

	let name = $derived(me ? me.name : remembered);
	let left = $derived(me?.changesLeft ?? 0);
	let wait = $derived(me?.changesResetAt ? untilText(me.changesResetAt, now) : 'a moment');
	let dieLabel = $derived(
		!me
			? 'New name'
			: left > 0
				? `New name (${left} ${left === 1 ? 'change' : 'changes'} left today)`
				: `New names again in ${wait}`
	);

	// What the server says now: another tab may have changed the name, or
	// the 24 hours may be up.
	async function refresh() {
		try {
			me = await fetchMe();
		} catch {
			// keep what's shown
		}
		now = Date.now();
	}

	async function loadOffers() {
		offers = null;
		loadFailed = false;
		if (left === 0) return; // the panel says when names come back
		try {
			const o = await nameOffers();
			if ('resetAt' in o) {
				if (me) me = { ...me, changesLeft: 0, changesResetAt: o.resetAt };
			} else {
				offers = o.names;
			}
		} catch {
			loadFailed = true;
		}
	}

	async function toggle() {
		if (open) return close();
		open = true;
		error = '';
		note = '';
		offers = null;
		await refresh();
		await loadOffers();
	}

	async function choose(next: string) {
		busy = true;
		choosing = next;
		error = '';
		note = '';
		try {
			me = await chooseName(next);
			announce = `Your name is now ${next}`;
			close();
		} catch (e) {
			if (e instanceof ApiError && (e.status === 409 || e.status === 429)) {
				// Another tab changed the name or used the last change.
				await refresh();
				await loadOffers();
				note = left > 0 ? 'That list was out of date. Here are new names.' : '';
			} else {
				error = 'Could not change your name. Try again.';
			}
		} finally {
			busy = false;
			choosing = null;
		}
	}

	function close() {
		open = false;
		die?.focus();
	}

	function onKey(e: KeyboardEvent) {
		if (open && e.key === 'Escape') close();
	}

	function onPointer(e: PointerEvent) {
		if (open && pill && !pill.contains(e.target as Node)) open = false;
	}

	// Tabbing out of the picker closes it.
	function onFocusOut(e: FocusEvent) {
		const to = e.relatedTarget as Node | null;
		if (open && pill && to && !pill.contains(to)) open = false;
	}

	const flip = (node: Element) => fly(node, { y: reducedMotion() ? 0 : -10, duration: reducedMotion() ? 0 : 180 });
</script>

<svelte:window onkeydown={onKey} onpointerdown={onPointer} />
<svelte:document
	onvisibilitychange={() => {
		if (!document.hidden && me?.name) refresh();
	}}
/>

<header>
	<div class="inner">
		<a class="brand" href="/">
			<svg viewBox="0 0 40 40" width="30" height="30" aria-hidden="true">
				<rect x="4" y="33" width="32" height="5" rx="2" fill="var(--accent)" />
				<polygon points="20,3 33,34 7,34" fill="var(--accent)" />
				<polygon points="16.2,13 23.8,13 25.5,17 14.5,17" fill="var(--bg)" />
				<polygon points="12.4,22 27.6,22 29.3,26 10.7,26" fill="var(--bg)" />
			</svg>
			<span class="title">Mamdani Chess</span>
		</a>
		{#if name}
			<div class="me" bind:this={pill} onfocusout={onFocusOut}>
				<span class="badge" aria-hidden="true">{initials(name)}</span>
				{#key name}
					<span class="name" title={name} in:flip>{name}</span>
				{/key}
				<button
					type="button"
					class="reroll"
					class:spent={me !== null && left === 0}
					disabled={me === null}
					onclick={toggle}
					bind:this={die}
					aria-expanded={open}
					aria-controls="name-picker"
					aria-label={dieLabel}
					title={dieLabel}
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
				{#if open}
					<div class="picker" id="name-picker" role="group" aria-label="Change your name">
						<div class="pick-head">
							<p class="pick-title">{left === 0 ? 'No changes left' : 'Pick a new name'}</p>
							<span class="left">
								<span class="pips" aria-hidden="true">
									{#each Array.from({ length: NAME_CHANGES }, (_, i) => i) as i (i)}
										<span class="pip" class:used={i >= left}></span>
									{/each}
								</span>
								{left} of {NAME_CHANGES} left today
							</span>
						</div>
						{#if left === 0}
							<p class="quiet">You're {name}. New names again in {wait}.</p>
						{:else}
							{#if note}<p class="quiet">{note}</p>{/if}
							{#if offers}
								<ul>
									{#each offers as offer, i (offer)}
										<li>
											<button
												type="button"
												class="offer"
												onclick={() => choose(offer)}
												disabled={busy}
												aria-busy={choosing === offer}
												{@attach (el) => {
													if (i === 0) el.focus();
												}}>{choosing === offer ? `Saving ${offer}…` : offer}</button
											>
										</li>
									{/each}
								</ul>
							{:else if loadFailed}
								<p class="error" role="alert">Could not load new names.</p>
								<button type="button" class="offer retry" onclick={loadOffers}>Try again</button>
							{:else}
								<p class="quiet">Drawing names…</p>
							{/if}
							<p class="quiet">Choosing one uses a change.</p>
						{/if}
						<button type="button" class="keep" onclick={close}>{left === 0 ? 'OK' : `Keep ${name}`}</button>
						{#if error}<p class="error" role="alert">{error}</p>{/if}
					</div>
				{/if}
			</div>
		{/if}
	</div>
	<p class="sr-only" aria-live="polite">{announce}</p>
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
	.reroll.spent {
		opacity: 0.45;
	}
	.reroll:disabled {
		cursor: default;
	}
	.me {
		position: relative;
	}
	/* The name picker: under the pill on desktop, across the screen on phones. */
	.picker {
		position: absolute;
		top: calc(100% + 8px);
		right: 0;
		z-index: 10;
		display: flex;
		flex-direction: column;
		gap: 10px;
		width: 300px;
		padding: 16px;
		border: 1px solid var(--line);
		border-radius: 12px;
		background: var(--surface);
		box-shadow: 0 14px 36px var(--hole);
	}
	.pick-head {
		display: flex;
		flex-direction: column;
		gap: 6px;
	}
	.left {
		display: flex;
		align-items: center;
		gap: 8px;
		flex-shrink: 0;
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 12px;
	}
	.pips {
		display: flex;
		gap: 4px;
	}
	.pip {
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: var(--accent);
	}
	.pip.used {
		background: transparent;
		box-shadow: inset 0 0 0 1.5px var(--line);
	}
	.pick-title {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 20px;
		text-transform: uppercase;
	}
	.picker ul {
		display: flex;
		flex-direction: column;
		gap: 6px;
		margin: 0;
		padding: 0;
		list-style: none;
	}
	.offer,
	.keep {
		width: 100%;
		min-height: 44px;
		padding: 0 14px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--surface-2);
		color: var(--text);
		font: inherit;
		font-weight: 600;
		text-align: left;
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
		cursor: pointer;
	}
	.offer:hover,
	.offer:focus-visible {
		border-color: var(--accent);
	}
	.offer:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.keep {
		background: transparent;
		color: var(--text-muted);
	}
	.quiet {
		margin: 0;
		color: var(--text-muted);
		font-size: 13px;
	}
	.error {
		margin: 0;
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
	/* Phones and narrow windows (canvas "Home (phone)"): a smaller mark and
	   the name as its initials; screen readers still hear the whole name.
	   Wider than the page's 640 px phone layout, so a long name never
	   squeezes the logo. */
	@media (max-width: 799px) {
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
		.me {
			flex-shrink: 0;
			gap: 6px;
		}
		.me {
			position: static;
		}
		.picker {
			position: fixed;
			top: 68px;
			left: 16px;
			right: 16px;
			width: auto;
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
