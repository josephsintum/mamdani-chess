<script lang="ts">
	import Die from './Die.svelte';
	import { THROW_MS, type DieTone } from './dice.ts';
	import { setTipsOn, tipsOn } from './tips.ts';

	// The game page's "?": what each thing on the board means, links to the
	// rules (in a new tab, so the game keeps going), and the tips switch.
	let { compact = false }: { compact?: boolean } = $props();

	let dialog: HTMLDialogElement | undefined;
	let tips = $state(tipsOn());

	const die = (value: number, tone: DieTone) => ({ value, tone, delay: 0, ms: THROW_MS, seed: value });
	const dice: { dice: ReturnType<typeof die>[]; text: string }[] = [
		{ dice: [die(3, 'dull')], text: 'The pothole roll, odd: nothing happens.' },
		{ dice: [die(6, 'pot')], text: 'Even: a pothole opens.' },
		{ dice: [die(3, 'where'), die(5, 'where')], text: 'File and rank: where it opens (here c5).' },
		{ dice: [die(5, 'save')], text: 'A saving roll: odd saves the piece.' }
	];

	function open() {
		tips = tipsOn();
		dialog?.showModal();
	}

	function setTips(on: boolean) {
		tips = on;
		setTipsOn(on);
	}
</script>

<button type="button" class={['help', { compact }]} onclick={open} aria-haspopup="dialog">
	<span class="mark" aria-hidden="true">?</span>
	<span class={{ 'sr-only': compact }}>What's on the board</span>
</button>

<dialog
	{@attach (el) => void (dialog = el)}
	aria-labelledby="help-title"
	onclick={(e) => {
		if (e.target === e.currentTarget) e.currentTarget.close(); // a click on the backdrop
	}}
