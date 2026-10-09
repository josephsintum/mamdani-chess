<script lang="ts">
	import { onMount } from 'svelte';
	import { MediaQuery } from 'svelte/reactivity';
	import { backOut } from 'svelte/easing';
	import { reducedMotion, setInstant } from '#lib/motion.ts';
	import { dev } from '$app/env';
	import { page } from '$app/state';
	import { afterNavigate, replaceState } from '$app/navigation';
	import Board from '#lib/Board.svelte';
	import Confetti from '#lib/Confetti.svelte';
	import DiceSummary from '#lib/DiceSummary.svelte';
	import DiceTray from '#lib/DiceTray.svelte';
	import GameHelp from '#lib/GameHelp.svelte';
	import MoveLog from '#lib/MoveLog.svelte';
	import MovesSheet, { LANDSCAPE } from '#lib/MovesSheet.svelte';
	import PlayerBar from '#lib/PlayerBar.svelte';
	import RecentMoves from '#lib/RecentMoves.svelte';
	import SoundToggle from '#lib/SoundToggle.svelte';
	import { Animator } from '#lib/animator.svelte.ts';
	import { dicePill, playOutMs, scanOf, type DicePill } from '#lib/dice.ts';
	import { checkSquare, endedHere, matedKing, materialLead, pillFor, repairsShown, resultCardOf, stageAt, tallyOf, wonHere, type Stage } from '#lib/board.ts';
	import { contextOf, endQuip, quipper } from '#lib/catchphrases.ts';
	import { BURST_HOLD_MS, countAt } from '#lib/feel.ts';
	import { applyMove, settlesGuess } from '#lib/pieces.ts';
	import { firstMoveLeft, paused, timeLeft } from '#lib/clock.ts';
	import { notify } from '#lib/toast.ts';
	import { markSeen, seenTips, setTipsOn, tipFor, TIPS, tipsOn } from '#lib/tips.ts';
	import { retryDelay } from '#lib/reconnect.ts';
	import { CORE, LATER, load, loadSoon, loop, play, SPECTATOR_LATER } from '#lib/sound.ts';
	import { endCue, moveSound, turnCues } from '#lib/soundCues.ts';
	import { canShare, shareLink } from '#lib/share.ts';
	import {
		createGame,
		followsRematch,
		gameExists,
		isStale,
		joinNotice,
		opponent,
		showsOffline,
		rematch,
		resign,
		sideName,
		trySendMove,
		type Color,
		type MoveJSON,
		type View
	} from '#lib/game.ts';

	// Codes are upper case; a link typed in lower case still finds the game.
	const code = (page.params.code ?? '').toUpperCase();
	// Arrived from quick match's opponent-found screen, which already said who's who.
	const fromMatch = page.state.matched === true;

	// Dev only: /game/CODE?instant turns every animation off, for fast play-testing.
	const instant = dev && page.url.searchParams.has('instant');
	setInstant(instant);
	const anim = new Animator(instant ? 0 : undefined);
	// Sounds follow the turn as it plays out, never a turn shown at once. The
	// player's own move sounded as they made it.
	anim.onreveal = (v, from, to, jumped) => {
		const own = v.last.find((e) => e.kind === 'moved')?.color === v.you;
		for (const c of turnCues(v, from, to, { skipMove: own, end: !endSounded, jumped })) play(c.sound, c.delayMs);
	};
	// The game's end sound already played: a resignation or a timeout that
	// arrived while a roll was still playing out, which then ends on the
	// finished game's view.
	let endSounded = false;
	// Phones get their own layout (canvas row "Phone game: playtest build"):
	// upright, or on their side, where the board sits left of everything else.
	const phone = new MediaQuery(`(max-width: 639px), ${LANDSCAPE}`);
	const landscape = new MediaQuery(LANDSCAPE);
	// The smallest phones on their side: the column beside the board is
	// about 266 px, so the bars keep only what matters.
	const narrowLand = new MediaQuery(`${LANDSCAPE} and (max-width: 699px)`);
	// Arrived from a rematch: the page that sent us here left a note, so the
	// start-of-game notice says "Rematch" rather than "You joined".
	const REMATCH_NOTE = 'rematch';
	const fromRematch = tookRematchNote(code);
	let sheet: MovesSheet | undefined = $state();
	let view = $derived(anim.view);
	let shown = $derived(anim.shown);
	// Your move, shown before the server confirms it (One Million Chessboards
	// style): the piece glides at once. If the server refuses the move, the
	// guess is dropped and the piece glides back.
	let optimistic = $state.raw<{ seq: number; move: MoveJSON } | null>(null);
	// The guess couldn't reach the server; it is sent again once it can.
	let unsent = $state(false);
	let connected = $state(false);
	// The server's clock minus ours, and the server's time now: the clocks
	// count down from it.
	let offset = 0;
	let serverNow = $state(Date.now());
	let notFound = $state(false);
	let lost = $state(false);
	let error = $state('');
	let busy = $state(false); // a request is on its way
	let confirmResign = $state(false);
	let copyHint = $state('');
	// On phones the start-of-game notice ("… joined") shows in the dice card
	// until the first move, instead of a toast over the opponent's bar.
	let joined = $state('');
	// Phones with a share sheet get Share link first; the rest copy.
	const sharable = canShare(page.url.href);
	// The game ended while this page watched it: the tally counts up, and the
	// winner gets confetti (cleared once it has fallen).
	let endedLive = $state(false);
	let cheer = $state(false);

	// A tip during your first games (tips.ts), once the turn has played out.
	// At the start it waits a moment, behind the "… joined" notice.
	const FIRST_TIP_MS = 1500;
	let tipTimer: ReturnType<typeof setTimeout> | undefined;
	let lastTipSeq: number | undefined; // the turn the last tip showed on
	function scheduleTip(next: View) {
		clearTimeout(tipTimer);
		if (instant || !tipsOn()) return;
		const tip = tipFor(next, seenTips(), anim.animated, lastTipSeq);
		if (!tip) return;
		tipTimer = setTimeout(
			() => {
				markSeen(tip);
				lastTipSeq = next.seq;
				notify.info(TIPS[tip], { id: 'tip', duration: 6000, action: { label: 'No more tips', onClick: () => setTipsOn(false) } });
			},
			anim.animated ? playOutMs(next.last) : FIRST_TIP_MS
		);
	}

	function receive(next: View) {
		if (isStale(anim.view, next)) return; // a slow reply, overtaken by the stream
		error = '';
		offset = next.clock.now - Date.now();
		serverNow = next.clock.now;
		if (settlesGuess(optimistic, next)) {
			optimistic = null;
			unsent = false;
		}
		if (next.result) confirmResign = false; // the game ended before you chose
		const prev = anim.view;
		if (endedHere(prev, next)) {
			endedLive = true;
			cheer = wonHere(prev, next) && !instant && !reducedMotion();
			// A resignation or a timeout: no turn plays out, so its sound is here.
			const ending = next.seq === prev?.seq ? endCue(next) : null;
			if (ending) {
				play(ending);
				endSounded = true;
			}
		}
		// The rest of the game's clips, once the page knows who's watching: a
		// finished game can still get a rematch offer, and a spectator hears
		// no stings.
		if (!prev) loadSoon(next.you === 'spectator' ? SPECTATOR_LATER : LATER);
		const notice = joinNotice(prev, next, fromMatch, fromRematch);
		// The game starts: the same moments as the notice. One seen live always
		// sounds; the first state a page gets doesn't when the page was
		// reloaded. Just after the page loads the clip may still be on its way.
		if (notice && (prev !== null || !reloadedHere())) play('notify', 0, { wait: 1500 });
		const offer = next.rematch.offer;
		if (prev && offer && offer !== prev.rematch.offer && next.you !== 'spectator' && offer !== next.you) play('challenge');
		if (notice && phone.current) joined = notice;
		else if (notice) notify.info(notice, { id: 'join' });
		anim.receive(next, { hidden: document.hidden });
		if (prev?.seq !== next.seq || prev.status !== next.status) scheduleTip(next);
		if (unsent) void resend();
		// A rematch accepted while this page is open: players go to it. Replace,
		// so Back returns to where they were before, not to a page that would
		// forward them again. A full load gives the new game a fresh stream.
		if (followsRematch(prev, next)) goToRematch(next.rematch.code, true);
	}

	function goToRematch(to: string | undefined, replace = false) {
		if (!to) return;
		leaveRematchNote(to);
		if (replace) location.replace(`/game/${to}`);
		else location.assign(`/game/${to}`);
	}
	function leaveRematchNote(to: string) {
		try {
			sessionStorage.setItem(REMATCH_NOTE, to);
		} catch {
			// No storage (a private window): the notice just says "You joined".
		}
	}
	function tookRematchNote(here: string): boolean {
		try {
			const to = sessionStorage.getItem(REMATCH_NOTE);
			sessionStorage.removeItem(REMATCH_NOTE);
			return to === here;
		} catch {
			return false;
		}
	}

	// Whether this page is the one the browser reloaded: a reload shows the
	// game as it is, silently. The navigation entry covers the whole tab, so
	// a game page reached later inside the app (afterNavigate's 'link' or
	// 'goto', not 'enter') wasn't reloaded.
	let entered = false;
	afterNavigate(({ type }) => (entered = type === 'enter'));
	function reloadedHere(): boolean {
		const nav = performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined;
		return entered && nav?.type === 'reload';
	}

	// Your clock beeps once a game, as it drops under 10 s while it runs.
	// lastLeft is your time at the last reading while it ran; a page opened
	// with less than 10 s left never saw it cross.
	const LOW_MS = 10_000;
	let lastLeft: number | undefined;
	let lowBeeped = '';
	function checkLowTime(v: View | null) {
		if (!v || v.status !== 'playing' || v.you === 'spectator' || v.clock.running !== v.you || paused(v.clock, serverNow)) return;
		const left = timeLeft(v.clock, v.you, serverNow);
		if (left < LOW_MS && lastLeft !== undefined && lastLeft >= LOW_MS && lowBeeped !== v.code) {
			lowBeeped = v.code;
			play('low');
		}
		lastLeft = left;
	}

	onMount(() => {
		if (page.params.code !== code) void replaceState(`/game/${code}${location.search}`, page.state);
		load(CORE);
		// Every stream this page has open. A new one replaces the others only
		// once it is up, so the server never sees the player leave in between
		// (which would withdraw a rematch offer, say).
		const streams = new Set<EventSource>();
		let generation = 0; // the latest connect(); older retries do nothing
		let retry: ReturnType<typeof setTimeout> | undefined;
		let disposed = false; // the page has gone: open nothing more
		let askedOnce = false; // the first failure is checked at once: a mistyped code shouldn't wait
		let failures = 0; // refused streams in a row, for the backoff; a stream that opens resets it
		const connect = () => {
			if (disposed) return;
			clearTimeout(retry);
			const gen = ++generation;
			const es = new EventSource(`/api/games/${code}/stream${view ? '?again=1' : ''}`);
			streams.add(es);
			es.onopen = () => {
				if (gen !== generation) {
					es.close(); // a newer stream is on its way
					streams.delete(es);
					return;
				}
				connected = true;
				failures = 0;
				for (const old of streams) {
					if (old !== es) {
						old.close();
						streams.delete(old);
					}
				}
			};
			es.onerror = () => {
				if (gen !== generation) return;
				connected = false;
				// Network errors reconnect by themselves. An error status (a
				// proxy's 502 while the server restarts, or a 404) closes the
				// stream for good: ask whether the game still exists, and if it
				// might, open a new stream.
				if (es.readyState !== EventSource.CLOSED) return;
				streams.delete(es);
				retry = setTimeout(async () => {
					const exists = await gameExists(code);
					if (gen !== generation || disposed) return; // something newer took over
					if (exists) connect();
					else if (view) lost = true;
					else notFound = true;
				}, view || askedOnce ? retryDelay(failures++) : 0);
				askedOnce = true;
			};
			es.addEventListener('state', (e) => receive(JSON.parse(e.data) as View));
		};
		connect();
		// The browser knows at once when the device loses its network; the
		// stream can take much longer to notice. Show it, and when the network
		// is back, start a fresh stream (the old one may be dead without
		// knowing it): its first state resends a move made meanwhile.
		const goneOffline = () => (connected = false);
		const backOnline = () => connect();
		window.addEventListener('offline', goneOffline);
		window.addEventListener('online', backOnline);
		// Only a game in progress has a running clock or a first-move deadline.
		const tick = setInterval(() => {
			if (view?.status === 'playing') serverNow = Date.now() + offset;
			checkLowTime(view);
		}, 100);
		// Coming back to a tab mid-animation jumps to the end of the roll.
		const finishOnReturn = () => {
			if (!document.hidden) anim.finish();
		};
		document.addEventListener('visibilitychange', finishOnReturn);
		return () => {
			disposed = true;
			for (const es of streams) es.close();
			window.removeEventListener('offline', goneOffline);
			window.removeEventListener('online', backOnline);
			clearTimeout(retry);
			clearTimeout(resendTimer);
			clearTimeout(tipTimer);
			clearInterval(tick);
			anim.stop();
			document.removeEventListener('visibilitychange', finishOnReturn);
		};
	});

	let animating = $derived(anim.animating);
	// The guess, while it is for the turn on screen.
	let guess = $derived(optimistic && optimistic.seq === view?.seq ? optimistic : null);
	let stage = $derived.by(() => {
		if (!view) return null;
		const base = stageAt(view, shown);
		if (!guess) return base;
		return { ...base, ...applyMove(base, guess.move) };
	});
	let checked = $derived(view && stage ? checkSquare(view, stage, { animating, guessing: !!guess }) : '');
	let savedSquare = $derived(view?.last.find((e, i) => e.kind === 'saving_roll' && e.saved && i < shown)?.sq ?? '');
	// The Mamdani's repairs, celebrated only on a turn that is playing out (never after a reload).
	let repairs = $derived(view && anim.animated && !instant ? repairsShown(view, shown) : []);
	// The dice on the board: the pill in the corner, the file and rank dice's
	// scan, and the target blinking on a re-roll. Only on a turn that plays out.
	let pill = $derived(view && anim.animated && !instant ? dicePill(view, shown) : null);
	let scan = $derived(view && anim.animated && !instant ? scanOf(view, shown) : null);
	let reroll = $derived(!!view && animating && view.last[shown - 1]?.kind === 'reroll');
	// A speech bubble for a big moment: only on a turn that is playing out,
	// like the repairs. A game that ended here gets its own line once the dice
	// stop: a win on time, or a mate by a pothole roll.
	const say = quipper();
	let quip = $derived.by(() => {
		if (!view || instant) return null;
		const end = endedLive && !animating ? endQuip(view) : null;
		if (end) return say(view.last, view.seq, shown, { end, key: `${code}:end`, winner: view.result?.winner === you });
		return anim.animated ? say(view.last, view.seq, shown, undefined, contextOf(view)) : null;
	});
	// The checkmate burst, once the dice stop, on a turn that played out here.
	let mated = $derived.by(() => {
		const sq = view && anim.animated && !instant && !animating ? matedKing(view) : '';
		return sq ? { sq, key: `${code}:mate` } : null;
	});
	let you = $derived(view?.you ?? 'spectator');
	let bottom = $derived<Color>(you === 'black' ? 'black' : 'white');
	let top = $derived(opponent(bottom));
	let lastMove = $derived.by(() => {
		if (guess) return { from: guess.move.from, to: guess.move.to };
		const m = view?.last.find((e) => e.kind === 'moved');
		return m?.from && m?.to ? { from: m.from, to: m.to } : null;
	});
	let playing = $derived(view?.status === 'playing');
	let isPlayer = $derived(you === 'white' || you === 'black');

	async function move(m: MoveJSON) {
		if (!view || busy) return;
		busy = true;
		play(stage ? moveSound(stage.board, m) : 'move');
		optimistic = { seq: view.seq, move: m };
		const sent = await trySendMove(code, m, view.seq);
		busy = false;
		if (sent === 'unsent') {
			// No connection (a train in a tunnel, say): keep the move on the
			// board and send it again when the server can be reached.
			unsent = true;
			retryLater();
		} else if (sent !== 'sent') {
			optimistic = null; // refused: glide back
			play('error');
			if (sent.state) receive(sent.state); // resync at once (this clears error)
			error = sent.refused;
		}
	}

	let resendTimer: ReturnType<typeof setTimeout> | undefined;
	function retryLater() {
		clearTimeout(resendTimer);
		resendTimer = setTimeout(resend, 1500);
	}

	// Sends an unsent move again. The same seq means the server refuses a
	// copy of a move it already has; then the stream's state settles it.
	async function resend() {
		const pending = guess;
		if (!unsent || !pending) return;
		if (!connected) return retryLater();
		unsent = false;
		const sent = await trySendMove(code, pending.move, pending.seq);
		if (sent === 'unsent') {
			unsent = true;
			retryLater();
		} else if (sent !== 'sent') {
			// Usually the server already has the move; its state settles it.
			if (optimistic === pending) optimistic = null;
			if (sent.state) receive(sent.state);
		}
	}

	async function offerRematch(decline = false) {
		busy = true;
		error = (await rematch(code, decline).catch(() => 'Could not reach the server. Try again.')) ?? '';
		busy = false;
	}

	async function doResign() {
		busy = true;
		error = (await resign(code)) ?? '';
		busy = false;
		confirmResign = false;
	}

	let hintTimer: ReturnType<typeof setTimeout> | undefined;

	async function copyLink() {
		try {
			await navigator.clipboard.writeText(page.url.href);
			copyHint = 'Link copied';
		} catch {
			// No clipboard on plain-http addresses: select the link instead.
			const link = document.getElementById('link');
			if (link instanceof HTMLInputElement) link.select();
			if (!phone.current) copyHint = 'Press Ctrl+C (⌘C on a Mac) to copy';
			else copyHint = link ? 'Tap and hold the link to copy it' : 'Copy the address bar to share';
		}
		// The phone's header shows the hint in place of the code, briefly.
		clearTimeout(hintTimer);
		hintTimer = setTimeout(() => (copyHint = ''), 2500);
	}

	async function share() {
		if ((await shareLink(page.url.href)) === 'failed') void copyLink();
	}

	async function newGame() {
		busy = true;
		try {
			// A full page load gives the new game a fresh stream.
			location.href = `/game/${await createGame()}`;
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			busy = false;
		}
	}

	let pausedForDice = $derived(!!view && paused(view.clock, serverNow));
	let firstMove = $derived(view ? firstMoveLeft(view.clock, serverNow) : null);

	let status = $derived.by(() => {
		if (!view) return '';
		if (view.status === 'waiting')
			return you === 'white' ? 'Waiting for your friend to open the link…' : 'Waiting for White’s friend to join…';
		if (view.result) return animating ? 'Last move played…' : 'Game over';
		if (unsent) return connected ? 'Sending your move…' : 'Reconnecting — your move will be sent';
		if (animating) return 'Dice are rolling…';
		const side = sideName(view.turn);
		const yours = view.turn === you;
		let text = yours ? 'Your move' : `${side} to move`;
		// Name who is in check: a bare "check!" was read as the reader's own king.
		if (view.check) text += yours ? ' — you’re in check' : ` — ${side} is in check`;
		if (firstMove !== null) text += ` · ${Math.ceil(firstMove / 1000)}s to make the first move`;
		return text;
	});

	// Phones have no visible status line, so the first-move countdown rides
	// on the pill of the side that has to move.
	function phonePill(p: { text: string }, c: Color): string {
		if (!p.text || firstMove === null || view?.turn !== c) return p.text;
		const left = `${Math.ceil(firstMove / 1000)}s`;
		return narrowLand.current ? left : `${p.text} · ${left}`; // a name needs the room
	}

	// The phone's game-over bar: what its rematch button does, if anything.
	let rematchAction = $derived.by((): { kind: 'go' | 'offer' | 'offered' | 'answer'; label: string } | null => {
		const r = view?.rematch;
		if (!view || !r || !resultCard || view.result?.reason === 'expired') return null;
		if (r.code) return { kind: 'go', label: isPlayer ? 'Go to rematch' : 'Watch rematch' };
		if (!isPlayer) return null;
		if (r.offer && r.offer !== you) return { kind: 'answer', label: '' };
		if (r.offer === you) return { kind: 'offered', label: 'Offered…' };
		return { kind: 'offer', label: 'Rematch' };
	});
	// On phones a rematch offer or decline replaces the result's detail line,
	// so the card keeps its height and the page never scrolls.
	let rematchNote = $derived.by(() => {
		const r = view?.rematch;
		if (!r || r.code || !isPlayer) return '';
		if (r.offer && r.offer !== you) return 'Your opponent wants a rematch.';
		if (r.declined) return 'Rematch declined.';
		return '';
	});

	// The result card flips in, with a slight overshoot. After a checkmate it
	// first waits out the burst (the card would cover it), and so do the tally
	// and the confetti.
	const FLIP_MS = 420;
	let hold = $derived(mated ? BURST_HOLD_MS : 0);
	function flipIn(_node: Element, { lift = '' }: { lift?: string } = {}) {
		const wait = hold;
		return {
			duration: reducedMotion() ? 0 : wait + FLIP_MS,
			css: (t: number) => {
				const p = Math.max(0, (t * (wait + FLIP_MS) - wait) / FLIP_MS);
				return `transform: ${lift} perspective(700px) rotateX(${-75 * (1 - backOut(p))}deg); opacity: ${Math.min(1, p * 2)}`;
			}
		};
	}

	// The tally's numbers count up one at a time once the card is in, each
	// landing with a pop. A game opened after it ended shows them at once, and
	// each number counts once: a later update to the finished game (a rematch
	// offer, the opponent leaving) redraws the tally, and shows it as it is.
	const COUNT_MS = 250;
	const COUNT_GAP_MS = 150;
	const counted: boolean[] = [];
	function countUp(value: number, i: number) {
		return (node: HTMLElement) => {
			node.textContent = String(value);
			if (!endedLive || instant || reducedMotion() || counted[i]) return;
			counted[i] = true;
			node.textContent = '0';
			let frame = 0;
			const timer = setTimeout(() => {
				const start = performance.now();
				frame = requestAnimationFrame(function step(now) {
					const t = (now - start) / COUNT_MS;
					node.textContent = String(countAt(value, t));
					if (t < 1) frame = requestAnimationFrame(step);
					else node.classList.add('pop');
				});
			}, hold + FLIP_MS + i * (COUNT_MS + COUNT_GAP_MS));
			return () => {
				clearTimeout(timer);
				cancelAnimationFrame(frame);
			};
		};
	}

	let resultCard = $derived(view && !animating ? resultCardOf(view) : null);
