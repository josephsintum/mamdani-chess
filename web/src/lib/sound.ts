// Sound effects and the waiting music, through Web Audio.
//
// Nothing downloads until sound can play: browsers block audio until the
// player taps or presses a key, so the clips a page asks for are fetched
// after that, once, and decoded once per visit. The files are imported, so
// they get hashed names under _app/immutable and a browser caches them for a
// year. Web Audio follows the iPhone's silent switch, which suits a game
// played in public. Credits and licences: sounds/CREDITS.md.

import { instantMode } from './motion.ts';
import capture from './sounds/capture.mp3';
import challenge from './sounds/challenge.mp3';
import check from './sounds/check.mp3';
import checkmate from './sounds/checkmate.mp3';
import dice from './sounds/dice.mp3';
import die from './sounds/die.mp3';
import draw from './sounds/draw.mp3';
import error from './sounds/error.mp3';
import fell from './sounds/fell.mp3';
import flawless from './sounds/flawless.mp3';
import loss from './sounds/loss.mp3';
import low from './sounds/low.mp3';
import mamdaniFell from './sounds/mamdani-fell.mp3';
import move from './sounds/move.mp3';
import notify from './sounds/notify.mp3';
import potholeCrack from './sounds/pothole-crack.mp3';
import potholeIce from './sounds/pothole-ice.mp3';
import closed from './sounds/closed.mp3';
import repairConstruction from './sounds/repair-construction.mp3';
import repairDrill from './sounds/repair-drill.mp3';
import reroll from './sounds/reroll.mp3';
import saved from './sounds/saved.mp3';
import victory from './sounds/victory.mp3';
import waiting from './sounds/waiting.mp3';

const urls = {
	move,
	capture,
	check,
	checkmate,
	die,
	dice,
	error,
	notify,
	reroll,
	'pothole-crack': potholeCrack,
	'pothole-ice': potholeIce,
	closed,
	saved,
	fell,
	'mamdani-fell': mamdaniFell,
	'repair-construction': repairConstruction,
	'repair-drill': repairDrill,
	victory,
	flawless,
	loss,
	draw,
	low,
	challenge,
	waiting
};

type Clip = keyof typeof urls;

// A sound with several clips plays one of them at random each time.
const GROUPS = {
	pothole: ['pothole-crack', 'pothole-ice'],
	repair: ['repair-construction', 'repair-drill']
} satisfies Record<string, Clip[]>;

export type Sound = Clip | keyof typeof GROUPS;

function clipsOf(s: Sound): readonly Clip[] {
	return s in GROUPS ? GROUPS[s as keyof typeof GROUPS] : [s as Clip];
}

/** Needed as soon as a game starts. */
export const CORE: Sound[] = ['move', 'capture', 'die', 'dice', 'reroll', 'pothole', 'closed', 'saved', 'fell', 'check', 'error', 'notify'];
/** Needed later in a game, fetched while the browser is idle. */
export const LATER: Sound[] = ['mamdani-fell', 'repair', 'checkmate', 'victory', 'flawless', 'loss', 'draw', 'low', 'challenge'];

const VOLUME = 0.7;
// The waiting music plays under everything else.
const MUSIC_VOLUME = 0.5;

let ctx: AudioContext | undefined;
let main: GainNode | undefined;
let armed = false;
// Clips a page asked for; fetched once audio is unlocked and sound is on.
const wanted = new Set<Clip>();
const loading = new Map<Clip, Promise<AudioBuffer | null>>();
// Aborts the downloads in flight when the page is left.
let leaving = new AbortController();
const decoded = new Map<Clip, AudioBuffer>();

interface Loop {
	sound: Clip;
	node?: AudioBufferSourceNode;
}
const loops = new Set<Loop>();

function running(): boolean {
	return ctx?.state === 'running';
}

// Creates the audio context and resumes it. Browsers allow this only after
// the player has interacted with the page, so it runs on every tap until the
// context is running (iOS can suspend it again, after a call for instance).
// With sound off there is no audio context at all.
function unlock() {
	if (typeof AudioContext === 'undefined' || !soundOn()) return;
	if (!ctx) {
		ctx = new AudioContext();
		main = ctx.createGain();
		main.gain.value = VOLUME;
		main.connect(ctx.destination);
		ctx.onstatechange = () => {
			if (running()) started();
		};
	}
	if (!running()) void ctx.resume().catch(() => {});
	else started();
}

// Audio can play: fetch what pages asked for, start any waiting music.
function started() {
	if (!soundOn()) return;
	for (const s of wanted) void fetchSound(s);
	for (const l of loops) startLoop(l);
}

// Listens for the first taps and key presses. A page reached by a click
// (quick match, a new game) has already been interacted with, so it tries
// at once as well.
function arm() {
	if (armed || typeof window === 'undefined' || instantMode()) return;
	armed = true;
	window.addEventListener('pointerdown', unlock, { capture: true });
	window.addEventListener('keydown', unlock, { capture: true });
	window.addEventListener('pagehide', leave);
	if (navigator.userActivation?.hasBeenActive) unlock();
}

