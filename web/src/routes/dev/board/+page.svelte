<script lang="ts">
	import Board from '#lib/Board.svelte';
	import DiceTray from '#lib/DiceTray.svelte';
	import MoveLog from '#lib/MoveLog.svelte';
	import PlayerBar from '#lib/PlayerBar.svelte';
	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
	import { stageAt } from '#lib/board.ts';
	import type { Color, MoveJSON, View } from '#lib/game.ts';
	import { setInstant } from '#lib/motion.ts';
	import { freeMoves, playTurn, positions, type RollScript } from '#lib/sandbox.ts';

	// Test bench for the board UI: one browser plays both sides, nothing goes
	// to the server, and the dice do what you pick. Moves follow how each piece
	// moves but ignore check.

	type RollKind = 'odd' | 'empty' | 'falls' | 'saved' | 'notSaved' | 'rerollFalls' | 'mamdaniFalls' | 'random';
	const rollKinds: { kind: RollKind; label: string; needsTarget: boolean }[] = [
		{ kind: 'odd', label: 'Odd: nothing happens', needsTarget: false },
		{ kind: 'empty', label: 'Pothole opens', needsTarget: true },
		{ kind: 'falls', label: 'Piece falls (no saving roll)', needsTarget: true },
		{ kind: 'saved', label: 'Saving roll: saved', needsTarget: true },
		{ kind: 'notSaved', label: 'Saving roll: lost', needsTarget: true },
		{ kind: 'rerollFalls', label: 'Re-roll on e1, then target', needsTarget: true },
		{ kind: 'mamdaniFalls', label: 'Mamdani falls', needsTarget: false },
		{ kind: 'random', label: 'Random', needsTarget: false }
	];

	const anim = new Animator();
	anim.receive(positions.start());

	let rollKind = $state<RollKind>('empty');
	let target = $state('d4');
	let anySide = $state(false);
	let slow = $state(false);
	let instant = $state(false); // no animation at all, for fast play-testing
	let you = $state<Color | 'spectator'>('white');

	let view = $derived(anim.view as View);
	let stage = $derived(stageAt(view, anim.shown));
	let legal = $derived(view.status === 'playing' ? freeMoves(view, anySide) : []);
	let bottom = $derived<Color>(you === 'black' ? 'black' : 'white');
	let top = $derived<Color>(bottom === 'white' ? 'black' : 'white');
	let lastMove = $derived.by(() => {
		const m = view.last.find((e) => e.kind === 'moved');
		return m?.from && m?.to ? { from: m.from, to: m.to } : null;
	});
	let savedSquare = $derived(view.last.find((e, i) => e.kind === 'saving_roll' && e.saved && i < anim.shown)?.sq ?? '');
	let needsTarget = $derived(rollKinds.find((r) => r.kind === rollKind)?.needsTarget ?? false);

	function d8(): number {
		return Math.floor(Math.random() * 8) + 1;
	}

	function script(v: View): RollScript {
		switch (rollKind) {
			case 'odd':
				return { pothole: 1 };
			case 'empty':
			case 'falls':
				return { pothole: 2, target };
			case 'saved':
				return { pothole: 4, target, save: 3 };
			case 'notSaved':
				return { pothole: 4, target, save: 6 };
			case 'rerollFalls':
				return { pothole: 6, rerolls: ['e1'], target };
			case 'mamdaniFalls':
				return v.mamdani ? { pothole: 8, target: v.mamdani, save: 2 } : { pothole: 1 };
			case 'random': {
				const holes = new Set(v.potholes.map((p) => p.sq));
				const squares = [...Array(64).keys()]
					.map((i) => 'abcdefgh'[i % 8] + (Math.floor(i / 8) + 1))
					.filter((s, i) => !holes.has(s) && v.board[i][1] !== 'K');
				const t = squares[Math.floor(Math.random() * squares.length)];
				const occupied = t === v.mamdani || v.board.some((p, i) => p && 'abcdefgh'[i % 8] + (Math.floor(i / 8) + 1) === t);
				return { pothole: d8(), target: t, save: occupied && Math.random() < 0.5 ? d8() : undefined };
			}
		}
	}

	function move(m: MoveJSON) {
		const next = playTurn(view, m, script(view));
		next.you = you;
		anim.stepMs = instant ? 0 : slow ? STEP_MS * 3 : STEP_MS;
		anim.receive(next);
	}

	function load(v: View) {
		v.you = you;
		anim.receive(v);
	}

	function end(result: NonNullable<View['result']>) {
		load({ ...JSON.parse(JSON.stringify(view)), status: 'over', result, last: [] });
	}

	function setYou(c: Color | 'spectator') {
		you = c;
		load({ ...JSON.parse(JSON.stringify(view)), you: c, last: view.last, seq: view.seq });
	}
</script>

<svelte:head>
	<title>Board sandbox · Pothole Chess</title>
</svelte:head>

