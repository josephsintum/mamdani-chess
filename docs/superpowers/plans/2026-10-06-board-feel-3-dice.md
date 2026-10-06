# Board Feel 3: The Dice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Throw the dice for real (spec section 9). The changes:
- The tray's dice fly in, bounce and tumble, coloured by what they decide.
- A pill on the board says what the roll means.
- The file and rank dice roll out of sync while an orange scan on the board follows their faces, lighting the matching coordinates, and lands on the target.
- A re-roll makes the target blink.
- Every step plays for its own time, from one table that the server's clock pause shares.

**Architecture:**
- **The shared table:** `web/src/lib/dice-timing.json` holds how long the browser plays each dice step. The Go server's `pauseFor` uses the same numbers (`playTime`, `MoveTime` in `game/clock.go`), and a Go test fails if they drift apart.
- **`web/src/lib/dice.ts`** holds the client's pure helpers:
  - the pacing for the `Animator`;
  - seeded tumble schedules, so the tray's dice and the board's scan, drawn apart, show the same faces at the same moment;
  - the scan path;
  - each step's die specs;
  - the board's pill.
- **Components:** `Die.svelte` throws a die from its spec. `Board.svelte` runs the scan, the lit coordinates, the delayed target, the re-roll blink and the pill. The tray (desktop) and the phone's dice card (kept below the board) throw their dice in step.

**Tech Stack:** Svelte 5.57 / SvelteKit 3, TypeScript, Vitest, Go (game package), Playwright playtests.

**Spec:** `docs/superpowers/specs/2026-10-06-board-feel-design.md`, section 9 and "Everywhere". Mockups: https://claude.ai/artifact/E6oCYQhp8xaDTZCVssczXy (section 9), and the phone options: https://claude.ai/artifact/A8xi3SjP9TzYfr6CaafrUV (option A chosen).

**Starts from:** `main` at `3d8cf2a`. Work on a new branch `board-feel-3`.

## Global Constraints

- **Colours** say what each die decides:
  - a pothole roll's die is grey when odd and road-works yellow (`--accent`) when even, shown only once it lands;
  - the file and rank dice are hazard orange (`--hazard`);
  - a saving die is cream (`--piece-light`) with dark text.
- **Timing table:** exactly these values, in ms:

  | Step | ms |
  | --- | --- |
  | move | 550 |
  | odd roll | 700 |
  | even roll | 600 |
  | target (file and rank dice with the scan) | 1250 |
  | re-roll | 500 |
  | saving roll | 750 |
  | fall | 600 |
  | hole opens | 550 |
  | repair | 550 |
  | cap closes a hole | 400 |
  | no pothole | 550 |
  | anything else | 550 |
  | server margin | 500 |
  | shortest pause | 2000 |

- **The throw:** a die is thrown in 450 ms. The file die tumbles 700 ms. The rank die is thrown 150 ms later and tumbles 850 ms, so the scan lands at 1000 ms. A pothole roll then takes about 1.85 s in all, and an odd roll about 0.7 s.
- **Phones:** the thrown dice stay in today's dice card below the board; the board doesn't move. The board's pill and scan are the same as on desktop.
- **Turns shown at once:** reduced motion and instant mode (`reducedMotion()`) skip the throw, the tumble and the scan, and show end states. A turn shown at once (reload, reconnect, a hidden tab) gets no pill and no scan, as with the bubbles (`anim.animated`).
- **Svelte:** avoid `$effect`. Imports use `./x.ts` in `lib` and `#lib/x.ts` in routes. Run `npx @sveltejs/mcp svelte-autofixer <file>` on every changed component.
- **Commits:** stage only the files you changed, with no Claude attribution. The PR description embeds screenshots, hosted on the `pr-screenshots` branch under `pr-<number>/`.

## Departures from the spec

Each of these is in the code below.