// The page is going away: stop its audio and its downloads, so neither is
// cut off mid-way (WebKit reports a cut-off request as an error). A page
// restored from the back-forward cache starts again on the next tap.
function leave() {
	leaving.abort();
	leaving = new AbortController();
	loading.clear();
	for (const l of loops) l.node = undefined;
	void ctx?.close().catch(() => {});
	ctx = undefined;
	main = undefined;
}

function fetchSound(s: Clip): Promise<AudioBuffer | null> {
	const had = loading.get(s);
	if (had) return had;
	if (!ctx) return Promise.resolve(null);
	const audio = ctx;
	const p = fetch(urls[s], { signal: leaving.signal })
		.then((r) => r.arrayBuffer())
		.then((bytes) => audio.decodeAudioData(bytes))
		.then((buffer) => {
			decoded.set(s, buffer);
			return buffer;
		})
		.catch(() => {
			loading.delete(s); // try again next time
			return null;
		});
	loading.set(s, p);
	return p;
}

/**
 * Asks for clips this page will play. They download once audio is unlocked
 * and sound is on; a page that never asks downloads nothing.
 */
export function load(sounds: readonly Sound[]) {
	for (const s of sounds) for (const c of clipsOf(s)) wanted.add(c);
	arm();
	if (running()) started();
}

/** Asks for clips when the browser is idle: for the sounds a game needs later. */
export function loadSoon(sounds: readonly Sound[]) {
	if (typeof window === 'undefined') return;
	const later = window.requestIdleCallback ?? ((f: () => void) => setTimeout(f, 1000));
	later(() => load(sounds));
}

/**
 * Plays a clip, `delayMs` from now. Silent when sound is off, in instant
 * mode, before the player has tapped, or when the clip hasn't arrived: a
 * sound that would come late is skipped, unless it may wait `wait` ms for
 * its clip (the start of a game, just after the page loaded).
 */
export function play(s: Sound, delayMs = 0, { wait = 0 } = {}) {
	if (!soundOn() || instantMode() || !ctx || !main) return;
	if (!running()) {
		void ctx.resume().catch(() => {});
		return;
	}
	const clips = clipsOf(s);
	const ready = clips.filter((c) => decoded.has(c));
	for (const c of clips) if (!decoded.has(c)) void fetchSound(c);
	if (ready.length === 0) {
		const asked = performance.now();
		if (wait > 0) void fetchSound(clips[0]).then((b) => b && performance.now() - asked < wait && play(s, delayMs));
		return;
	}
	const buffer = decoded.get(ready[Math.floor(Math.random() * ready.length)])!;
	const node = ctx.createBufferSource();
	node.buffer = buffer;
	node.connect(main);
	node.start(ctx.currentTime + delayMs / 1000);
}

function saveData(): boolean {
	const c = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection;
	return c?.saveData === true;
}

function startLoop(l: Loop) {
	if (l.node || !ctx || !main || !running() || !soundOn() || instantMode() || saveData()) return;
	const buffer = decoded.get(l.sound);
	if (!buffer) {
		void fetchSound(l.sound).then((b) => b && loops.has(l) && startLoop(l));
		return;
	}
	const gain = ctx.createGain();
	gain.gain.value = MUSIC_VOLUME;
	gain.connect(main);
	const node = ctx.createBufferSource();
	node.buffer = buffer;
	node.loop = true;
	node.connect(gain);
	node.start();
	l.node = node;
}

/**
 * Plays a clip on repeat (the waiting music) until the returned function is
 * called. It starts once audio is unlocked, and not at all when the browser
 * asks to save data.
 */
export function loop(s: Clip): () => void {
	const l: Loop = { sound: s };
	loops.add(l);
	wanted.add(s);
	arm();
	startLoop(l);
	return () => {
		loops.delete(l);
		l.node?.stop();
		l.node = undefined;
	};
}

const OFF_KEY = 'sound-off';
// The choice made on this page, which holds even when storage is blocked.
let chosen: boolean | undefined;

/** Sound is on unless this browser turned it off. */
export function soundOn(): boolean {
	if (chosen !== undefined) return chosen;
	try {
		return localStorage.getItem(OFF_KEY) === null;
	} catch {
		return true;
	}
}

/** Turns all sound on or off at once, the waiting music too, and remembers it. */
export function setSoundOn(on: boolean) {
	chosen = on;
	try {
		if (on) localStorage.removeItem(OFF_KEY);
		else localStorage.setItem(OFF_KEY, '1');
	} catch {
		// Blocked storage: the choice lasts while the page is open.
	}
	if (main) main.gain.value = on ? VOLUME : 0;
	if (on) {
		arm();
		unlock(); // the click that turned it on counts as the tap
	}
}