<main>
	<header>
		<a href="/" class="logo">Pothole Chess</a>
		<span class="tag">Board sandbox · dev only</span>
	</header>
	<p class="status">
		{view.result ? 'Game over' : anim.animating ? 'Dice are rolling…' : `${view.turn === 'white' ? 'White' : 'Black'} to move`}
		<span class="muted">· moves ignore check, dice are scripted, nothing goes to the server</span>
	</p>

	<div class="layout">
		<div class="side">
			<section class="panel" aria-labelledby="roll-heading">
				<h2 id="roll-heading">Next roll</h2>
				{#each rollKinds as r (r.kind)}
					<label class="choice"><input type="radio" name="roll" value={r.kind} bind:group={rollKind} /> {r.label}</label>
				{/each}
				<label class="field" class:disabled={!needsTarget}>
					Target square
					<input type="text" bind:value={target} maxlength="2" disabled={!needsTarget} />
				</label>
			</section>

			<section class="panel" aria-labelledby="view-heading">
				<h2 id="view-heading">View</h2>
				<label class="field">
					Seen as
					<select value={you} onchange={(e) => setYou(e.currentTarget.value as Color | 'spectator')}>
						<option value="white">White</option>
						<option value="black">Black (flipped)</option>
						<option value="spectator">Spectator</option>
					</select>
				</label>
				<label class="choice"><input type="checkbox" bind:checked={anySide} /> Move either side any time</label>
				<label class="choice"><input type="checkbox" bind:checked={slow} /> Slow dice (3×)</label>
				<label class="choice">
					<input
						type="checkbox"
						checked={instant}
						onchange={(e) => {
							instant = e.currentTarget.checked;
							setInstant(instant);
						}}
					/>
					Instant (no animation)
				</label>
			</section>

			<section class="panel" aria-labelledby="pos-heading">
				<h2 id="pos-heading">Positions</h2>
				<div class="buttons">
					<button onclick={() => load(positions.start())}>Start</button>
					<button onclick={() => load(positions.blockedLines())}>Blocked lines</button>
					<button onclick={() => load(positions.promotion())}>Promotion</button>
					<button onclick={() => load(positions.castling())}>Castling</button>
				</div>
			</section>

			<section class="panel" aria-labelledby="end-heading">
				<h2 id="end-heading">Result card</h2>
				<div class="buttons">
					<button onclick={() => end({ winner: 'white', draw: false, reason: 'checkmate' })}>Checkmate</button>
					<button onclick={() => end({ winner: 'black', draw: false, reason: 'resignation' })}>Resignation</button>
					<button onclick={() => end({ draw: true, reason: 'stalemate' })}>Draw</button>
				</div>
			</section>
		</div>

		<div class="board-col">
			<PlayerBar color={top} you={you === top} lost={stage.lost[top]} toMove={!view.result && view.turn === top && !anim.animating} />
			<Board
				{stage}
				{legal}
				{lastMove}
				flipped={bottom === 'black'}
				interactive={!anim.animating && view.status === 'playing'}
				dim={!!view.result && !anim.animating}
				saved={savedSquare}
				onmove={move}
			/>
			<PlayerBar color={bottom} you={you === bottom} lost={stage.lost[bottom]} toMove={!view.result && view.turn === bottom && !anim.animating} />
		</div>

		<div class="side">
			<DiceTray {view} shown={anim.shown} />
			<MoveLog log={view.log} rolling={anim.animating} />
		</div>
	</div>
</main>

<style>
	main {
		max-width: 1400px;
		margin: 0 auto;
		padding: 16px 24px 40px;
		display: grid;
		gap: 12px;
	}
	header {
		display: flex;
		align-items: baseline;
		gap: 12px;
	}
	.logo {
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 28px;
		text-transform: uppercase;
		color: var(--text);
		text-decoration: none;
	}
	.tag {
		font-family: var(--font-mono);
		font-size: 13px;
		color: var(--hazard);
	}
	.status {
		margin: 0;
		font-size: 20px;
		color: var(--text);
	}
	.muted {
		font-size: 14px;
		color: var(--text-muted);
	}
	.layout {
		display: grid;
		grid-template-columns: 280px minmax(0, 600px) minmax(260px, 340px);
		gap: 24px;
		align-items: start;
	}
	@media (max-width: 1100px) {
		.layout {
			grid-template-columns: minmax(0, 600px);
		}
	}
	.side,
	.board-col {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.panel {
		display: flex;
		flex-direction: column;
		gap: 8px;
		padding: 16px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 14px;
	}
	h2 {
		margin: 0 0 4px;
		font-family: var(--font-display);
		font-weight: 700;
		font-size: 18px;
		text-transform: uppercase;
		color: var(--text);
	}
	.choice,
	.field {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 32px;
		color: var(--text-body);
		font-size: 14px;
	}
	.field {
		justify-content: space-between;
	}
	.field.disabled {
		opacity: 0.5;
	}
	input[type='text'],
	select {
		min-height: 36px;
		padding: 0 10px;
		border: 1px solid var(--line);
		border-radius: 8px;
		background: var(--surface-2);
		color: var(--text);
		font: inherit;
	}
	input[type='text'] {
		width: 64px;
		font-family: var(--font-mono);
	}
	.buttons {
		display: flex;
		flex-wrap: wrap;
		gap: 8px;
	}
	button {
		min-height: 40px;
		padding: 0 14px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--surface-2);
		color: var(--text);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
</style>