- **The phone keeps its dice card below the board** (decided 6 Oct over the spec's "across the top", after the phone-options mockup).
- **Nothing gives an outcome away before its dice land:**
  - a pothole roll's die tumbles grey and takes its colour when it lands;
  - the pill and the tray's and phone card's lines wait for the dice;
  - the dashed target appears only once the scan reaches it.
- **When the turn passes:** the board becomes playable as soon as the last step is revealed, as it does today. The server still pauses the clock for that last step's whole play time.

## Review Focus

- **A long roll** (re-roll, saving roll, the cap closing a hole, a fall): the next clock never starts while the dice still play. `pauseFor` covers the move plus every step plus the margin, and the client reveals the last step earlier than that. Task 1 tests the longest turn.
- **The tray redraws its steps on every revealed step:** a thrown die must not be thrown again or restart its tumble (`untrack` in `Die.svelte`). Checked in Task 4 by watching the tray in the sandbox across a whole pothole roll.
- **A turn shown at once** (reload mid-roll, a tab coming back) shows no scan and no pill. Checked in Task 6 by reloading mid-roll.
- **Reduced motion:** no throw, no tumble, no scan; the target shows at once and the pill shows the outcome, then goes. Checked in Task 6.
- **The scan never touches the target before its last frame.** Task 2 tests it with several seeds.

---

### Task 1: One timing table for the browser and the server's clock pause

**Files:**
- Create: `web/src/lib/dice-timing.json`, `game/dice_timing_test.go`
- Modify: `game/clock.go` (`StepTime` → `MoveTime`, new `playTime`, `pauseFor`), `game/clock_test.go`

**Interfaces:**
- Produces: `dice-timing.json` with `moveMs`, `marginMs`, `minPauseMs` and `playMs.{rolled_pothole_odd, rolled_pothole_even, target, reroll, saving_roll, fell, pothole_opened, repaired, pothole_closed, no_pothole, other}` (read by Task 2's `dice.ts`). In Go: `MoveTime`, `playTime(e rules.Event) time.Duration`.

- [ ] **Step 1: Write the table and the failing tests**

Create `web/src/lib/dice-timing.json`:


```json
{
	"//": "How long the browser plays each dice step, in ms. The server's clock pause (game/clock.go, pauseFor) uses the same numbers; game/dice_timing_test.go fails if they drift apart.",
	"moveMs": 550,
	"marginMs": 500,
	"minPauseMs": 2000,
	"playMs": {
		"rolled_pothole_odd": 700,
		"rolled_pothole_even": 600,
		"target": 1250,
		"reroll": 500,
		"saving_roll": 750,
		"fell": 600,
		"pothole_opened": 550,
		"repaired": 550,
		"pothole_closed": 400,
		"no_pothole": 550,
		"other": 550
	}
}
```


Create `game/dice_timing_test.go`:


```go
package game

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"mamdani-chess/rules"
)

// The browser plays the dice from web/src/lib/dice-timing.json; the server
// must pause the next clock by the same numbers, or a clock starts mid-roll.
func TestDiceTimingMatchesTheBrowser(t *testing.T) {
	raw, err := os.ReadFile("../web/src/lib/dice-timing.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		MoveMs     int            `json:"moveMs"`
		MarginMs   int            `json:"marginMs"`
		MinPauseMs int            `json:"minPauseMs"`
		PlayMs     map[string]int `json:"playMs"`
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	if MoveTime != ms(table.MoveMs) || PauseMargin != ms(table.MarginMs) || ResolveDelay != ms(table.MinPauseMs) {
		t.Errorf("move %v margin %v min %v, the browser has %d, %d, %d ms", MoveTime, PauseMargin, ResolveDelay, table.MoveMs, table.MarginMs, table.MinPauseMs)
	}
	samples := map[string]rules.Event{
		"rolled_pothole_odd":  {Kind: rules.RolledPothole, Roll: 3},
		"rolled_pothole_even": {Kind: rules.RolledPothole, Roll: 6},
		"target":              {Kind: rules.Target},
		"reroll":              {Kind: rules.Reroll},
		"saving_roll":         {Kind: rules.SavingRoll},
		"fell":                {Kind: rules.Fell},
		"pothole_opened":      {Kind: rules.PotholeOpened},
		"repaired":            {Kind: rules.Repaired},
		"pothole_closed":      {Kind: rules.PotholeClosed},
		"no_pothole":          {Kind: rules.NoPothole},
		"other":               {Kind: rules.Captured},
	}
	if len(table.PlayMs) != len(samples) {
		t.Errorf("the browser times %d kinds of step, the server %d", len(table.PlayMs), len(samples))
	}
	for name, e := range samples {
		want, ok := table.PlayMs[name]
		if !ok {
			t.Errorf("the browser has no time for %s", name)
			continue
		}
		if got := playTime(e); got != ms(want) {
			t.Errorf("%s: the server plays %v, the browser %d ms", name, got, want)
		}
	}
}
```


Update `game/clock_test.go`. 
Save this patch as `/tmp/t1-clock-test.patch` and apply it from the repository root with `git apply /tmp/t1-clock-test.patch`:

```diff
diff --git a/game/clock_test.go b/game/clock_test.go
index 0c14840..495b914 100644
--- a/game/clock_test.go
+++ b/game/clock_test.go
@@ -236,19 +236,28 @@ func TestThePauseCoversTheDiceAnimation(t *testing.T) {
 		out := make([]rules.Event, len(kinds))
 		for i, k := range kinds {
 			out[i] = rules.Event{Kind: k}
+			if k == rules.RolledPothole {
+				out[i].Roll = 2
+			}
 		}
 		return out
 	}
+	odd := []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 3}}
 	cases := []struct {
 		name string
 		ev   []rules.Event
 		want time.Duration
 	}{
 		{"checkmate, no roll", ev(rules.Moved), ResolveDelay},
-		{"odd roll", ev(rules.Moved, rules.RolledPothole), ResolveDelay},
-		{"pothole opens", ev(rules.Moved, rules.RolledPothole, rules.Target, rules.PotholeOpened), 3*StepTime + PauseMargin},
-		{"re-roll, saving roll, fall", ev(rules.Moved, rules.Captured, rules.RolledPothole, rules.Reroll, rules.Target,
-			rules.SavingRoll, rules.Fell, rules.PotholeOpened), 6*StepTime + PauseMargin},
+		{"odd roll", odd, ResolveDelay},
+		// The move glides, then each step plays in turn, then the margin.
+		{"pothole opens", ev(rules.Moved, rules.RolledPothole, rules.Target, rules.PotholeOpened),
+			MoveTime + 600*time.Millisecond + 1250*time.Millisecond + 550*time.Millisecond + PauseMargin},
+		// The longest kind of turn: a re-roll, a second target, a saving roll
+		// that fails, the cap closing the oldest hole, the fall, the new hole.
+		{"re-roll, saving roll, cap, fall", ev(rules.Moved, rules.Captured, rules.RolledPothole, rules.Target, rules.Reroll, rules.Target,
+			rules.SavingRoll, rules.Fell, rules.PotholeClosed, rules.PotholeOpened),
+			MoveTime + (600+1250+500+1250+750+600+400+550)*time.Millisecond + PauseMargin},
 	}
 	for _, c := range cases {
 		if got := pauseFor(c.ev); got != c.want {
@@ -257,16 +266,18 @@ func TestThePauseCoversTheDiceAnimation(t *testing.T) {
 	}
 }
 
-// A roll that opens a pothole animates for three steps, so Black's
-// first-move deadline starts when the animation ends, not after 2 s.
+// A roll that opens a pothole animates for longer than 2 s (the roll, the
+// file and rank dice, the hole), so Black's first-move deadline starts when
+// the animation ends.
 func TestALongRollDelaysTheNextClock(t *testing.T) {
 	synctest.Test(t, func(t *testing.T) {
 		g, _, _ := seated(t, NewHub(&script{rolls: []int{2, 2, 4}}, nil)) // even, then b4: a pothole opens
 		play(t, g, "alice", "e2e4", 0)
 		v := recvView(t, g, "bob")
-		want := time.Now().Add(3*StepTime + PauseMargin + FirstMoveTime).UnixMilli()
+		roll := MoveTime + 600*time.Millisecond + 1250*time.Millisecond + 550*time.Millisecond + PauseMargin
+		want := time.Now().Add(roll + FirstMoveTime).UnixMilli()
 		if v.Clock.FirstMoveDeadline != want {
-			t.Fatalf("first-move deadline %d, want %d (after the 3-step roll)", v.Clock.FirstMoveDeadline, want)
+			t.Fatalf("first-move deadline %d, want %d (after the roll's %v)", v.Clock.FirstMoveDeadline, want, roll)
 		}
 		finish(t, g)
 	})
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `go test ./game/ -run 'TestDiceTiming|TestThePause|TestALongRoll'`
Expected: build failure: `undefined: MoveTime`, `undefined: playTime`.

- [ ] **Step 3: Implement**


Save this patch as `/tmp/t1-clock.patch` and apply it from the repository root with `git apply /tmp/t1-clock.patch`:

```diff
diff --git a/game/clock.go b/game/clock.go
index 5cafe01..a569eac 100644
--- a/game/clock.go
+++ b/game/clock.go
@@ -14,9 +14,11 @@ const (
 	// out: the next player's clock (or first-move deadline) starts after it.
 	// A longer roll gets a longer pause (see pauseFor).
 	ResolveDelay = 2 * time.Second
-	// StepTime is how long the browser shows each dice step. It must match
-	// STEP_MS in web/src/lib/animator.svelte.ts.
-	StepTime = 550 * time.Millisecond
+	// MoveTime is how long the browser shows the move before the dice roll.
+	// It and each step's time (playTime) come from the browser's table,
+	// web/src/lib/dice-timing.json; TestDiceTimingMatchesTheBrowser keeps
+	// them the same.
+	MoveTime = 550 * time.Millisecond
 	// PauseMargin covers the state's trip to the browser on top of the
 	// animation itself.
 	PauseMargin = 500 * time.Millisecond
@@ -29,19 +31,47 @@ const (
 )
 
 // pauseFor is how long the next clock waits after a turn with events ev:
-// as long as the browser takes to play the dice, one StepTime per event
-// from the pothole roll on, plus PauseMargin, and never less than
-// ResolveDelay. So a long roll (a re-roll, a saving roll, a fall) costs the
-// next player no clock time.
+// as long as the browser takes to play it (the move, then every step from
+// the pothole roll on, each for its own time), plus PauseMargin, and never
+// less than ResolveDelay. So a long roll (a re-roll, a saving roll, a fall)
+// costs the next player no clock time.
 func pauseFor(ev []rules.Event) time.Duration {
 	for i, e := range ev {
 		if e.Kind == rules.RolledPothole {
-			return max(ResolveDelay, time.Duration(len(ev)-i)*StepTime+PauseMargin)
+			total := MoveTime + PauseMargin
+			for _, step := range ev[i:] {
+				total += playTime(step)
+			}
+			return max(ResolveDelay, total)
 		}
 	}
 	return ResolveDelay
 }
 
+// playTime is how long the browser plays one dice step: the throw, the
+// file and rank dice with the scan, a fall, a hole cracking open.
+func playTime(e rules.Event) time.Duration {
+	ms := 550
+	switch e.Kind {
+	case rules.RolledPothole:
+		ms = 600 // even: the pothole roll, then the file and rank dice follow
+		if e.Roll%2 == 1 {
+			ms = 700 // odd: the throw, and the pill saying nothing happens
+		}
+	case rules.Target:
+		ms = 1250 // the file and rank dice, out of sync, with the scan
+	case rules.Reroll:
+		ms = 500 // the target blinks twice
+	case rules.SavingRoll:
+		ms = 750
+	case rules.Fell:
+		ms = 600
+	case rules.PotholeClosed:
+		ms = 400
+	}
+	return time.Duration(ms) * time.Millisecond
+}
+
 // Result reasons the game package adds to the rules package's.
 const (
 	Timeout rules.Reason = "timeout"
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `gofmt -l game/ && go test -race ./game/`
Expected: no files listed; `ok  mamdani-chess/game`.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/dice-timing.json game/dice_timing_test.go game/clock.go game/clock_test.go
git commit -m "game: the clock pause and the browser time each dice step from one table"
```

---

### Task 2: The dice helpers: pacing, seeded tumbles, the scan, die specs, the pill

**Files:**
- Create: `web/src/lib/dice.ts`, `web/src/lib/dice.test.ts`

**Interfaces:**
- Consumes: `dice-timing.json` (Task 1); `pieceName`, `EventJSON`, `View` from `game.ts`.
- Produces, used by later tasks:
  - **Timing:** `MOVE_MS`, `THROW_MS = 450`, `FILE_MS = 700`, `RANK_DELAY_MS = 150`, `RANK_MS = 850`, `SCAN_MS = 1000`, `SAVE_MS = 450`; `playMs(e)`, `dicePace(shownLast?)`, `turnMs(last)`.
  - **Faces:** `tumbleFaces(final, ms, seed)` and `scanFrames(sq, seed)`, each a list of `{ at, face }` or `{ at, sq }`; `diceSeed(seq, index)`.
  - **The board:** `scanOf(view, shown)`, returning `{ sq, key, seed } | null`; `type DicePill` (`{ key, text, tone, at?, then?, last }`) and `dicePill(view, shown)`.
  - **Dice:** `type DieSpec` (`{ value, tone, delay, ms, seed }`), `type DieTone` (`'dull' | 'pot' | 'where' | 'save'`), and `stepDice(view, i)`.

- [ ] **Step 1: Write the failing tests**

Create `web/src/lib/dice.test.ts`:


```ts
import { describe, expect, it } from 'vitest';
import { dicePace, dicePill, stepDice, FILE_MS, MOVE_MS, playMs, RANK_DELAY_MS, RANK_MS, scanFrames, SCAN_MS, scanOf, tumbleFaces, turnMs } from './dice.ts';
import TIMING from './dice-timing.json';
import type { EventJSON, View } from './game.ts';

const move: EventJSON = { kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' };

describe('the timing table', () => {
	it('times each step from the shared table', () => {
		expect(MOVE_MS).toBe(TIMING.moveMs);
		expect(playMs({ kind: 'rolled_pothole', roll: 3 })).toBe(TIMING.playMs.rolled_pothole_odd);
		expect(playMs({ kind: 'rolled_pothole', roll: 6 })).toBe(TIMING.playMs.rolled_pothole_even);
		expect(playMs({ kind: 'target', sq: 'd5' })).toBe(TIMING.playMs.target);
		expect(playMs({ kind: 'captured', sq: 'd5' })).toBe(TIMING.playMs.other);
	});
	it('waits for the move before the roll, then for the step just shown', () => {
		expect(dicePace(undefined)).toBe(MOVE_MS);
		expect(dicePace({ kind: 'target', sq: 'd5' })).toBe(TIMING.playMs.target);
	});
	it('plays a whole turn in the time the server pauses the clock for', () => {
		const last: EventJSON[] = [move, { kind: 'rolled_pothole', roll: 2 }, { kind: 'target', sq: 'd5' }, { kind: 'pothole_opened', sq: 'd5' }];
		expect(turnMs(last)).toBe(MOVE_MS + 600 + 1250 + 550);
		expect(turnMs([move])).toBe(0);
	});
	it('fits the scan inside its step', () => {
		expect(SCAN_MS).toBe(RANK_DELAY_MS + RANK_MS);
		expect(FILE_MS).toBeLessThan(SCAN_MS);
		expect(SCAN_MS).toBeLessThan(TIMING.playMs.target);
	});
});

describe('tumbleFaces', () => {
	it('slows down, lands on the value at the end, and never shows it before', () => {
		for (const seed of [1, 7, 42, 99, 1234]) {
			for (const final of [1, 4, 8]) {
				const frames = tumbleFaces(final, 700, seed);
				expect(frames.at(-1)).toEqual({ at: 700, face: final });
				for (const f of frames.slice(0, -1)) {
					expect(f.face).not.toBe(final);
					expect(f.face).toBeGreaterThanOrEqual(1);
					expect(f.face).toBeLessThanOrEqual(8);
				}
				const gaps = frames.slice(1).map((f, i) => f.at - frames[i].at);
				expect(gaps[1]).toBeLessThan(gaps.at(-2)!); // early faces flick faster than late ones
			}
		}
	});
	it('is the same for the same seed, so the tray and the board agree', () => {
		expect(tumbleFaces(5, 850, 3)).toEqual(tumbleFaces(5, 850, 3));
		expect(tumbleFaces(5, 850, 3)).not.toEqual(tumbleFaces(5, 850, 4));
	});
});

describe('scanFrames', () => {
	it('follows the file and rank dice and lands on the target only at the end', () => {
		for (const seed of [2, 5, 11, 300]) {
			const frames = scanFrames('d5', seed);
			expect(frames.at(-1)).toEqual({ at: SCAN_MS, sq: 'd5' });
			for (const f of frames.slice(0, -1)) expect(f.sq).not.toBe('d5');
			// Once the file die lands its column stays.
			for (const f of frames.filter((x) => x.at >= FILE_MS)) expect(f.sq[0]).toBe('d');
		}
	});
});

describe('the board during a roll', () => {
	const view = (last: EventJSON[], seq = 7) => ({ seq, last, board: Array(64).fill(''), mamdani: '', potholes: [] }) as unknown as View;
	it('says an odd roll, and lets the pill go', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 3 }]);
		expect(dicePill(v, 2)).toEqual({ key: '7:1', text: 'd8 3 · no pothole', tone: 'odd', at: 450, last: true });
	});
	it('says an even roll, then the square once the file and rank dice land', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 6 }, { kind: 'target', sq: 'd5' }, { kind: 'pothole_opened', sq: 'd5' }]);
		expect(dicePill(v, 2)).toMatchObject({ text: 'd8 6 · pothole', tone: 'pot', last: false });
		expect(dicePill(v, 3)).toEqual({ key: '7:2', text: 'd8 6 · pothole', tone: 'pot', then: { text: 'd5', tone: 'where', at: SCAN_MS }, last: true });
		// The hole opening keeps the target's pill.
		expect(dicePill(v, 4)?.key).toBe('7:2');
		expect(scanOf(v, 3)).toEqual({ sq: 'd5', key: '7:2', seed: 7 * 97 + 2 * 2 });
		expect(scanOf(v, 4)).toEqual({ sq: 'd5', key: '7:2', seed: 7 * 97 + 2 * 2 });
	});
	it('names a re-roll and a saving roll', () => {
		const v = view([
			move,
			{ kind: 'rolled_pothole', roll: 2 },
			{ kind: 'target', sq: 'e1' },
			{ kind: 'reroll', sq: 'e1', reason: 'king' },
			{ kind: 'target', sq: 'b4' },
			{ kind: 'saving_roll', sq: 'b4', piece: 'wN', roll: 5, saved: true }
		]);
		expect(dicePill(v, 4)).toMatchObject({ text: 'e1 · re-roll', tone: 'pot' });
		expect(dicePill(v, 6)).toEqual({ key: '7:5', text: 'White knight b4 · saving roll', tone: 'save', then: { text: 'White knight b4 · saved', tone: 'save', at: 450 }, last: true });
		expect(scanOf(v, 4)).toBeNull(); // the re-roll: no scan while the target blinks
		expect(scanOf(v, 5)?.sq).toBe('b4');
	});
	it('says nothing before the roll', () => {
		expect(dicePill(view([move, { kind: 'rolled_pothole', roll: 3 }]), 1)).toBeNull();
	});
});