>
	<div class="body">
		<header>
			<h2 id="help-title">What's on the board</h2>
			<button type="button" class="close" aria-label="Close" onclick={() => dialog?.close()}>×</button>
		</header>

		<h3>The dice</h3>
		<ul>
			{#each dice as row (row.text)}
				<li>
					<span class="key">{#each row.dice as d, i (i)}<Die die={d} small />{/each}</span>
					{row.text}
				</li>
			{/each}
		</ul>

		<h3>The board</h3>
		<ul>
			<li><span class="key"><span class="sq hole"><i></i><i></i><i></i></span></span>A pothole. Each cone is one of its roller's moves left.</li>
			<li><span class="key"><span class="sq target"></span></span>The square the dice picked.</li>
			<li><span class="key"><span class="sq cross">×</span></span>A square a pothole cuts the picked-up piece off from.</li>
			<li><span class="key"><span class="sq legal"></span></span>Where the picked-up piece can go.</li>
			<li><span class="key"><span class="sq last"></span></span>The last move.</li>
			<li><span class="key"><span class="sq check"></span></span>A king in check.</li>
			<li>
				<span class="key"><img class="mamdani" src="/mamdani/piece.webp" alt="" /></span>
				The Mamdani. Either player can move it, and it repairs potholes next to it (👍).
			</li>
		</ul>

		<label class="switch">
			<input type="checkbox" checked={tips} onchange={(e) => setTips(e.currentTarget.checked)} />
			Show tips during games
		</label>

		<p class="links">
			<a href="/how-to-play" target="_blank" rel="noopener">How to play ↗</a>
			<a href="/rules" target="_blank" rel="noopener">Full rules ↗</a>
		</p>
	</div>
</dialog>

<style>
	.help {
		display: flex;
		align-items: center;
		gap: 8px;
		min-height: 40px;
		padding: 0 12px 0 6px;
		border: 1px solid var(--line);
		border-radius: 18px;
		background: var(--surface);
		color: var(--text-body);
		font: inherit;
		font-size: 14px;
		cursor: pointer;
	}
	.help:hover {
		color: var(--text);
		border-color: var(--text-muted);
	}
	.mark {
		display: grid;
		place-items: center;
		width: 24px;
		height: 24px;
		border-radius: 50%;
		background: var(--surface-2);
		color: var(--accent);
		font-family: var(--font-mono);
		font-weight: 600;
	}
	.help.compact {
		justify-content: center;
		width: 44px;
		height: 44px;
		padding: 0;
		border: 0;
		background: none;
	}
	dialog {
		width: min(460px, calc(100vw - 32px));
		max-height: calc(100dvh - 32px);
		padding: 0;
		border: 1px solid var(--surface-2);
		border-radius: 16px;
		background: var(--surface);
		color: var(--text-body);
	}
	dialog::backdrop {
		background: rgb(8 9 10 / 0.7);
	}
	.body {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 20px 20px 24px;
	}
	header {
		position: sticky;
		top: 0;
		z-index: 1;
		display: flex;
		align-items: center;
		justify-content: space-between;
		margin: -20px -20px 0;
		padding: 12px 20px 4px;
		background: var(--surface);
	}
	h2 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 26px;
		text-transform: uppercase;
	}
	.close {
		width: 44px;
		height: 44px;
		margin-right: -10px;
		border: 0;
		background: none;
		color: var(--text-muted);
		font-size: 28px;
		cursor: pointer;
	}
	h3 {
		margin: 8px 0 0;
		color: var(--text-muted);
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	ul {
		display: flex;
		flex-direction: column;
		gap: 10px;
		margin: 0;
		padding: 0;
		list-style: none;
		font-size: 15px;
		line-height: 1.4;
	}
	li {
		display: grid;
		grid-template-columns: 64px minmax(0, 1fr);
		gap: 12px;
		align-items: center;
	}
	.key {
		display: flex;
		gap: 6px;
		align-items: center;
	}
	/* Squares drawn as the board draws them. */
	.sq {
		position: relative;
		display: grid;
		place-items: center;
		width: 32px;
		height: 32px;
		border-radius: 4px;
		background: var(--board-light);
	}
	.sq.legal {
		background: linear-gradient(var(--legal-fill), var(--legal-fill)), var(--board-light);
	}
	.sq.last {
		background: var(--board-last-light);
	}
	.sq.check {
		background: radial-gradient(circle, var(--hazard) 0%, transparent 72%), var(--board-light);
	}
	.sq.target {
		box-shadow: inset 0 0 0 3px var(--target-ring);
	}
	.sq.cross {
		color: var(--hazard);
		font-size: 22px;
		font-weight: 700;
	}
	.sq.hole::before {
		content: '';
		width: 76%;
		height: 76%;
		border-radius: 46% 54% 42% 58% / 55% 45% 55% 45%;
		background: var(--hole);
		box-shadow: 0 0 0 1.5px var(--hazard);
	}
	.sq.hole i {
		position: absolute;
		bottom: 1px;
		width: 7px;
		height: 9px;
		background: var(--hazard);
		clip-path: polygon(50% 0, 100% 100%, 0 100%);
	}
	.sq.hole i:nth-child(1) {
		left: 6px;
	}
	.sq.hole i:nth-child(2) {
		left: 13px;
	}
	.sq.hole i:nth-child(3) {
		left: 20px;
	}
	.mamdani {
		width: 30px;
		height: 30px;
		box-sizing: border-box;
		border: 2px solid var(--accent);
		border-radius: 22%;
		object-fit: cover;
	}
	.switch {
		display: flex;
		align-items: center;
		gap: 10px;
		min-height: 44px;
		margin-top: 8px;
		padding-top: 8px;
		border-top: 1px solid var(--surface-2);
		color: var(--text);
		cursor: pointer;
	}
	.switch input {
		width: 20px;
		height: 20px;
		accent-color: var(--accent);
	}
	.links {
		display: flex;
		gap: 24px;
		margin: 0;
	}
	.links a {
		color: var(--accent);
		font-weight: 600;
		text-decoration: none;
	}
</style>
