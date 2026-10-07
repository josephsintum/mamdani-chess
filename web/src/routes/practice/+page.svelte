<script lang="ts">
	import { onMount } from 'svelte';
	import DiceSummary from '#lib/DiceSummary.svelte';
	import DiceTray from '#lib/DiceTray.svelte';
	import GameHelp from '#lib/GameHelp.svelte';
	import Header from '#lib/Header.svelte';
	import MoveLog from '#lib/MoveLog.svelte';
	import ScriptedBoard from '#lib/ScriptedBoard.svelte';
	import SiteFooter from '#lib/SiteFooter.svelte';
	import { Animator } from '#lib/animator.svelte.ts';
	import { createPractice, fetchView, gameExists, isStale, reasons, sideName, trySendMove, type MoveJSON, type View } from '#lib/game.ts';
	import { applyMove } from '#lib/pieces.ts';
	import { retryDelay } from '#lib/reconnect.ts';

	// Practice: you play both sides of a game on the server, with its rules
	// engine and dice but no clock. It isn't saved or listed, and starting a
	// new one ends the last. The code lives in sessionStorage, so a reload
	// carries on with the same game.

	const KEY = 'practice';
	const anim = new Animator();
	let code = $state('');
	let flipped = $state(false);
	let gone = $state(false); // the server no longer has the game (a restart)
	let error = $state('');
	let starting = $state(false);

	let view = $derived(anim.view);
	let status = $derived.by(() => {
		if (!view) return 'Setting up the board…';
		const r = view.result;
		if (!r) return anim.animating ? 'Dice are rolling…' : `${sideName(view.turn)} to move${view.check ? ' · check' : ''}`;
		const why = reasons[r.reason] ?? r.reason;
		return r.draw ? `Draw · ${why}` : `${sideName(r.winner ?? 'white')} wins · ${why}`;
	});

	let close = () => {};
	let alive = true; // false once the page has gone: start nothing more

	// Follows the game's stream. A refused stream (a restart, say) is
	// retried while the game still exists.
	function follow(c: string) {
		close();
		let es: EventSource | undefined;
		let retry: ReturnType<typeof setTimeout> | undefined;
		let failures = 0;
		let done = false;
		const connect = () => {
			es = new EventSource(`/api/games/${c}/stream`);
			es.onopen = () => (failures = 0);
			es.onerror = () => {
				if (es?.readyState !== EventSource.CLOSED) return; // it reconnects by itself
				retry = setTimeout(async () => {
					const exists = await gameExists(c);
					if (done) return; // left meanwhile: a new game, or the page
					if (exists) connect();
					else gone = true;
				}, retryDelay(failures++));
			};
			es.addEventListener('state', (e) => anim.receive(JSON.parse(e.data) as View, { hidden: document.hidden }));
		};
		connect();
		close = () => {
			done = true;
			clearTimeout(retry);
			es?.close();
		};
	}

	async function start() {
		starting = true;
		error = '';
		try {
			const c = await createPractice();
			if (!alive) return;
			code = c;
			try {
				sessionStorage.setItem(KEY, code);
			} catch {
				// Blocked storage: a reload starts a new game.
			}
			gone = false;
			follow(code);
		} catch {
			error = 'Could not start a practice game. Try again in a moment.';
		}
		starting = false;
	}

	async function move(m: MoveJSON) {
		const before = view;
		const c = code;
		if (!before) return;
		// Shown at once; the server's turn (with its dice) replaces it.
		anim.receive({ ...before, ...applyMove(before, m), legal: [], last: [] });
		const out = await trySendMove(c, m, before.seq);
		if (c !== code || !alive) return; // a new game took over meanwhile
		if (out === 'sent') {
			error = '';
			return;
		}
		error = out === 'unsent' ? 'The move didn’t reach the server. Check your connection.' : out.refused;
		// Undo the guess: the server's state, or the board before the move.
		const now = (out !== 'unsent' && out.state) || (await fetchView(c).catch(() => before));
		if (c === code && !isStale(anim.view, now)) anim.receive(now);
	}

	onMount(() => {
		let saved: string | null = null;
		try {
			saved = sessionStorage.getItem(KEY);
		} catch {
			// Blocked storage: start a new game.
		}
		if (saved) {
			fetchView(saved)
				.then((v) => {
					if (!alive) return;
					if (!v.practice) throw new Error('not a practice game');
					code = saved!;
					anim.receive(v);
					follow(saved!);
				})
				.catch(() => {
					if (alive) void start();
				});
		} else {
			void start();
		}
		return () => {
			alive = false;
			close();
			anim.stop();
		};
	});