describe('stepDice', () => {
	const view = (last: EventJSON[]) => ({ seq: 3, last }) as unknown as View;
	it('throws one die for the roll, grey when odd and yellow when even', () => {
		expect(stepDice(view([move, { kind: 'rolled_pothole', roll: 3 }]), 1)).toEqual([{ value: 3, tone: 'dull', delay: 0, ms: 450, seed: 3 * 97 + 2 }]);
		expect(stepDice(view([move, { kind: 'rolled_pothole', roll: 6 }]), 1)[0].tone).toBe('pot');
	});
	it('throws the file and rank dice out of sync, with the scan\'s seeds', () => {
		const v = view([move, { kind: 'rolled_pothole', roll: 6 }, { kind: 'target', sq: 'd5' }]);
		const seed = 3 * 97 + 2 * 2;
		expect(stepDice(v, 2)).toEqual([
			{ value: 4, tone: 'where', delay: 0, ms: FILE_MS, seed: seed + 1 },
			{ value: 5, tone: 'where', delay: RANK_DELAY_MS, ms: RANK_MS, seed: seed + 2 }
		]);
	});
	it('throws a cream die for a saving roll, and none for the rest', () => {
		const v = view([move, { kind: 'saving_roll', sq: 'b4', piece: 'wN', roll: 5, saved: true }, { kind: 'fell', sq: 'b4', piece: 'wN' }]);
		expect(stepDice(v, 1)[0]).toMatchObject({ value: 5, tone: 'save' });
		expect(stepDice(v, 2)).toEqual([]);
	});
});
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `pnpm --dir web exec vitest run src/lib/dice.test.ts`
Expected: FAIL: `Cannot find module './dice.ts'` (or `Failed to resolve import`).

- [ ] **Step 3: Write the module**

Create `web/src/lib/dice.ts`:


