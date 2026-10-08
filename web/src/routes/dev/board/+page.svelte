<script lang="ts">
	import DiceTray from '#lib/DiceTray.svelte';
	import MoveLog from '#lib/MoveLog.svelte';
	import PlayerBar from '#lib/PlayerBar.svelte';
	import ScriptedBoard from '#lib/ScriptedBoard.svelte';
	import { Animator } from '#lib/animator.svelte.ts';
	import { dicePace } from '#lib/dice.ts';
	import { pieceOn, pillFor, squareAt, squareIndex, stageAt } from '#lib/board.ts';
	import { opponent, sideName, squareName, type Color, type MoveJSON, type View } from '#lib/game.ts';
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
	let targetMode = $state<'square' | 'random'>('square');
	let anySide = $state(false);
	let slow = $state(false);
	let instant = $state(false); // no animation at all, for fast play-testing
	let you = $state<Color | 'spectator'>('white');

	let view = $derived(anim.view!);
	let stage = $derived(stageAt(view, anim.shown));
	let legal = $derived(view.status === 'playing' ? freeMoves(view, anySide) : []);
	let bottom = $derived<Color>(you === 'black' ? 'black' : 'white');
	let top = $derived(opponent(bottom));
	let topPill = $derived(pillFor(view, top, anim.animating));
	let bottomPill = $derived(pillFor(view, bottom, anim.animating));
	// The Result card buttons end the game at once: each checkmate bursts once.
	let endings = $state(0);
	let needsTarget = $derived(rollKinds.find((r) => r.kind === rollKind)?.needsTarget ?? false);

	function d8(): number {
		return Math.floor(Math.random() * 8) + 1;
	}

	/**
	 * Where the placement dice land: the typed square, or, with Random, two d8s
	 * re-rolled past kings as the server does (an open pothole resets).
	 */
	function placement(v: View): Pick<RollScript, 'rerolls' | 'target'> {
		if (targetMode === 'square') return { target };
		const rerolls: NonNullable<RollScript['rerolls']> = [];
		for (let i = 0; i < 64; i++) {
			const sq = squareAt(d8() - 1, d8() - 1);
			if (pieceOn(v, sq)[1] === 'K') rerolls.push({ sq, reason: 'king' });
			else return { rerolls, target: sq };
		}
		return { rerolls, target };
	}

	function script(v: View): RollScript {
		switch (rollKind) {
			case 'odd':
				return { pothole: 1 };
			case 'empty':
			case 'falls':
				return { pothole: 2, ...placement(v) };
			case 'saved':
				return { pothole: 4, ...placement(v), save: 3 };
			case 'notSaved':
				return { pothole: 4, ...placement(v), save: 6 };
			case 'rerollFalls': {
				const p = placement(v);
				return { pothole: 6, rerolls: ['e1', ...(p.rerolls ?? [])], target: p.target };
			}
			case 'mamdaniFalls':
				return v.mamdani ? { pothole: 8, target: v.mamdani, save: 2 } : { pothole: 1 };
			case 'random': {
				const squares = [...Array(64).keys()].map(squareName).filter((_, i) => v.board[i][1] !== 'K');
				const t = squares[Math.floor(Math.random() * squares.length)];
				const occupied = t === v.mamdani || !!v.board[squareIndex(t)];
				return { pothole: d8(), target: t, save: occupied && Math.random() < 0.5 ? d8() : undefined };
			}
		}
	}

	function move(m: MoveJSON) {
		const next = playTurn(view, m, script(view));
		next.you = you;
		anim.pace = instant ? 0 : slow ? (e) => dicePace(e) * 3 : dicePace;
		anim.receive(next);
	}

	function load(v: View) {
		v.you = you;
		anim.receive(v);
	}

	function end(result: NonNullable<View['result']>) {
		endings += 1;
		load({ ...$state.snapshot(view), status: 'over', result, last: [] });
	}

	function setYou(c: Color | 'spectator') {
		you = c;
		load($state.snapshot(view));
	}

	function setInstantMode(on: boolean) {
		instant = on;
		setInstant(on);
	}
</script>

<svelte:head>
	<title>Board sandbox · Mamdani Chess</title>
</svelte:head>

<main>
	<header>
		<a href="/" class="logo">Mamdani Chess</a>
		<span class="tag">Board sandbox · dev only</span>
	</header>
	<p class="status">
		{view.result ? 'Game over' : anim.animating ? 'Dice are rolling…' : `${sideName(view.turn)} to move`}
		<span class="muted">· moves ignore check, dice are scripted, nothing goes to the server</span>
	</p>

	<div class="layout">
		<div class="side">
			<section class="panel" aria-labelledby="roll-heading">
				<h2 id="roll-heading">Next roll</h2>
				{#each rollKinds as r (r.kind)}
					<label class="choice"><input type="radio" name="roll" value={r.kind} bind:group={rollKind} /> {r.label}</label>
				{/each}
				<fieldset class="target" class:disabled={!needsTarget} disabled={!needsTarget}>
					<legend>Target</legend>
					<label class="choice"><input type="radio" name="target" value="random" bind:group={targetMode} /> Random</label>
					<label class="field">
						<span class="choice"><input type="radio" name="target" value="square" bind:group={targetMode} /> Square</span>
						<input type="text" bind:value={target} maxlength="2" aria-label="Target square" onfocus={() => (targetMode = 'square')} />
					</label>
				</fieldset>
			</section>

			<section class="panel" aria-labelledby="view-heading">
				<h2 id="view-heading">View</h2>
				<label class="field">
					Seen as
					<select bind:value={() => you, setYou}>
						<option value="white">White</option>
						<option value="black">Black (flipped)</option>
						<option value="spectator">Spectator</option>
					</select>
				</label>
				<label class="choice"><input type="checkbox" bind:checked={anySide} /> Move either side any time</label>
				<label class="choice"><input type="checkbox" bind:checked={slow} /> Slow dice (3×)</label>
				<label class="choice">
					<input type="checkbox" bind:checked={() => instant, setInstantMode} />
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
					<button onclick={() => load(positions.repair())}>Repair</button>
					<button onclick={() => load(positions.twoLeft())}>Two pieces left</button>
				</div>
			</section>

			<section class="panel" aria-labelledby="end-heading">
				<h2 id="end-heading">Result card</h2>
				<div class="buttons">
					<button onclick={() => end({ winner: 'white', draw: false, reason: 'checkmate' })}>Checkmate</button>
					<button onclick={() => end({ winner: 'black', draw: false, reason: 'resignation' })}>Resignation</button>
					<button onclick={() => end({ winner: 'black', draw: false, reason: 'timeout' })}>Out of time</button>
					<button onclick={() => end({ draw: true, reason: 'stalemate' })}>Draw</button>
				</div>
			</section>
		</div>

		<div class="board-col">
			<PlayerBar color={top} you={you === top} lost={stage.lost[top]} pill={topPill.text} pillTone={topPill.tone} />
			<ScriptedBoard {anim} {legal} flipped={bottom === 'black'} interactive still={instant} id="sandbox" ending={endings} onmove={move} />
			<PlayerBar color={bottom} you={you === bottom} lost={stage.lost[bottom]} pill={bottomPill.text} pillTone={bottomPill.tone} />
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
	.target.disabled {
		opacity: 0.5;
	}
	.target {
		margin: 4px 0 0;
		padding: 0;
		border: 0;
	}
	.target legend {
		padding: 0;
		margin-bottom: 4px;
		color: var(--text-body);
		font-size: 14px;
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
