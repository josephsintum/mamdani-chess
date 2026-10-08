<script lang="ts">
	// One of the move log's marks for what the dice did: a pothole opened, a
	// piece fell, or the Mamdani repaired a hole. Decorative: the log entry's
	// label says it in words.
	let { kind, piece = '' }: { kind: 'hole' | 'fell' | 'repair'; piece?: string } = $props();
</script>

{#if kind === 'fell'}
	<span class="mark fell" aria-hidden="true"
		>{#if piece === 'M'}<img class="mamdani" src="/mamdani/piece.webp" alt="" />{:else}<img src="/pieces/{piece}.svg" alt="" />{/if}<b>↓</b></span
	>
{:else}
	<span class="mark {kind}" aria-hidden="true"></span>
{/if}

<style>
	.mark {
		display: inline-flex;
		align-items: center;
		flex-shrink: 0;
	}
	/* A pothole as the board draws it: a dark hole with an orange ring. */
	.hole {
		width: 10px;
		height: 10px;
		border-radius: 46% 54% 42% 58% / 55% 45% 55% 45%;
		background: var(--hole);
		box-shadow: 0 0 0 1.5px var(--hazard);
	}
	/* A traffic cone: the Mamdani's repair. */
	.repair {
		width: 9px;
		height: 11px;
		background: linear-gradient(transparent 38%, var(--piece-light) 38% 56%, transparent 56%), var(--hazard);
		clip-path: polygon(50% 0, 100% 100%, 0 100%);
	}
	.fell img {
		width: 14px;
		height: 14px;
		opacity: 0.75;
		filter: drop-shadow(1px 0 0 var(--piece-outline)) drop-shadow(-1px 0 0 var(--piece-outline));
	}
	.fell .mamdani {
		box-sizing: border-box;
		border: 1px solid var(--accent);
		border-radius: 22%;
		object-fit: cover;
		filter: none;
	}
	.fell b {
		font-size: 11px;
		line-height: 1;
		color: var(--hazard-text);
	}
</style>