</script>

<svelte:head>
	<title>Practice · Mamdani Chess</title>
</svelte:head>

<Header />

<main>
	<header class="intro">
		<h1>Practice</h1>
		<p class="lede">
			Play both sides with the real rules and dice, at your own pace. There's no clock, nothing is saved, and nobody else
			can move here.
		</p>
	</header>

	<div class="layout">
		<div class="board-col">
			<p class="status" aria-live="polite">{status}</p>
			{#if view}
				<ScriptedBoard
					{anim}
					legal={view.legal ?? []}
					{flipped}
					interactive={!gone}
					id="practice-{code}"
					ending={view.result ? 1 : 0}
					onmove={(m) => void move(m)}
				/>
				<div class="phone-dice"><DiceSummary {view} shown={anim.shown} wrap /></div>
			{:else}
				<div class="placeholder" aria-hidden="true"></div>
			{/if}
			{#if gone}
				<p class="error" role="alert">This practice game ended when the server restarted.</p>
			{:else if error}
				<p class="error" role="alert">{error}</p>
			{/if}
			<div class="actions">
				<button type="button" class="primary" onclick={() => void start()} disabled={starting}>New game</button>
				<button type="button" onclick={() => (flipped = !flipped)} aria-pressed={flipped}>Flip board</button>
				<GameHelp />
			</div>
		</div>
		{#if view}
			<div class="side">
				<div class="tray"><DiceTray {view} shown={anim.shown} /></div>
				<MoveLog log={view.log} rolling={anim.animating} />
			</div>
		{/if}
	</div>

	<SiteFooter />
</main>

<style>
	main {
		max-width: 1120px;
		margin: 0 auto;
		padding: 40px 32px 0;
	}
	.intro {
		display: flex;
		flex-direction: column;
		gap: 12px;
		max-width: 760px;
		margin-bottom: 28px;
	}
	h1 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: clamp(48px, 8vw, 72px);
		line-height: 0.92;
		text-transform: uppercase;
	}
	.lede {
		margin: 0;
		font-size: 18px;
		line-height: 1.55;
	}
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 600px) minmax(260px, 340px);
		gap: 32px;
		align-items: start;
		padding-bottom: 56px;
	}
	.board-col,
	.side {
		display: flex;
		flex-direction: column;
		gap: 12px;
		min-width: 0;
	}
	.status {
		margin: 0;
		color: var(--text);
		font-size: 20px;
	}
	.placeholder {
		aspect-ratio: 1;
		border-radius: 6px;
		background: var(--surface);
	}
	.phone-dice {
		display: none;
	}
	.error {
		margin: 0;
		color: var(--hazard-text);
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px;
	}
	button {
		min-height: 44px;
		padding: 0 18px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--surface);
		color: var(--text);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	button.primary {
		border-color: var(--accent);
		background: var(--accent);
		color: var(--accent-text);
	}
	button:disabled {
		opacity: 0.6;
		cursor: default;
	}
	/* A phone on its side: a board you can see whole while you move. */
	@media (orientation: landscape) and (max-height: 500px) {
		.layout {
			grid-template-columns: minmax(0, calc(100dvh - 8px)) minmax(240px, 340px);
		}
		.board-col {
			max-width: calc(100dvh - 8px);
		}
	}
	@media (max-width: 799px) {
		main {
			padding: 24px 16px 0;
		}
		.layout {
			grid-template-columns: minmax(0, 1fr);
		}
		.phone-dice {
			display: block;
		}
		.tray {
			display: none;
		}
	}
</style>
