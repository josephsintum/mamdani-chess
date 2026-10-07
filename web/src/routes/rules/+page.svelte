<script lang="ts">
	import Header from '#lib/Header.svelte';
	import MiniBoard from '#lib/MiniBoard.svelte';
	import { boardFromFen } from '#lib/lobby.ts';
	import { emptyView, freeMoves } from '#lib/sandbox.ts';
	import { HOLE_CAP, HOLE_ROUNDS } from '#lib/wire.gen.ts';

	// The rules, as the live rules doc has them (RULES.md is its copy): short
	// sections with a board each, then every rule in full for the edge cases.
	// How-to-play shows each one played.

	const start = boardFromFen('rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR');
	const midgame = boardFromFen('r1bqkb1r/1p3ppp/p1n1pn2/3N4/3P4/5N2/PP2PPPP/R1BQKB1R');
	const reachView = { ...emptyView(), mamdani: 'd4', potholes: [{ sq: 'f6', by: 'white' as const, left: 3 }] };
	const reach = freeMoves(reachView, true).map((m) => m.to);

	const sections = [
		{ id: 'setup', title: 'Setup' },
		{ id: 'turn', title: 'A turn' },
		{ id: 'potholes', title: 'Potholes' },
		{ id: 'mamdani', title: 'The Mamdani' },
		{ id: 'saving', title: 'Saving rolls' },
		{ id: 'winning', title: 'Winning and draws' },
		{ id: 'full', title: 'Every rule' }
	];

	const steps = [
		{ name: 'Move.', text: 'Move one of your pieces, or the Mamdani.' },
		{ name: 'Count down.', text: 'Each pothole you rolled loses a round: a cone comes off. One with none left closes.' },
		{ name: 'Repair.', text: 'Any pothole next to the Mamdani is repaired.' },
		{ name: 'Roll a d8.', text: 'Odd: nothing happens. Even: a pothole opens.' },
		{ name: 'Place it.', text: `Two d8s pick the file (1 = a … 8 = h) and the rank. With ${HOLE_CAP} already open, the oldest closes first.` }
	];
</script>

<svelte:head>
	<title>Rules · Mamdani Chess</title>
</svelte:head>

