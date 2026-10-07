<script lang="ts">
	// The footer on the site's pages (not the game): where to learn the game,
	// the original game's credit, and that this is a fan project.
</script>

<footer>
	<nav aria-label="Site">
		<a href="/how-to-play">How to play</a>
		<a href="/rules">Rules</a>
		<a href="/practice">Practice</a>
		<a href="/about">About</a>
	</nav>
	<p>
		Based on Pot-Hole Chess by Peter Spicer and Michael Chamberlain (2001). An unofficial fan project, not affiliated with
		Zohran Mamdani.
	</p>
</footer>

<style>
	footer {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding: 24px 0 40px;
		border-top: 1px solid var(--surface-2);
		color: var(--text-muted);
		font-size: 13px;
	}
	nav {
		display: flex;
		flex-wrap: wrap;
		gap: 0 20px;
		margin: -10px 0;
	}
	a {
		display: inline-flex;
		align-items: center;
		min-height: 40px;
		color: var(--text-body);
		font-size: 14px;
		text-decoration: none;
	}
	a:hover {
		color: var(--text);
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	p {
		margin: 0;
	}
</style>