```ts
// The dice's pacing and faces (spec: board feel, section 9). Each step of a
// roll plays for its time from dice-timing.json, the table the server's
// clock pause uses too. A die tumbles through faces from a seeded schedule,
// so the tray's dice and the board's scan, drawn apart, show the same faces
// at the same moment.

import TIMING from './dice-timing.json';
import { pieceName, type EventJSON, type View } from './game.ts';

/** How long the move shows before the dice roll. */
export const MOVE_MS = TIMING.moveMs;
/** A thrown die flies in, bounces twice and settles. */
export const THROW_MS = 450;
/** The file die tumbles this long; the rank die is thrown RANK_DELAY_MS later and tumbles RANK_MS. */
export const FILE_MS = 700;
export const RANK_DELAY_MS = 150;
export const RANK_MS = 850;
/** The scan runs until the rank die lands. */
export const SCAN_MS = RANK_DELAY_MS + RANK_MS;
/** A saving die lands, then the pill says whether the piece is saved. */
export const SAVE_MS = THROW_MS;

/** Index of the pothole roll in a turn's events; the end if none was rolled (as board.ts's firstDiceStep). */
function firstDiceStep(last: EventJSON[]): number {
	const i = last.findIndex((e) => e.kind === 'rolled_pothole');
	return i < 0 ? last.length : i;
}

/** How long the browser plays one dice step, from the shared table. */
export function playMs(e: EventJSON): number {
	const t = TIMING.playMs as Record<string, number>;
	if (e.kind === 'rolled_pothole') return (e.roll ?? 1) % 2 === 1 ? t.rolled_pothole_odd : t.rolled_pothole_even;
	return t[e.kind] ?? t.other;
}

/** The Animator's wait before showing the next step: the move first, then the step just shown. */
export function dicePace(shownLast: EventJSON | undefined): number {
	return shownLast ? playMs(shownLast) : MOVE_MS;
}

/** How long a whole turn takes to play, from the move to the last step's end (0 with no roll). */
export function turnMs(last: EventJSON[]): number {
	const first = firstDiceStep(last);
	if (first === last.length) return 0;
	return MOVE_MS + last.slice(first).reduce((sum, e) => sum + playMs(e), 0);
}

/**
 * A die's faces from `seed`: random faces that flick fast, then slower, and
 * never show `final` until the last frame, at `ms`.
 */
export function tumbleFaces(final: number, ms: number, seed: number): { at: number; face: number }[] {
	let s = (seed * 9301 + 49297) % 233280 || 1;
	const rand = () => (s = (s * 16807) % 2147483647) / 2147483647;
	const frames: { at: number; face: number }[] = [];
	let at = 0;
	let prev = 0;
	while (at < ms) {
		let face = 1 + Math.floor(rand() * 8);
		while (face === final || face === prev) face = (face % 8) + 1;
		frames.push({ at, face });
		prev = face;
		at += Math.round(55 + 140 * (at / ms));
	}
	frames.push({ at: ms, face: final });
	return frames;
}

/** A die's face at `t` ms after it was thrown (its first face before then). */
function faceAt(frames: { at: number; face: number }[], t: number): number {
	let face = frames[0].face;
	for (const f of frames) if (f.at <= t) face = f.face;
	return face;
}

const FILES = 'abcdefgh';

/**
 * Where the orange scan stands while the file and rank dice tumble for a
 * target `sq`: the square their current faces point to. The file die lands
 * first and locks the column; the scan reaches `sq` only when the rank die
 * lands, at SCAN_MS.
 */
export function scanFrames(sq: string, seed: number): { at: number; sq: string }[] {
	const file = tumbleFaces(FILES.indexOf(sq[0]) + 1, FILE_MS, seed + 1);
	const rank = tumbleFaces(Number(sq[1]), RANK_MS, seed + 2).map((f) => ({ at: f.at + RANK_DELAY_MS, face: f.face }));
	const times = [...new Set([...file.map((f) => f.at), ...rank.map((f) => f.at)])].sort((a, b) => a - b);
	const frames: { at: number; sq: string }[] = [];
	for (const at of times) {
		const here = FILES[faceAt(file, at) - 1] + faceAt(rank, at);
		if (frames.at(-1)?.sq !== here) frames.push({ at, sq: here });
	}
	return frames;
}

/** The seed for the dice of step `index` of turn `seq`: the tray and the board use the same one. */
export function diceSeed(seq: number, index: number): number {
	return seq * 97 + index * 2;
}

/** The latest target revealed, unless a re-roll followed it: the scan the board runs. */
export function scanOf(view: View, shown: number): { sq: string; key: string; seed: number } | null {
	for (let i = Math.min(shown, view.last.length) - 1; i >= firstDiceStep(view.last); i--) {
		const e = view.last[i];
		if (e.kind === 'reroll') return null;
		if (e.kind === 'target' && e.sq) return { sq: e.sq, key: `${view.seq}:${i}`, seed: diceSeed(view.seq, i) };
	}
	return null;
}

export type PillTone = 'odd' | 'pot' | 'where' | 'save';
export interface DicePill {
	key: string;
	text: string;
	tone: PillTone;
	/** When it shows, in ms after its step: a roll's pill waits for its die to land. */
	at?: number;
	/** What it changes to `at` ms after it shows: the square once the dice land, a saving roll's result. */
	then?: { text: string; tone: PillTone; at: number };
	/** The turn's last pill: it fades on its own. */
	last: boolean;
}

const capitalize = (s: string) => s.charAt(0).toUpperCase() + s.slice(1);
const PILL_KINDS = ['rolled_pothole', 'target', 'reroll', 'saving_roll', 'no_pothole'];

/** The pill for one dice event, or null for events the pill doesn't name. */
function pillFor(view: View, i: number): Omit<DicePill, 'last'> | null {
	const e = view.last[i];
	const key = `${view.seq}:${i}`;
	const roll = view.last.find((x) => x.kind === 'rolled_pothole')?.roll ?? 0;
	switch (e.kind) {
		case 'rolled_pothole':
			return (e.roll ?? 1) % 2 === 1
				? { key, text: `d8 ${e.roll} · no pothole`, tone: 'odd', at: THROW_MS }
				: { key, text: `d8 ${e.roll} · pothole`, tone: 'pot', at: THROW_MS };
		case 'target':
			return { key, text: `d8 ${roll} · pothole`, tone: 'pot', then: { text: e.sq ?? '', tone: 'where', at: SCAN_MS } };
		case 'reroll':
			return { key, text: `${e.sq} · re-roll`, tone: 'pot' };
		case 'saving_roll': {
			const who = `${capitalize(pieceName(e.piece))} ${e.sq}`;
			return { key, text: `${who} · saving roll`, tone: 'save', then: { text: `${who} · ${e.saved ? 'saved' : 'falls'}`, tone: 'save', at: SAVE_MS } };
		}
		case 'no_pothole':
			return { key, text: 'no pothole this turn', tone: 'odd' };
	}
	return null;
}

/** The pill on the board for the first `shown` events of a turn: the latest dice event's, or null before the roll. */
export function dicePill(view: View, shown: number): DicePill | null {
	const first = firstDiceStep(view.last);
	for (let i = Math.min(shown, view.last.length) - 1; i >= first; i--) {
		const pill = pillFor(view, i);
		if (!pill) continue;
		const last = !view.last.slice(i + 1).some((e) => PILL_KINDS.includes(e.kind));
		return { ...pill, last };
	}
	return null;
}

/**
 * A die to throw: its value and color (grey for an odd pothole roll,
 * yellow for even, orange for the file and rank dice, cream for a saving
 * roll), when it is thrown and how long it tumbles, and its seed.
 */
export type DieTone = 'dull' | 'pot' | 'where' | 'save';
export interface DieSpec {
	value: number;
	tone: DieTone;
	delay: number;
	ms: number;
	seed: number;
}

/** The dice thrown for event `i` of a turn: one for a roll, two (file and rank) for a target, none otherwise. */
export function stepDice(view: View, i: number): DieSpec[] {
	const e = view.last[i];
	const seed = diceSeed(view.seq, i);
	switch (e.kind) {
		case 'rolled_pothole':
			return [{ value: e.roll ?? 0, tone: (e.roll ?? 1) % 2 === 1 ? 'dull' : 'pot', delay: 0, ms: THROW_MS, seed }];
		case 'target': {
			const sq = e.sq ?? 'a1';
			return [
				{ value: FILES.indexOf(sq[0]) + 1, tone: 'where', delay: 0, ms: FILE_MS, seed: seed + 1 },
				{ value: Number(sq[1]), tone: 'where', delay: RANK_DELAY_MS, ms: RANK_MS, seed: seed + 2 }
			];
		}
		case 'saving_roll':
			return [{ value: e.roll ?? 0, tone: 'save', delay: 0, ms: THROW_MS, seed }];
	}
	return [];
}
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `pnpm --dir web exec vitest run src/lib/dice.test.ts`
Expected: PASS (14 tests).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/dice.ts web/src/lib/dice.test.ts
git commit -m "web: dice helpers: per-step pacing, seeded tumbles, the scan path, die specs, the pill"
```

---

### Task 3: The Animator plays each step for its own time

**Files:**
- Modify: `web/src/lib/animator.svelte.ts`, `web/src/lib/animator.test.ts`, `web/src/lib/feel.ts` (a comment)
- Modify: `web/src/routes/game/[code]/+page.svelte`, `web/src/routes/dev/board/+page.svelte` (the Animator lines)

**Interfaces:**
- Consumes: `dicePace` (Task 2).
- Produces: `type Pace = number | ((shownLast: EventJSON | undefined) => number)`. `Animator.pace` replaces `stepMs`, and `new Animator()` paces from the table. `STEP_MS` is removed.

- [ ] **Step 1: Write the failing test**


Save this patch as `/tmp/t3-animator-test.patch` and apply it from the repository root with `git apply /tmp/t3-animator-test.patch`:

```diff
diff --git a/web/src/lib/animator.test.ts b/web/src/lib/animator.test.ts
index 62fe804..ed87080 100644
--- a/web/src/lib/animator.test.ts
+++ b/web/src/lib/animator.test.ts
@@ -103,7 +103,7 @@ describe('Animator', () => {
 	it('takes a new step time for the next turn', () => {
 		const a = new Animator(100);
 		a.receive(view(0));
-		a.stepMs = 1000;
+		a.pace = 1000;
 		a.receive(view(1, roll));
 		vi.advanceTimersByTime(999);
 		expect(a.shown).toBe(1);
@@ -111,3 +111,26 @@ describe('Animator', () => {
 		expect(a.shown).toBe(2);
 	});
 });
+
+describe('Animator with the dice table', () => {
+	beforeEach(() => vi.useFakeTimers());
+	afterEach(() => vi.useRealTimers());
+
+	it('waits for the move, then for each step as long as it plays', async () => {
+		const { MOVE_MS, playMs } = await import('./dice.ts');
+		const a = new Animator();
+		a.receive(view(0));
+		a.receive(view(1, roll));
+		expect(a.shown).toBe(1);
+		vi.advanceTimersByTime(MOVE_MS - 1);
+		expect(a.shown).toBe(1);
+		vi.advanceTimersByTime(1);
+		expect(a.shown).toBe(2); // the roll
+		vi.advanceTimersByTime(playMs(roll[1]));
+		expect(a.shown).toBe(3); // the target
+		vi.advanceTimersByTime(playMs(roll[2]) - 1);
+		expect(a.shown).toBe(3); // the scan still runs
+		vi.advanceTimersByTime(1);
+		expect(a.shown).toBe(4);
+	});
+});
```


- [ ] **Step 2: Run it to make sure it fails**

