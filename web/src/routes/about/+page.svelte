<script lang="ts">
	import Header from '#lib/Header.svelte';
	import SiteFooter from '#lib/SiteFooter.svelte';

	// What this site is, who it owes, and what it keeps about you.
</script>

<svelte:head>
	<title>About · Mamdani Chess</title>
</svelte:head>

<Header />

<main>
	<article>
		<header class="intro">
			<h1>About</h1>
			<p class="lede">
				Mamdani Chess is chess with potholes, to play with friends in the browser. It's the original Pot-Hole Chess plus
				one new piece, the Mamdani, which both players share and which repairs potholes.
			</p>
		</header>

		<section aria-labelledby="unofficial">
			<h2 id="unofficial">Unofficial and ad-free</h2>
			<p>
				This is a fan project. It isn't affiliated with or endorsed by Zohran Mamdani, any campaign or any city
				office. There are no ads, nothing to buy and no sign-up.
			</p>
		</section>

		<section aria-labelledby="credits">
			<h2 id="credits">Credits</h2>
			<ul>
				<li>
					<strong>The game:</strong>
					<a href="https://www.chessvariants.com/boardrules.dir/potholechess.html" target="_blank" rel="noopener"
						>Pot-Hole Chess</a
					>
					by Peter Spicer and Michael Chamberlain (2001). Mamdani Chess makes potholes last longer and adds the Mamdani.
				</li>
				<li>
					<strong>The Mamdani:</strong> from
					<a href="https://www.instagram.com/reel/Dd8tV_MxEQL/" target="_blank" rel="noopener">The Mamdani Patch</a>, a
					reel by
					<a href="https://www.instagram.com/bardelo_bardalini/" target="_blank" rel="noopener">@bardelo_bardalini</a>.
				</li>
				<li>
					<strong>Pieces:</strong> the mpchess set by
					<a href="https://github.com/chupinmaxime" target="_blank" rel="noopener">Maxime Chupin</a>, under the GPL-3.0,
					with heavier lines.
				</li>
				<li>
					<strong>Chess sounds:</strong> the SFX set by
					<a href="https://github.com/Enigmahack" target="_blank" rel="noopener">Enigmahack</a>, from
					<a href="https://github.com/lichess-org/lila" target="_blank" rel="noopener">Lichess</a>, under the AGPL-3.0 or later.
				</li>
				<li>
					<strong>Fonts:</strong> Barlow Condensed and IBM Plex, under the SIL Open Font License.
				</li>
			</ul>
		</section>

		<section aria-labelledby="privacy">
			<h2 id="privacy">What we keep</h2>
			<ul>
				<li>
					<strong>One cookie.</strong> It holds a random ID that tells the server you're the same guest, so you keep your
					seat in a game. Scripts on the page can't read it.
				</li>
				<li>
					<strong>Your name.</strong> The server gives you a made-up name when you first play and keeps it with your ID.
					This browser keeps a copy so the header can show it at once.
				</li>
				<li>
					<strong>Games are public.</strong> Anyone with a game's link can watch it, games being played are listed on
					the home page, and every game between players is saved with its moves and rolls.
				</li>
				<li>
					<strong>No ads, no analytics.</strong> Our logs record games and requests with a shortened guest ID, not your IP
					address.
				</li>
				<li>
					<strong>Two outside services.</strong> The fonts load from Google Fonts, and the home page embeds a reel from
					Instagram. Each sees your visit as any site you load from would.
				</li>
			</ul>
		</section>

		<section aria-labelledby="source">
			<h2 id="source">Source</h2>
			<p>
				The code is public on
				<a href="https://github.com/josephsintum/mamdani-chess" target="_blank" rel="noopener">GitHub</a>.
			</p>
		</section>

		<SiteFooter />
	</article>
</main>

<style>
	main {
		max-width: 1120px;
		margin: 0 auto;
		padding: 56px 32px 0;
	}
	article {
		display: flex;
		flex-direction: column;
		gap: 48px;
		max-width: 760px;
	}
	.intro {
		display: flex;
		flex-direction: column;
		gap: 16px;
	}
	h1 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: clamp(56px, 9vw, 80px);
		line-height: 0.92;
		text-transform: uppercase;
	}
	.lede {
		font-size: 19px;
	}
	section {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	h2 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 36px;
		text-transform: uppercase;
	}
	p,
	ul {
		margin: 0;
		font-size: 17px;
		line-height: 1.6;
	}
	ul {
		display: flex;
		flex-direction: column;
		gap: 10px;
		padding-left: 20px;
	}
	strong {
		color: var(--text);
	}
	a {
		color: var(--accent);
	}
	@media (max-width: 799px) {
		main {
			padding: 32px 16px 0;
		}
		article {
			gap: 40px;
		}
		h2 {
			font-size: 30px;
		}
	}
</style>