</script>

<svelte:head>
	<title>Game {code} · Mamdani Chess</title>
</svelte:head>

<!-- A player's bar; the phone's is compact, which ignores pausedForDice. -->
{#snippet bar(view: View, stage: Stage, c: Color, compact: boolean)}
	{@const p = pillFor(view, c, animating)}
	<PlayerBar
		{compact}
		color={c}
		name={view.players[c]}
		you={you === c}
		lost={stage.lost[c]}
		taken={view.taken[c]}
		lead={materialLead(stage.board)[c]}
		pill={compact ? phonePill(p, c) : p.text}
		pillTone={p.tone}
		toMove={playing && view.turn === c && !animating}
		clockMs={timeLeft(view.clock, c, serverNow)}
		seq={view.seq}
		ticking={view.clock.running === c && !pausedForDice}
		pausedForDice={playing && view.turn === c && pausedForDice}
		offline={showsOffline(view, c)}
	/>
{/snippet}

{#snippet board(view: View, stage: Stage, dim: boolean, cornerPill: DicePill | null)}
	<Board
		{stage}
		legal={view.legal}
		{lastMove}
		flipped={bottom === 'black'}
		interactive={!animating && !busy && !optimistic && playing && view.you !== 'spectator'}
		{dim}
		check={checked}
		saved={savedSquare}
		{repairs}
		{quip}
		{mated}
		pill={cornerPill}
		{scan}
		{reroll}
		onmove={move}
		onrefuse={() => play('error')}
	/>
{/snippet}

{#snippet flag()}
	<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M4 22V4"></path><path d="M4 4h12l-2 4 2 4H4"></path></svg>
{/snippet}

{#if phone.current && view && stage && !notFound}
	<div class="phone" class:land={landscape.current}>
		<header class="ph-head">
			<a href="/" class="ph-logo">Mamdani Chess</a>
			<span class="ph-meta">
				{#if !connected && !lost}<span class="ph-chip warn">Reconnecting…</span>{/if}
				{#if you === 'spectator'}<span class="ph-chip">Watching</span>{/if}
				<span class="ph-code">{copyHint && view.status !== 'waiting' ? copyHint : code}</span>
				<SoundToggle compact />
				<GameHelp compact />
				<button type="button" class="ph-icon" aria-label="Copy game link" onclick={copyLink}>
					<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="9" y="9" width="12" height="12" rx="2"></rect><path d="M5 15V5a2 2 0 0 1 2-2h10"></path></svg>
				</button>
			</span>
		</header>
		{#if lost}
			<p class="ph-lost" role="alert">
				This game is no longer on the server. <a href="/">Start a new one</a>.
			</p>
		{/if}
		<!-- The pills and the dice card show this; screen readers hear it. -->
		<p class="sr-only" aria-live="polite">{status}</p>

		<div class="ph-play">
			<div class="ph-recent"><RecentMoves log={view.log} rolling={animating} onclick={(e) => sheet?.open(e.currentTarget)} /></div>
			<div class="ph-top">{@render bar(view, stage, top, true)}</div>
			<div class="ph-board">{@render board(view, stage, !!resultCard || view.status === 'waiting', null)}</div>
			<div class="ph-me">{@render bar(view, stage, bottom, true)}</div>
		</div>

		<!-- The card and the bottom bar keep one height whatever shows, so the
		     board never moves when a card changes. -->
		<div class="ph-bottom">
		{#if view.status === 'waiting' && you === 'white'}
			<section class="ph-card" aria-label="Invite a friend" {@attach () => loop('waiting')}>
				{#if sharable}
					<span class="ph-title">Send this link to your friend</span>
					<div class="ph-row">
						<button type="button" class="primary share" onclick={share}>
							<svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M12 15V3" /><path d="m7 8 5-5 5 5" /><path d="M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-7" /></svg>
							Share link
						</button>
						<button type="button" class="outline" onclick={copyLink}>Copy link</button>
					</div>
				{:else}
					<label for="link" class="ph-title">Send this link to your friend</label>
					<div class="ph-row">
						<input id="link" readonly value={page.url.href} />
						<button type="button" class="primary" onclick={copyLink}>Copy link</button>
					</div>
				{/if}
				{#if copyHint}<span class="muted">{copyHint}</span>{/if}
			</section>
		{:else if confirmResign}
			<section class="ph-card danger-line" aria-label="Resign">
				<span class="ph-title">Resign this game? <span class="muted">{sideName(top)} wins.</span></span>
				<div class="ph-two">
					<button type="button" class="outline" onclick={() => (confirmResign = false)}>Keep playing</button>
					<button type="button" class="danger" onclick={doResign} disabled={busy}>Yes, resign</button>
				</div>
			</section>
		{:else if resultCard}
			<section class="ph-card accent-line" class:quiet={resultCard.lost} role="status" aria-label="Game over" in:flipIn>
				<span class="ph-result"><span class="ph-headline">{resultCard.title}</span><span class="ph-kicker">{resultCard.kicker}</span></span>
				<span class="ph-detail">{rematchNote || resultCard.detail}</span>
				<span class="ph-tally">
					{#each tallyOf(view) as row, i (row.label)}{#if row.short}<span><b {@attach countUp(row.value, i)}></b> {row.short}</span>{/if}{/each}
				</span>
			</section>
		{:else if error}
			<p class="ph-card ph-error" role="alert">{error}</p>
		{:else if unsent}
			<p class="ph-card ph-detail" role="status">{status}</p>
		{:else if joined && playing && view.seq === 0}
			{@const [head, ...rest] = joined.split(' · ')}
			<p class="ph-card notice" role="status"><span class="ph-notice-head">{head}</span><span>{rest.join(' · ')}</span></p>
		{:else}
			<DiceSummary {view} {shown} />
		{/if}

		{#if view.status !== 'waiting' && !confirmResign}
			<nav class="ph-nav" aria-label="Game actions" class:single={!resultCard && !(isPlayer && playing) && you !== 'spectator'} class:three={!!rematchAction}>
				{#if rematchAction}
					{#if rematchAction.kind === 'answer'}
						<button type="button" class="primary" onclick={() => offerRematch()} disabled={busy}>Accept</button>
						<button type="button" class="outline" onclick={() => offerRematch(true)} disabled={busy}>Decline</button>
					{:else}
						{#if rematchAction.kind === 'go'}
							<button type="button" class="primary" onclick={() => goToRematch(view.rematch.code)}>{rematchAction.label}</button>
						{:else}
							<button type="button" class="primary" onclick={() => offerRematch()} disabled={busy || rematchAction.kind === 'offered'}
								>{rematchAction.label}</button
							>
						{/if}
						<button type="button" class="outline" onclick={newGame} disabled={busy}>New game</button>
					{/if}
				{:else if resultCard}<button type="button" class="primary" onclick={newGame} disabled={busy}>New game</button>{/if}
				<button type="button" class="solid" onclick={(e) => sheet?.open(e.currentTarget)}>
					<svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true"><path d="M8 6h13"></path><path d="M8 12h13"></path><path d="M8 18h13"></path><path d="M3 6h.01"></path><path d="M3 12h.01"></path><path d="M3 18h.01"></path></svg>
					<!-- A literal, so the leading space survives (Svelte trims it from plain text). -->
					<span>Moves{#if !rematchAction}<span class="and-rolls">{' and rolls'}</span>{/if}</span>
				</button>
				{#if you === 'spectator' && !resultCard}
					<a class="primary" href="/">Play a game</a>
				{/if}
				{#if !resultCard && isPlayer && playing}
					<button type="button" class="outline" onclick={() => (confirmResign = true)} disabled={busy}>
						{@render flag()}
						Resign
					</button>
				{/if}
			</nav>
		{/if}
		</div>
	</div>
	<MovesSheet bind:this={sheet} {view} {shown} rolling={animating} />
{:else}
<main>
	<header>
		<a href="/" class="logo">Mamdani Chess</a>
		<span class="code">{code}</span>
		{#if view && !connected && !lost}<span class="warn">Reconnecting…</span>{/if}
		<span class="help-slot"><SoundToggle /><GameHelp /></span>
	</header>

	{#if lost}
		<p class="error" role="alert">
			This game is no longer on the server. <a href="/">Start a new one</a>.
		</p>
	{/if}

	{#if notFound}
		<section class="missing" role="alert">
			<h1>Game not found</h1>
			<p>
				No game has the code <span class="code">{code}</span>. Check the code, or the game may have ended more than a
				day ago.
			</p>
			<a class="home-link" href="/">Back to the home page</a>
		</section>
	{:else if !view || !stage}
		<p class="connecting">Connecting…</p>
	{:else}
		<p class="status" aria-live="polite">
			{status}
			{#if you === 'spectator'}<span class="muted">(watching)</span>{/if}
		</p>

		{#if view.status === 'waiting' && you === 'white'}
			<div class="share" {@attach () => loop('waiting')}>
				<label for="link">Send this link to your friend</label>
				<div class="share-row">
					<input id="link" readonly value={page.url.href} />
					<button class="primary" onclick={copyLink}>Copy link</button>
				</div>
				{#if copyHint}<span class="muted">{copyHint}</span>{/if}
			</div>
		{/if}

		<div class="layout">
			<div class="log-col">
				<MoveLog log={view.log} rolling={animating} />
			</div>

			<div class="board-col">
				{@render bar(view, stage, top, false)}
				<div class="board-wrap">
					{@render board(view, stage, !!resultCard, pill)}
					{#if resultCard}
						<div class="result" class:won={resultCard.title === 'You win'} class:lost={resultCard.lost} role="status" in:flipIn={{ lift: 'translate(-50%, -50%)' }}>
							<span class="kicker">{resultCard.kicker}</span>
							<h1>{resultCard.title}</h1>
							<p>{resultCard.detail}</p>
						</div>
					{/if}
				</div>
				{@render bar(view, stage, bottom, false)}
			</div>

			<div class="side-col">
				<DiceTray {view} {shown} />
				{#if error}<p class="error" role="alert">{error}</p>{/if}

				{#if resultCard}
					<dl class="stats">
						{#each tallyOf(view) as row, i (row.label)}
							<div><dt>{row.label}</dt><dd {@attach countUp(row.value, i)}></dd></div>
						{/each}
						{#if view.stats.mamdaniFell}<div><dt>The Mamdani</dt><dd>fell in</dd></div>{/if}
					</dl>
					{#if isPlayer && view.result?.reason !== 'expired'}
						{@const offer = view.rematch.offer}
						<div class="actions" aria-live="polite">
							{#if view.rematch.code}
								<a class="primary" href={`/game/${view.rematch.code}`} data-sveltekit-reload onclick={() => leaveRematchNote(view.rematch.code ?? '')}>Go to the rematch</a>
							{:else if offer && offer !== you}
								<span class="note">Your opponent wants a rematch.</span>
								<button class="primary" onclick={() => offerRematch()} disabled={busy}>Accept</button>
								<button class="outline" onclick={() => offerRematch(true)} disabled={busy}>Decline</button>
							{:else if offer === you}
								<button class="primary" disabled>Rematch offered…</button>
							{:else}
								{#if view.rematch.declined}<span class="note">Rematch declined.</span>{/if}
								<button class="primary" onclick={() => offerRematch()} disabled={busy}>Rematch</button>
							{/if}
						</div>
					{:else if view.rematch.code}
						<a class="primary" href={`/game/${view.rematch.code}`} data-sveltekit-reload>Watch rematch</a>
					{/if}
					<div class="actions">
						<button class={isPlayer ? 'outline' : 'primary'} onclick={newGame} disabled={busy}>New game</button>
						<a class="outline" href="/">Home</a>
					</div>
				{:else if isPlayer && playing}
					{#if confirmResign}
						<div class="confirm" role="group" aria-label="Confirm resignation">
							<span>Resign this game?</span>
							<button class="danger" onclick={doResign} disabled={busy}>Yes, resign</button>
							<button class="outline" onclick={() => (confirmResign = false)}>Keep playing</button>
						</div>
					{:else}
						<button class="outline resign" onclick={() => (confirmResign = true)} disabled={busy}>
							{@render flag()}
							Resign
						</button>
					{/if}
				{/if}
			</div>
		</div>
	{/if}
</main>
{/if}
{#if cheer && resultCard}<Confetti delay={hold} ondone={() => (cheer = false)} />{/if}

<style>
	/* Phone layout (under 640px): one screen, no scrolling. */
	.phone {
		display: flex;
		flex-direction: column;
		height: 100vh;
		height: 100dvh;
		overflow: hidden;
	}
	.ph-head {
		display: flex;
		align-items: center;
		justify-content: space-between;
		flex-shrink: 0;
		height: 48px;
		padding: 0 4px 0 12px;
	}
	.ph-logo {
		flex-shrink: 0;
		white-space: nowrap;
		font-family: var(--font-display);
		font-size: 20px;
		font-weight: 800;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--text);
		text-decoration: none;
	}
	.ph-meta {
		display: flex;
		align-items: center;
		gap: 6px;
		min-width: 0;
	}
	.ph-code {
		overflow: hidden;
		font-family: var(--font-mono);
		font-size: 14px;
		font-weight: 600;
		white-space: nowrap;
		text-overflow: ellipsis;
		color: var(--text-muted);
	}
	.ph-chip {
		padding: 2px 8px;
		border: 1px solid var(--line);
		border-radius: 999px;
		font-size: 12px;
		color: var(--text-muted);
	}
	.ph-chip.warn {
		border-color: var(--hazard);
		color: var(--hazard-text);
	}
	.phone .ph-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 44px;
		height: 44px;
		padding: 0;
		border: 0;
		background: none;
		color: var(--text-body);
		cursor: pointer;
	}
	.ph-lost {
		margin: 0 8px 4px;
		font-size: 14px;
		color: var(--hazard-text);
	}
	/* Bars and board take the space left over; the board is as big as fits. */
	.ph-play {
		display: flex;
		flex-direction: column;
		/* Spare height on tall phones gathers at the top, under the header,
		   so the board sits just above the card and buttons, in thumb reach. */
		justify-content: flex-end;
		flex: 1 1 auto;
		min-height: 0;
		container-type: size;
	}
	/* The latest moves fill the spare height under the header (RecentMoves
	   hides when not even a row fits), so the board stays in thumb reach. */
	.ph-recent {
		flex: 1 1 0;
		min-height: 0;
		container: recent / size;
	}
	.land .ph-recent {
		display: none;
	}
	.ph-board {
		width: min(calc(100cqw - 16px), calc(100cqh - 92px));
		margin: 2px auto;
	}
	.ph-card {
		display: flex;
		flex-direction: column;
		gap: 10px;
		flex-shrink: 0;
		margin: 6px 8px 0;
		padding: 12px;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 12px;
	}
	/* The result card: tighter, so its three lines (result, detail, tally)
	   fit the bottom block's height and the board doesn't move. */
	.ph-card.accent-line {
		gap: 2px;
		padding: 6px 12px;
		border-color: var(--accent-line);
	}
	.ph-card.danger-line {
		border-color: var(--hazard);
	}
	/* The loser's result: no yellow. */
	.ph-card.accent-line.quiet {
		border-color: var(--line);
	}
	.quiet .ph-headline {
		color: var(--text);
	}
	/* "… joined": in the dice card's place until the first move. */
	.ph-card.notice {
		gap: 2px;
		border-color: var(--accent-line);
		background: var(--accent-wash);
		font-size: 14px;
	}
	.ph-notice-head {
		font-family: var(--font-display);
		font-size: 22px;
		font-weight: 800;
		letter-spacing: 0.02em;
		text-transform: uppercase;
		color: var(--text);
	}
	.ph-title {
		font-weight: 600;
		color: var(--text);
	}
	.ph-row {
		display: flex;
		gap: 8px;
	}
	.ph-row input {
		flex: 1;
		min-width: 0;
		height: 44px;
		box-sizing: border-box;
		padding: 0 10px;
		border: 1px solid var(--line);
		border-radius: 10px;
		background: var(--bg);
		color: var(--text-body);
		font-family: var(--font-mono);
		font-size: 13px;
	}
	.ph-row .primary {
		flex-grow: 0;
	}
	.ph-row .share {
		flex-grow: 1;
	}
	.ph-two {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
	}
	.ph-result {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 8px;
	}
	.ph-headline {
		font-family: var(--font-display);
		font-size: 28px;
		font-weight: 800;
		line-height: 1;
		text-transform: uppercase;
		color: var(--accent);
	}
	.ph-kicker {
		font-family: var(--font-mono);
		font-size: 12px;
		font-weight: 600;
		letter-spacing: 0.04em;
		text-transform: uppercase;
		color: var(--text-muted);
	}
	.ph-detail {
		font-size: 14px;
	}
	/* The tally, one line under the result. */
	.ph-tally {
		overflow: hidden;
		line-height: 1.1;
		font-family: var(--font-mono);
		font-size: 11px;
		white-space: nowrap;
		text-overflow: ellipsis;
		color: var(--text-muted);
	}
	/* Too narrow for it: the headline already wraps. The moves sheet has the numbers. */
	@media (max-width: 359px) {
		.ph-tally {
			display: none;
		}
	}
	.ph-tally > span + span::before {
		content: ' · ';
	}
	.ph-tally b {
		display: inline-block;
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: var(--text);
	}
	/* The result card keeps one line, so it fits the bottom block's height
	   and the board doesn't move at game over. */
	.ph-card.accent-line .ph-detail,
	.ph-card.accent-line .ph-kicker {
		overflow: hidden;
		white-space: nowrap;
		text-overflow: ellipsis;
	}
	.ph-card.accent-line .ph-detail {
		line-height: 1.25;
	}
	.ph-error {
		font-size: 14px;
		color: var(--hazard-text);
	}
	.ph-bottom {
		display: flex;
		flex-direction: column;
		flex-shrink: 0;
		gap: 8px;
		/* The height of the playing state (dice card and bottom bar), which
		   every other state fits in. */
		min-height: calc(143px + max(12px, env(safe-area-inset-bottom)));
	}
	.ph-nav {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 8px;
		flex-shrink: 0;
		margin-top: auto;
		padding: 8px 8px max(12px, env(safe-area-inset-bottom));
		border-top: 1px solid var(--surface-2);
	}
	.ph-nav.single {
		grid-template-columns: 1fr;
	}
	.ph-nav.three {
		grid-template-columns: repeat(3, minmax(0, 1fr));
	}
	.phone button {
		display: flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 44px;
		padding: 0 14px;
		border-radius: 10px;
		font: inherit;
		font-weight: 600;
		cursor: pointer;
	}
	.phone .solid {
		border: 0;
		background: var(--surface-2);
		color: var(--text);
	}
	.phone .outline {
		border: 1px solid var(--line);
		background: none;
		color: var(--text);
	}
	.phone .primary {
		border: 0;
		background: var(--accent);
		color: var(--accent-text);
	}
	.phone .danger {
		border: 0;
		background: var(--hazard);
		color: var(--accent-text);
	}
	/* A link styled as a bottom-bar button ("Play a game"). */
	.ph-nav a {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 44px;
		border-radius: 10px;
		font-weight: 600;
		text-decoration: none;
	}
	/* The narrowest phones: "Moves", so the button keeps one line. */
	@media (max-width: 379px) {
		.and-rolls {
			display: none;
		}
	}
	/* A phone on its side: the board on the left at full height; the header,
	   the players, the card and the buttons in a column beside it. */
	.phone.land {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		grid-template-rows: auto auto 1fr auto auto;
		grid-template-areas: 'board head' 'board top' 'board card' 'board me' 'board nav';
		column-gap: 14px;
		padding: 0 12px 0 8px;
	}
	.land .ph-play,
	.land .ph-bottom {
		display: contents;
	}
	.land .ph-head {
		grid-area: head;
		height: 44px;
		padding: 0;
	}
	.land .ph-top {
		grid-area: top;
	}
	.land .ph-me {
		grid-area: me;
	}
	/* As tall as the screen, but leaving the column at least 266 px, so a
	   name, the Waiting pill and the clock fit side by side. */
	.land .ph-board {
		grid-area: board;
		align-self: center;
		width: min(calc(100dvh - 16px), calc(100vw - 300px));
		margin: 0;
	}
	.land .ph-bottom > :global(:not(.ph-nav)) {
		grid-area: card;
		align-self: center;
		margin: 0;
	}
	.land .ph-nav {
		grid-area: nav;
		margin: 0;
		padding: 4px 0 8px;
		border-top: 0;
	}
	/* The smallest phones on their side: the header's icons narrow so the
	   code still shows, and "Moves" keeps one line. */
	@media (max-width: 699px) {
		.land .ph-code {
			flex-shrink: 0;
		}
		.land .ph-icon,
		.land :global(.help.compact) {
			width: 36px;
		}
		.land .and-rolls {
			display: none;
		}
	}
	/* The narrowest phones on their side (an iPhone SE): the invite's two
	   buttons stack, so neither wraps. */
	@media (max-width: 699px) {
		.land .ph-row {
			flex-direction: column;
		}
	}
	.land .ph-lost {
		grid-area: card;
		align-self: start;
		margin: 0;
	}
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
	.code {
		font-family: var(--font-mono);
		color: var(--text-muted);
	}
	.help-slot {
		display: flex;
		gap: 8px;
		align-self: center;
		margin-left: auto;
	}
	.warn {
		color: var(--hazard);
	}
	p {
		margin: 0;
	}
	.status {
		font-size: 20px;
		color: var(--text);
	}
	.muted {
		color: var(--text-muted);
	}
	.connecting {
		color: var(--text-muted);
	}
	.missing {
		display: flex;
		flex-direction: column;
		align-items: flex-start;
		gap: 14px;
		max-width: 520px;
		margin-top: 48px;
		padding: 24px;
		border: 1px solid var(--line);
		border-radius: 14px;
		background: var(--surface);
	}
	.missing h1 {
		margin: 0;
		color: var(--text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 36px;
		text-transform: uppercase;
	}
	.missing p {
		line-height: 1.5;
	}
	.home-link {
		display: inline-flex;
		align-items: center;
		min-height: 44px;
		padding: 0 18px;
		border-radius: 10px;
		background: var(--accent);
		color: var(--accent-text);
		font-weight: 600;
		text-decoration: none;
	}
	.share {
		display: grid;
		gap: 6px;
		max-width: 560px;
	}
	.share label {
		color: var(--text-body);
	}
	.share-row {
		display: flex;
		gap: 8px;
	}
	.share input {
		flex-grow: 1;
		min-width: 0;
		min-height: 44px;
		padding: 0 12px;
		border: 1px solid var(--line);
		border-radius: 8px;
		background: var(--surface-2);
		color: var(--text);
		font-family: var(--font-mono);
		font-size: 14px;
	}
	.layout {
		/* The board shrinks with the window's height, so both player bars
		   (and your clock) fit without scrolling: about 260 px go to the page
		   padding, header, status line, bars and gaps. */
		--board: clamp(320px, calc(100dvh - 260px), 600px);
		display: grid;
		grid-template-columns: 260px minmax(0, var(--board)) minmax(260px, 340px);
		gap: 24px;
		align-items: start;
	}
	@media (max-width: 1100px) {
		.layout {
			grid-template-columns: minmax(0, var(--board));
		}
		.log-col {
			order: 3;
		}
	}
	.board-col,
	.side-col {
		display: flex;
		flex-direction: column;
		gap: 12px;
	}
	.board-wrap {
		position: relative;
	}
	.result {
		position: absolute;
		top: 50%;
		left: 50%;
		transform: translate(-50%, -50%);
		width: min(380px, 86%);
		box-sizing: border-box;
		padding: 28px;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 10px;
		text-align: center;
		background: var(--surface);
		border: 1px solid var(--line);
		border-radius: 18px;
		box-shadow: 0 24px 60px var(--hole);
	}
	.kicker {
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 13px;
		letter-spacing: 0.12em;
		text-transform: uppercase;
		color: var(--accent);
	}
	.result h1 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 52px;
		line-height: 0.95;
		text-transform: uppercase;
		color: var(--text);
	}
	.result p {
		color: var(--text-body);
	}
	.result.won h1 {
		color: var(--accent);
	}
	.result.lost .kicker {
		color: var(--text-muted);
	}
	.stats {
		margin: 0;
		background: var(--surface);
		border: 1px solid var(--surface-2);
		border-radius: 14px;
		overflow: hidden;
	}
	.stats div {
		display: flex;
		justify-content: space-between;
		gap: 12px;
		padding: 14px 18px;
		border-bottom: 1px solid var(--surface-2);
	}
	.stats div:last-child {
		border-bottom: 0;
	}
	.stats dt {
		color: var(--text-body);
	}
	.stats dd {
		margin: 0;
		font-family: var(--font-mono);
		font-weight: 600;
		font-variant-numeric: tabular-nums;
		color: var(--text);
	}
	/* A tally number lands with a yellow pop. */
	.stats dd:global(.pop),
	.ph-tally b:global(.pop) {
		animation: tally-pop 0.3s ease-out;
	}
	@keyframes tally-pop {
		0% {
			color: var(--accent);
			transform: scale(1.45);
		}
		100% {
			transform: scale(1);
		}
	}
	@media (prefers-reduced-motion: reduce) {
		.stats dd:global(.pop),
		.ph-tally b:global(.pop) {
			animation: none;
		}
	}
	.actions,
	.confirm {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px;
	}
	.confirm span,
	.actions .note {
		width: 100%;
		color: var(--text);
	}
	button,
	.outline {
		display: inline-flex;
		align-items: center;
		justify-content: center;
		gap: 8px;
		min-height: 48px;
		padding: 0 22px;
		border-radius: 12px;
		font: inherit;
		font-weight: 600;
		text-decoration: none;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.primary {
		flex-grow: 1;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		min-height: 48px;
		border-radius: 12px;
		text-decoration: none;
		border: 0;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 22px;
		text-transform: uppercase;
	}
	.share .primary {
		flex-grow: 0;
	}
	.outline {
		border: 1px solid var(--line);
		background: none;
		color: var(--text);
	}
	.resign {
		width: 100%;
	}
	.danger {
		border: 0;
		background: var(--hazard);
		color: var(--accent-text);
	}
	.error {
		color: var(--hazard);
	}
	a {
		color: var(--accent);
	}
</style>