Run: `pnpm --dir web exec vitest run src/lib/animator.test.ts`
Expected: FAIL, two tests. The new test reveals the target too early (`expected 4 to be 3`), because steps still come every 550 ms. The existing "takes a new step time" test fails too, because `a.pace = 1000` has no effect yet (`pace` doesn't exist).

- [ ] **Step 3: Implement**


Save this patch as `/tmp/t3-animator.patch` and apply it from the repository root with `git apply /tmp/t3-animator.patch`:

```diff
diff --git a/web/src/lib/animator.svelte.ts b/web/src/lib/animator.svelte.ts
index ed6aa96..202b7d2 100644
--- a/web/src/lib/animator.svelte.ts
+++ b/web/src/lib/animator.svelte.ts
@@ -2,14 +2,16 @@
 // /dev/board sandbox, so the sandbox shows exactly what players see.
 
 import { firstDiceStep } from './board.ts';
-import type { View } from './game.ts';
+import { dicePace } from './dice.ts';
+import type { EventJSON, View } from './game.ts';
 
 /**
- * Time between dice steps; a whole roll takes about two seconds. The server
- * pauses the next clock for the roll using the same value (StepTime in
- * game/clock.go): change both together.
+ * How long to wait before showing the next dice step: a fixed number of ms,
+ * or a function of the step just shown (undefined before the roll). The
+ * game uses dicePace, from the table the server's clock pause shares, so a
+ * clock never starts while the dice still play.
  */
-export const STEP_MS = 550;
+export type Pace = number | ((shownLast: EventJSON | undefined) => number);
 
 export class Animator {
 	view = $state<View | null>(null);
@@ -22,12 +24,18 @@ export class Animator {
 	 * replays them.
 	 */
 	animated = $state(false);
-	/** Delay between steps, in ms; applies from the next step on. */
-	stepMs: number;
+	/** The wait before each step; applies from the next step on. */
+	pace: Pace;
 	#timer: ReturnType<typeof setTimeout> | undefined;
 
-	constructor(stepMs = STEP_MS) {
-		this.stepMs = stepMs;
+	constructor(pace: Pace = dicePace) {
+		this.pace = pace;
+	}
+
+	#wait(): number {
+		if (typeof this.pace === 'number') return this.pace;
+		const last = this.view?.last ?? [];
+		return this.pace(this.shown > firstDiceStep(last) ? last[this.shown - 1] : undefined);
 	}
 
 	get animating(): boolean {
@@ -72,6 +80,6 @@ export class Animator {
 		this.#timer = setTimeout(() => {
 			this.shown += 1;
 			this.#tick();
-		}, this.stepMs);
+		}, this.#wait());
 	}
 }
```



Save this patch as `/tmp/t3-feel.patch` and apply it from the repository root with `git apply /tmp/t3-feel.patch`:

```diff
diff --git a/web/src/lib/feel.ts b/web/src/lib/feel.ts
index b310990..dc49c31 100644
--- a/web/src/lib/feel.ts
+++ b/web/src/lib/feel.ts
@@ -79,7 +79,7 @@ export function rippleDelay(from: string, to: string): number {
 	return Math.round(15 * Math.hypot(df, dr));
 }
 
-/** How long a fall into a pothole plays: a teeter, then the drop. It fits one dice step (STEP_MS). */
+/** How long a fall into a pothole plays: a teeter, then the drop. It fits the fall's dice step (dice-timing.json). */
 export const FALL_MS = 550;
 
 /** A board shake: how far it moves and for how long. */
```


In `web/src/routes/game/[code]/+page.svelte`:
- change `import { Animator, STEP_MS } from '#lib/animator.svelte.ts';` to `import { Animator } from '#lib/animator.svelte.ts';`;
- change `const anim = new Animator(instant ? 0 : STEP_MS);` to `const anim = new Animator(instant ? 0 : undefined);`.

In `web/src/routes/dev/board/+page.svelte`:
- change the same import line, and add a new line after it, indented with one tab like the other imports: `import { dicePace } from '#lib/dice.ts';`;
- change `anim.stepMs = instant ? 0 : slow ? STEP_MS * 3 : STEP_MS;` to `anim.pace = instant ? 0 : slow ? (e) => dicePace(e) * 3 : dicePace;`.

- [ ] **Step 4: Run the tests and the check**

Run: `pnpm --dir web exec vitest run src/lib/animator.test.ts && pnpm --dir web exec svelte-check --threshold error`
Expected: PASS; `0 ERRORS`.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/animator.svelte.ts web/src/lib/animator.test.ts web/src/lib/feel.ts 'web/src/routes/game/[code]/+page.svelte' web/src/routes/dev/board/+page.svelte
git commit -m "web: the dice play each step for its own time, from the shared table"
```

---

### Task 4: Thrown dice in the tray and the phone's dice card

**Files:**
- Modify: `web/src/lib/Die.svelte` (rewrite), `web/src/lib/board.ts`, `web/src/lib/board.test.ts`, `web/src/lib/DiceTray.svelte`, `web/src/lib/DiceSummary.svelte`

**Interfaces:**
- Consumes: `stepDice`, `DieSpec`, `THROW_MS`, `SCAN_MS`, `SAVE_MS` and `tumbleFaces` (Task 2).
- Produces:
  - `DiceStep.dice` becomes `DieSpec[]`, plus `DiceStep.revealAt?`;
  - `DiceChip.dice?: DieSpec[]`; `DiceSummary.lineAt?`;
  - `<Die die={spec} small? />`.

- [ ] **Step 1: Update the tests to the die specs**


Save this patch as `/tmp/t4-board-test.patch` and apply it from the repository root with `git apply /tmp/t4-board-test.patch`:

```diff
diff --git a/web/src/lib/board.test.ts b/web/src/lib/board.test.ts
index 11e2fd9..b9aba61 100644
--- a/web/src/lib/board.test.ts
+++ b/web/src/lib/board.test.ts
@@ -204,8 +204,8 @@ describe('diceSteps', () => {
 			'Black knight falls in',
 			'Pothole opens on g8'
 		]);
-		expect(steps[0].dice).toEqual([2]);
-		expect(steps[1].dice).toEqual([5, 1]);
+		expect(steps[0].dice.map((d) => [d.value, d.tone])).toEqual([[2, 'pot']]);
+		expect(steps[1].dice.map((d) => [d.value, d.tone])).toEqual([[5, 'where'], [1, 'where']]);
 		expect(steps[1].detail).toBe('File 5 = e, rank 1. White king is there');
 		expect(steps[2].detail).toBe('Kings never fall');
 		expect(steps[3].detail).toBe('File 7 = g, rank 8. Black knight is there');
@@ -225,10 +225,12 @@ describe('diceSteps', () => {
 		];
 		const v = makeView({ d2: 'wP' }, { last: saved, mamdani: 'a5' });
 		const steps = diceSteps(v, saved.length);
-		expect(steps[2]).toMatchObject({ title: 'Odd. White pawn is saved', dice: [5], tone: 'good' });
+		expect(steps[2]).toMatchObject({ title: 'Odd. White pawn is saved', tone: 'good' });
+		expect(steps[2].dice.map((d) => [d.value, d.tone])).toEqual([[5, 'save']]);
 		expect(steps[2].detail).toBe('The Mamdani on a5 has a clear line to d2.');
 		const odd = makeView({}, { last: [saved[0], { kind: 'rolled_pothole', roll: 7, color: 'black' }] });
-		expect(diceSteps(odd, 2)[0]).toMatchObject({ title: 'Odd. No pothole', dice: [7], tone: 'muted' });
+		expect(diceSteps(odd, 2)[0]).toMatchObject({ title: 'Odd. No pothole', tone: 'muted' });
+		expect(diceSteps(odd, 2)[0].dice.map((d) => [d.value, d.tone])).toEqual([[7, 'dull']]);
 	});
 });
 
```


- [ ] **Step 2: Run them to make sure they fail**

Run: `pnpm --dir web exec vitest run src/lib/board.test.ts`
Expected: FAIL: `steps[0].dice.map` reads plain numbers (`d.value` is undefined), and `diceSteps(odd, 2)[0].dice` doesn't match `[[7, 'dull']]`.

- [ ] **Step 3: Give each step and chip its dice**


Save this patch as `/tmp/t4-board.patch` and apply it from the repository root with `git apply /tmp/t4-board.patch`:

```diff
diff --git a/web/src/lib/board.ts b/web/src/lib/board.ts
index 17527d2..c150cf8 100644
--- a/web/src/lib/board.ts
+++ b/web/src/lib/board.ts
@@ -1,6 +1,7 @@
 // Pure board logic for the game page: blocked lines, the board while the
 // dice play out, and the dice tray's lines. No DOM, so it is unit-tested.
 
