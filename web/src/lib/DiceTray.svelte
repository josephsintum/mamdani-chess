<script lang="ts">
	import { diceSteps } from './board.ts';
	import Die from './Die.svelte';
	import type { View } from './game.ts';

	let { view, shown }: { view: View; shown: number } = $props();

	let mover = $derived(view.last.find((e) => e.kind === 'moved')?.color);
	let steps = $derived(diceSteps(view, shown));
	let resolving = $derived(shown < view.last.length);
	// Potholes closed or repaired by the move itself, before the roll.
	let notes = $derived.by(() => {
		const out: string[] = [];
		for (const e of view.last) {
			if (e.kind === 'rolled_pothole') break;
			if (e.kind === 'pothole_closed') out.push(`Pothole on ${e.sq} closes`);
			if (e.kind === 'repaired') out.push(`The Mamdani repairs ${e.sq}`);
		}
		return out;
	});
</script>

<section class="tray" aria-labelledby="dice-heading" aria-live="polite">
	<div class="head">
		<h2 id="dice-heading">{mover ? `${mover === 'white' ? 'White' : 'Black'}'s roll` : 'Dice'}</h2>
		{#if resolving}<span class="status">Resolving</span>{/if}
	</div>
	{#if !mover}
		<p class="muted">The dice roll after every move.</p>
	{:else}
		{#each notes as note (note)}
			<p class="note">{note}</p>
		{/each}
		{#if steps.length === 0}
			<p class="muted">{view.result ? 'The move ended the game: no roll.' : 'Rolling…'}</p>
		{/if}
		<ol>
			{#each steps as step, i (i)}
				<li class={step.tone}>
					{#if step.dice.length}
						<span class="dice">
							{#each step.dice as d, j (j)}
								<Die die={d} />
							{/each}
						</span>
					{/if}
					<span class="text" class:late={step.revealAt} style="--late: {step.revealAt ?? 0}ms">
						<span class="title">{step.title}</span>
						<span class="detail">{step.detail}</span>
					</span>
				</li>
			{/each}
		</ol>
	{/if}
</section>

<style>
	.tray {
		display: flex;
		flex-direction: column;
		gap: 12px;
		padding: 20px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 14px;
	}
	.head {
		display: flex;
		justify-content: space-between;
		align-items: center;
	}
	h2 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 700;
		font-size: 22px;
		text-transform: uppercase;
		color: var(--text);
	}
	.status {
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		color: var(--accent);
		text-transform: uppercase;
		letter-spacing: 0.08em;
	}
	p {
		margin: 0;
	}
	.muted,
	.note {
		color: var(--text-muted);
		font-size: 14px;
	}
	ol {
		margin: 0;
		padding: 0;
		list-style: none;
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	li {
		display: flex;
		align-items: center;
		gap: 14px;
		animation: appear 0.25s ease-out;
	}
	@keyframes appear {
		from {
			opacity: 0;
			transform: translateY(4px);
		}
	}
	.dice {
		display: flex;
		gap: 6px;
		flex-shrink: 0;
	}
	.text {
		display: flex;
		flex-direction: column;
		gap: 2px;
	}
	.title {
		font-weight: 600;
		color: var(--text);
	}
	.detail {
		font-size: 13px;
		color: var(--text-muted);
	}
	li.muted .title {
		color: var(--text-body);
	}
	li.good {
		padding: 10px 12px;
		margin: 0 -12px;
		background: var(--accent-wash);
		border: 1px solid var(--accent-line);
		border-radius: 12px;
	}
	li.good .title {
		color: var(--accent);
	}
	li.hazard .title {
		color: var(--hazard-text);
	}
	/* What the dice decide shows once they land. */
	.late {
		animation: text-in 0.2s ease-out var(--late) both;
	}
	@keyframes text-in {
		from {
			opacity: 0;
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.late {
			animation: none;
		}
	}
</style>
