<script lang="ts">
	import DiceTray from './DiceTray.svelte';
	import type { View } from './game.ts';
	import { reducedMotion } from './motion.ts';
	import MoveLog from './MoveLog.svelte';

	// The phone's "Moves and rolls": a sheet over the game with the log, the
	// current turn's dice step by step, and the stats once the game is over.
	// A native <dialog>: showModal() traps focus and closes on Escape.
	let { view, shown, rolling }: { view: View; shown: number; rolling: boolean } = $props();

	let dialog: HTMLDialogElement | undefined;
	let still = $state(false); // no slide under reduced motion or instant mode
	let returnTo: HTMLElement | null = null;

	/** Opens the sheet; focus goes back to `from` when it closes. */
	export function open(from: HTMLElement | null) {
		returnTo = from;
		still = reducedMotion();
		dialog?.showModal();
	}

	function keep(node: HTMLDialogElement) {
		dialog = node;
		return () => (dialog = undefined);
	}

	function close() {
		dialog?.close();
	}

	// A tap on the dimmed backdrop lands on the dialog itself.
	function backdrop(e: MouseEvent) {
		if (e.target === dialog) close();
	}
</script>

<dialog {@attach keep} class="sheet" class:still aria-labelledby="log-heading" onclick={backdrop} onclose={() => returnTo?.focus()}>
	<div class="body">
		<span class="handle" aria-hidden="true"></span>
		<button type="button" class="close" onclick={close}>Close</button>
		{#if view.result}
			<dl class="stats">
				<div><dt>Lost to potholes</dt><dd>White {view.lost.white.length} · Black {view.lost.black.length}</dd></div>
				<div><dt>Saving rolls</dt><dd>{view.stats.saved} of {view.stats.savingRolls} saved</dd></div>
				<div><dt>Repaired by the Mamdani</dt><dd>{view.stats.repaired} {view.stats.repaired === 1 ? 'pothole' : 'potholes'}</dd></div>
				{#if view.stats.mamdaniFell}<div><dt>The Mamdani</dt><dd>fell in</dd></div>{/if}
			</dl>
		{/if}
		<MoveLog log={view.log} {rolling} />
		<DiceTray {view} {shown} />
	</div>
</dialog>

<style>
	.sheet {
		position: fixed;
		inset: auto 0 0;
		width: 100%;
		max-width: 100%;
		height: min(440px, 70dvh);
		max-height: 100%;
		margin: 0;
		padding: 0;
		box-sizing: border-box;
		border: 0;
		border-top: 1px solid var(--line);
		border-radius: 16px 16px 0 0;
		background: var(--surface);
		color: var(--text-body);
		box-shadow: 0 -12px 32px var(--hole);
	}
	.sheet[open] {
		animation: slide-up 250ms ease-out;
	}
	.sheet.still[open] {
		animation: none;
	}
	.sheet::backdrop {
		background: rgb(8 9 10 / 62%);
	}
	@keyframes slide-up {
		from {
			transform: translateY(100%);
		}
	}
	.body {
		position: relative;
		display: flex;
		flex-direction: column;
		gap: 12px;
		height: 100%;
		box-sizing: border-box;
		padding: 56px 12px max(16px, env(safe-area-inset-bottom));
		overflow-y: auto;
	}
	/* The sheet scrolls; its sections keep their height. */
	.body > :global(*) {
		flex-shrink: 0;
	}
	.handle {
		position: absolute;
		top: 8px;
		left: calc(50% - 20px);
		width: 40px;
		height: 4px;
		border-radius: 2px;
		background: var(--line);
	}
	.close {
		position: absolute;
		top: 8px;
		right: 4px;
		height: 44px;
		padding: 0 14px;
		border: 0;
		background: none;
		color: var(--accent);
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.stats {
		display: grid;
		gap: 6px;
		margin: 0;
		font-size: 14px;
	}
	.stats div {
		display: flex;
		justify-content: space-between;
		gap: 12px;
	}
	.stats dt {
		color: var(--text-muted);
	}
	.stats dd {
		margin: 0;
		color: var(--text);
	}
	@media (prefers-reduced-motion: reduce) {
		.sheet[open] {
			animation: none;
		}
	}
</style>