+import { SAVE_MS, SCAN_MS, stepDice, THROW_MS, type DieSpec } from './dice.ts';
 import { pieceName, squareName, type Color, type EventJSON, type View } from './game.ts';
 
 export function squareIndex(name: string): number {
@@ -150,8 +151,18 @@ function fallsWith(last: EventJSON[], i: number): number {
 export interface DiceStep {
 	title: string;
 	detail: string;
-	dice: number[]; // d8 values shown as diamonds
+	dice: DieSpec[]; // the d8s thrown for this step
 	tone: 'normal' | 'muted' | 'good' | 'hazard';
+	/** When its line shows, in ms: what the dice decided waits for them to land. */
+	revealAt?: number;
+}
+
+/** When a step's outcome can show: once its dice have landed. */
+function revealAt(e: EventJSON): number | undefined {
+	if (e.kind === 'rolled_pothole') return THROW_MS;
+	if (e.kind === 'target') return SCAN_MS;
+	if (e.kind === 'saving_roll') return SAVE_MS;
+	return undefined;
 }
 
 const rerollReasons: Record<string, string> = {
@@ -180,8 +191,9 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 				steps.push({
 					title: even ? 'Even. A pothole opens' : 'Odd. No pothole',
 					detail: `d8 rolled ${e.roll}`,
-					dice: [e.roll ?? 0],
-					tone: even ? 'normal' : 'muted'
+					dice: stepDice(view, i),
+					tone: even ? 'normal' : 'muted',
+					revealAt: revealAt(e)
 				});
 				break;
 			}
@@ -192,7 +204,7 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 				const before = stageAt(view, i);
 				const occupant = sq === before.mamdani ? 'M' : before.board[squareIndex(sq)];
 				const there = occupant ? `${capitalize(pieceName(occupant))} is there` : 'Empty square';
-				steps.push({ title: `Square ${sq}`, detail: `File ${file} = ${sq[0]}, rank ${rank}. ${there}`, dice: [file, rank], tone: 'normal' });
+				steps.push({ title: `Square ${sq}`, detail: `File ${file} = ${sq[0]}, rank ${rank}. ${there}`, dice: stepDice(view, i), tone: 'normal', revealAt: revealAt(e) });
 				break;
 			}
 			case 'reroll':
@@ -206,8 +218,9 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 						e.piece === 'M'
 							? 'The Mamdani always gets a saving roll.'
 							: `The Mamdani on ${view.mamdani || stageAt(view, i).mamdani} has a clear line to ${e.sq}.`,
-					dice: [e.roll ?? 0],
-					tone: e.saved ? 'good' : 'hazard'
+					dice: stepDice(view, i),
+					tone: e.saved ? 'good' : 'hazard',
+					revealAt: revealAt(e)
 				});
 				break;
 			}
@@ -242,6 +255,8 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 export interface DiceChip {
 	text: string;
 	kind: 'die' | 'square' | 'pending' | 'good' | 'bad' | 'plain';
+	/** The d8s thrown for it: the roll, the file and rank dice, a saving die. */
+	dice?: DieSpec[];
 }
 
 /** The phone's one-line version of a turn's dice (the steps are in the moves sheet). */
@@ -250,6 +265,8 @@ export interface DiceSummary {
 	chips: DiceChip[]; // as far as the dice have played
 	line: string;
 	tone: 'normal' | 'good' | 'hazard' | 'muted';
+	/** When the line shows, in ms: once the latest step's dice land. */
+	lineAt?: number;
 }
 
 /**
@@ -270,18 +287,18 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 	let square = '';
 	let rolled = false;
 	let capped = ''; // a hole the cap closes, told after the new one opens
-	for (const e of view.last.slice(0, shown)) {
+	view.last.slice(0, shown).forEach((e, i) => {
 		switch (e.kind) {
 			case 'rolled_pothole': {
 				rolled = true;
 				const even = (e.roll ?? 1) % 2 === 0;
-				chips.push({ text: String(e.roll), kind: 'die' });
+				chips.push({ text: String(e.roll), kind: 'die', dice: stepDice(view, i) });
 				[line, tone, pending] = even ? ['Even: a pothole opens · finding its square…', 'normal', true] : ['Odd: nothing happens', 'muted', false];
 				break;
 			}
 			case 'target':
 				square = e.sq ?? '';
-				chips.push({ text: square, kind: 'square' });
+				chips.push({ text: square, kind: 'square', dice: stepDice(view, i) });
 				pending = false;
 				break;
 			case 'reroll': {
@@ -296,7 +313,7 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 				break;
 			}
 			case 'saving_roll':
-				chips.push({ text: `save ${e.roll}`, kind: e.saved ? 'good' : 'bad' });
+				chips.push({ text: `save ${e.roll}`, kind: e.saved ? 'good' : 'bad', dice: stepDice(view, i) });
 				if (e.saved) [line, tone] = [`${capitalize(pieceName(e.piece))} saved`, 'good'];
 				break;
 			case 'fell':
@@ -326,10 +343,12 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 				[line, tone, pending] = ['No pothole: no square could take one', 'muted', false];
 				break;
 		}
-	}
+	});
 	if (pending) chips.push({ text: '?', kind: 'pending' });
 	const roll = view.last.find((e) => e.kind === 'rolled_pothole');
-	return { who: roll?.color ?? mover, chips, line, tone };
+	const latest = view.last[Math.min(shown, view.last.length) - 1];
+	const lineAt = latest ? revealAt(latest) : undefined;
+	return { who: roll?.color ?? mover, chips, line, tone, ...(lineAt ? { lineAt } : {}) };
 }
 
 /** Whether the dice, not the move, delivered checkmate: a mating move ends the game before any roll. */
```


- [ ] **Step 4: Run the tests to make sure they pass**

Run: `pnpm --dir web exec vitest run`
Expected: PASS, all tests.

- [ ] **Step 5: Throw the dice**

Replace `web/src/lib/Die.svelte` with:


```svelte
<script lang="ts">
	import { untrack } from 'svelte';
	import { THROW_MS, tumbleFaces, type DieSpec } from './dice.ts';
	import { reducedMotion } from './motion.ts';

	// A d8 shown as a diamond, thrown into the tray: it flies in spinning,
	// bounces twice and settles, its face tumbling on the seeded schedule the
	// board's scan follows, slowing until it lands on its value.
	let { die, small = false }: { die: DieSpec; small?: boolean } = $props();

	let face = $state(0); // 0: landed, showing the value

	// Thrown once, when the die appears: the tray redraws its steps as the
	// roll plays, with equal specs, and must not throw it again.
	function roll(node: HTMLElement) {
		const d = untrack(() => die);
		if (reducedMotion()) return;
		const frames = tumbleFaces(d.value, d.ms, d.seed);
		face = frames[0].face;
		const timers = frames.map((f) => setTimeout(() => (face = f.at >= d.ms ? 0 : f.face), d.delay + f.at));
		node.animate(
			[
				{ transform: 'translate(-28px, -14px) rotate(-215deg) scale(0.7)', opacity: 0 },
				{ transform: 'translate(0, -8px) rotate(-15deg) scale(1)', opacity: 1, offset: 0.5 },
				{ transform: 'translate(0, 0) rotate(35deg)', offset: 0.72 },
				{ transform: 'translate(0, -3px) rotate(45deg)', offset: 0.86 },
				{ transform: 'translate(0, 0) rotate(45deg)' }
			],
			{ duration: THROW_MS, delay: d.delay, easing: 'ease-out', fill: 'backwards' }
		);
		return () => timers.forEach(clearTimeout);
	}
</script>

<!-- A pothole roll's die shows even (yellow) or odd only once it lands. -->
<span class="die {face && (die.tone === 'pot' || die.tone === 'dull') ? '' : die.tone}" class:small {@attach roll}>
	<!-- The tray is a live region: screen readers get the value, never the tumbling faces. -->
	<span aria-hidden="true">{face || die.value}</span>
	<span class="sr-only">{die.value}</span>
</span>

<style>
	.die {
		display: grid;
		place-items: center;
		width: 30px;
		height: 30px;
		margin: 6px;
		transform: rotate(45deg);
		background: var(--line);
		border-radius: 5px;
	}
	/* The phone's dice card: fits a 26px chip row. */
	.die.small {
		width: 22px;
		height: 22px;
		margin: 0 4px;
		border-radius: 4px;
	}
	.die.small span {
		font-size: 13px;
	}
	.die span {
		transform: rotate(-45deg);
		font-family: var(--font-mono);
		font-weight: 600;
		font-size: 15px;
		color: var(--text);
	}
	.die .sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
	/* What each die decides: an odd pothole roll is grey, an even one yellow;
	   the file and rank dice are orange; a saving die is cream. */
	.die.dull span {
		color: var(--text-muted);
	}
	.die.pot {
		background: var(--accent);
	}
	.die.where {
		background: var(--hazard);
	}
	.die.save {
		background: var(--piece-light);
	}
	.die.pot span,
	.die.where span,
	.die.save span {
		color: var(--accent-text);
	}