{#snippet played(scene: string)}
	<a class="played" href="/how-to-play#{scene}">See it played →</a>
{/snippet}

<Header />

<main>
	<nav class="toc" aria-label="Rules sections">
		<span class="toc-head">On this page</span>
		{#each sections as s (s.id)}
			<a href="#{s.id}">{s.title}</a>
		{/each}
	</nav>

	<article>
		<header class="intro">
			<h1>The rules</h1>
			<p class="lede">
				Chess, with two additions. Potholes open at random and swallow pieces. The Mamdani, a neutral piece either
				player can move, blocks lines and repairs potholes. Everything else is ordinary chess.
			</p>
			<a class="played" href="/how-to-play">New here? See how to play →</a>
		</header>

		<section id="setup" class="with-figure">
			<div class="text">
				<h2>Setup</h2>
				<p>
					A normal chess position, plus the Mamdani on a5, a square that is empty at the start. White moves first. Each
					player has 10 minutes, plus 5 seconds a move.
				</p>
			</div>
			<figure>
				<MiniBoard board={start} mamdani="a5" label="The starting position, with the Mamdani on a5." />
				<figcaption>The Mamdani starts on a5.</figcaption>
			</figure>
		</section>

		<section id="turn">
			<h2>A turn</h2>
			<p>Every turn is one move, then one pothole roll:</p>
			<ol class="steps">
				{#each steps as step, i (i)}
					<li><span class="num">{String(i + 1).padStart(2, '0')}</span><span><strong>{step.name}</strong> {step.text}</span></li>
				{/each}
			</ol>
			<p class="note">A move that checkmates ends the game at once, with no roll. Your clock pauses while the dice roll.</p>
			{@render played('dice')}
		</section>

		<section id="potholes" class="with-figure">
			<div class="text">
				<h2>Potholes</h2>
				<ul>
					<li>Whatever stands on the square falls in and is lost, unless a saving roll saves it.</li>
					<li>Kings never fall. If the dice land on a king, they roll again.</li>
					<li>A pothole next to the Mamdani is repaired the moment it opens.</li>
					<li>
						Nothing can land on a pothole. Bishops, rooks, queens and the Mamdani can't slide across one, so it blocks
						attacks and checks like a piece. Knights can jump it.
					</li>
					<li>It stays open for {HOLE_ROUNDS} of its roller's moves. The cones on its edge count them down.</li>
					<li>At most {HOLE_CAP} are open at once. When another opens, the oldest closes.</li>
				</ul>
				{@render played('roads')}
			</div>
			<figure>
				<MiniBoard
					board={midgame}
					mamdani="a5"
					potholes={[{ sq: 'f4', by: 'white', left: 2 }]}
					target="d5"
					cones
					label="A pothole on f4 with two rounds left, and the dice's pick, d5, ringed."
				/>
				<figcaption>A pothole on f4 with 2 rounds left. The dice have just picked d5.</figcaption>
			</figure>
		</section>

		<section id="mamdani" class="with-figure">
			<div class="text">
				<h2>The Mamdani</h2>
				<ul>
					<li>
						It belongs to neither player. It moves like a queen, onto empty squares only, and can't jump pieces or
						potholes.
					</li>
					<li>Either player can move it instead of a piece. That uses your whole move.</li>
					<li>It never captures, and nobody can capture it.</li>
					<li>It blocks lines like any piece: it can block a check, or a king's escape.</li>
					<li>You can't move it if that leaves your own king in check.</li>
					<li>It repairs any pothole on the 8 squares around it.</li>
					<li>If it falls into a pothole, it's gone for the rest of the game.</li>
				</ul>
				{@render played('blocks')}
			</div>
			<figure>
				<MiniBoard
					board={reachView.board}
					mamdani="d4"
					potholes={reachView.potholes}
					dots={reach}
					label="The Mamdani on d4 and the squares it can reach. A pothole on f6 blocks its diagonal."
				/>
				<figcaption>Where the Mamdani can go. The pothole on f6 blocks its diagonal.</figcaption>
			</figure>
		</section>

		<section id="saving">
			<h2>Saving rolls</h2>
			<p>A saving roll is one d8. Odd saves the piece, and the pothole never opens.</p>
			<div class="table-wrap">
				<table>
					<thead>
						<tr>
							<th scope="col">On the square</th>
							<th scope="col">Gets a roll when</th>
							<th scope="col">Odd</th>
							<th scope="col">Even</th>
						</tr>
					</thead>
					<tbody>
						<tr>
							<th scope="row">A player's piece</th>
							<td>The Mamdani has a clear queen line to it</td>
							<td class="good">Saved</td>
							<td class="bad">Falls in</td>
						</tr>
						<tr>
							<th scope="row">The Mamdani</th>
							<td>Always</td>
							<td class="good">Saved</td>
							<td class="bad">Gone for good</td>
						</tr>
					</tbody>
				</table>
			</div>
			<p class="note">
				The piece's owner rolls; for the Mamdani, the player who rolled the pothole. Once the Mamdani has fallen, there are
				no more saving rolls.
			</p>
			{@render played('saving')}
		</section>

		<section id="winning">
			<h2>Winning and draws</h2>
			<ul>
				<li>Checkmate wins. So does your opponent running out of time, or resigning.</li>
				<li>
					A roll can checkmate. Kings never fall, but a pothole can open on a king's last escape square, or swallow the
					piece blocking a check.
				</li>
				<li>Stalemate is a draw, but only if you have no legal Mamdani move either.</li>
				<li>The 50-move rule, repetition and too little material to mate are draws, with the changes below.</li>
			</ul>
			{@render played('mate')}
		</section>

		<section id="full">
			<details>
				<summary><h2>Every rule, in full</h2></summary>
				<div class="full">
					<h3>When the dice land on a square</h3>
					<ul>
						<li><strong>A king:</strong> kings never fall. Both d8s roll again.</li>
						<li><strong>A pothole:</strong> both d8s roll again.</li>
						<li>
							<strong>A square that would expose the roller's king:</strong> if the piece falling would leave the player
							who just moved in check once the hole closes, or the cap closing the oldest pothole would, both d8s roll
							again.
						</li>
						<li><strong>Next to the Mamdani:</strong> the pothole is repaired the moment it opens. Nothing falls.</li>
						<li><strong>Too many re-rolls:</strong> after 64 re-rolls without a square, no pothole opens that turn.</li>
						<li><strong>An empty square:</strong> the pothole opens, with {HOLE_ROUNDS} rounds.</li>
						<li><strong>Any other piece:</strong> it falls in and is lost, unless a saving roll saves it.</li>
						<li><strong>The Mamdani:</strong> it always gets a saving roll.</li>
					</ul>

					<h3>Check</h3>
					<ul>
						<li>
							Your move is illegal if your king would be in check once your potholes on their last round close and the
							potholes next to the Mamdani are repaired.
						</li>
						<li>
							If your own pothole on its last round is the only thing blocking a check and no move fixes it, that is
							checkmate: your next move closes it.
						</li>
						<li>A move that checkmates ends the game at once. No pothole roll follows.</li>
					</ul>

					<h3>Castling, pawns and en passant</h3>
					<p>
						Castling, en passant and promotion work as normal, and a pothole blocks like a piece. You can't castle
						through or onto one, the rook's path over b1 or b8 included. A pawn can't double-step across one. A pothole
						on the en passant square cancels that capture.
					</p>

					<h3>Draws</h3>
					<ul>
						<li><strong>Stalemate:</strong> only if you also have no legal Mamdani move.</li>
						<li>
							<strong>50-move rule:</strong> moving the Mamdani doesn't reset the count. Losing a piece to a pothole
							does; the Mamdani falling doesn't.
						</li>
						<li>
							<strong>Repetition:</strong> a position repeats only if the pieces, the Mamdani, the open potholes and the
							rounds each has left are all the same.
						</li>
						<li>
							<strong>Too little material:</strong> a draw. Running out of time when your opponent can't mate is a draw
							too.
						</li>
					</ul>

					<h3>The clock</h3>
					<ul>
						<li>10 minutes each, plus 5 seconds a move. Your clock starts after your first move.</li>
						<li>Each side has 60 seconds for its first move, or the game is called off with no result.</li>
						<li>The clock pauses while the dice roll.</li>
						<li>A game nobody joins expires.</li>
					</ul>
				</div>
			</details>
		</section>

		<footer>
			<p>
				Based on <a href="https://www.chessvariants.com/boardrules.dir/potholechess.html" target="_blank" rel="noopener"
					>Pot-Hole Chess</a
				> by Peter Spicer and Michael Chamberlain (2001).
			</p>
			<p>
				The Mamdani comes from
				<a href="https://www.instagram.com/reel/Dd8tV_MxEQL/" target="_blank" rel="noopener">The Mamdani Patch</a>
				by
				<a href="https://www.instagram.com/bardelo_bardalini/" target="_blank" rel="noopener">@bardelo_bardalini</a>.
			</p>
			<p>
				Pieces: the mpchess set by
				<a href="https://github.com/chupinmaxime" target="_blank" rel="noopener">Maxime Chupin</a>, GPL-3.0, slightly
				changed.
			</p>
		</footer>
	</article>
</main>

<style>
	main {
		display: grid;
		grid-template-columns: 200px minmax(0, 1fr);
		gap: 64px;
		align-items: start;
		max-width: 1120px;
		margin: 0 auto;
		padding: 56px 32px 80px;
	}
	.toc {
		position: sticky;
		top: 24px;
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	.toc-head {
		margin-bottom: 8px;
		color: var(--text-muted);
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 12px;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	.toc a {
		padding: 8px 0;
		color: var(--text);
		text-decoration: none;
	}
	.toc a:hover {
		color: var(--accent);
	}
	article {
		display: flex;
		flex-direction: column;
		gap: 64px;
		max-width: 760px;
		min-width: 0;
	}
	section {
		display: flex;
		flex-direction: column;
		gap: 14px;
		scroll-margin-top: 24px;
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
		margin: 0;
		font-size: 19px;
		line-height: 1.55;
	}
	h2 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 40px;
		text-transform: uppercase;
	}
	h3 {
		margin: 8px 0 0;
		color: var(--text);
		font-size: 18px;
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
	.note {
		color: var(--text-muted);
		font-size: 15px;
	}
	.played {
		align-self: flex-start;
		color: var(--accent);
		font-weight: 600;
		text-decoration: none;
	}
	.played:hover {
		text-decoration: underline;
		text-underline-offset: 3px;
	}
	.with-figure {
		display: grid;
		grid-template-columns: minmax(0, 1fr) 240px;
		gap: 40px;
		align-items: start;
	}
	.text {
		display: flex;
		flex-direction: column;
		gap: 14px;
	}
	figure {
		display: flex;
		flex-direction: column;
		gap: 8px;
		margin: 0;
	}
	figcaption {
		color: var(--text-muted);
		font-size: 13px;
	}
	.steps {
		display: flex;
		flex-direction: column;
		margin: 0;
		padding: 0;
		list-style: none;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		overflow: hidden;
	}
	.steps li {
		display: grid;
		grid-template-columns: 48px minmax(0, 1fr);
		gap: 12px;
		padding: 16px 20px;
		line-height: 1.5;
	}
	.steps li + li {
		border-top: 1px solid var(--surface-2);
	}
	.num {
		color: var(--accent);
		font-family: var(--font-mono);
		font-weight: 600;
	}
	.table-wrap {
		overflow-x: auto;
		border-radius: 14px;
		background: var(--surface);
	}
	table {
		width: 100%;
		border-collapse: collapse;
		font-size: 15px;
	}
	th,
	td {
		padding: 14px 18px;
		text-align: left;
		vertical-align: top;
	}
	thead th {
		color: var(--text-muted);
		font-weight: 600;
	}
	tbody th {
		color: var(--text);
		font-weight: 600;
	}
	thead th,
	tbody tr:not(:last-child) > * {
		border-bottom: 1px solid var(--surface-2);
	}
	.good {
		color: var(--accent);
	}
	.bad {
		color: var(--hazard-text);
	}
	details {
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		background: var(--surface);
	}
	summary {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 16px;
		padding: 18px 20px;
		cursor: pointer;
		list-style: none;
	}
	summary::-webkit-details-marker {
		display: none;
	}
	summary::after {
		content: '+';
		color: var(--accent);
		font-family: var(--font-mono);
		font-size: 24px;
	}
	details[open] summary::after {
		content: '−';
	}
	summary h2 {
		font-size: 28px;
	}
	.full {
		display: flex;
		flex-direction: column;
		gap: 14px;
		padding: 0 20px 24px;
	}
	.full p,
	.full ul {
		font-size: 16px;
	}
	footer {
		display: flex;
		flex-direction: column;
		gap: 6px;
		padding-top: 24px;
		border-top: 1px solid var(--surface-2);
	}
	footer p {
		color: var(--text-muted);
		font-size: 13px;
	}
	footer a {
		color: var(--text-body);
	}

	/* Phones and narrow windows: one column, the contents as wrapping chips. */
	@media (max-width: 799px) {
		main {
			grid-template-columns: minmax(0, 1fr);
			gap: 32px;
			padding: 32px 16px 56px;
		}
		.toc {
			position: static;
			flex-direction: row;
			flex-wrap: wrap;
			gap: 8px;
		}
		.toc-head {
			flex-basis: 100%;
			margin-bottom: 0;
		}
		.toc a {
			padding: 6px 12px;
			border: 1px solid var(--surface-2);
			border-radius: 16px;
			font-size: 14px;
		}
		article {
			gap: 48px;
		}
		h2 {
			font-size: 34px;
		}
		.with-figure {
			grid-template-columns: minmax(0, 1fr);
			gap: 20px;
		}
		figure {
			max-width: 240px;
		}
		th,
		td {
			padding: 12px;
		}
	}
</style>