</style>
```



Save this patch as `/tmp/t4-tray.patch` and apply it from the repository root with `git apply /tmp/t4-tray.patch`:

```diff
diff --git a/web/src/lib/DiceTray.svelte b/web/src/lib/DiceTray.svelte
index 474b1c0..d02b7fd 100644
--- a/web/src/lib/DiceTray.svelte
+++ b/web/src/lib/DiceTray.svelte
@@ -40,11 +40,11 @@
 					{#if step.dice.length}
 						<span class="dice">
 							{#each step.dice as d, j (j)}
-								<Die value={d} highlight={step.tone === 'good'} />
+								<Die die={d} />
 							{/each}
 						</span>
 					{/if}
-					<span class="text">
+					<span class="text" class:late={step.revealAt} style="--late: {step.revealAt ?? 0}ms">
 						<span class="title">{step.title}</span>
 						<span class="detail">{step.detail}</span>
 					</span>
@@ -147,4 +147,18 @@
 	li.hazard .title {
 		color: var(--hazard-text);
 	}
+	/* What the dice decide shows once they land. */
+	.late {
+		animation: text-in 0.2s ease-out var(--late) both;
+	}
+	@keyframes text-in {
+		from {
+			opacity: 0;
+		}
+	}
+	@media (prefers-reduced-motion: reduce) {
+		.late {
+			animation: none;
+		}
+	}
 </style>
```



Save this patch as `/tmp/t4-summary.patch` and apply it from the repository root with `git apply /tmp/t4-summary.patch`:

```diff
diff --git a/web/src/lib/DiceSummary.svelte b/web/src/lib/DiceSummary.svelte
index 634bd1e..64b767c 100644
--- a/web/src/lib/DiceSummary.svelte
+++ b/web/src/lib/DiceSummary.svelte
@@ -1,5 +1,6 @@
 <script lang="ts">
 	import { diceSummary } from './board.ts';
+	import { SAVE_MS, SCAN_MS } from './dice.ts';
 	import Die from './Die.svelte';
 	import type { View } from './game.ts';
 
@@ -17,14 +18,22 @@
 		<span class="who" aria-hidden="true">{side}</span>
 		{#each summary.chips as chip, i (i)}
 			{#if i > 0}<span class="arrow" aria-hidden="true">→</span>{/if}
-			{#if chip.kind === 'die'}
-				<Die value={Number(chip.text)} small />
+			{#if chip.dice?.length}
+				<!-- The thrown dice, then what they decide once they land: the square, or saved / lost. -->
+				{#each chip.dice as d, j (j)}<Die die={d} small />{/each}
+				{#if chip.kind === 'square'}
+					<span class="chip square late" style="--late: {SCAN_MS}ms">{chip.text}</span>
+				{:else if chip.kind === 'good' || chip.kind === 'bad'}
+					<span class="chip {chip.kind} late" style="--late: {SAVE_MS}ms">{chip.kind === 'good' ? 'saved' : 'lost'}</span>
+				{/if}
 			{:else}
 				<span class="chip {chip.kind}">{chip.text}</span>
 			{/if}
 		{/each}
 	</div>
-	<p class="line {summary.tone}" aria-live="polite">{summary.line}</p>
+	<p class="line {summary.tone}" aria-live="polite">
+		{#key summary.line}<span class:late={summary.lineAt} style="--late: {summary.lineAt ?? 0}ms">{summary.line}</span>{/key}
+	</p>
 </section>
 
 <style>
@@ -111,4 +120,19 @@
 	.line.hazard {
 		color: var(--hazard-text);
 	}
+	/* What the dice decide shows once they land. */
+	.late {
+		animation: chip-in 0.2s ease-out var(--late) both;
+	}
+	@keyframes chip-in {
+		from {
+			opacity: 0;
+			transform: scale(0.6);
+		}
+	}
+	@media (prefers-reduced-motion: reduce) {
+		.late {
+			animation: none;
+		}
+	}
 </style>
```


- [ ] **Step 6: Check, autofix, and watch it**

Run: `pnpm --dir web exec svelte-check --threshold error`, then `npx @sveltejs/mcp svelte-autofixer` on `web/src/lib/Die.svelte`, `web/src/lib/DiceTray.svelte` and `web/src/lib/DiceSummary.svelte`.
Expected: `0 ERRORS`; no issues.

In `/dev/board`, pick "Pothole opens" with target d5 and move e2–e4. Watch the tray:
- A grey die flies in, tumbles and lands on 2, turning yellow; "Even. A pothole opens" then shows.
- Two orange dice follow, the second 150 ms later. They tumble, slowing, and land on 4 and 5; "Square d5" then shows.
- No die is thrown twice as later steps appear.

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/Die.svelte web/src/lib/board.ts web/src/lib/board.test.ts web/src/lib/DiceTray.svelte web/src/lib/DiceSummary.svelte
git commit -m "web: the dice are thrown into the tray and the phone card, coloured by what they decide"
```

---

### Task 5: The board: the scan, lit coordinates, the target waiting, the re-roll blink, the pill

**Files:**
- Modify: `web/src/lib/Board.svelte`
- Modify: `web/src/routes/game/[code]/+page.svelte`, `web/src/routes/dev/board/+page.svelte`

**Interfaces:**
- Consumes: `scanFrames`, `SCAN_MS`, `DicePill`, `dicePill` and `scanOf` (Task 2).
- Produces: Board props `pill?: DicePill | null`, `scan?: { sq, key, seed } | null` and `reroll?: boolean`.

- [ ] **Step 1: Draw them on the board**


Save this patch as `/tmp/t5-board.patch` and apply it from the repository root with `git apply /tmp/t5-board.patch`:

```diff
diff --git a/web/src/lib/Board.svelte b/web/src/lib/Board.svelte
index ba1be4f..914b812 100644
--- a/web/src/lib/Board.svelte
+++ b/web/src/lib/Board.svelte
@@ -3,7 +3,9 @@
 	import { pieceName, squareName, type MoveJSON } from './game.ts';
 	import { biggerShake, BURST_HOLD_MS, burstShards, captureShake, cellOf, coordinates, FALL_MS, fitShift, GLIDE_EASE, knockOffset, rippleDelay, SHAKE, shakeFrames, TRAIL_FADE_MS, trailColor, trailOf, type Shake, WHIP_TAIL_MS, whipFrames, whiplash } from './feel.ts';
 	import { lineParts, spoken } from './catchphrases.ts';
+	import { scanFrames, SCAN_MS, type DicePill } from './dice.ts';
 	import { exitMs, reducedMotion } from './motion.ts';
+	import { untrack } from 'svelte';
 	import { moveDuration, reconcile, type PieceRef } from './pieces.ts';
 
 	let {
@@ -18,6 +20,9 @@
 		repairs = [],
 		quip = null,
 		mated = null,
+		pill = null,
+		scan = null,
+		reroll = false,
 		onmove
 	}: {
 		stage: Stage;
@@ -40,6 +45,12 @@
 		quip?: { sq: string; emoji: string; line: string; key: string; delay?: number } | null;
 		/** The mated king's square, burst once per game (by key). */
 		mated?: { sq: string; key: string } | null;
+		/** The pill on the board's corner saying what the dice mean (dice.ts dicePill). */
+		pill?: DicePill | null;
+		/** The file and rank dice's scan for a target (dice.ts scanOf): it runs once per key. */
+		scan?: { sq: string; key: string; seed: number } | null;
+		/** The dice hit a square they must re-roll: the target blinks. */
+		reroll?: boolean;
 		onmove: (move: MoveJSON) => void;
 	} = $props();
 
@@ -449,6 +460,37 @@
 	function drop(_node: Element) {
 		return { duration: ms(260), css: (t: number) => `transform: scale(${1.5 - 0.5 * t}); opacity: ${t}` };
 	}
+
+	// The square the file and rank dice point to as they tumble; its letter
+	// and number glow on the coordinates outside the board.
+	let lit = $state('');
+	let landed = $state(''); // the scan whose dice have landed, by key
+	function scanner(node: HTMLElement) {
+		const s = untrack(() => scan);
+		if (!s) return;
+		const timers = scanFrames(s.sq, s.seed).map((f) =>
+			setTimeout(() => {
+				node.style.cssText = place(f.sq);
+				lit = f.sq;
+			}, f.at)
+		);
+		timers.push(
+			setTimeout(() => {
+				node.classList.add('landed');
+				landed = s.key;
+			}, SCAN_MS)
+		);
+		timers.push(setTimeout(() => (lit = ''), SCAN_MS + 700));
+		return () => {
+			timers.forEach(clearTimeout);
+			lit = '';
+		};
+	}
+
+	/** How long the turn's last pill stays: an odd roll's briefly, a square or a saving roll's a little longer. */
+	function pillHold(p: DicePill): number {
+		return p.tone === 'odd' && !p.then ? 600 : (p.then?.at ?? 0) + 1400;
+	}
 </script>
 
 {#snippet coneShape()}
@@ -460,7 +502,7 @@
 <!-- Coordinates sit outside the board, so the squares stay clean. The
      label column and row are the same size, so the frame stays square. -->
 <div class="frame" class:dim {@attach (node) => void (frameEl = node)}>
-<div class="ranks" aria-hidden="true">{#each labels.ranks as r (r)}<span>{r}</span>{/each}</div>
+<div class="ranks" aria-hidden="true">{#each labels.ranks as r (r)}<span class:on={lit[1] === r}>{r}</span>{/each}</div>
 <div class="board" class:dim class:late={!!mated && !playedBefore.has(mated.key)} role="group" aria-label="Chessboard" style="--glide-ease: {GLIDE_EASE}; --trail-fade: {TRAIL_FADE_MS}ms; --burst-hold: {BURST_HOLD_MS}ms" {@attach dragArea}>
 	{#each order as index (index)}
 		{@const sq = squareName(index)}
@@ -513,9 +555,15 @@
 				</span>
 			</span>
 		{/each}
-		{#if stage.target}
+		{#if scan && !reducedMotion()}
+			{#key scan.key}
+				<span class="slot scan" {@attach scanner}></span>
+			{/key}
+		{/if}
+		<!-- The target waits for the scan to land on it. -->
+		{#if stage.target && !(scan && scan.sq === stage.target && landed !== scan.key && !reducedMotion())}
 			{#key stage.target}
-				<span class="slot" style={place(stage.target)}><span class="target" in:drop></span></span>
+				<span class="slot" style={place(stage.target)}><span class="target" class:blink={reroll} in:drop></span></span>
 			{/key}
 		{/if}
 	</div>
@@ -604,6 +652,15 @@
 		{/if}
 	</div>
 
+	{#if pill}
+		{#key pill.key}
+			<span class="pills" class:last={pill.last} style="--at: {pill.at ?? 0}ms; --hold: {(pill.at ?? 0) + pillHold(pill)}ms; --then: {pill.then?.at ?? 0}ms" aria-hidden="true">
+				<span class="pill {pill.tone}" class:swap={!!pill.then}>{pill.text}</span>
+				{#if pill.then}<span class="pill {pill.then.tone} next">{pill.then.text}</span>{/if}
+			</span>
+		{/key}
+	{/if}
+
 	<!-- Screen readers hear the bubble's line; the bubble itself is drawn above. -->
 	<p class="sr-only" aria-live="polite">{quip ? spoken(quip.line) : ''}</p>
 
@@ -618,7 +675,7 @@
 		</div>
 	{/if}
 </div>
-<div class="files" aria-hidden="true">{#each labels.files as f (f)}<span>{f}</span>{/each}</div>
+<div class="files" aria-hidden="true">{#each labels.files as f (f)}<span class:on={lit[0] === f}>{f}</span>{/each}</div>
 </div>
 
 <style>
@@ -1453,4 +1510,107 @@
 			animation: bubble-out 0.25s ease-in calc(var(--quip-delay, 0ms) + 2.8s) forwards;
 		}
 	}
+
+	/* The dice on the board: the scan square follows the file and rank dice,
+	   the matching coordinates glow, the target blinks on a re-roll, and a
+	   pill in the corner says what the roll means. */
+	.ranks span.on,
+	.files span.on {
+		color: var(--hazard);
+		font-weight: 700;
+	}
+	.scan {
+		background: var(--hazard);
+		box-shadow: 0 0 18px 4px color-mix(in srgb, var(--hazard) 60%, transparent);
+		opacity: 0.85;
+		transition: opacity 0.25s ease;
+	}
+	.scan:global(.landed) {
+		opacity: 0;
+	}
+	.target.blink {
+		animation: target-blink 0.4s steps(2, jump-none) 2;
+	}
+	@keyframes target-blink {
+		50% {
+			opacity: 0.15;
+		}
+	}
+	.pills {
+		position: absolute;
+		top: 6px;
+		right: 6px;
+		z-index: 7;
+		display: grid;
+		justify-items: end;
+		pointer-events: none;
+		animation: pill-in 0.16s ease-out var(--at) both;
+	}
+	.pills.last {
+		animation:
+			pill-in 0.16s ease-out var(--at) both,
+			pill-out 0.22s ease-in var(--hold) forwards;
+	}
+	.pill {
+		grid-area: 1 / 1;
+		padding: 5px 9px;
+		border-radius: 999px;
+		font: 600 12px/1 var(--font-mono);
+		white-space: nowrap;
+		background: var(--surface);
+		color: var(--text);
+		box-shadow: 0 0 0 1px var(--line);
+	}
+	.pill.pot {
+		background: var(--accent);
+		color: var(--accent-text);
+		box-shadow: none;
+	}
+	.pill.where {
+		background: var(--hazard);
+		color: var(--accent-text);
+		box-shadow: none;
+	}
+	.pill.save {
+		background: var(--piece-light);
+		color: var(--piece-dark);
+		box-shadow: none;
+	}
+	.pill.swap {
+		animation: pill-out 0.15s ease-in var(--then) forwards;
+	}
+	.pill.next {
+		opacity: 0;
+		animation: pill-in 0.18s ease-out var(--then) forwards;
+	}
+	@keyframes pill-in {
+		from {
+			opacity: 0;
+			transform: scale(0.8);
+		}
+		to {
+			opacity: 1;
+		}
+	}
+	@keyframes pill-out {
+		to {
+			opacity: 0;
+		}
+	}
+	/* Reduced motion: no scan; the pill shows what the dice decided, then goes. */
+	@media (prefers-reduced-motion: reduce) {
+		.pills,
+		.pill.next {
+			animation: pill-in 1ms linear both;
+		}
+		.pill.swap {
+			animation: pill-out 1ms linear forwards;
+		}
+		.pills.last {
+			animation: pill-out 1ms linear var(--hold) forwards;
+		}
+		.target.blink {
+			animation: none;
+		}
+	}
 </style>
```


- [ ] **Step 2: Pass them in**

In `web/src/routes/game/[code]/+page.svelte`:
- add `import { dicePill, scanOf } from '#lib/dice.ts';` after the Animator import;
- after the `let repairs = ...` line, add:

```ts
	// The dice on the board: the pill in the corner, the file and rank dice's
	// scan, and the target blinking on a re-roll. Only on a turn that plays out.
	let pill = $derived(view && anim.animated && !instant ? dicePill(view, shown) : null);
	let scan = $derived(view && anim.animated && !instant ? scanOf(view, shown) : null);
	let reroll = $derived(!!view && animating && view.last[shown - 1]?.kind === 'reroll');
```

- pass `{pill}`, `{scan}` and `{reroll}` to both of its `<Board ... />` elements (phone and desktop), after `{mated}`.

In `web/src/routes/dev/board/+page.svelte`:
- change the dice import to `import { dicePace, dicePill, scanOf } from '#lib/dice.ts';`;
- after its `let repairs = ...` line, add:

```ts
	// The dice on the board, as in a game.
	let pill = $derived(anim.animated && !instant ? dicePill(view, anim.shown) : null);
	let scan = $derived(anim.animated && !instant ? scanOf(view, anim.shown) : null);
	let reroll = $derived(anim.animating && view.last[anim.shown - 1]?.kind === 'reroll');
```

- pass `{pill}`, `{scan}` and `{reroll}` to its `<Board ... />` after `{mated}`.

- [ ] **Step 3: Check and autofix**

Run: `pnpm --dir web exec svelte-check --threshold error`, then `npx @sveltejs/mcp svelte-autofixer web/src/lib/Board.svelte`.
Expected: `0 ERRORS`; no issues.

- [ ] **Step 4: Watch it in the sandbox**

- **Pothole opens on d5** (move e2–e4):
  - "d8 2 · pothole" appears in yellow once the die lands.
  - An orange square scans the board, following the dice's faces, with the matching file letter and rank number glowing orange outside the board. It settles on the d-file first, then lands on d5.
  - The pill turns orange and reads "d5", the dashed target drops, and the hole opens.
- **Odd** (nothing happens): a grey "d8 n · no pothole" pill shows for about 0.6 s, then fades.
- **"Re-roll on e1, then target"** (c5): the scan lands on e1, the target blinks twice, the pill reads "e1 · re-roll", then a second scan finds c5.
- **Saving roll: saved, on c7** (move e2–e4): the pill reads "Black pawn c7 · saving roll" in cream, then "… · saved".
- **With reduced motion:** no scan; the target shows at once.

- [ ] **Step 5: Run the suite and commit**

Run: `pnpm --dir web test`
Expected: PASS.

```bash
git add web/src/lib/Board.svelte 'web/src/routes/game/[code]/+page.svelte' web/src/routes/dev/board/+page.svelte
git commit -m "web: the scan follows the file and rank dice on the board; a pill says what the roll means"
```

---

### Task 6: Whole games, phones, and the PR's screenshots

**Files:** none (a check; fix and re-run if anything fails).

- [ ] **Step 1: Build and serve the production bundle**

Run: `pnpm --dir web build && go build -o /tmp/pc-server ./cmd/server && PORT=8097 DB_PATH=/tmp/pc.db /tmp/pc-server` (in the background).
Expected: the server is listening on :8097.

- [ ] **Step 2: Playtest with full animations**

```bash
pnpm --dir web playtest --base http://localhost:8097 --turn-ms 15000 --games 3 --max-plies 60
pnpm --dir web playtest --base http://localhost:8097 --turn-ms 15000 --games 3 --max-plies 60 --browser webkit
pnpm --dir web playtest --base http://localhost:8097 --turn-ms 15000 --games 3 --max-plies 60 --phone
```

Expected: `3/3 games finished cleanly` each.

- [ ] **Step 3: A phone game until a pothole roll; a reload mid-roll; reduced motion**

- **Phone game:** on two 393 × 760 windows, play until a turn opens a pothole. On the phone card, the yellow die, then the two orange dice, are thrown in step with the board's scan; the square shows once they land.
- **Reload mid-roll:** reload during a roll. The turn shows at once, with no pill and no scan.
- **Reduced motion:** the dice show their values at once and the target appears without a scan.

- [ ] **Step 4: Every suite**

Run: `pnpm --dir web check && pnpm --dir web test && pnpm --dir web build && go vet ./... && go test -race -short ./...`
Expected: all pass.

- [ ] **Step 5: Screenshots for the PR**

Capture the pothole roll (yellow die, scan, landed), the re-roll, the saving roll and the phone card. Push them to the `pr-screenshots` branch under `pr-<number>/` and embed them in the PR description.

