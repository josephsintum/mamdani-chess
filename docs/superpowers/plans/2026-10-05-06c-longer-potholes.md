# Milestone 06c: Longer Potholes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Potholes last 3 rounds with at most 5 open (the oldest closes), a roll can deliver checkmate, small traffic cones count each hole down, and the Mamdani's repairs are celebrated on the board, as the 06c spec describes.

**Architecture:**
- **`rules`:** `Position.Potholes` becomes a list of up to 5 holes, each with its roller, rounds left and opening order. A move counts down the mover's holes; opening a 6th closes the oldest; the checkmate re-roll goes.
- **`game`:** each pothole in the view and the live list carries `left`; `pothole_closed` carries its roller's `color`; the move log notes a cap close.
- **`store`:** migration 5 adds `games.rules`; old unfinished games end as `retired`, and only current-rules games are ever loaded.
- **Frontend:** the board model knows rounds left, the cap and repairs; `Board` draws cones and the repair celebration; the sandbox plays the new rules.
- **Docs:** the live rules doc, `RULES.md` and `CLAUDE.md`.

**Tech Stack:** Go 1.27, `modernc.org/sqlite` through `database/sql`, SvelteKit 3 / Svelte 5.57, Vitest, Playwright (playtest).

**Spec:** [docs/superpowers/specs/2026-10-05-06c-longer-potholes-design.md](../specs/2026-10-05-06c-longer-potholes-design.md). Read it first; this plan argues from it. The rules: [RULES.md](../../../RULES.md) (live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB).

**How this plan was made:** the whole milestone was built in a scratch copy first and checked: Go under `-race` and the full random-game suite, Vitest (129 tests), type checks, the Svelte autofixer, the production build, and whole games through the real UI (playtests in Chromium and WebKit, at phone size, and with full animations against a production build). A bug found by hand on the way (repaired holes stayed drawn on the board, Task 4) has a playtest check that was shown to catch it (Task 5). Each task below is one commit of that build, applied to `main` in order, and each commit was checked to compile and pass its tests on its own.

## Global Constraints

- **Rules constants:** `HoleRounds = 3`, `HoleCap = 5` in `rules/position.go`, mirrored as `HOLE_ROUNDS`, `HOLE_CAP` in `web/src/lib/board.ts`. Change both together.
- **No rule settings struct and no second engine:** games saved under the old rules are retired, not replayed (spec decision, option B).
- **Migrations are append only:** add migration 5; never edit 1–4.
- **JSON:** a pothole is `{"sq":"d4","by":"white","left":3}`; a close is `{"kind":"pothole_closed","sq":"c4","color":"black"}`. Nothing else in the protocol changes.
- **No new Go or npm dependencies.** Don't touch `go.mod`. Don't edit `names/` (it may hold uncommitted changes from another session; leave them alone).
- **Frontend:**
  - `#lib/...` imports with the file extension (no `$lib`), `$app/env`, not `$app/environment`.
  - No `$effect`.
  - Run `npx @sveltejs/mcp svelte-autofixer <file>` on every component you change. The game page's "use SvelteSet" suggestion is pre-existing and intended (a plain `Set` of streams that nothing renders).
  - Board transitions: never give one leaving item a 0 ms exit while others in the same update have timed exits (see Task 4).
- **Phones:** below 640 px the game page uses its phone layout and must never scroll (`playtest --phone` checks it).
- **Commits:** stage only the files you changed (`git add <paths>`, never `git add -A`), and add no Claude attribution (no `Co-Authored-By: Claude`, no "Generated with Claude Code").
- **Branch:** work on `m06c-longer-potholes`, not `main`. Every push to `main` deploys to Railway, and this deploy ends every game in progress (old rules), so it must not go out during a playtest.
- **The Rust server (`server_rs/`) is not changed.** Its parity tool still compiles but no longer matches; that's expected.

## Review Focus

1. **A turn whose leaving holes mix a repair with a timed close.** With Svelte 5.57, a 0 ms exit for the repaired holes left every leaving hole of that turn drawn on the board (found by hand in Safari, reproduced in Chromium and WebKit). Fixed with a 1 ms exit (Task 4). Pinned by the playtest's hole check (Task 5), run with full animations in Task 7; it caught the bug when it was put back.
2. **The cap closing a hole that shields the roller's own king.** The roller would end their own turn in check. The target is re-rolled with reason `exposes`. Pinned by `TestRerollIfCapCloseExposesRoller` (Task 1) and the random-game invariant that the player who just moved is never in check.
3. **A reload, reconnect or hidden tab mid-turn.** Celebrations must not replay. Pinned by the animator test "marks only a turn that plays out as animated" (Task 3); the pages pass repairs only when `anim.animated` (Task 4).
4. **Games saved before the deploy.** An open old game must end as `retired` and never be replayed (its turns would replay differently). Pinned by `TestMigration5RetiresOldGames` (Task 2).
5. **Five holes with cones on a phone.** The page must not scroll and the cones must still read at about 43 px squares. Pinned by `playtest --phone` (Task 7) and the screenshot check in Task 4.

## How to apply the diffs

Changes are unified diffs against the state the previous task leaves: save a block to a file and run `git apply <file>` from the repo root, or make the same edit by hand. Each diff applies cleanly on top of the tasks before it, in order, starting from `main` at the commit that adds this plan. The one binary file (the Mamdani piece image) is generated by a command in Task 4.

**The main checkout holds an uncommitted prototype** of Tasks 3–4 (Board, MiniBoard, sandbox, game.ts, the piece image) from the design session. This plan supersedes it. Work in a fresh worktree (`git worktree add ../m06c -b m06c-longer-potholes main`), and once the branch lands, ask the user before discarding the prototype in the main checkout (`git checkout -- web/` there, never touching `names/`).

## File map

| File | Task | Responsibility |
| --- | --- | --- |
| `rules/position.go`, `rules/play.go`, `rules/turn.go`, `rules/event.go`, `rules/movegen.go` | 1 | The hole list, countdown, cap, no mate re-roll, #16 on the last round, repetition key |
| `rules/*_test.go` | 1 | Helpers for holes; cap, mate-by-roll, #16, repetition, random-game invariants |
| `game/view.go`, `game/game.go`, `game/live.go` | 1 | `left` on potholes, `color` on closes, "c4 closes" in the move log |
| `store/migrate.go`, `store/games.go`, `game/clock.go` | 2 | Migration 5, `rules = 2` on new games, restore only those, `retired` |
| `web/src/lib/game.ts`, `board.ts`, `animator.svelte.ts`, `sandbox.ts`, `routes/+page.svelte` | 3 | Types, `stageAt` for the cap, text, `matedByRoll`, `repairsShown`, `animated`; the sandbox's rules |
| `web/src/lib/Board.svelte`, `MiniBoard.svelte`, `static/mamdani/piece.webp` | 4 | Cones, the repair celebration, tooltips, the Mamdani art |
| `web/src/routes/game/[code]/+page.svelte`, `routes/dev/board/+page.svelte` | 4 | Repairs wired in; "by a pothole"; the sandbox's target choice and Repair preset |
| `web/scripts/playtest.js` | 5 | Fail when a closed pothole stays drawn |
| `RULES.md`, `CLAUDE.md`, the live rules doc, the roadmap | 6, 7 | The new rules written down |

---

### Task 1: The engine: longer potholes, the cap, mates by roll

**Files:**
- Modify: `rules/position.go`, `rules/play.go`, `rules/turn.go`, `rules/event.go`, `rules/movegen.go` (comments)
- Modify (tests): `rules/helpers_test.go`, `rules/turn_test.go`, `rules/game_test.go`, `rules/random_test.go`, `rules/movegen_test.go`, `rules/board_test.go`, `rules/san_test.go`
- Modify (follow-through, so `game` compiles and carries the new data): `game/view.go`, `game/game.go`, `game/live.go`, `game/game_test.go`, `game/view_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `rules.HoleRounds = 3`, `rules.HoleCap = 5`.
  - `rules.Hole{Sq Square; By Color; Left int8; Seq uint32}`; `Sq == NoSquare` marks a free slot. `Position.Potholes [HoleCap]Hole`.
  - `ReasonCheckmate` is removed. `ReasonExposes` now also covers the cap closing a hole that shields the roller's king.
  - Events: a roll that opens a 6th hole emits `PotholeClosed` (with `Color` = the closed hole's roller) before `PotholeOpened`. Countdown closes after a move also carry `Color`.
  - `game.Pothole` gains `Left int \`json:"left"\``; `potholesJSON(p *rules.Position) []Pothole` lists holes oldest first (view and live list). `EventJSON` for `pothole_closed` carries `color`.
  - `describe` (the move log's dice text) adds `"<sq> closes"` for a close after the roll.

How it works (the spec's "Engine" section, as built):
- `play`: after the move, each of the mover's holes loses a round (`Left--`); at 0 it closes. Then `repair` clears every hole next to the Mamdani.
- `resolve` opens through `openHole`: with `HoleCap` open, the oldest (lowest `Seq`) closes first. `Seq` is one more than the newest open hole's, so a position depends only on what's on the board.
- `rerollReason`: no checkmate test. The "exposes" test removes the target's occupant and, if the cap would close a hole, that hole too, then asks whether the roller is in check.
- `threatened` (#16) ignores only the side to move's holes with `Left == 1`.
- `Key` lists holes in square order with roller and rounds left (`Seq` is left out: at most one hole opens per move, so roller plus rounds left fix the age order).

- [ ] **Step 1: Write the failing tests**

````diff
diff --git a/game/game_test.go b/game/game_test.go
index dd4577c..857422e 100644
--- a/game/game_test.go
+++ b/game/game_test.go
@@ -157,7 +157,7 @@ func TestPotholeShowsInView(t *testing.T) {
 		t.Fatal(err)
 	}
 	v := recv(t, a)
-	if !slices.Equal(v.Potholes, []Pothole{{Sq: "d4", By: "white"}}) {
+	if !slices.Equal(v.Potholes, []Pothole{{Sq: "d4", By: "white", Left: 3}}) {
 		t.Errorf("potholes %v", v.Potholes)
 	}
 	kinds := []rules.EventKind{}
@@ -173,6 +173,28 @@ func TestPotholeShowsInView(t *testing.T) {
 	}
 }
 
+func TestPotholesCountDownOldestFirst(t *testing.T) {
+	// White opens d4, Black opens h3, then White's move counts d4 down.
+	g := create(t, NewHub(&script{rolls: []int{2, 4, 4, 4, 8, 3}}, nil), "alice")
+	a := join(t, g, "alice")
+	join(t, g, "bob")
+	recv(t, a)
+	for i, m := range []struct{ guest, uci string }{{"alice", "e2e4"}, {"bob", "e7e5"}, {"alice", "g1f3"}} {
+		if err := g.Move(m.guest, mv(t, m.uci), i); err != nil {
+			t.Fatal(err)
+		}
+		recv(t, a)
+	}
+	v := recvView(t, g, "alice")
+	want := []Pothole{{Sq: "d4", By: "white", Left: 2}, {Sq: "h3", By: "black", Left: 3}}
+	if !slices.Equal(v.Potholes, want) {
+		t.Errorf("potholes %v, want %v", v.Potholes, want)
+	}
+	if l := g.live.Load(); !slices.Equal(l.Potholes, want) {
+		t.Errorf("live potholes %v, want %v", l.Potholes, want)
+	}
+}
+
 func TestCheckmateEndsTheGame(t *testing.T) {
 	g := create(t, NewHub(odd{}, nil), "alice")
 	a := join(t, g, "alice")
diff --git a/game/view_test.go b/game/view_test.go
index 9c0a8dd..38884ae 100644
--- a/game/view_test.go
+++ b/game/view_test.go
@@ -23,6 +23,9 @@ func TestEventJSONOmitsFieldsThatDontApply(t *testing.T) {
 		{rules.Event{Kind: rules.Moved, Move: rules.Move{From: rules.E7, To: rules.E8, Promo: rules.Queen}, Piece: rules.NewPiece(rules.White, rules.Pawn), Color: rules.White},
 			`{"kind":"moved","from":"e7","to":"e8","promo":"q","piece":"wP","color":"white"}`},
 		{rules.Event{Kind: rules.NoPothole}, `{"kind":"no_pothole"}`},
+		{rules.Event{Kind: rules.PotholeClosed, Square: rules.C4, Color: rules.Black},
+			`{"kind":"pothole_closed","sq":"c4","color":"black"}`},
+		{rules.Event{Kind: rules.Repaired, Square: rules.C4}, `{"kind":"repaired","sq":"c4"}`},
 	}
 	for _, c := range cases {
 		b, err := json.Marshal(eventJSON(c.e))
@@ -70,3 +73,19 @@ func TestDescribe(t *testing.T) {
 		t.Errorf("got  %q\nwant %q", got, want)
 	}
 }
+
+func TestDescribeShowsOnlyTheCapClosing(t *testing.T) {
+	// f5 runs out of rounds before the roll: every turn has those, so the
+	// log leaves it out. c4 is the cap making room for e4.
+	ev := []rules.Event{
+		{Kind: rules.Moved},
+		{Kind: rules.PotholeClosed, Square: rules.F5, Color: rules.White},
+		{Kind: rules.RolledPothole, Roll: 2},
+		{Kind: rules.Target, Square: rules.E4},
+		{Kind: rules.PotholeClosed, Square: rules.C4, Color: rules.Black},
+		{Kind: rules.PotholeOpened, Square: rules.E4, Color: rules.White},
+	}
+	if got, want := describe(ev), "d8 2 → e4 · c4 closes"; got != want {
+		t.Errorf("got  %q\nwant %q", got, want)
+	}
+}
diff --git a/rules/board_test.go b/rules/board_test.go
index 1a4c94c..645f2f0 100644
--- a/rules/board_test.go
+++ b/rules/board_test.go
@@ -40,7 +40,7 @@ func TestStartPosition(t *testing.T) {
 	if p.Board[E1] != NewPiece(White, King) || p.Board[D8] != NewPiece(Black, Queen) || p.Board[E4] != NoPiece {
 		t.Error("pieces are misplaced")
 	}
-	if p.Mamdani != A5 || p.Potholes != [2]Square{NoSquare, NoSquare} {
+	if p.Mamdani != A5 || p.Potholes != noHoles() {
 		t.Errorf("mamdani %v potholes %v", p.Mamdani, p.Potholes)
 	}
 	if p.Turn != White || p.Castling != WhiteKingside|WhiteQueenside|BlackKingside|BlackQueenside || p.EP != NoSquare || p.Fullmove != 1 {
diff --git a/rules/game_test.go b/rules/game_test.go
index 3bc279e..2b0ebc1 100644
--- a/rules/game_test.go
+++ b/rules/game_test.go
@@ -27,10 +27,10 @@ func TestFoolsMate(t *testing.T) {
 
 func TestMamdaniCanBlockMate(t *testing.T) {
 	const fen = "7k/8/8/8/8/8/6PP/r6K w - - 0 1" // back-rank check from a1
-	if g := NewGameFrom(setup(t, fen, D4, NoSquare, NoSquare)); g.Result.Over {
+	if g := NewGameFrom(setup(t, fen, D4)); g.Result.Over {
 		t.Error("Md1 blocks the check, so it is not mate")
 	}
-	g := NewGameFrom(setup(t, fen, NoSquare, NoSquare, NoSquare))
+	g := NewGameFrom(setup(t, fen, NoSquare))
 	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
 		t.Errorf("without the Mamdani it is mate: %+v", g.Result)
 	}
@@ -38,10 +38,10 @@ func TestMamdaniCanBlockMate(t *testing.T) {
 
 func TestStalemateCountsMamdaniMoves(t *testing.T) {
 	const fen = "k7/8/1Q6/8/8/8/8/7K b - - 0 1"
-	if g := NewGameFrom(setup(t, fen, H4, NoSquare, NoSquare)); g.Result.Over {
+	if g := NewGameFrom(setup(t, fen, H4)); g.Result.Over {
 		t.Error("Black can still move the Mamdani, so it is not stalemate")
 	}
-	g := NewGameFrom(setup(t, fen, NoSquare, NoSquare, NoSquare))
+	g := NewGameFrom(setup(t, fen, NoSquare))
 	if g.Result.Reason != Stalemate || !g.Result.Draw {
 		t.Errorf("want stalemate, got %+v", g.Result)
 	}
@@ -59,6 +59,24 @@ func TestThreefoldRepetition(t *testing.T) {
 	}
 }
 
+func TestRepetitionKeyCountsRoundsLeft(t *testing.T) {
+	const fen = "4k3/8/8/8/8/8/8/4K3 w - - 0 1"
+	base := setup(t, fen, NoSquare, hole(C4, White, 2), hole(F5, Black, 3))
+	// The same holes in other slots, opened in another order: same key.
+	if same := setup(t, fen, NoSquare, hole(F5, Black, 3), hole(C4, White, 2)); same.Key() != base.Key() {
+		t.Error("slot order and Seq changed the key")
+	}
+	for name, p := range map[string]Position{
+		"rounds left": setup(t, fen, NoSquare, hole(C4, White, 1), hole(F5, Black, 3)),
+		"roller":      setup(t, fen, NoSquare, hole(C4, Black, 2), hole(F5, Black, 3)),
+		"square":      setup(t, fen, NoSquare, hole(C3, White, 2), hole(F5, Black, 3)),
+	} {
+		if p.Key() == base.Key() {
+			t.Errorf("a different %s gave the same key", name)
+		}
+	}
+}
+
 func TestFiftyMoveRule(t *testing.T) {
 	p := StartPosition()
 	p.Halfmove = 99
@@ -81,7 +99,7 @@ func TestInsufficientMaterial(t *testing.T) {
 		{"4k3/8/8/8/8/8/8/3RK3 w - - 0 1", NoSquare, false},
 	}
 	for _, c := range cases {
-		g := NewGameFrom(setup(t, c.fen, c.mamdani, NoSquare, NoSquare))
+		g := NewGameFrom(setup(t, c.fen, c.mamdani))
 		if over := g.Result.Reason == InsufficientMaterial; over != c.over {
 			t.Errorf("%s mamdani %v: insufficient=%v, want %v", c.fen, c.mamdani, over, c.over)
 		}
@@ -132,13 +150,18 @@ func TestBoardMateIsNotUndoneByDice(t *testing.T) {
 	}
 }
 
-func TestCheckHeldOffOnlyByOwnPotholeIsMate(t *testing.T) {
-	// White's own hole on e4 blocks the e8 rook. Every white move closes it,
-	// and nothing can block the e-file or step off it: that is checkmate,
-	// not stalemate.
-	p := setup(t, "k3r3/8/8/8/8/8/3P1P2/3RKR2 w - - 0 1", NoSquare, E4, NoSquare)
-	g := NewGameFrom(p)
-	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
+func TestCheckHeldOffOnlyByOwnLastRoundPotholeIsMate(t *testing.T) {
+	// White's own hole on e4 blocks the e8 rook. On its last round, every
+	// white move closes it, and nothing can block the e-file or step off
+	// it: that is checkmate, not stalemate.
+	const fen = "k3r3/8/8/8/8/8/3P1P2/3RKR2 w - - 0 1"
+	g := NewGameFrom(setup(t, fen, NoSquare, hole(E4, White, 1)))
+	if want := (Result{Over: true, Winner: Black, Reason: Checkmate}); g.Result != want {
 		t.Errorf("result %+v, want Black wins by checkmate", g.Result)
 	}
+	// With two rounds left it outlasts White's next move: Ke2 is safe.
+	g = NewGameFrom(setup(t, fen, NoSquare, hole(E4, White, 2)))
+	if g.Result.Over || !legal(t, g.Pos, "e1e2") {
+		t.Errorf("result %+v, want the game on with Ke2 legal", g.Result)
+	}
 }
diff --git a/rules/helpers_test.go b/rules/helpers_test.go
index 572ae75..3a673af 100644
--- a/rules/helpers_test.go
+++ b/rules/helpers_test.go
@@ -6,18 +6,45 @@ import (
 )
 
 // setup parses fen and places the Mamdani (NoSquare for none) and the open
-// potholes rolled by White and Black (NoSquare for none).
-func setup(t *testing.T, fen string, mamdani, whiteHole, blackHole Square) Position {
+// potholes, oldest first.
+func setup(t *testing.T, fen string, mamdani Square, holes ...Hole) Position {
 	t.Helper()
 	p, err := ParseFEN(fen)
 	if err != nil {
 		t.Fatal(err)
 	}
 	p.Mamdani = mamdani
-	p.Potholes = [2]Square{whiteHole, blackHole}
+	for i, h := range holes {
+		h.Seq = uint32(i + 1)
+		p.Potholes[i] = h
+	}
 	return p
 }
 
+// hole is an open pothole on s rolled by c with left rounds to go.
+func hole(s Square, c Color, left int8) Hole { return Hole{Sq: s, By: c, Left: left} }
+
+// open returns the open potholes, oldest first.
+func open(p Position) []Hole {
+	var hs []Hole
+	for _, h := range p.Potholes {
+		if h.Sq != NoSquare {
+			hs = append(hs, h)
+		}
+	}
+	slices.SortFunc(hs, func(a, b Hole) int { return int(a.Seq) - int(b.Seq) })
+	return hs
+}
+
+// openSquares returns the open potholes' squares, oldest first.
+func openSquares(p Position) []Square {
+	var ss []Square
+	for _, h := range open(p) {
+		ss = append(ss, h.Sq)
+	}
+	return ss
+}
+
 func dice(rolls ...int) *ScriptedDice { return &ScriptedDice{Rolls: rolls} }
 
 func mv(t *testing.T, uci string) Move {
diff --git a/rules/movegen_test.go b/rules/movegen_test.go
index fbeeda2..11d553a 100644
--- a/rules/movegen_test.go
+++ b/rules/movegen_test.go
@@ -3,7 +3,7 @@ package rules
 import "testing"
 
 func TestSlidersStopAtPotholes(t *testing.T) {
-	p := setup(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", NoSquare, D1, NoSquare)
+	p := setup(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", NoSquare, hole(D1, White, 1))
 	for _, uci := range []string{"a1b1", "a1c1", "a1a8"} {
 		if !legal(t, p, uci) {
 			t.Errorf("%s should be legal", uci)
@@ -15,7 +15,7 @@ func TestSlidersStopAtPotholes(t *testing.T) {
 }
 
 func TestKnightsJumpPotholesButCannotLand(t *testing.T) {
-	p := setup(t, "4k3/8/8/8/8/8/8/1N2K3 w - - 0 1", NoSquare, C3, C2)
+	p := setup(t, "4k3/8/8/8/8/8/8/1N2K3 w - - 0 1", NoSquare, hole(C3, White, 1), hole(C2, Black, 1))
 	if !legal(t, p, "b1d2") || !legal(t, p, "b1a3") {
 		t.Error("knight should jump over the pothole on c2")
 	}
@@ -25,18 +25,18 @@ func TestKnightsJumpPotholesButCannotLand(t *testing.T) {
 }
 
 func TestPawnsAndPotholes(t *testing.T) {
-	p := setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, NoSquare, E3)
+	p := setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, hole(E3, Black, 1))
 	if legal(t, p, "e2e3") || legal(t, p, "e2e4") {
 		t.Error("pawn moved into or across a pothole on e3")
 	}
-	p = setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, NoSquare, E4)
+	p = setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, hole(E4, Black, 1))
 	if !legal(t, p, "e2e3") || legal(t, p, "e2e4") {
 		t.Error("pothole on e4 should block only the double step")
 	}
 }
 
 func TestPotholeBlocksCheck(t *testing.T) {
-	p := setup(t, "4k3/8/8/8/8/8/8/4K2r w - - 0 1", NoSquare, NoSquare, F1)
+	p := setup(t, "4k3/8/8/8/8/8/8/4K2r w - - 0 1", NoSquare, hole(F1, Black, 1))
 	if p.InCheck(White) {
 		t.Error("a pothole on f1 should block the rook's check")
 	}
@@ -44,19 +44,19 @@ func TestPotholeBlocksCheck(t *testing.T) {
 
 func TestCastlingAndPotholes(t *testing.T) {
 	const fen = "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1"
-	p := setup(t, fen, NoSquare, NoSquare, NoSquare)
+	p := setup(t, fen, NoSquare)
 	if !legal(t, p, "e1g1") || !legal(t, p, "e1c1") {
 		t.Fatal("both castles should be legal")
 	}
-	p = setup(t, fen, NoSquare, NoSquare, B1)
+	p = setup(t, fen, NoSquare, hole(B1, Black, 1))
 	if legal(t, p, "e1c1") || !legal(t, p, "e1g1") {
 		t.Error("a pothole on b1 should block only queenside castling")
 	}
-	p = setup(t, fen, NoSquare, NoSquare, F1)
+	p = setup(t, fen, NoSquare, hole(F1, Black, 1))
 	if legal(t, p, "e1g1") {
 		t.Error("a pothole on f1 should block kingside castling")
 	}
-	p = setup(t, fen, B1, NoSquare, NoSquare)
+	p = setup(t, fen, B1)
 	if legal(t, p, "e1c1") {
 		t.Error("the Mamdani on b1 should block queenside castling")
 	}
@@ -64,11 +64,11 @@ func TestCastlingAndPotholes(t *testing.T) {
 
 func TestPotholeCancelsEnPassant(t *testing.T) {
 	const fen = "4k3/8/8/3Pp3/8/8/8/4K3 w - e6 0 1"
-	p := setup(t, fen, NoSquare, NoSquare, NoSquare)
+	p := setup(t, fen, NoSquare)
 	if !legal(t, p, "d5e6") {
 		t.Fatal("en passant should be legal")
 	}
-	p = setup(t, fen, NoSquare, NoSquare, E6)
+	p = setup(t, fen, NoSquare, hole(E6, Black, 1))
 	if legal(t, p, "d5e6") {
 		t.Error("a pothole on e6 should cancel en passant")
 	}
@@ -95,7 +95,7 @@ func TestMamdaniCanGoStraightBack(t *testing.T) {
 }
 
 func TestMamdaniCannotUnpinOwnKing(t *testing.T) {
-	p := setup(t, "4k3/8/8/8/8/8/8/r3K3 w - - 0 1", C1, NoSquare, NoSquare)
+	p := setup(t, "4k3/8/8/8/8/8/8/r3K3 w - - 0 1", C1)
 	if !legal(t, p, "c1b1") || !legal(t, p, "c1d1") {
 		t.Error("the Mamdani may slide along the pin line")
 	}
@@ -106,18 +106,18 @@ func TestMamdaniCannotUnpinOwnKing(t *testing.T) {
 
 func TestMoveIllegalIfOwnPotholeClosingExposesKing(t *testing.T) {
 	const fen = "k3r3/8/8/8/8/8/8/4K3 w - - 0 1"
-	p := setup(t, fen, NoSquare, E4, NoSquare)
+	p := setup(t, fen, NoSquare, hole(E4, White, 1))
 	if got := len(p.LegalMoves()); got != 4 || legal(t, p, "e1e2") {
 		t.Errorf("White's own pothole closes after this move, so only king moves off the e-file are legal; got %v", p.LegalMoves())
 	}
-	p = setup(t, fen, NoSquare, NoSquare, E4)
+	p = setup(t, fen, NoSquare, hole(E4, Black, 1))
 	if !legal(t, p, "e1e2") {
 		t.Error("Black's pothole stays open, so e1e2 is safe")
 	}
 }
 
 func TestMoveIllegalIfRepairExposesKing(t *testing.T) {
-	p := setup(t, "k3r3/8/8/8/8/8/8/4K3 w - - 0 1", H3, NoSquare, E4)
+	p := setup(t, "k3r3/8/8/8/8/8/8/4K3 w - - 0 1", H3, hole(E4, Black, 1))
 	if legal(t, p, "h3f3") || legal(t, p, "h3f5") {
 		t.Error("moving the Mamdani next to e4 repairs it and exposes the king")
 	}
diff --git a/rules/random_test.go b/rules/random_test.go
index 74032d0..cf5f62b 100644
--- a/rules/random_test.go
+++ b/rules/random_test.go
@@ -3,6 +3,7 @@ package rules
 import (
 	"fmt"
 	"math/rand/v2"
+	"slices"
 	"testing"
 )
 
@@ -36,10 +37,11 @@ func playRandomGame(t *testing.T, seed int) {
 		moves := g.Pos.LegalMoves()
 		m := moves[r.IntN(len(moves))]
 		before := g.Pos
-		if _, err := g.Play(m, d); err != nil {
+		ev, err := g.Play(m, d)
+		if err != nil {
 			t.Fatalf("seed %d ply %d %s: %v", seed, ply, m, err)
 		}
-		checkInvariants(t, seed, ply, before, m, g)
+		checkInvariants(t, seed, ply, before, m, ev, g)
 	}
 	replayed, err := Replay(StartPosition(), g.Turns)
 	if err != nil {
@@ -50,7 +52,7 @@ func playRandomGame(t *testing.T, seed int) {
 	}
 }
 
-func checkInvariants(t *testing.T, seed, ply int, before Position, m Move, g *Game) {
+func checkInvariants(t *testing.T, seed, ply int, before Position, m Move, ev []Event, g *Game) {
 	t.Helper()
 	p := &g.Pos
 	fail := func(format string, args ...any) {
@@ -69,11 +71,20 @@ func checkInvariants(t *testing.T, seed, ply int, before Position, m Move, g *Ga
 	if p.King(White) == NoSquare || p.King(Black) == NoSquare {
 		fail("a king fell")
 	}
-	for _, s := range p.Potholes {
-		if s != NoSquare && (p.Board[s] != NoPiece || s == p.Mamdani) {
-			fail("something stands on the pothole at %v", s)
+	for _, h := range p.Potholes {
+		if h.Sq == NoSquare {
+			continue
+		}
+		if p.Board[h.Sq] != NoPiece || h.Sq == p.Mamdani {
+			fail("something stands on the pothole at %v", h.Sq)
+		}
+		if h.Left < 1 || h.Left > HoleRounds {
+			fail("the pothole at %v has %d rounds left", h.Sq, h.Left)
 		}
 	}
+	if err := checkHoles(before, ev, p); err != nil {
+		fail("%v", err)
+	}
 	if p.Mamdani != NoSquare && p.Board[p.Mamdani] != NoPiece {
 		fail("the Mamdani shares %v with a piece", p.Mamdani)
 	}
@@ -88,10 +99,78 @@ func checkInvariants(t *testing.T, seed, ply int, before Position, m Move, g *Ga
 	if q.Mated() && g.Result.Reason != Checkmate {
 		fail("the dice undid a checkmate made on the board")
 	}
-	if g.Result.Reason == Checkmate {
-		// A roll never wins: the move alone must already have been mate.
-		if !q.Mated() {
-			fail("the dice delivered checkmate")
+}
+
+// checkHoles accounts for every hole across one turn: each of the mover's
+// holes loses a round and closes at zero, the other player's stay as they
+// were, and any other close is a repair or the cap pushing out the oldest
+// to make room for a new hole. At most one hole opens, the mover's, with
+// every round to go.
+func checkHoles(before Position, ev []Event, after *Position) error {
+	mover := before.Turn
+	rolled := false
+	var counted, repaired, capped, opened []Square
+	for _, e := range ev {
+		switch e.Kind {
+		case RolledPothole:
+			rolled = true
+		case PotholeClosed:
+			if rolled {
+				capped = append(capped, e.Square)
+			} else {
+				counted = append(counted, e.Square)
+			}
+		case Repaired:
+			repaired = append(repaired, e.Square)
+		case PotholeOpened:
+			opened = append(opened, e.Square)
+		}
+	}
+	if len(opened) > 1 || len(capped) > 1 {
+		return fmt.Errorf("opened %v and capped %v in one turn", opened, capped)
+	}
+	if len(capped) == 1 && len(opened) == 0 {
+		return fmt.Errorf("the cap closed %v but nothing opened", capped[0])
+	}
+	oldest, count := before.oldestHole()
+	for _, h := range before.Potholes {
+		if h.Sq == NoSquare {
+			continue
+		}
+		want := h
+		if h.By == mover {
+			want.Left--
+		}
+		switch {
+		case want.Left == 0:
+			if !slices.Contains(counted, h.Sq) {
+				return fmt.Errorf("%v reached its last round but did not close", h.Sq)
+			}
+		case slices.Contains(counted, h.Sq):
+			return fmt.Errorf("%v closed with %d rounds left", h.Sq, want.Left)
+		case slices.Contains(repaired, h.Sq):
+		case slices.Contains(capped, h.Sq):
+			if count < HoleCap || h != before.Potholes[oldest] {
+				return fmt.Errorf("the cap closed %v, not the oldest of %d", h.Sq, count)
+			}
+		case !slices.Contains(after.Potholes[:], want):
+			return fmt.Errorf("%v went from %+v to missing or changed", h.Sq, h)
+		}
+	}
+	for _, s := range opened {
+		i := slices.IndexFunc(after.Potholes[:], func(h Hole) bool { return h.Sq == s })
+		if i < 0 {
+			return fmt.Errorf("%v opened but is not in the list", s)
+		}
+		h := after.Potholes[i]
+		if h.By != mover || h.Left != HoleRounds {
+			return fmt.Errorf("the new hole %+v is not the mover's with every round", h)
+		}
+		for _, o := range after.Potholes {
+			if o.Sq != NoSquare && o.Sq != s && o.Seq >= h.Seq {
+				return fmt.Errorf("the new hole %+v is not the newest: %+v", h, o)
+			}
 		}
 	}
+	return nil
 }
diff --git a/rules/san_test.go b/rules/san_test.go
index b6537a1..ac43ccb 100644
--- a/rules/san_test.go
+++ b/rules/san_test.go
@@ -22,7 +22,7 @@ func TestSAN(t *testing.T) {
 		{"rnbqkbnr/pppp1ppp/8/4p3/6P1/5P2/PPPPP2P/RNBQKBNR b KQkq g3 0 2", NoSquare, "d8h4", "Qh4#"},
 	}
 	for _, c := range cases {
-		p := setup(t, c.fen, c.mamdani, NoSquare, NoSquare)
+		p := setup(t, c.fen, c.mamdani)
 		if got := p.SAN(mv(t, c.uci)); got != c.want {
 			t.Errorf("%s %s: got %q, want %q", c.fen, c.uci, got, c.want)
 		}
diff --git a/rules/turn_test.go b/rules/turn_test.go
index 8ba4f2b..40ae691 100644
--- a/rules/turn_test.go
+++ b/rules/turn_test.go
@@ -11,8 +11,8 @@ func TestOddRollNoPothole(t *testing.T) {
 	if want := []EventKind{Moved, RolledPothole}; !slices.Equal(kinds(ev), want) {
 		t.Errorf("events %v, want %v", kinds(ev), want)
 	}
-	if p.Potholes != [2]Square{NoSquare, NoSquare} || p.Turn != Black {
-		t.Errorf("potholes %v turn %v", p.Potholes, p.Turn)
+	if len(open(p)) != 0 || p.Turn != Black {
+		t.Errorf("potholes %v turn %v", open(p), p.Turn)
 	}
 }
 
@@ -29,36 +29,124 @@ func TestIllegalMovesRejected(t *testing.T) {
 			t.Errorf("%s: got %v, want ErrIllegalMove", uci, err)
 		}
 	}
-	p := setup(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare, NoSquare, NoSquare)
+	p := setup(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare)
 	if _, _, err := Apply(p, mv(t, "a7a8"), dice()); !errors.Is(err, ErrIllegalMove) {
 		t.Errorf("promotion without a piece: got %v, want ErrIllegalMove", err)
 	}
 }
 
-func TestPotholeOpensAndClosesAfterRollersNextMove(t *testing.T) {
+func TestPotholeClosesAfterRollersThirdMove(t *testing.T) {
 	// Even roll, then file 4 rank 4: d4.
 	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 4, 4))
 	if want := []EventKind{Moved, RolledPothole, Target, PotholeOpened}; !slices.Equal(kinds(ev), want) {
 		t.Fatalf("events %v, want %v", kinds(ev), want)
 	}
-	if p.Potholes[White] != D4 {
-		t.Fatalf("white pothole %v, want d4", p.Potholes[White])
+	if e, _ := find(ev, PotholeOpened); e.Color != White {
+		t.Errorf("opened by %v, want White", e.Color)
 	}
-	p, _ = apply(t, p, "g8f6", dice(1))
-	if p.Potholes[White] != D4 {
-		t.Fatal("pothole closed before White's next move")
+	// Only White's moves count it down; Black's leave it alone.
+	for i, step := range []struct {
+		uci  string
+		left int8
+	}{
+		{"g8f6", 3}, {"g1f3", 2}, {"b8c6", 2}, {"f1c4", 1}, {"c6b8", 1},
+	} {
+		p, ev = apply(t, p, step.uci, dice(1))
+		if got := open(p); len(got) != 1 || got[0] != (Hole{Sq: D4, By: White, Left: step.left, Seq: 1}) {
+			t.Fatalf("after move %d (%s): holes %v, want d4 with %d left", i+1, step.uci, got, step.left)
+		}
+		if _, ok := find(ev, PotholeClosed); ok {
+			t.Fatalf("%s closed a hole: %v", step.uci, ev)
+		}
+	}
+	p, ev = apply(t, p, "b1c3", dice(1))
+	if want := []EventKind{Moved, PotholeClosed, RolledPothole}; !slices.Equal(kinds(ev), want) {
+		t.Fatalf("events %v, want %v", kinds(ev), want)
 	}
-	p, ev = apply(t, p, "g1f3", dice(1))
-	if e, ok := find(ev, PotholeClosed); !ok || e.Square != D4 || p.Potholes[White] != NoSquare {
-		t.Errorf("pothole should close after White's next move: %v", ev)
+	if e := ev[1]; e.Square != D4 || e.Color != White || len(open(p)) != 0 {
+		t.Errorf("d4 should close after White's third move: %+v, open %v", e, open(p))
 	}
 }
 
-func TestTwoPotholesAtOnce(t *testing.T) {
+func TestBothPlayersHolesOpenAtOnce(t *testing.T) {
 	p, _ := apply(t, StartPosition(), "e2e4", dice(2, 4, 4)) // White: d4
 	p, _ = apply(t, p, "e7e5", dice(4, 8, 3))                // Black: h3
-	if p.Potholes != [2]Square{D4, H3} {
-		t.Errorf("potholes %v, want [d4 h3]", p.Potholes)
+	want := []Hole{{Sq: D4, By: White, Left: 3, Seq: 1}, {Sq: H3, By: Black, Left: 3, Seq: 2}}
+	if got := open(p); !slices.Equal(got, want) {
+		t.Errorf("holes %v, want %v", got, want)
+	}
+}
+
+// fiveHoles is the start position with HoleCap holes open, none next to
+// the Mamdani or on the a5-d2 diagonal, and none on its last round. The
+// oldest, on d5, sits in a middle slot, so the cap must go by Seq.
+func fiveHoles() Position {
+	p := StartPosition()
+	p.Potholes = [HoleCap]Hole{
+		{Sq: F4, By: White, Left: 2, Seq: 2},
+		{Sq: G5, By: Black, Left: 3, Seq: 5},
+		{Sq: D5, By: Black, Left: 2, Seq: 1},
+		{Sq: H4, By: White, Left: 3, Seq: 4},
+		{Sq: C6, By: Black, Left: 2, Seq: 3},
+	}
+	return p
+}
+
+func TestCapClosesOldest(t *testing.T) {
+	p, ev := apply(t, fiveHoles(), "e2e3", dice(2, 4, 4)) // d4
+	want := []EventKind{Moved, RolledPothole, Target, PotholeClosed, PotholeOpened}
+	if !slices.Equal(kinds(ev), want) {
+		t.Fatalf("events %v, want %v", kinds(ev), want)
+	}
+	if e := ev[3]; e.Square != D5 || e.Color != Black {
+		t.Errorf("closed %+v, want Black's d5", e)
+	}
+	if e := ev[4]; e.Square != D4 || e.Color != White {
+		t.Errorf("opened %+v, want White's d4", e)
+	}
+	if got, want := openSquares(p), []Square{F4, C6, H4, G5, D4}; !slices.Equal(got, want) {
+		t.Errorf("holes %v, want %v", got, want)
+	}
+	if got := open(p)[4]; got.Left != HoleRounds || got.By != White {
+		t.Errorf("new hole %+v, want White's with %d rounds", got, HoleRounds)
+	}
+}
+
+func TestNothingOpensSoCapClosesNothing(t *testing.T) {
+	for name, rolls := range map[string][]int{
+		"saved":    {2, 4, 2, 5}, // the d2 pawn, saved
+		"repaired": {2, 2, 4},    // b4, next to the Mamdani
+	} {
+		before := fiveHoles()
+		p, ev := apply(t, before, "e2e4", dice(rolls...))
+		if _, ok := find(ev, PotholeClosed); ok {
+			t.Errorf("%s: a hole closed: %v", name, ev)
+		}
+		if got, want := openSquares(p), openSquares(before); !slices.Equal(got, want) {
+			t.Errorf("%s: holes %v, want %v", name, got, want)
+		}
+	}
+}
+
+func TestRerollIfCapCloseExposesRoller(t *testing.T) {
+	// Black's oldest hole on e4 shields the white king from the e8 rook.
+	// Any hole that opens now pushes it out, so only a target the Mamdani
+	// repairs at once (a1, next to b2) leaves White safe.
+	const fen = "k3r3/8/8/8/8/8/7P/4K3 w - - 0 1"
+	holes := []Hole{hole(E4, Black, 2), hole(B6, White, 3), hole(C6, Black, 3), hole(G6, White, 2), hole(H6, Black, 2)}
+	p := setup(t, fen, B2, holes...)
+	p, ev := apply(t, p, "h2h3", dice(2, 8, 1, 1, 1)) // h1, then a1
+	if e, ok := find(ev, Reroll); !ok || e.Square != H1 || e.Reason != ReasonExposes {
+		t.Fatalf("want an exposes re-roll on h1: %v", ev)
+	}
+	if e, ok := find(ev, Repaired); !ok || e.Square != A1 || !p.IsPothole(E4) {
+		t.Errorf("a1 should be repaired at once and e4 stay open: %v", ev)
+	}
+	// With room under the cap, h1 opens and e4 stays.
+	p = setup(t, fen, B2, holes[:4]...)
+	p, ev = apply(t, p, "h2h3", dice(2, 8, 1))
+	if e, ok := find(ev, PotholeOpened); !ok || e.Square != H1 || !p.IsPothole(E4) {
+		t.Errorf("h1 should open with four holes open: %v", ev)
 	}
 }
 
@@ -74,7 +162,7 @@ func TestRerollKing(t *testing.T) {
 
 func TestRerollExistingPothole(t *testing.T) {
 	p := StartPosition()
-	p.Potholes[Black] = D4
+	p.Potholes[0] = hole(D4, Black, 2)
 	_, ev := apply(t, p, "e2e4", dice(6, 4, 4, 8, 3)) // d4, then h3
 	if e, ok := find(ev, Reroll); !ok || e.Reason != ReasonPothole {
 		t.Errorf("want a pothole re-roll: %v", ev)
@@ -97,8 +185,8 @@ func TestNextToMamdaniRepairedOnOpening(t *testing.T) {
 	if e, ok := find(ev, Repaired); !ok || e.Square != B4 {
 		t.Errorf("want b4 repaired at once: %v", ev)
 	}
-	if p.Potholes != [2]Square{NoSquare, NoSquare} {
-		t.Errorf("no pothole should stay open: %v", p.Potholes)
+	if len(open(p)) != 0 {
+		t.Errorf("no pothole should stay open: %v", open(p))
 	}
 }
 
@@ -110,8 +198,8 @@ func TestPieceWithoutMamdaniLineFalls(t *testing.T) {
 	if _, ok := find(ev, SavingRoll); ok {
 		t.Error("no saving roll without a clear Mamdani line")
 	}
-	if p.Board[G8] != NoPiece || p.Potholes[White] != G8 || p.Halfmove != 0 {
-		t.Errorf("board %v pothole %v halfmove %d", p.Board[G8], p.Potholes[White], p.Halfmove)
+	if p.Board[G8] != NoPiece || !slices.Equal(openSquares(p), []Square{G8}) || p.Halfmove != 0 {
+		t.Errorf("board %v potholes %v halfmove %d", p.Board[G8], openSquares(p), p.Halfmove)
 	}
 }
 
@@ -123,7 +211,7 @@ func TestSavingRoll(t *testing.T) {
 	if !ok || !e.Saved || e.Color != White || e.Roll != 5 {
 		t.Fatalf("want a successful white saving roll: %v", ev)
 	}
-	if p.Board[D2] != NewPiece(White, Pawn) || p.Potholes[White] != NoSquare {
+	if p.Board[D2] != NewPiece(White, Pawn) || len(open(p)) != 0 {
 		t.Error("a saved piece stays and no pothole opens")
 	}
 
@@ -131,7 +219,7 @@ func TestSavingRoll(t *testing.T) {
 	if e, _ := find(ev, SavingRoll); e.Saved {
 		t.Fatal("even saving roll should fail")
 	}
-	if p.Board[D2] != NoPiece || p.Potholes[White] != D2 {
+	if p.Board[D2] != NoPiece || !slices.Equal(openSquares(p), []Square{D2}) {
 		t.Error("the pawn falls and the pothole opens")
 	}
 }
@@ -149,8 +237,8 @@ func TestMamdaniFalls(t *testing.T) {
 	if e, ok := find(ev, Fell); !ok || e.Piece != MamdaniPiece {
 		t.Fatalf("Mamdani should fall: %v", ev)
 	}
-	if p.Mamdani != NoSquare || p.Potholes[White] != A5 {
-		t.Fatalf("mamdani %v pothole %v", p.Mamdani, p.Potholes[White])
+	if p.Mamdani != NoSquare || !slices.Equal(openSquares(p), []Square{A5}) {
+		t.Fatalf("mamdani %v potholes %v", p.Mamdani, openSquares(p))
 	}
 	// With the Mamdani gone there are no more saving rolls: a7 falls at once.
 	_, ev = apply(t, p, "e7e5", dice(2, 1, 7))
@@ -161,31 +249,48 @@ func TestMamdaniFalls(t *testing.T) {
 
 func TestRerollIfFallExposesRoller(t *testing.T) {
 	// The e2 bishop shields the white king from the e8 rook.
-	p := setup(t, "k3r3/8/8/8/8/8/4B2P/4K3 w - - 0 1", NoSquare, NoSquare, NoSquare)
+	p := setup(t, "k3r3/8/8/8/8/8/4B2P/4K3 w - - 0 1", NoSquare)
 	_, ev := apply(t, p, "h2h3", dice(2, 5, 2, 8, 8)) // e2, then h8
 	if e, ok := find(ev, Reroll); !ok || e.Square != E2 || e.Reason != ReasonExposes {
 		t.Errorf("want an exposes re-roll on e2: %v", ev)
 	}
 }
 
-func TestRerollIfResultWouldCheckmate(t *testing.T) {
-	// Ra8+ is answered only by the c7 knight (Nxa8 or Ne8). If it fell,
-	// Black would be mated by the roll, so the dice go again.
-	p := setup(t, "7k/2n3pp/8/8/8/8/8/R5K1 w - - 0 1", NoSquare, NoSquare, NoSquare)
-	p, ev := apply(t, p, "a1a8", dice(2, 3, 7, 1, 1)) // c7, then a1
-	if e, ok := find(ev, Reroll); !ok || e.Square != C7 || e.Reason != ReasonCheckmate {
-		t.Errorf("want a checkmate re-roll on c7: %v", ev)
+func TestRollMatesOnLastEscapeSquare(t *testing.T) {
+	// Nf7+ leaves the h8 king one way out, g8. The roll opens a hole there.
+	g := NewGameFrom(setup(t, "7k/6pp/8/6N1/8/8/8/4K3 w - - 0 1", NoSquare))
+	ev, err := g.Play(mv(t, "g5f7"), dice(2, 7, 8)) // g8
+	if err != nil {
+		t.Fatal(err)
+	}
+	if e, ok := find(ev, PotholeOpened); !ok || e.Square != G8 {
+		t.Fatalf("g8 should open: %v", ev)
+	}
+	if want := (Result{Over: true, Winner: White, Reason: Checkmate}); g.Result != want {
+		t.Errorf("result %+v, want White wins by checkmate", g.Result)
+	}
+}
+
+func TestRollMatesWhenOnlyDefenderFalls(t *testing.T) {
+	// Ra8+ is answered only by the c7 knight (Nxa8 or Ne8). It falls.
+	g := NewGameFrom(setup(t, "7k/2n3pp/8/8/8/8/8/R5K1 w - - 0 1", NoSquare))
+	ev, err := g.Play(mv(t, "a1a8"), dice(2, 3, 7)) // c7
+	if err != nil {
+		t.Fatal(err)
+	}
+	if e, ok := find(ev, Fell); !ok || e.Square != C7 {
+		t.Fatalf("the c7 knight should fall: %v", ev)
 	}
-	if p.Board[C7] != NewPiece(Black, Knight) {
-		t.Error("the knight must survive")
+	if want := (Result{Over: true, Winner: White, Reason: Checkmate}); g.Result != want {
+		t.Errorf("result %+v, want White wins by checkmate", g.Result)
 	}
 }
 
 func TestRepairStepAfterMove(t *testing.T) {
 	p := StartPosition()
-	p.Potholes[Black] = D4
+	p.Potholes[0] = hole(D4, Black, 2)
 	p, ev := apply(t, p, "a5c5", dice(1)) // c5 touches d4
-	if e, ok := find(ev, Repaired); !ok || e.Square != D4 || p.Potholes[Black] != NoSquare {
+	if e, ok := find(ev, Repaired); !ok || e.Square != D4 || len(open(p)) != 0 {
 		t.Errorf("moving next to d4 should repair it: %v", ev)
 	}
 }
````

- [ ] **Step 2: Run them to see them fail**

Run: `go vet ./rules ./game`
Expected: compile errors such as `undefined: Hole`, `undefined: HoleRounds`, `p.Potholes[...] (variable of type Square) ...`.

- [ ] **Step 3: Implement the engine and the view**

````diff
diff --git a/game/game.go b/game/game.go
index 5a72850..177a862 100644
--- a/game/game.go
+++ b/game/game.go
@@ -496,7 +496,7 @@ func (g *Game) viewFor(r role) *View {
 		Status:   g.status(),
 		You:      "spectator",
 		Mamdani:  squareName(p.Mamdani),
-		Potholes: []Pothole{},
+		Potholes: potholesJSON(p),
 		Turn:     colorName(p.Turn),
 		Check:    p.InCheck(p.Turn),
 		Legal:    []MoveJSON{},
@@ -517,11 +517,6 @@ func (g *Game) viewFor(r role) *View {
 	for s, pc := range p.Board {
 		v.Board[s] = pieceCode(pc)
 	}
-	for c, s := range p.Potholes {
-		if s != rules.NoSquare {
-			v.Potholes = append(v.Potholes, Pothole{Sq: s.String(), By: colorName(rules.Color(c))})
-		}
-	}
 	if v.Status == Playing && seated && color == p.Turn {
 		for _, m := range p.LegalMoves() {
 			v.Legal = append(v.Legal, moveJSON(m))
diff --git a/game/live.go b/game/live.go
index 6df4731..f999166 100644
--- a/game/live.go
+++ b/game/live.go
@@ -34,7 +34,7 @@ func (g *Game) publish() {
 		Black:    g.names[rules.Black],
 		Move:     len(g.g.Turns)/2 + 1,
 		Mamdani:  squareName(p.Mamdani),
-		Potholes: []Pothole{},
+		Potholes: potholesJSON(p),
 		Watching: g.watching(),
 		status:   g.status(),
 		created:  g.created,
@@ -43,11 +43,6 @@ func (g *Game) publish() {
 	for s, pc := range p.Board {
 		l.Board[s] = pieceCode(pc)
 	}
-	for c, s := range p.Potholes {
-		if s != rules.NoSquare {
-			l.Potholes = append(l.Potholes, Pothole{Sq: s.String(), By: colorName(rules.Color(c))})
-		}
-	}
 	if n := len(g.g.Turns); n > 0 {
 		m := moveJSON(g.g.Turns[n-1].Move)
 		l.Last = &m
diff --git a/game/view.go b/game/view.go
index b5f55d4..6b1fd75 100644
--- a/game/view.go
+++ b/game/view.go
@@ -1,7 +1,9 @@
 package game
 
 import (
+	"cmp"
 	"encoding/json"
+	"slices"
 	"strconv"
 	"strings"
 
@@ -45,10 +47,29 @@ type View struct {
 	data []byte // the encoded view, set once before it is shared (see JSON)
 }
 
-// Pothole is an open pothole and the color that rolled it.
+// Pothole is an open pothole, the color that rolled it and its rounds left:
+// how many more of By's moves it stays open for (1 to rules.HoleRounds).
 type Pothole struct {
-	Sq string `json:"sq"`
-	By string `json:"by"`
+	Sq   string `json:"sq"`
+	By   string `json:"by"`
+	Left int    `json:"left"`
+}
+
+// potholesJSON lists p's open potholes, oldest first, so the next one the
+// cap would close comes first. It is never nil: no holes encodes as [].
+func potholesJSON(p *rules.Position) []Pothole {
+	holes := make([]rules.Hole, 0, rules.HoleCap)
+	for _, h := range p.Potholes {
+		if h.Sq != rules.NoSquare {
+			holes = append(holes, h)
+		}
+	}
+	slices.SortFunc(holes, func(a, b rules.Hole) int { return cmp.Compare(a.Seq, b.Seq) })
+	out := make([]Pothole, len(holes))
+	for i, h := range holes {
+		out[i] = Pothole{Sq: h.Sq.String(), By: colorName(h.By), Left: int(h.Left)}
+	}
+	return out
 }
 
 // MoveJSON is a move on the wire. Promo is "", "q", "r", "b" or "n".
@@ -177,8 +198,10 @@ func eventJSON(e rules.Event) EventJSON {
 		j.Piece, j.Color = pieceCode(e.Piece), colorName(e.Color)
 	case rules.Captured, rules.Fell:
 		j.Sq, j.Piece = squareName(e.Square), pieceCode(e.Piece)
-	case rules.PotholeClosed, rules.Repaired, rules.Target:
+	case rules.Repaired, rules.Target:
 		j.Sq = squareName(e.Square)
+	case rules.PotholeClosed:
+		j.Sq, j.Color = squareName(e.Square), colorName(e.Color)
 	case rules.RolledPothole:
 		j.Roll, j.Color = e.Roll, colorName(e.Color)
 	case rules.Reroll:
@@ -193,6 +216,9 @@ func eventJSON(e rules.Event) EventJSON {
 }
 
 // describe sums up what the dice did on a turn, e.g. "d8 4 → d3 · save 5 ✓".
+// A hole closing after the roll is the cap making room ("c4 closes"); holes
+// that count down to zero close before the roll and are left out, as every
+// turn has them.
 func describe(events []rules.Event) string {
 	var parts []string
 	rolled := false
@@ -205,6 +231,10 @@ func describe(events []rules.Event) string {
 			} else {
 				parts = append(parts, "repairs "+e.Square.String())
 			}
+		case rules.PotholeClosed:
+			if rolled {
+				parts = append(parts, e.Square.String()+" closes")
+			}
 		case rules.RolledPothole:
 			rolled = true
 			parts = append(parts, "d8 "+strconv.Itoa(e.Roll))
diff --git a/rules/event.go b/rules/event.go
index 2d967d8..a96852a 100644
--- a/rules/event.go
+++ b/rules/event.go
@@ -6,7 +6,7 @@ type EventKind string
 const (
 	Moved         EventKind = "moved"          // Move, Piece (MamdaniPiece for a Mamdani move), Color = mover
 	Captured      EventKind = "captured"       // Square, Piece
-	PotholeClosed EventKind = "pothole_closed" // Square
+	PotholeClosed EventKind = "pothole_closed" // Square, Color = roller; at its last round, or the oldest pushed out by the cap
 	Repaired      EventKind = "repaired"       // Square: by the repair step, or a new pothole next to the Mamdani
 	RolledPothole EventKind = "rolled_pothole" // Roll: odd = nothing, even = a pothole opens
 	Target        EventKind = "target"         // Square picked by the two placement d8s
@@ -21,10 +21,9 @@ const (
 type RerollReason string
 
 const (
-	ReasonKing      RerollReason = "king"      // kings never fall
-	ReasonPothole   RerollReason = "pothole"   // already a pothole
-	ReasonExposes   RerollReason = "exposes"   // the fall would leave the roller in check
-	ReasonCheckmate RerollReason = "checkmate" // the result would checkmate the next player
+	ReasonKing    RerollReason = "king"    // kings never fall
+	ReasonPothole RerollReason = "pothole" // already a pothole
+	ReasonExposes RerollReason = "exposes" // the fall, or the cap's close, would leave the roller in check
 )
 
 // Event is one thing that happened during a turn. Fields not listed for a
diff --git a/rules/movegen.go b/rules/movegen.go
index 6d92231..5ed1a8e 100644
--- a/rules/movegen.go
+++ b/rules/movegen.go
@@ -53,7 +53,7 @@ func ParseMove(s string) (Move, error) {
 
 // LegalMoves returns every legal move for the side to move: its own pieces
 // and the Mamdani. A move is legal only if the mover's king is safe after
-// the move, the close step and the repair step.
+// the move, the countdown and the repair step.
 func (p *Position) LegalMoves() []Move {
 	var buf [maxMoves]Move
 	pseudo := p.pseudoMoves(buf[:0])
@@ -72,7 +72,7 @@ func (p *Position) LegalMoves() []Move {
 const maxMoves = 320
 
 // safe reports whether pseudo-legal m leaves the mover's king unattacked
-// once the close and repair steps have run.
+// once the countdown and repair steps have run.
 func (p *Position) safe(m Move) bool {
 	q := *p
 	q.play(m, nil)
diff --git a/rules/play.go b/rules/play.go
index d1c212e..1cb2c23 100644
--- a/rules/play.go
+++ b/rules/play.go
@@ -1,7 +1,7 @@
 package rules
 
-// play makes move m for the side to move, then runs the close and repair
-// steps and passes the turn. It assumes m is pseudo-legal. Events are
+// play makes move m for the side to move, then runs the countdown and
+// repair steps and passes the turn. It assumes m is pseudo-legal. Events are
 // appended to *ev; pass nil when only the resulting position matters, and
 // nothing is recorded or allocated.
 func (p *Position) play(m Move, ev *[]Event) {
@@ -17,10 +17,17 @@ func (p *Position) play(m Move, ev *[]Event) {
 	if mover == Black {
 		p.Fullmove++
 	}
-	// Close: the mover's own pothole from their previous turn.
-	if s := p.Potholes[mover]; s != NoSquare {
-		p.Potholes[mover] = NoSquare
-		emit(ev, Event{Kind: PotholeClosed, Square: s})
+	// Count down: each of the mover's own holes loses a round, and any
+	// with none left closes. The other player's holes wait for their moves.
+	for i := range p.Potholes {
+		h := &p.Potholes[i]
+		if h.Sq == NoSquare || h.By != mover {
+			continue
+		}
+		h.Left--
+		if h.Left == 0 {
+			p.closeHole(i, ev)
+		}
 	}
 	p.repair(ev)
 	p.Turn = mover.Other()
@@ -101,9 +108,9 @@ func (p *Position) repair(ev *[]Event) {
 	if p.Mamdani == NoSquare {
 		return
 	}
-	for c, s := range p.Potholes {
-		if s != NoSquare && adjacent(s, p.Mamdani) {
-			p.Potholes[c] = NoSquare
+	for i := range p.Potholes {
+		if s := p.Potholes[i].Sq; s != NoSquare && adjacent(s, p.Mamdani) {
+			p.Potholes[i].Sq = NoSquare
 			emit(ev, Event{Kind: Repaired, Square: s})
 		}
 	}
diff --git a/rules/position.go b/rules/position.go
index 7c26cd4..2acfa5a 100644
--- a/rules/position.go
+++ b/rules/position.go
@@ -24,10 +24,9 @@ type Position struct {
 	// Mamdani is the Mamdani's square, or NoSquare once it has fallen
 	// (and in plain-chess positions such as perft).
 	Mamdani Square
-	// Potholes holds the open pothole each color rolled, or NoSquare.
-	// Each player has at most one: theirs closes on their next move,
-	// before they can roll again.
-	Potholes [2]Square
+	// Potholes are the open potholes, in no particular order. A slot
+	// with Sq NoSquare is free.
+	Potholes [HoleCap]Hole
 	Turn     Color
 	Castling Castling
 	EP       Square // en passant target square, or NoSquare
@@ -63,17 +62,102 @@ func (p *Position) take(s Square) Piece {
 // pieces returns c's pieces of kind k.
 func (p *Position) pieces(c Color, k Kind) Bitboard { return p.byColor[c] & p.byKind[k] }
 
+// HoleRounds is how many of its roller's moves a pothole stays open for:
+// it closes when the roller finishes their third move after opening it.
+const HoleRounds = 3
+
+// HoleCap is the most potholes open at once. Opening one more closes the
+// oldest first.
+const HoleCap = 5
+
+// Hole is one open pothole.
+type Hole struct {
+	Sq Square // NoSquare for a free slot in Position.Potholes
+	By Color  // who rolled it; it counts down on their moves
+	// Left is the rounds left: the roller's moves until it closes, 1 to
+	// HoleRounds. A hole with 1 left closes when its roller next moves.
+	Left int8
+	// Seq orders the open holes by opening, so the cap knows which is
+	// oldest. A new hole gets one more than the newest still open, so it
+	// depends only on the holes on the board, not on the game's history.
+	Seq uint32
+}
+
+// noHoles returns a pothole list with every slot free.
+func noHoles() [HoleCap]Hole {
+	var h [HoleCap]Hole
+	for i := range h {
+		h[i].Sq = NoSquare
+	}
+	return h
+}
+
 // potholes returns the open potholes as a set.
 func (p *Position) potholes() Bitboard {
 	var b Bitboard
-	for _, s := range p.Potholes {
-		if s != NoSquare {
-			b |= bit(s)
+	for _, h := range p.Potholes {
+		if h.Sq != NoSquare {
+			b |= bit(h.Sq)
 		}
 	}
 	return b
 }
 
+// oldestHole returns the index in Potholes of the open hole opened first,
+// or -1 if none is open, and how many are open.
+func (p *Position) oldestHole() (oldest, open int) {
+	oldest = -1
+	for i, h := range p.Potholes {
+		if h.Sq == NoSquare {
+			continue
+		}
+		open++
+		if oldest < 0 || h.Seq < p.Potholes[oldest].Seq {
+			oldest = i
+		}
+	}
+	return oldest, open
+}
+
+// capVictim returns the index of the hole the cap would close if a new one
+// opened now, or -1 if there is room.
+func (p *Position) capVictim() int {
+	if oldest, open := p.oldestHole(); open >= HoleCap {
+		return oldest
+	}
+	return -1
+}
+
+// openHole opens a pothole on s rolled by c, with every round to go. With
+// HoleCap already open the oldest closes first, so the events read
+// pothole_closed, then pothole_opened.
+func (p *Position) openHole(s Square, c Color, ev *[]Event) {
+	if i := p.capVictim(); i >= 0 {
+		p.closeHole(i, ev)
+	}
+	var seq uint32
+	free := -1
+	for i, h := range p.Potholes {
+		if h.Sq == NoSquare {
+			if free < 0 {
+				free = i
+			}
+		} else if h.Seq > seq {
+			seq = h.Seq
+		}
+	}
+	p.Potholes[free] = Hole{Sq: s, By: c, Left: HoleRounds, Seq: seq + 1}
+	emit(ev, Event{Kind: PotholeOpened, Square: s, Color: c})
+}
+
+// closeHole closes the hole in slot i as it reaches the end of its rounds
+// or is pushed out by the cap. Color on the event is the hole's roller.
+func (p *Position) closeHole(i int, ev *[]Event) {
+	h := p.Potholes[i]
+	p.Potholes[i].Sq = NoSquare
+	emit(ev, Event{Kind: PotholeClosed, Square: h.Sq, Color: h.By})
+}
+
 // blocked returns every square that stops a slider: pieces, the Mamdani
 // and potholes.
 func (p *Position) blocked() Bitboard {
@@ -98,7 +182,7 @@ func StartPosition() Position {
 
 // IsPothole reports whether s holds an open pothole.
 func (p *Position) IsPothole(s Square) bool {
-	return s != NoSquare && (p.Potholes[White] == s || p.Potholes[Black] == s)
+	return s != NoSquare && p.potholes().Has(s)
 }
 
 // Blocked reports whether s stops a slider: a piece, the Mamdani or a pothole.
@@ -112,19 +196,39 @@ func (p *Position) King(c Color) Square {
 	return p.pieces(c, King).First()
 }
 
-// Key identifies a position for repetition: pieces, Mamdani, open potholes,
-// side to move, castling rights and en passant square.
+// Key identifies a position for repetition: pieces, Mamdani, open potholes
+// with their rollers and rounds left, side to move, castling rights and en
+// passant square.
 type Key struct {
 	Board    [64]Piece
 	Mamdani  Square
-	Potholes [2]Square
+	Potholes [HoleCap]Hole
 	Turn     Color
 	Castling Castling
 	EP       Square
 }
 
+// Key returns p's repetition key. Holes are listed in square order, free
+// slots last, so the same holes match whichever slots they sit in. Seq is
+// left out: it only says which hole is oldest, and that already follows
+// from each hole's roller and rounds left, since at most one hole opens
+// per move.
 func (p *Position) Key() Key {
-	return Key{p.Board, p.Mamdani, p.Potholes, p.Turn, p.Castling, p.EP}
+	holes := noHoles()
+	n := 0
+	for _, h := range p.Potholes {
+		if h.Sq == NoSquare {
+			continue
+		}
+		h.Seq = 0
+		i := n
+		for ; i > 0 && holes[i-1].Sq > h.Sq; i-- {
+			holes[i] = holes[i-1]
+		}
+		holes[i] = h
+		n++
+	}
+	return Key{p.Board, p.Mamdani, holes, p.Turn, p.Castling, p.EP}
 }
 
 var fenPieces = map[byte]Piece{
@@ -137,7 +241,7 @@ var fenPieces = map[byte]Piece{
 // ParseFEN reads standard FEN. The result has no Mamdani and no potholes;
 // set those fields directly when a position needs them.
 func ParseFEN(fen string) (Position, error) {
-	p := Position{Mamdani: NoSquare, Potholes: [2]Square{NoSquare, NoSquare}, EP: NoSquare}
+	p := Position{Mamdani: NoSquare, Potholes: noHoles(), EP: NoSquare}
 	fields := strings.Fields(fen)
 	if len(fields) != 6 {
 		return p, fmt.Errorf("fen %q: want 6 fields", fen)
diff --git a/rules/turn.go b/rules/turn.go
index bc3a673..b16258b 100644
--- a/rules/turn.go
+++ b/rules/turn.go
@@ -19,10 +19,12 @@ var ErrBadDie = errors.New("die roll outside 1..8")
 // maxRerolls caps placement re-rolls; past it no pothole opens this turn.
 const maxRerolls = 64
 
-// Apply plays one full turn: move, close, repair, pothole roll, placement
-// and resolution. It returns the new position and what happened, in order.
-// A move that checkmates ends the game at once: no pothole roll follows, so
-// the dice can't undo a mate made on the board. p is not modified.
+// Apply plays one full turn: move, countdown, repair, pothole roll,
+// placement and resolution. It returns the new position and what happened,
+// in order. A move that checkmates ends the game at once: no pothole roll
+// follows, so the dice can't undo a mate made on the board. The roll itself
+// may leave the next player mated; the game sees that like any other mate.
+// p is not modified.
 func Apply(p Position, m Move, dice Dice) (Position, []Event, error) {
 	if !p.isLegal(m) {
 		return p, nil, ErrIllegalMove
@@ -62,18 +64,23 @@ func (c *checkedDice) D8() int {
 }
 
 // Mated reports whether the side to move is checkmated: no legal move, and
-// its king attacked once its own open pothole is counted as closed. Every
-// move closes that pothole, so a check it is only holding off can't be
-// escaped either.
+// its king attacked once its own holes on their last round are counted as
+// closed. Every move closes those, so a check they are only holding off
+// can't be escaped either. A hole with rounds to spare outlasts the next
+// move, so a check it holds off leaves a stalemate, not a mate.
 func (p *Position) Mated() bool {
 	return p.threatened() && !p.hasLegalMove()
 }
 
 // threatened reports whether the side to move's king is attacked, ignoring
-// its own open pothole.
+// its own holes that close on its next move.
 func (p *Position) threatened() bool {
 	q := *p
-	q.Potholes[q.Turn] = NoSquare
+	for i, h := range q.Potholes {
+		if h.Sq != NoSquare && h.By == q.Turn && h.Left == 1 {
+			q.Potholes[i].Sq = NoSquare
+		}
+	}
 	return q.InCheck(q.Turn)
 }
 
@@ -111,18 +118,18 @@ func (p *Position) rerollReason(s Square, mover Color) RerollReason {
 	// Would the fall leave the roller in check once the hole is gone?
 	// While open, the hole blocks the same lines the piece did, so the test
 	// is made without it: the roller must not inherit an unanswerable check
-	// when their own pothole closes after their next move.
+	// when their own pothole closes. With HoleCap open, the oldest hole
+	// closes as this one opens, and it may be the one shielding the
+	// roller's king: then the roller would be in check on the other
+	// player's turn, so that is tested too.
 	gone := *p
 	gone.remove(s)
+	if i := gone.capVictim(); i >= 0 {
+		gone.Potholes[i].Sq = NoSquare
+	}
 	if gone.InCheck(mover) {
 		return ReasonExposes
 	}
-	// Would the outcome checkmate the next player? A roll never wins.
-	opened := gone
-	opened.Potholes[mover] = s
-	if opened.Mated() {
-		return ReasonCheckmate
-	}
 	return ""
 }
 
@@ -169,8 +176,8 @@ func (p *Position) resolve(s Square, mover Color, dice Dice, ev []Event) []Event
 		p.Halfmove = 0
 		p.Castling &^= rightsLost(s)
 	}
-	p.Potholes[mover] = s
-	return append(ev, Event{Kind: PotholeOpened, Square: s, Color: mover})
+	p.openHole(s, mover, &ev)
+	return ev
 }
 
 // mamdaniReaches reports whether the Mamdani has a clear queen line to s:
````

- [ ] **Step 4: Run the tests**

Run: `go vet ./... && go test -race -short ./...`
Expected: every package `ok`.
Run once: `go test ./rules -run TestRandomGames -count=1`
Expected: `ok` (about 4 s without `-race`).
Run: `go test ./rules -run TestPerft -count=1`
Expected: `ok`, unchanged counts.

- [ ] **Step 5: Commit**

```bash
git add rules/position.go rules/play.go rules/turn.go rules/event.go rules/movegen.go rules/helpers_test.go rules/turn_test.go rules/game_test.go rules/random_test.go rules/movegen_test.go rules/board_test.go rules/san_test.go game/view.go game/game.go game/live.go game/game_test.go game/view_test.go
git commit -m "rules, game: potholes last 3 rounds, at most 5 open, a roll can mate; the view carries rounds left"
```

---

### Task 2: Saved games: record the rules, retire the old ones

**Files:**
- Modify: `store/migrate.go` (migration 5), `store/games.go`, `game/clock.go` (`Retired`)
- Modify (tests): `store/games_test.go`, `store/guests_test.go`

**Interfaces:**
- Consumes: nothing from Task 1 (independent of the engine).
- Produces:
  - `games.rules INTEGER NOT NULL DEFAULT 1`; `CreateGame` writes `rulesVersion = 2`.
  - `LoadForRestore` returns only `rules = 2` games. A link to an old game gets the existing "Game not found" page.
  - `game.Retired rules.Reason = "retired"`, counted by `noWinner`.

`TestMigration4KeepsOldGames` used to load an open old game through `LoadForRestore`; migration 5 now retires that game, so the test checks the rows directly (`openAt` helper).

- [ ] **Step 1: Write the failing tests**

````diff
diff --git a/store/games_test.go b/store/games_test.go
index e975be2..9645e77 100644
--- a/store/games_test.go
+++ b/store/games_test.go
@@ -2,7 +2,9 @@ package store
 
 import (
 	"context"
+	"database/sql"
 	"errors"
+	"fmt"
 	"reflect"
 	"testing"
 	"time"
@@ -140,3 +142,53 @@ func must(t *testing.T, err error) {
 		t.Fatal(err)
 	}
 }
+
+// A database from milestone 06a (schema version 4) has an unfinished game
+// and one that ended. Migration 5 retires the unfinished one, leaves the
+// finished one as it was, and neither loads again: their turns were played
+// under the old rules. A game created afterwards is saved under the current
+// rules and loads.
+func TestMigration5RetiresOldGames(t *testing.T) {
+	ctx := context.Background()
+	before := time.Now().UnixMilli()
+	s := openAt(t, 4,
+		fmt.Sprintf(`INSERT INTO games (code, white, black, created_at) VALUES ('OPEN01', 'a', 'b', %d)`, t0.UnixMilli()),
+		fmt.Sprintf(`INSERT INTO games (code, white, black, created_at, ended_at, result, winner)
+		 VALUES ('DONE01', 'a', 'b', %d, %d, 'checkmate', 'white')`, t0.UnixMilli(), t0.Add(time.Minute).UnixMilli()),
+	)
+	after := time.Now().UnixMilli()
+
+	type row struct {
+		ended  sql.NullInt64
+		result string
+		winner string
+		rules  int
+	}
+	read := func(code string) row {
+		var r row
+		must(t, s.db.QueryRowContext(ctx,
+			`SELECT ended_at, COALESCE(result, ''), COALESCE(winner, ''), rules FROM games WHERE code = ?`, code).Scan(&r.ended, &r.result, &r.winner, &r.rules))
+		return r
+	}
+	// strftime('%s') counts whole seconds.
+	if r := read("OPEN01"); r.result != "retired" || r.winner != "" || r.rules != 1 ||
+		r.ended.Int64 < before/1000*1000 || r.ended.Int64 > after {
+		t.Errorf("OPEN01 %+v, want retired between %d and %d under rules 1", r, before, after)
+	}
+	if r := read("DONE01"); r.result != "checkmate" || r.winner != "white" || r.ended.Int64 != t0.Add(time.Minute).UnixMilli() {
+		t.Errorf("DONE01 %+v, want unchanged", r)
+	}
+	if got, err := s.LoadForRestore(ctx, t0); err != nil || len(got) != 0 {
+		t.Fatalf("loaded %+v (%v), want no old games", got, err)
+	}
+
+	must(t, s.CreateGame(ctx, Game{Code: "NEW001", White: "a", CreatedAt: t0}))
+	if r := read("NEW001"); r.rules != 2 {
+		t.Errorf("NEW001 saved under rules %d, want 2", r.rules)
+	}
+	got, err := s.LoadForRestore(ctx, t0)
+	must(t, err)
+	if len(got) != 1 || got[0].Code != "NEW001" {
+		t.Fatalf("loaded %+v, want only NEW001", got)
+	}
+}
diff --git a/store/guests_test.go b/store/guests_test.go
index bbe36c8..8ff8ea4 100644
--- a/store/guests_test.go
+++ b/store/guests_test.go
@@ -179,29 +179,38 @@ func TestFreshNamesPreferUnusedOnes(t *testing.T) {
 	}
 }
 
-// A database from milestone 05 (schema version 3, a game without names)
-// gains the guests table and name columns, and its games still load.
-func TestMigration4KeepsOldGames(t *testing.T) {
-	ctx := context.Background()
+// openAt builds a database at schema version v, runs seed against it, and
+// opens it with Open, which applies every later migration.
+func openAt(t *testing.T, v int, seed ...string) *Store {
+	t.Helper()
 	path := filepath.Join(t.TempDir(), "old.db")
 	db, err := sql.Open("sqlite", "file:"+path)
 	must(t, err)
 	must(t, exec(db, `CREATE TABLE schema_version (version INTEGER NOT NULL)`))
-	for i, m := range migrations[:3] {
+	for i, m := range migrations[:v] {
 		must(t, exec(db, m))
 		must(t, exec(db, `INSERT INTO schema_version (version) VALUES (?)`, i+1))
 	}
-	must(t, exec(db, `INSERT INTO games (code, white, black, created_at) VALUES ('OLD123', 'alice', 'bob', ?)`, t0.UnixMilli()))
+	for _, q := range seed {
+		must(t, exec(db, q))
+	}
 	db.Close()
-
 	s, err := Open(path)
 	must(t, err)
-	defer s.Close()
-	got, err := s.LoadForRestore(ctx, t0)
-	must(t, err)
-	want := []SavedGame{{Game: Game{Code: "OLD123", White: "alice", Black: "bob", CreatedAt: t0}}}
-	if !reflect.DeepEqual(got, want) {
-		t.Fatalf("loaded\n%+v\nwant\n%+v", got, want)
+	t.Cleanup(func() { s.Close() })
+	return s
+}
+
+// A database from milestone 05 (schema version 3, a game without names)
+// gains the guests table and name columns, and keeps its game.
+func TestMigration4KeepsOldGames(t *testing.T) {
+	ctx := context.Background()
+	s := openAt(t, 3, fmt.Sprintf(
+		`INSERT INTO games (code, white, black, created_at) VALUES ('OLD123', 'alice', 'bob', %d)`, t0.UnixMilli()))
+	var name sql.NullString
+	must(t, s.db.QueryRowContext(ctx, `SELECT white_name FROM games WHERE code = 'OLD123'`).Scan(&name))
+	if name.Valid {
+		t.Errorf("white_name %q, want NULL", name.String)
 	}
 	if _, err := s.EnsureGuest(ctx, "alice", draws(t, "pigeon-astoria")); err != nil {
 		t.Fatalf("guests table missing after migration: %v", err)
````

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./store -run 'TestMigration' -count=1`
Expected: FAIL (`no such column: rules`).

- [ ] **Step 3: Implement**

````diff
diff --git a/game/clock.go b/game/clock.go
index 8bf8d29..5cafe01 100644
--- a/game/clock.go
+++ b/game/clock.go
@@ -52,11 +52,15 @@ const (
 	Aborted rules.Reason = "aborted"
 	// Expired means nobody ever took Black's seat. Nobody wins.
 	Expired rules.Reason = "expired"
+	// Retired means the game was saved under rules since replaced, and
+	// the store ended it rather than replay it. Nobody wins. Retired games
+	// are never loaded, so no view ever shows this.
+	Retired rules.Reason = "retired"
 )
 
 // noWinner reports whether a finished game with reason r has no winner
 // and isn't a draw either.
-func noWinner(r rules.Reason) bool { return r == Aborted || r == Expired }
+func noWinner(r rules.Reason) bool { return r == Aborted || r == Expired || r == Retired }
 
 // winnerName is "white" or "black", or "" for a draw, a game nobody won,
 // or a game still on.
diff --git a/store/games.go b/store/games.go
index 83a96f4..24b583e 100644
--- a/store/games.go
+++ b/store/games.go
@@ -16,6 +16,11 @@ import (
 // ErrCodeTaken is returned by CreateGame when a saved game already has the code.
 var ErrCodeTaken = errors.New("game code already taken")
 
+// rulesVersion is the rules new games are saved under: 2 since potholes
+// last three rounds (milestone 06c). Games saved under other rules are
+// never loaded, since their turns would not replay the same.
+const rulesVersion = 2
+
 // Game is a saved game's seats. Black is "" until someone joins. The names
 // are the players' names when they sat down ("" for games saved before
 // names existed).
@@ -41,7 +46,7 @@ type Turn struct {
 }
 
 // Result is how a game ended. Winner is "white", "black", or "" for a draw
-// or a game nobody won (aborted, expired).
+// or a game nobody won (aborted, expired, retired).
 type Result struct {
 	EndedAt time.Time
 	Reason  string
@@ -58,8 +63,8 @@ type SavedGame struct {
 // CreateGame saves a new game.
 func (s *Store) CreateGame(ctx context.Context, g Game) error {
 	_, err := s.db.ExecContext(ctx,
-		`INSERT INTO games (code, white, black, white_name, black_name, created_at, rematch_of) VALUES (?, ?, ?, ?, ?, ?, ?)`,
-		g.Code, g.White, nullable(g.Black), nullable(g.WhiteName), nullable(g.BlackName), g.CreatedAt.UnixMilli(), nullable(g.RematchOf))
+		`INSERT INTO games (code, white, black, white_name, black_name, created_at, rematch_of, rules) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
+		g.Code, g.White, nullable(g.Black), nullable(g.WhiteName), nullable(g.BlackName), g.CreatedAt.UnixMilli(), nullable(g.RematchOf), rulesVersion)
 	var se *sqlite.Error
 	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
 		return ErrCodeTaken
@@ -112,12 +117,13 @@ func (s *Store) ExpireWaiting(ctx context.Context, cutoff, now time.Time) (int64
 }
 
 // LoadForRestore returns every unfinished game and every game that ended
-// after endedAfter, oldest first, each with its turns in order.
+// after endedAfter, oldest first, each with its turns in order. Only games
+// saved under the current rules are returned.
 func (s *Store) LoadForRestore(ctx context.Context, endedAfter time.Time) ([]SavedGame, error) {
 	rows, err := s.db.QueryContext(ctx,
 		`SELECT code, white, black, white_name, black_name, created_at, rematch_of, ended_at, result, winner
-		 FROM games WHERE ended_at IS NULL OR ended_at > ? ORDER BY created_at`,
-		endedAfter.UnixMilli())
+		 FROM games WHERE rules = ? AND (ended_at IS NULL OR ended_at > ?) ORDER BY created_at`,
+		rulesVersion, endedAfter.UnixMilli())
 	if err != nil {
 		return nil, err
 	}
diff --git a/store/migrate.go b/store/migrate.go
index b73337c..ac182c2 100644
--- a/store/migrate.go
+++ b/store/migrate.go
@@ -51,6 +51,13 @@ var migrations = []string{
 	 CREATE INDEX guests_name ON guests(name);
 	 ALTER TABLE games ADD COLUMN white_name TEXT;
 	 ALTER TABLE games ADD COLUMN black_name TEXT;`,
+	// 5: longer potholes (milestone 06c). rules says which rules a game's
+	// turns replay under; games saved before are rules 1. Rather than keep
+	// the old engine to replay them, unfinished ones end as 'retired', and
+	// only games under the current rules are ever loaded.
+	`ALTER TABLE games ADD COLUMN rules INTEGER NOT NULL DEFAULT 1;
+	 UPDATE games SET ended_at = CAST(strftime('%s','now') AS INTEGER) * 1000, result = 'retired'
+	  WHERE ended_at IS NULL;`,
 }
 
 func (s *Store) migrate(ctx context.Context) error {
````

- [ ] **Step 4: Run the tests**

Run: `go test -race -short ./...`
Expected: every package `ok`.

- [ ] **Step 5: Commit**

```bash
git add store/migrate.go store/games.go game/clock.go store/games_test.go store/guests_test.go
git commit -m "store, game: games record their rules; games under the old rules are retired"
```

---

### Task 3: The web model and the sandbox's rules

**Files:**
- Modify: `web/src/lib/game.ts`, `web/src/lib/board.ts`, `web/src/lib/animator.svelte.ts`, `web/src/lib/sandbox.ts`, `web/src/routes/+page.svelte` (the hero's hole gets `left`)
- Modify (tests): `web/src/lib/board.test.ts`, `web/src/lib/sandbox.test.ts`, `web/src/lib/animator.test.ts`

**Interfaces:**
- Consumes: the JSON from Task 1 (`left`, `color` on closes).
- Produces:
  - `View['potholes']` items are `{ sq: string; by: Color; left: number }`.
  - `board.ts`: `HOLE_ROUNDS`, `HOLE_CAP`; `matedByRoll(view: Pick<View, 'result' | 'last'>): boolean`; `repairsShown(view: View, shown: number): { sq: string; key: string; hole: boolean }[]` (key `${seq}:${index}`; `hole` is true for a repair before the roll).
  - `stageAt` keeps a hole the cap closes until its `pothole_closed` event is revealed.
  - `diceSteps`: "It closes after 3 of White's moves"; a cap close is the step "At most 5 potholes / The oldest, on c4, closes"; the checkmate re-roll reason is gone. `diceSummary` chips read `d4 · opens · c4 closes`.
  - `Animator.animated: boolean`: true only while the current turn plays out step by step.
  - `sandbox.ts`: holes count down and the cap closes the oldest, as the server does; `RollScript.rerolls` items may be `{ sq, reason: 'king' | 'pothole' }`; a typed target on a king re-rolls; the `repair` preset (Mamdani c3, Black hole f6); the move log text matches the server's `describe` ("repairs f6", "d4 closes").

- [ ] **Step 1: Write the failing tests**

````diff
diff --git a/web/src/lib/animator.test.ts b/web/src/lib/animator.test.ts
index d867671..62fe804 100644
--- a/web/src/lib/animator.test.ts
+++ b/web/src/lib/animator.test.ts
@@ -37,6 +37,21 @@ describe('Animator', () => {
 		expect(a.animating).toBe(false);
 	});
 
+	it('marks only a turn that plays out as animated, so one-off effects never replay', () => {
+		const a = new Animator(100);
+		a.receive(view(3, roll)); // first load
+		expect(a.animated).toBe(false);
+		a.receive(view(4, roll)); // the next turn
+		expect(a.animated).toBe(true);
+		vi.advanceTimersByTime(1000);
+		a.receive(view(4, roll)); // an update for the same turn keeps it
+		expect(a.animated).toBe(true);
+		a.receive(view(6, roll)); // a reconnect that skipped a turn
+		expect(a.animated).toBe(false);
+		a.receive(view(7, roll), { hidden: true }); // a hidden tab
+		expect(a.animated).toBe(false);
+	});
+
 	it('jumps straight to a view that skips turns', () => {
 		const a = new Animator(100);
 		a.receive(view(0));
diff --git a/web/src/lib/board.test.ts b/web/src/lib/board.test.ts
index ffcf8ef..8cc1c76 100644
--- a/web/src/lib/board.test.ts
+++ b/web/src/lib/board.test.ts
@@ -1,5 +1,5 @@
 import { describe, expect, it } from 'vitest';
-import { blockedSquares, checkSquare, diceSteps, diceSummary, firstDiceStep, pillFor, squareIndex, stageAt } from './board.ts';
+import { blockedSquares, checkSquare, diceSteps, diceSummary, firstDiceStep, matedByRoll, pillFor, repairsShown, squareIndex, stageAt } from './board.ts';
 import type { EventJSON, View } from './game.ts';
 
 /** A view with the given pieces ({"e1": "wK"}), potholes and Mamdani square. */
@@ -40,32 +40,32 @@ describe('squareIndex', () => {
 
 describe('blockedSquares', () => {
 	it('marks the squares a rook could reach past a pothole', () => {
-		const v = makeView({ a1: 'wR', h1: 'bN' }, { potholes: [{ sq: 'd1', by: 'black' }] });
+		const v = makeView({ a1: 'wR', h1: 'bN' }, { potholes: [{ sq: 'd1', by: 'black', left: 1 }] });
 		expect(blockedSquares(v, 'a1').sort()).toEqual(['e1', 'f1', 'g1', 'h1']);
 	});
 
 	it('stops at the mover’s own piece', () => {
-		const v = makeView({ a1: 'wR', f1: 'wK' }, { potholes: [{ sq: 'd1', by: 'black' }] });
+		const v = makeView({ a1: 'wR', f1: 'wK' }, { potholes: [{ sq: 'd1', by: 'black', left: 1 }] });
 		expect(blockedSquares(v, 'a1')).toEqual(['e1']);
 	});
 
 	it('marks nothing when a piece stands before the pothole', () => {
-		const v = makeView({ a1: 'wR', b1: 'wN' }, { potholes: [{ sq: 'd1', by: 'black' }] });
+		const v = makeView({ a1: 'wR', b1: 'wN' }, { potholes: [{ sq: 'd1', by: 'black', left: 1 }] });
 		expect(blockedSquares(v, 'a1')).toEqual([]);
 	});
 
 	it('follows diagonals for bishops and ignores the other lines', () => {
-		const v = makeView({ c1: 'wB' }, { potholes: [{ sq: 'e3', by: 'white' }, { sq: 'c4', by: 'black' }] });
+		const v = makeView({ c1: 'wB' }, { potholes: [{ sq: 'e3', by: 'white', left: 1 }, { sq: 'c4', by: 'black', left: 1 }] });
 		expect(blockedSquares(v, 'c1').sort()).toEqual(['f4', 'g5', 'h6']);
 	});
 
 	it('never marks a capture for the Mamdani', () => {
-		const v = makeView({ e8: 'bK' }, { mamdani: 'a4', potholes: [{ sq: 'c6', by: 'white' }] });
+		const v = makeView({ e8: 'bK' }, { mamdani: 'a4', potholes: [{ sq: 'c6', by: 'white', left: 1 }] });
 		expect(blockedSquares(v, 'a4').sort()).toEqual(['d7']);
 	});
 
 	it('marks nothing for pieces that jump or step', () => {
-		const v = makeView({ b1: 'wN', e1: 'wK' }, { potholes: [{ sq: 'c3', by: 'black' }, { sq: 'e2', by: 'black' }] });
+		const v = makeView({ b1: 'wN', e1: 'wK' }, { potholes: [{ sq: 'c3', by: 'black', left: 1 }, { sq: 'e2', by: 'black', left: 1 }] });
 		expect(blockedSquares(v, 'b1')).toEqual([]);
 		expect(blockedSquares(v, 'e1')).toEqual([]);
 	});
@@ -92,7 +92,7 @@ describe('firstDiceStep', () => {
 
 describe('stageAt', () => {
 	// The final position: the knight is gone and g8 holds White's pothole.
-	const final = makeView({ e4: 'wP', e1: 'wK' }, { last: fellOnG8, potholes: [{ sq: 'g8', by: 'white' }] });
+	const final = makeView({ e4: 'wP', e1: 'wK' }, { last: fellOnG8, potholes: [{ sq: 'g8', by: 'white', left: 1 }] });
 
 	it('keeps the fallen piece and hides the new pothole until they are revealed', () => {
 		const s = stageAt(final, 1);
@@ -115,10 +115,33 @@ describe('stageAt', () => {
 	it('shows the final position once every step is revealed', () => {
 		const s = stageAt(final, fellOnG8.length);
 		expect(s.board[squareIndex('g8')]).toBe('');
-		expect(s.potholes).toEqual([{ sq: 'g8', by: 'white' }]);
+		expect(s.potholes).toEqual([{ sq: 'g8', by: 'white', left: 1 }]);
 		expect(s.target).toBe('');
 	});
 
+	it('keeps a hole the cap closes until the new one opens', () => {
+		// Five holes were open; White's roll opens a sixth on d4 and the oldest, c4, closes.
+		const last: EventJSON[] = [
+			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
+			{ kind: 'rolled_pothole', roll: 2, color: 'white' },
+			{ kind: 'target', sq: 'd4' },
+			{ kind: 'pothole_closed', sq: 'c4', color: 'black' },
+			{ kind: 'pothole_opened', sq: 'd4', color: 'white' }
+		];
+		const v = makeView({}, { last, potholes: [{ sq: 'd4', by: 'white', left: 3 }] });
+		expect(stageAt(v, 3).potholes).toEqual([{ sq: 'c4', by: 'black', left: 1 }]);
+		expect(stageAt(v, 5).potholes).toEqual([{ sq: 'd4', by: 'white', left: 3 }]);
+	});
+
+	it('closes a hole on its schedule before the roll at once', () => {
+		const last: EventJSON[] = [
+			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
+			{ kind: 'pothole_closed', sq: 'c4', color: 'white' },
+			{ kind: 'rolled_pothole', roll: 1, color: 'white' }
+		];
+		expect(stageAt(makeView({}, { last }), 1).potholes).toEqual([]);
+	});
+
 	it('puts a fallen Mamdani back until its fall is revealed', () => {
 		const last: EventJSON[] = [
 			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
@@ -128,7 +151,7 @@ describe('stageAt', () => {
 			{ kind: 'fell', sq: 'a5', piece: 'M' },
 			{ kind: 'pothole_opened', sq: 'a5', color: 'white' }
 		];
-		const v = makeView({}, { last, mamdani: '', potholes: [{ sq: 'a5', by: 'white' }] });
+		const v = makeView({}, { last, mamdani: '', potholes: [{ sq: 'a5', by: 'white', left: 1 }] });
 		expect(stageAt(v, 4).mamdani).toBe('a5');
 		expect(stageAt(v, 6).mamdani).toBe('');
 	});
@@ -174,6 +197,61 @@ describe('diceSteps', () => {
 	});
 });
 
+describe('diceSteps for the cap', () => {
+	it('says when the cap closes the oldest hole, and how long the new one lasts', () => {
+		const last: EventJSON[] = [
+			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
+			{ kind: 'pothole_closed', sq: 'h3', color: 'white' },
+			{ kind: 'rolled_pothole', roll: 2, color: 'white' },
+			{ kind: 'target', sq: 'd4' },
+			{ kind: 'pothole_closed', sq: 'c4', color: 'black' },
+			{ kind: 'pothole_opened', sq: 'd4', color: 'white' }
+		];
+		const steps = diceSteps(makeView({}, { last }), last.length);
+		expect(steps.map((s) => [s.title, s.detail])).toEqual([
+			['Even. A pothole opens', 'd8 rolled 2'],
+			['Square d4', 'File 4 = d, rank 4. Empty square'],
+			['At most 5 potholes', 'The oldest, on c4, closes'],
+			['Pothole opens on d4', 'It closes after 3 of White’s moves']
+		]);
+		const sum = diceSummary(makeView({}, { last }), last.length);
+		expect(sum.chips.map((c) => c.text)).toEqual(['2', 'd4', 'opens', 'c4 closes']);
+	});
+});
+
+describe('matedByRoll', () => {
+	const mate = { winner: 'white' as const, draw: false, reason: 'checkmate' };
+	it('is true when the game ended in checkmate after a roll', () => {
+		const last: EventJSON[] = [
+			{ kind: 'moved', from: 'e2', to: 'e4', piece: 'wP', color: 'white' },
+			{ kind: 'rolled_pothole', roll: 2, color: 'white' }
+		];
+		expect(matedByRoll({ result: mate, last })).toBe(true);
+	});
+	it('is false for a mating move, which ends the game before any roll', () => {
+		expect(matedByRoll({ result: mate, last: [{ kind: 'moved', from: 'd1', to: 'h5', piece: 'wQ', color: 'white' }] })).toBe(false);
+		expect(matedByRoll({ result: { draw: true, reason: 'stalemate' }, last: [{ kind: 'rolled_pothole', roll: 2 }] })).toBe(false);
+	});
+});
+
+describe('repairsShown', () => {
+	const last: EventJSON[] = [
+		{ kind: 'moved', from: 'c3', to: 'e5', piece: 'M', color: 'white' },
+		{ kind: 'repaired', sq: 'f6' },
+		{ kind: 'rolled_pothole', roll: 2, color: 'white' },
+		{ kind: 'target', sq: 'd4' },
+		{ kind: 'repaired', sq: 'd4' }
+	];
+	const v = makeView({}, { last, seq: 7 });
+	it('lists repairs revealed so far: the move’s fixes an open hole, the roll’s fixes one before it opens', () => {
+		expect(repairsShown(v, 2)).toEqual([{ sq: 'f6', key: '7:1', hole: true }]);
+		expect(repairsShown(v, 5)).toEqual([
+			{ sq: 'f6', key: '7:1', hole: true },
+			{ sq: 'd4', key: '7:4', hole: false }
+		]);
+	});
+});
+
 describe('checkSquare', () => {
 	const v = makeView({ e1: 'wK', e8: 'bK', e5: 'bQ' }, { check: true, turn: 'white' });
 	const stage = stageAt(v, v.last.length);
@@ -240,7 +318,7 @@ describe('diceSummary', () => {
 
 	it('opens a pothole on an empty square', () => {
 		const last: EventJSON[] = [moved, { kind: 'rolled_pothole', roll: 4, color: 'white' }, { kind: 'target', sq: 'd4' }, { kind: 'pothole_opened', sq: 'd4', color: 'white' }];
-		expect(sum(last)).toEqual({ who: 'white', chips: ['4:die', 'd4:square', 'opens:bad'], line: 'Pothole on d4 · closes when White moves', tone: 'hazard' });
+		expect(sum(last)).toEqual({ who: 'white', chips: ['4:die', 'd4:square', 'opens:bad'], line: 'Pothole on d4 · closes after 3 of White’s moves', tone: 'hazard' });
 	});
 
 	it('repairs a pothole next to the Mamdani at once', () => {
diff --git a/web/src/lib/sandbox.test.ts b/web/src/lib/sandbox.test.ts
index 5a71bcf..b49d688 100644
--- a/web/src/lib/sandbox.test.ts
+++ b/web/src/lib/sandbox.test.ts
@@ -68,15 +68,39 @@ describe('playTurn', () => {
 		expect(v.log).toEqual([{ san: 'e2–e4', color: 'white', dice: 'd8 1' }]);
 	});
 
-	it('opens a pothole on an empty square and closes it after the roller’s next move', () => {
+	it('opens a pothole on an empty square and closes it after 3 of the roller’s moves', () => {
 		let v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'd4' });
-		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white' }]);
+		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white', left: 3 }]);
 		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'pothole_opened']);
 		v = playTurn(v, { from: 'e7', to: 'e5' }, odd);
-		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white' }]);
+		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white', left: 3 }]); // Black's move doesn't count
 		v = playTurn(v, { from: 'g1', to: 'f3' }, odd);
+		v = playTurn(v, { from: 'b8', to: 'c6' }, odd);
+		v = playTurn(v, { from: 'b1', to: 'c3' }, odd);
+		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white', left: 1 }]);
+		v = playTurn(v, { from: 'g8', to: 'f6' }, odd);
+		v = playTurn(v, { from: 'f1', to: 'e2' }, odd);
 		expect(v.potholes).toEqual([]);
-		expect(v.last[1]).toEqual({ kind: 'pothole_closed', sq: 'd4' });
+		expect(v.last[1]).toEqual({ kind: 'pothole_closed', sq: 'd4', color: 'white' });
+	});
+
+	it('closes the oldest hole when a sixth opens, and logs it as the server does', () => {
+		let v = startView();
+		// Targets away from the Mamdani on a5, so none is repaired on arrival.
+		const plies: [string, string, string][] = [
+			['a2', 'a3', 'd4'], ['h7', 'h6', 'e5'], ['b2', 'b3', 'f4'], ['g7', 'g6', 'c5'], ['c2', 'c3', 'h4'], ['f7', 'f6', 'd5']
+		];
+		for (const [from, to, target] of plies) v = playTurn(v, { from, to }, { pothole: 2, target });
+		expect(v.potholes.map((h) => h.sq)).toEqual(['e5', 'f4', 'c5', 'h4', 'd5']);
+		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'pothole_closed', 'pothole_opened']);
+		expect(v.last[3]).toEqual({ kind: 'pothole_closed', sq: 'd4', color: 'white' });
+		expect(v.log.at(-1)?.dice).toBe('d8 2 → d5 · d4 closes');
+	});
+
+	it('logs a repair made by the Mamdani’s move, as the server does', () => {
+		const v = playTurn(positions.repair(), { from: 'c3', to: 'e5' }, odd);
+		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'repaired', 'rolled_pothole']);
+		expect(v.log.at(-1)?.dice).toBe('repairs f6 · d8 1');
 	});
 
 	it('drops a piece without a saving roll and counts it as lost', () => {
@@ -113,11 +137,18 @@ describe('playTurn', () => {
 		// The sandbox's default script: every even roll targets d4.
 		let v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'd4' });
 		v = playTurn(v, { from: 'e7', to: 'e5' }, { pothole: 2, target: 'd4' });
-		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white' }]);
+		expect(v.potholes).toEqual([{ sq: 'd4', by: 'white', left: 3 }]);
 		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'reroll']);
 		expect(v.last.at(-1)).toEqual({ kind: 'reroll', sq: 'd4', reason: 'pothole' });
 	});
 
+	it('re-rolls a typed target on a king instead of dropping the king', () => {
+		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, target: 'e1' });
+		expect(v.board[squareIndex('e1')]).toBe('wK');
+		expect(v.potholes).toEqual([]);
+		expect(v.last.at(-1)).toEqual({ kind: 'reroll', sq: 'e1', reason: 'king' });
+	});
+
 	it('plays re-rolls before the final target', () => {
 		const v = playTurn(startView(), { from: 'e2', to: 'e4' }, { pothole: 2, rerolls: ['e1'], target: 'h6' });
 		expect(v.last.map((e) => e.kind)).toEqual(['moved', 'rolled_pothole', 'target', 'reroll', 'target', 'pothole_opened']);
````

- [ ] **Step 2: Run them to see them fail**

Run: `pnpm --dir web test`
Expected: FAIL: `matedByRoll`/`repairsShown` not exported, texts differ ("closes when White moves"), `a.animated` is `undefined`, the sandbox closes holes after one move.

- [ ] **Step 3: Implement**

````diff
diff --git a/web/src/lib/animator.svelte.ts b/web/src/lib/animator.svelte.ts
index 96ec78d..ed6aa96 100644
--- a/web/src/lib/animator.svelte.ts
+++ b/web/src/lib/animator.svelte.ts
@@ -15,6 +15,13 @@ export class Animator {
 	view = $state<View | null>(null);
 	/** Events of view.last revealed so far. */
 	shown = $state(0);
+	/**
+	 * Whether the current turn is playing out step by step, rather than shown
+	 * at once (first load, reconnect, a hidden tab). One-off effects such as a
+	 * repair celebration play only for an animated turn, so a reload never
+	 * replays them.
+	 */
+	animated = $state(false);
 	/** Delay between steps, in ms; applies from the next step on. */
 	stepMs: number;
 	#timer: ReturnType<typeof setTimeout> | undefined;
@@ -43,6 +50,7 @@ export class Animator {
 		}
 		const animate = this.view !== null && next.seq === this.view.seq + 1 && next.last.length > 0 && !hidden;
 		this.view = next;
+		this.animated = animate;
 		clearTimeout(this.#timer);
 		this.shown = animate ? firstDiceStep(next.last) : next.last.length;
 		this.#tick();
diff --git a/web/src/lib/board.ts b/web/src/lib/board.ts
index 6dce244..1a2ec34 100644
--- a/web/src/lib/board.ts
+++ b/web/src/lib/board.ts
@@ -67,6 +67,10 @@ export function blockedSquares(view: Pick<View, 'board' | 'potholes' | 'mamdani'
 }
 
 /** Index of the first dice event (the pothole roll); the end if none was rolled. */
+/** How many of its roller's moves a pothole lasts, and the most open at once (rules/position.go). */
+export const HOLE_ROUNDS = 3;
+export const HOLE_CAP = 5;
+
 export function firstDiceStep(last: EventJSON[]): number {
 	const i = last.findIndex((e) => e.kind === 'rolled_pothole');
 	return i < 0 ? last.length : i;
@@ -102,6 +106,7 @@ export function stageAt(view: View, shown: number): Stage {
 	let mamdani = view.mamdani;
 	let target = '';
 	const lost = { white: [...view.lost.white], black: [...view.lost.black] };
+	const roll = firstDiceStep(view.last);
 	view.last.forEach((e, i) => {
 		const revealed = i < shown;
 		if (e.kind === 'target' && revealed) target = e.sq ?? '';
@@ -117,6 +122,8 @@ export function stageAt(view: View, shown: number): Stage {
 			}
 		}
 		if (e.kind === 'pothole_opened' && !revealed) potholes = potholes.filter((p) => p.sq !== e.sq);
+		// The cap closes the oldest hole as a new one opens: until then it stays.
+		if (e.kind === 'pothole_closed' && !revealed && i > roll && e.sq && e.color) potholes = [...potholes, { sq: e.sq, by: e.color, left: 1 }];
 	});
 	return { board, potholes, mamdani, target: shown < view.last.length ? target : '', lost };
 }
@@ -131,8 +138,7 @@ export interface DiceStep {
 const rerollReasons: Record<string, string> = {
 	king: 'Kings never fall',
 	pothole: 'Already a pothole',
-	exposes: 'The fall would expose the roller’s king',
-	checkmate: 'The result would checkmate: a roll never wins on its own'
+	exposes: 'The fall would expose the roller’s king'
 };
 
 function capitalize(s: string): string {
@@ -192,7 +198,7 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 			case 'pothole_opened':
 				steps.push({
 					title: `Pothole opens on ${e.sq}`,
-					detail: `It closes when ${colorTitle(e.color)} finishes their next move`,
+					detail: `It closes after ${HOLE_ROUNDS} of ${colorTitle(e.color)}’s moves`,
 					dice: [],
 					tone: 'hazard'
 				});
@@ -200,6 +206,11 @@ export function diceSteps(view: View, shown: number): DiceStep[] {
 			case 'repaired':
 				if (rolled) steps.push({ title: 'Repaired at once', detail: `${e.sq} is next to the Mamdani`, dice: [], tone: 'good' });
 				break;
+			case 'pothole_closed':
+				// Before the roll a hole closes on its own schedule (the tray notes it);
+				// during the roll only the cap closes one.
+				if (rolled) steps.push({ title: `At most ${HOLE_CAP} potholes`, detail: `The oldest, on ${e.sq}, closes`, dice: [], tone: 'muted' });
+				break;
 			case 'no_pothole':
 				steps.push({ title: 'No pothole this turn', detail: '64 re-rolls found no valid square', dice: [], tone: 'muted' });
 				break;
@@ -238,9 +249,12 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 	let tone: DiceSummary['tone'] = 'muted';
 	let pending = true; // the next die or square is still to come
 	let square = '';
+	let rolled = false;
+	let capped = ''; // a hole the cap closes, told after the new one opens
 	for (const e of view.last.slice(0, shown)) {
 		switch (e.kind) {
 			case 'rolled_pothole': {
+				rolled = true;
 				const even = (e.roll ?? 1) % 2 === 0;
 				chips.push({ text: String(e.roll), kind: 'die' });
 				[line, tone, pending] = even ? ['Even: a pothole opens · finding its square…', 'normal', true] : ['Odd: nothing happens', 'muted', false];
@@ -274,8 +288,9 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 				// After a fall the fall is the news; on an empty square the hole is.
 				if (chips[chips.length - 1]?.kind === 'square') {
 					chips.push({ text: 'opens', kind: 'bad' });
-					[line, tone] = [`Pothole on ${e.sq} · closes when ${colorTitle(e.color)} moves`, 'hazard'];
+					[line, tone] = [`Pothole on ${e.sq} · closes after ${HOLE_ROUNDS} of ${colorTitle(e.color)}’s moves`, 'hazard'];
 				}
+				if (capped) chips.push({ text: `${capped} closes`, kind: 'plain' });
 				break;
 			case 'repaired':
 				if (square === e.sq) {
@@ -283,6 +298,10 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 					[line, tone] = [`The Mamdani repairs ${e.sq} at once`, 'good'];
 				}
 				break;
+			case 'pothole_closed':
+				// Only the cap closes a hole during the roll.
+				if (rolled) capped = e.sq ?? '';
+				break;
 			case 'no_pothole':
 				chips.push({ text: 'none', kind: 'plain' });
 				[line, tone, pending] = ['No pothole: no square could take one', 'muted', false];
@@ -294,6 +313,21 @@ export function diceSummary(view: View, shown: number): DiceSummary {
 	return { who: roll?.color ?? mover, chips, line, tone };
 }
 
+/** Whether the dice, not the move, delivered checkmate: a mating move ends the game before any roll. */
+export function matedByRoll(view: Pick<View, 'result' | 'last'>): boolean {
+	return view.result?.reason === 'checkmate' && view.last.some((e) => e.kind === 'rolled_pothole');
+}
+
+/**
+ * The repairs to celebrate on the board: repaired events revealed so far.
+ * One made by the Mamdani's move (before the roll) fixes an open hole; one
+ * during the roll fixes a hole before it opens. Each key plays once.
+ */
+export function repairsShown(view: View, shown: number): { sq: string; key: string; hole: boolean }[] {
+	const roll = firstDiceStep(view.last);
+	return view.last.flatMap((e, i) => (e.kind === 'repaired' && e.sq && i < shown ? [{ sq: e.sq, key: `${view.seq}:${i}`, hole: i < roll }] : []));
+}
+
 /** The pill on a player's bar: whose move, check, waiting, checkmated. */
 export function pillFor(view: View, color: Color, animating: boolean): { text: string; tone: 'turn' | 'check' | 'muted' } {
 	if (view.status === 'waiting') return color === 'black' ? { text: 'Waiting…', tone: 'muted' } : { text: '', tone: 'muted' };
diff --git a/web/src/lib/game.ts b/web/src/lib/game.ts
index 2524822..163b4d9 100644
--- a/web/src/lib/game.ts
+++ b/web/src/lib/game.ts
@@ -29,7 +29,7 @@ export interface View {
 	you: Color | 'spectator';
 	board: string[]; // 64 entries, index 0 = a1; "" or "wP", "bQ", ...
 	mamdani: string; // "" once it has fallen
-	potholes: { sq: string; by: Color }[];
+	potholes: { sq: string; by: Color; left: number }[]; // left: the roller's moves until it closes (1 to 3)
 	turn: Color;
 	check: boolean;
 	legal: MoveJSON[];
@@ -183,8 +183,7 @@ export function pieceName(code = ''): string {
 const rerollReasons: Record<string, string> = {
 	king: 'kings never fall',
 	pothole: 'already a pothole',
-	exposes: 'would expose the roller’s king',
-	checkmate: 'would decide the game'
+	exposes: 'would expose the roller’s king'
 };
 
 /** One plain-English line per turn event. */
diff --git a/web/src/lib/sandbox.ts b/web/src/lib/sandbox.ts
index 5ccf0f9..989c12d 100644
--- a/web/src/lib/sandbox.ts
+++ b/web/src/lib/sandbox.ts
@@ -3,7 +3,7 @@
 // plays through the real board, dice tray and animation code. This is not
 // the rules engine: the Go server is the only source of truth for play.
 
-import { squareIndex } from './board.ts';
+import { HOLE_CAP, HOLE_ROUNDS, squareIndex } from './board.ts';
 import { applyMove } from './pieces.ts';
 import type { Color, EventJSON, MoveJSON, View } from './game.ts';
 
@@ -11,8 +11,8 @@ import type { Color, EventJSON, MoveJSON, View } from './game.ts';
 export interface RollScript {
 	/** The pothole d8: odd means nothing happens. */
 	pothole: number;
-	/** Squares re-rolled (shown as "kings never fall") before the final target. */
-	rerolls?: string[];
+	/** Squares re-rolled before the final target: a bare square is a king ("kings never fall"). */
+	rerolls?: (string | { sq: string; reason: 'king' | 'pothole' })[];
 	/** The square the placement dice pick. */
 	target?: string;
 	/** A saving roll for a piece or the Mamdani on the target: odd saves. */
@@ -76,8 +76,8 @@ export const positions = {
 		const v = place(emptyView(), { e1: 'wK', a1: 'wR', c1: 'wB', d1: 'wQ', e8: 'bK', h8: 'bR', f8: 'bB' });
 		v.mamdani = 'h4';
 		v.potholes = [
-			{ sq: 'a4', by: 'black' },
-			{ sq: 'f4', by: 'white' }
+			{ sq: 'a4', by: 'black', left: 2 },
+			{ sq: 'f4', by: 'white', left: 1 }
 		];
 		return v;
 	},
@@ -87,6 +87,16 @@ export const positions = {
 		v.mamdani = 'a5';
 		return v;
 	},
+	/**
+	 * The Mamdani's repairs. Move it c3–e5 to fix the pothole on f6, or roll
+	 * "Pothole opens" on d4 (next to it) to see one fixed before it opens.
+	 */
+	repair: (): View => {
+		const v = place(emptyView(), { e1: 'wK', b2: 'wP', g2: 'wP', e8: 'bK', b7: 'bP', g7: 'bP' });
+		v.mamdani = 'c3';
+		v.potholes = [{ sq: 'f6', by: 'black', left: 3 }];
+		return v;
+	},
 	/** King and rooks on their squares: castle with e1–g1 or e1–c1. */
 	castling: (): View => {
 		const v = place(emptyView(), { e1: 'wK', a1: 'wR', h1: 'wR', e8: 'bK', a8: 'bR', h8: 'bR' });
@@ -229,28 +239,40 @@ export function playTurn(prev: View, move: MoveJSON, roll: RollScript): View {
 	}
 	Object.assign(v, applyMove(v, move));
 
-	// Close the mover's own pothole, then repair any next to the Mamdani.
-	for (const p of v.potholes.filter((h) => h.by === mover)) ev.push({ kind: 'pothole_closed', sq: p.sq });
-	v.potholes = v.potholes.filter((h) => h.by !== mover);
+	// Count down the mover's potholes (one closes after HOLE_ROUNDS of its
+	// roller's moves), then repair any next to the Mamdani.
+	for (const h of v.potholes) if (h.by === mover) h.left--;
+	for (const p of v.potholes.filter((h) => h.left <= 0)) ev.push({ kind: 'pothole_closed', sq: p.sq, color: p.by });
+	v.potholes = v.potholes.filter((h) => h.left > 0);
+	const dice: string[] = []; // the move log's text, as the server's describe writes it
 	for (const p of v.potholes.filter((h) => v.mamdani && adjacent(h.sq, v.mamdani))) {
 		ev.push({ kind: 'repaired', sq: p.sq });
 		v.stats.repaired++;
+		dice.push(`repairs ${p.sq}`);
 	}
 	v.potholes = v.potholes.filter((h) => !(v.mamdani && adjacent(h.sq, v.mamdani)));
 
 	// The dice.
-	const dice: string[] = [`d8 ${roll.pothole}`];
+	dice.push(`d8 ${roll.pothole}`);
+	const rollPart = dice.length - 1;
 	ev.push({ kind: 'rolled_pothole', roll: roll.pothole, color: mover });
 	if (roll.pothole % 2 === 0 && roll.target) {
 		for (const r of roll.rerolls ?? []) {
-			ev.push({ kind: 'target', sq: r }, { kind: 'reroll', sq: r, reason: 'king' });
+			const { sq: rs, reason } = typeof r === 'string' ? { sq: r, reason: 'king' } : r;
+			ev.push({ kind: 'target', sq: rs }, { kind: 'reroll', sq: rs, reason });
 		}
 		const t = roll.target;
 		ev.push({ kind: 'target', sq: t });
 		let text = `→ ${t}`;
 		const occupant = t === v.mamdani ? 'M' : v.board[squareIndex(t)];
 		let opens = true;
-		if (v.potholes.some((h) => h.sq === t)) {
+		if (occupant?.[1] === 'K') {
+			// Kings never fall: the server re-rolls both d8s. A scripted target
+			// would land here again, so the sandbox stops at the re-roll.
+			ev.push({ kind: 'reroll', sq: t, reason: 'king' });
+			opens = false;
+			text += ' re-roll (kings never fall)';
+		} else if (v.potholes.some((h) => h.sq === t)) {
 			// Already a pothole: the server re-rolls both d8s. A scripted target
 			// would land here again, so the sandbox stops at the re-roll.
 			ev.push({ kind: 'reroll', sq: t, reason: 'pothole' });
@@ -285,11 +307,19 @@ export function playTurn(prev: View, move: MoveJSON, roll: RollScript): View {
 				}
 			}
 		}
+		dice[rollPart] += ` ${text}`;
 		if (opens) {
+			// Past the cap the oldest hole closes as the new one opens.
+			if (v.potholes.length >= HOLE_CAP) {
+				const oldest = v.potholes.shift();
+				if (oldest) {
+					ev.push({ kind: 'pothole_closed', sq: oldest.sq, color: oldest.by });
+					dice.push(`${oldest.sq} closes`);
+				}
+			}
 			ev.push({ kind: 'pothole_opened', sq: t, color: mover });
-			v.potholes.push({ sq: t, by: mover });
+			v.potholes.push({ sq: t, by: mover, left: HOLE_ROUNDS });
 		}
-		dice[0] += ` ${text}`;
 	}
 
 	v.last = ev;
diff --git a/web/src/routes/+page.svelte b/web/src/routes/+page.svelte
index 6f202b9..22a62d7 100644
--- a/web/src/routes/+page.svelte
+++ b/web/src/routes/+page.svelte
@@ -130,7 +130,7 @@
 		<div class="demo">
 			<MiniBoard
 				board={demo}
-				potholes={[{ sq: 'f4', by: 'white' }]}
+				potholes={[{ sq: 'f4', by: 'white', left: 3 }]}
 				mamdani="a5"
 				last={{ from: 'e7', to: 'e6' }}
 				label="A game in progress: a pothole is open on f4 and the Mamdani stands on a5."
````

- [ ] **Step 4: Run the checks**

Run: `pnpm --dir web check && pnpm --dir web test`
Expected: `0 ERRORS 0 WARNINGS`; `Tests 129 passed (129)`.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/game.ts web/src/lib/board.ts web/src/lib/animator.svelte.ts web/src/lib/sandbox.ts web/src/routes/+page.svelte web/src/lib/board.test.ts web/src/lib/sandbox.test.ts web/src/lib/animator.test.ts
git commit -m "web: the board model knows rounds left, the cap and repairs; the sandbox plays the new rules"
```

---

### Task 4: The board: cones, the repair celebration, the Mamdani art

**Files:**
- Modify: `web/src/lib/Board.svelte`, `web/src/lib/MiniBoard.svelte`
- Modify: `web/src/routes/game/[code]/+page.svelte`, `web/src/routes/dev/board/+page.svelte`
- Replace: `web/static/mamdani/piece.webp` (binary; Step 1)

**Interfaces:**
- Consumes: `repairsShown`, `matedByRoll`, `Animator.animated`, `left` on potholes (Task 3).
- Produces:
  - `Board` prop `repairs?: { sq: string; key: string; hole: boolean }[]`: each key celebrates once.
  - Cones: one per round left on each hole's front edge, the last blinking (still under reduced motion); a departing cone lifts away (`liftCone`, 300 ms).
  - The celebration (about 1.3 s after the Mamdani's glide): cone drops, hole shrinks under it, cone lifts, flash and sparks, 👍 above the Mamdani (below it on the top screen row). It plays to the end (`born`/`linger`, 2.4 s) even after the next move. Reduced motion: no cone, flash or sparks; a still 👍 for about 1 s.
  - Pothole squares: `aria-label` and tooltip "c4, pothole (White's, 2 rounds left)" or "…closes after their next move".
  - The Mamdani on both boards is `/mamdani/piece.webp` in a rounded square with the yellow border (pulled forward from milestone 07; the art is the project owner's AI illustration).
  - Game page: celebrations only when `anim.animated && !instant`; the result card says "mated by a pothole after …" and its kicker "checkmate · by a pothole" when `matedByRoll`.
  - Sandbox: a Random/Square choice for the target (Random throws two d8s, re-rolling past kings and open holes), the Repair preset button.

**Why `closeUp` gives a repaired hole 1 ms, not 0:** with Svelte 5.57, in a turn where a repaired hole left with a 0 ms exit while another hole left with a timed exit (a scheduled close, 350 ms), every leaving hole stayed drawn: the board kept the old holes while its labels had the new ones. Reproduced in Chromium and WebKit; a 1 ms exit fixes it. Under reduced motion every exit is 0 ms, which works, so `ms(1)` is right.

- [ ] **Step 1: The Mamdani piece image**

Generate it from the source illustration (1254×1254 PNG, in the project owner's Downloads; ask for it if it's not there). The crop keeps the head down to the tie knot:

```bash
cwebp -quiet -crop 75 40 1150 1150 -resize 256 256 -q 82 -m 6 ~/Downloads/"NYC Skyline Smile Portrait.png" -o web/static/mamdani/piece.webp
```

Expected: a 256×256 WebP of about 15 KB. (The main checkout's uncommitted `web/static/mamdani/piece.webp` is this same file; copying it is equivalent.)

- [ ] **Step 2: The board, the small board and the pages**

````diff
diff --git a/web/src/lib/Board.svelte b/web/src/lib/Board.svelte
index fd508a9..3fda63d 100644
--- a/web/src/lib/Board.svelte
+++ b/web/src/lib/Board.svelte
@@ -13,6 +13,7 @@
 		dim = false,
 		check = '',
 		saved = '',
+		repairs = [],
 		onmove
 	}: {
 		stage: Stage;
@@ -25,6 +26,12 @@
 		check?: string;
 		/** A square whose piece just survived a saving roll, or "". */
 		saved?: string;
+		/**
+		 * Potholes the Mamdani has just repaired, to celebrate. `hole` is true
+		 * when an open pothole is fixed by the Mamdani's move, false when the
+		 * dice land next to it and the pothole never opens. Each key plays once.
+		 */
+		repairs?: { sq: string; key: string; hole: boolean }[];
 		onmove: (move: MoveJSON) => void;
 	} = $props();
 
@@ -61,6 +68,10 @@
 	let targets = $derived(new Set(legal.filter((m) => m.from === current).map((m) => m.to)));
 	let movable = $derived(new Set(active ? legal.map((m) => m.from) : []));
 	let blocked = $derived(new Set(current ? blockedSquares(stage, current) : []));
+	let repairedSquares = $derived(new Set(repairs.map((r) => r.sq)));
+	// A pothole fixed by the Mamdani's move waits for the Mamdani to arrive.
+	let glide = $derived(lastMove && lastMove.to === stage.mamdani && !reducedMotion() ? moveDuration(lastMove.from, lastMove.to) : 0);
+	let sparks = Array.from({ length: 10 }, (_, i) => i * 36);
 
 	// Each piece keeps an id across positions so it can glide (see pieces.ts).
 	// prevPieces is plain bookkeeping for the next reconcile, not state.
@@ -105,7 +116,12 @@
 		const piece = pieceAt(sq);
 		let text = sq;
 		if (piece) text += `, ${pieceName(piece)}`;
-		if (stage.potholes.some((h) => h.sq === sq)) text += ', pothole';
+		const hole = stage.potholes.find((h) => h.sq === sq);
+		if (hole) {
+			text += `, pothole (${hole.by === 'white' ? 'White' : 'Black'}’s`;
+			text += hole.left === 1 ? ', closes after their next move' : `, ${hole.left} rounds left`;
+			text += ')';
+		}
 		return text;
 	}
 
@@ -226,17 +242,48 @@
 		return { duration: ms(420), css: (t: number) => `transform: scale(${t < 0.7 ? t / 0.7 : 1 + 0.12 * Math.sin(((t - 0.7) / 0.3) * Math.PI)}) rotate(${(1 - t) * -20}deg); opacity: ${Math.min(1, t * 2)}` };
 	}
 
-	/** A pothole closes (its roller moved again, or the Mamdani repaired it). */
-	function closeUp(_node: Element) {
+	/**
+	 * A pothole closes on its schedule, or the cap closes it. A repaired one
+	 * hides at once: the celebration draws its own hole shrinking under the
+	 * cone. (1 ms, not 0: with Svelte 5.57, a turn whose leaving holes mixed a
+	 * 0 ms exit with a timed one left them all on the board.)
+	 */
+	function closeUp(_node: Element, { sq }: { sq: string }) {
+		if (repairedSquares.has(sq)) return { duration: ms(1), css: () => 'opacity: 0' };
 		return { duration: ms(350), css: (t: number) => `transform: scale(${t}); opacity: ${t}` };
 	}
 
+	/**
+	 * A repair celebration plays to the end even when the next move arrives
+	 * first (the opponent can reply as soon as the dice stop): the node stays
+	 * until its CSS animations are done.
+	 */
+	const CELEBRATION_MS = 2400;
+	function born(node: HTMLElement) {
+		node.dataset.born = String(performance.now());
+	}
+	function linger(node: Element) {
+		const age = performance.now() - Number((node as HTMLElement).dataset.born);
+		return { duration: Math.max(0, CELEBRATION_MS - age) };
+	}
+
+	/** A round passes: one of a hole's cones lifts away. */
+	function liftCone(_node: Element) {
+		return { duration: ms(300), css: (t: number) => `opacity: ${t}; transform: translateY(${(1 - t) * -60}%)` };
+	}
+
 	/** The target ring drops onto its square. */
 	function drop(_node: Element) {
 		return { duration: ms(260), css: (t: number) => `transform: scale(${1.5 - 0.5 * t}); opacity: ${t}` };
 	}
 </script>
 
+{#snippet coneShape()}
+	<path class="cone-body" d="M17.5 4h5l9.5 29h-24z" />
+	<path class="cone-band" d="M14.6 13h10.8l1.7 5H12.9zM11.6 22h16.8l1.7 5H9.9z" />
+	<rect class="cone-base" x="4" y="32" width="32" height="5" rx="1.5" />
+{/snippet}
+
 <div class="board" class:dim role="group" aria-label="Chessboard" {@attach dragArea}>
 	{#each order as index, n (index)}
 		{@const sq = squareName(index)}
@@ -250,6 +297,7 @@
 			class:movable={movable.has(sq)}
 			class:check={check === sq}
 			aria-label={label(sq)}
+			title={stage.potholes.some((h) => h.sq === sq) ? label(sq) : undefined}
 			aria-pressed={current === sq}
 			onclick={() => tap(sq)}
 			onpointerdown={(e) => pointerDown(e, sq)}
@@ -268,7 +316,18 @@
 
 	<div class="layer" aria-hidden="true">
 		{#each stage.potholes as h (h.sq)}
-			<span class="slot" style={place(h.sq)}><span class="hole" in:crack out:closeUp></span></span>
+			<span class="slot" style={place(h.sq)}>
+				<span class="hole" in:crack out:closeUp={{ sq: h.sq }}>
+					<!-- One cone per round left, on the hole's front edge. -->
+					<span class="cones">
+						{#each { length: h.left }, n (n)}
+							<svg class="mini-cone" class:last={h.left === 1} viewBox="0 0 40 40" out:liftCone>
+								{@render coneShape()}
+							</svg>
+						{/each}
+					</span>
+				</span>
+			</span>
 		{/each}
 		{#if stage.target}
 			{#key stage.target}
@@ -287,7 +346,7 @@
 			>
 				<span class="piece" out:leave={{ fell: stage.target === p.sq }}>
 					{#if p.code === 'M'}
-						<span class="mamdani">M</span>
+						<img class="mamdani" src="/mamdani/piece.webp" alt="" draggable="false" />
 					{:else}
 						<img src="/pieces/{p.code}.svg" alt="" draggable="false" />
 					{/if}
@@ -296,6 +355,24 @@
 		{/each}
 	</div>
 
+	<div class="layer celebrate" aria-hidden="true">
+		{#each repairs as r (r.key)}
+			<span class="slot fix" style="{place(r.sq)}; --delay: {r.hole ? glide : 0}ms" {@attach born} out:linger>
+				{#if r.hole}<span class="hole patched"></span>{/if}
+				<svg class="cone" viewBox="0 0 40 40">
+					{@render coneShape()}
+				</svg>
+				<span class="flash"></span>
+				{#each sparks as a, i (a)}<span class="spark" class:far={i % 2 === 0} style="--a: {a}deg"></span>{/each}
+			</span>
+		{/each}
+		{#if repairs.length > 0 && stage.mamdani}
+			{#key repairs[0].key}
+				<span class="slot fix" class:top={cell(stage.mamdani).row === 0} style="{place(stage.mamdani)}; --delay: {repairs[0].hole ? glide : 0}ms" {@attach born} out:linger|global><span class="thumb">👍</span></span>
+			{/key}
+		{/if}
+	</div>
+
 	{#if pending}
 		<div class="promote" role="dialog" aria-label="Promote to" tabindex="-1" onkeydown={promoKey} {@attach focusFirst}>
 			{#each promoOptions as option (option.promo)}
@@ -469,19 +546,15 @@
 		height: 92%;
 		filter: drop-shadow(0 2px 2px var(--hole));
 	}
-	.mamdani {
-		display: grid;
-		place-items: center;
-		width: 72%;
+	.piece .mamdani {
+		box-sizing: border-box;
+		width: 74%;
+		height: auto;
 		aspect-ratio: 1;
-		border-radius: 50%;
-		background: var(--surface);
+		object-fit: cover;
+		border-radius: 22%;
 		border: 3px solid var(--accent);
-		color: var(--accent);
-		font-family: var(--font-display);
-		font-weight: 800;
-		font-size: clamp(14px, 4vmin, 28px);
-		box-shadow: 0 2px 4px var(--hole);
+		background: var(--surface);
 	}
 	.hole {
 		width: 76%;
@@ -492,6 +565,41 @@
 			0 0 0 3px var(--hazard),
 			inset 0 4px 10px var(--bg);
 	}
+	/* Rounds left: one traffic cone per round, standing on the hole's front
+	   edge; the last one blinks. */
+	.hole {
+		position: relative;
+		display: grid;
+		place-items: center;
+	}
+	.cones {
+		position: absolute;
+		left: 50%;
+		bottom: -16%;
+		display: flex;
+		justify-content: center;
+		gap: 2%;
+		width: 116%;
+		transform: translateX(-50%);
+	}
+	.mini-cone {
+		width: 32%;
+		aspect-ratio: 1;
+		overflow: visible;
+		filter: drop-shadow(0 1px 1px var(--hole));
+	}
+	.mini-cone path,
+	.mini-cone rect {
+		stroke-width: 2.5;
+	}
+	.mini-cone.last {
+		animation: blink 1s ease-in-out infinite;
+	}
+	@keyframes blink {
+		50% {
+			opacity: 0.25;
+		}
+	}
 	.target {
 		width: 92%;
 		height: 92%;
@@ -527,6 +635,215 @@
 		color: var(--text);
 		font: inherit;
 	}
+	/* The Mamdani's repair: a cone drops on the hole, the hole shrinks under
+	   it, the cone lifts, sparks burst and the Mamdani gives a thumbs up.
+	   About 1.3 s after --delay; it never holds up the turn. */
+	.fix {
+		container-type: size;
+		z-index: 4;
+	}
+	.fix > * {
+		grid-area: 1 / 1;
+	}
+	.patched {
+		animation: patch 0.4s ease-in calc(var(--delay) + 250ms) both;
+	}
+	@keyframes patch {
+		to {
+			transform: scale(0);
+			opacity: 0;
+		}
+	}
+	.cone {
+		width: 62%;
+		height: 62%;
+		overflow: visible;
+		animation: cone 0.85s var(--delay) both;
+	}
+	.cone-body {
+		fill: var(--hazard);
+	}
+	.cone-band {
+		fill: var(--piece-light);
+	}
+	.cone-body,
+	.cone-band,
+	.cone-base {
+		stroke: var(--piece-dark);
+		stroke-width: 1.5;
+		stroke-linejoin: round;
+	}
+	.cone-base {
+		fill: var(--hazard);
+	}
+	@keyframes cone {
+		0% {
+			transform: translateY(-90%) scale(1.1);
+			opacity: 0;
+			animation-timing-function: cubic-bezier(0.5, 0, 1, 0.6);
+		}
+		22% {
+			transform: translateY(0) scale(1, 0.86);
+			opacity: 1;
+		}
+		30% {
+			transform: translateY(-6%) scale(1);
+		}
+		76% {
+			transform: translateY(0) scale(1);
+			opacity: 1;
+			animation-timing-function: ease-in;
+		}
+		100% {
+			transform: translateY(-40%) scale(0.7);
+			opacity: 0;
+		}
+	}
+	.flash {
+		width: 100%;
+		height: 100%;
+		border-radius: 50%;
+		background: radial-gradient(circle, var(--accent) 0%, transparent 65%);
+		opacity: 0;
+		animation: flash 0.55s ease-out calc(var(--delay) + 620ms) forwards;
+	}
+	@keyframes flash {
+		0% {
+			transform: scale(0.3);
+			opacity: 0.95;
+		}
+		100% {
+			transform: scale(1.9);
+			opacity: 0;
+		}
+	}
+	.spark {
+		width: 22%;
+		aspect-ratio: 1;
+		background: var(--accent);
+		clip-path: polygon(50% 0, 62% 38%, 100% 50%, 62% 62%, 50% 100%, 38% 62%, 0 50%, 38% 38%);
+		opacity: 0;
+		animation: spark 0.5s ease-out calc(var(--delay) + 650ms) forwards;
+	}
+	@keyframes spark {
+		0% {
+			transform: rotate(var(--a)) translateY(0) scale(0.4);
+			opacity: 1;
+		}
+		70% {
+			opacity: 1;
+		}
+		100% {
+			transform: rotate(var(--a)) translateY(-300%) scale(1);
+			opacity: 0;
+		}
+	}
+	.spark.far {
+		width: 28%;
+		animation-name: spark-far;
+		animation-duration: 0.6s;
+	}
+	@keyframes spark-far {
+		0% {
+			transform: rotate(var(--a)) translateY(0) scale(0.4);
+			opacity: 1;
+		}
+		70% {
+			opacity: 1;
+		}
+		100% {
+			transform: rotate(var(--a)) translateY(-360%) scale(1.1) rotate(45deg);
+			opacity: 0;
+		}
+	}
+	.thumb {
+		font-size: 66cqh;
+		line-height: 1;
+		transform-origin: 50% 100%;
+		filter: drop-shadow(0 2px 2px var(--hole));
+		animation: thumb 1.1s calc(var(--delay) + 700ms) both;
+	}
+	/* On the top row the thumb would leave the board, so it drops in below. */
+	.top .thumb {
+		animation-name: thumb-below;
+	}
+	@keyframes thumb-below {
+		0% {
+			transform: translateY(20cqh) scale(0);
+			opacity: 0;
+			animation-timing-function: cubic-bezier(0.3, 1.6, 0.6, 1);
+		}
+		30% {
+			transform: translateY(62cqh) scale(1.15) rotate(-12deg);
+			opacity: 1;
+		}
+		45% {
+			transform: translateY(62cqh) scale(1) rotate(10deg);
+		}
+		78% {
+			transform: translateY(62cqh) scale(1) rotate(0);
+			opacity: 1;
+		}
+		100% {
+			transform: translateY(70cqh) scale(0.9);
+			opacity: 0;
+		}
+	}
+	@keyframes thumb {
+		0% {
+			transform: translateY(-20cqh) scale(0);
+			opacity: 0;
+			animation-timing-function: cubic-bezier(0.3, 1.6, 0.6, 1);
+		}
+		30% {
+			transform: translateY(-62cqh) scale(1.15) rotate(-12deg);
+			opacity: 1;
+		}
+		45% {
+			transform: translateY(-62cqh) scale(1) rotate(10deg);
+		}
+		58% {
+			transform: translateY(-64cqh) scale(1) rotate(-4deg);
+		}
+		78% {
+			transform: translateY(-68cqh) scale(1) rotate(0);
+			opacity: 1;
+		}
+		100% {
+			transform: translateY(-82cqh) scale(0.9);
+			opacity: 0;
+		}
+	}
+	@media (prefers-reduced-motion: reduce) {
+		.mini-cone.last {
+			animation: none;
+		}
+		.cone,
+		.spark,
+		.flash,
+		.patched {
+			display: none;
+		}
+		.thumb,
+		.top .thumb {
+			--rise: -58cqh;
+			animation: hold 1s both;
+		}
+		.top .thumb {
+			--rise: 58cqh;
+		}
+		@keyframes hold {
+			0%,
+			90% {
+				transform: translateY(var(--rise));
+				opacity: 1;
+			}
+			100% {
+				transform: translateY(var(--rise));
+				opacity: 0;
+			}
+		}
+	}
 	@media (prefers-reduced-motion: reduce) {
 		.board.dim,
 		.piece-slot,
diff --git a/web/src/lib/MiniBoard.svelte b/web/src/lib/MiniBoard.svelte
index 4a891cd..3a93dac 100644
--- a/web/src/lib/MiniBoard.svelte
+++ b/web/src/lib/MiniBoard.svelte
@@ -33,7 +33,7 @@
 		>
 			{#if holes.has(sq)}<span class="hole"></span>{/if}
 			{#if mamdani === sq}
-				<span class="mamdani">M</span>
+				<img class="mamdani" src="/mamdani/piece.webp" alt="" draggable="false" />
 			{:else if board[index]}
 				<img src="/pieces/{board[index]}.svg" alt="" draggable="false" />
 			{/if}
@@ -85,17 +85,13 @@
 		box-shadow: 0 0 0 max(1.5px, 0.5cqw) var(--hazard);
 	}
 	.mamdani {
-		display: grid;
-		place-items: center;
-		width: 72%;
+		box-sizing: border-box;
+		width: 74%;
+		height: auto;
 		aspect-ratio: 1;
-		border-radius: 50%;
-		background: var(--surface);
+		object-fit: cover;
+		border-radius: 22%;
 		border: max(1.5px, 0.5cqw) solid var(--accent);
-		color: var(--accent);
-		font-family: var(--font-display);
-		font-weight: 800;
-		font-size: 5cqw;
-		line-height: 1;
+		background: var(--surface);
 	}
 </style>
diff --git a/web/src/routes/dev/board/+page.svelte b/web/src/routes/dev/board/+page.svelte
index 8fce653..5c31f25 100644
--- a/web/src/routes/dev/board/+page.svelte
+++ b/web/src/routes/dev/board/+page.svelte
@@ -4,7 +4,7 @@
 	import MoveLog from '#lib/MoveLog.svelte';
 	import PlayerBar from '#lib/PlayerBar.svelte';
 	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
-	import { pillFor, stageAt } from '#lib/board.ts';
+	import { pillFor, repairsShown, stageAt } from '#lib/board.ts';
 	import type { Color, MoveJSON, View } from '#lib/game.ts';
 	import { setInstant } from '#lib/motion.ts';
 	import { freeMoves, playTurn, positions, type RollScript } from '#lib/sandbox.ts';
@@ -30,6 +30,7 @@
 
 	let rollKind = $state<RollKind>('empty');
 	let target = $state('d4');
+	let targetMode = $state<'square' | 'random'>('square');
 	let anySide = $state(false);
 	let slow = $state(false);
 	let instant = $state(false); // no animation at all, for fast play-testing
@@ -45,25 +46,46 @@
 		return m?.from && m?.to ? { from: m.from, to: m.to } : null;
 	});
 	let savedSquare = $derived(view.last.find((e, i) => e.kind === 'saving_roll' && e.saved && i < anim.shown)?.sq ?? '');
+	// The Mamdani's repairs, celebrated only on a turn that is playing out.
+	let repairs = $derived(anim.animated && !instant ? repairsShown(view, anim.shown) : []);
 	let needsTarget = $derived(rollKinds.find((r) => r.kind === rollKind)?.needsTarget ?? false);
 
 	function d8(): number {
 		return Math.floor(Math.random() * 8) + 1;
 	}
 
+	/**
+	 * Where the placement dice land: the typed square, or, with Random, two d8s
+	 * re-rolled past kings and open potholes as the server does.
+	 */
+	function placement(v: View): Pick<RollScript, 'rerolls' | 'target'> {
+		if (targetMode === 'square') return { target };
+		const rerolls: NonNullable<RollScript['rerolls']> = [];
+		for (let i = 0; i < 64; i++) {
+			const sq = 'abcdefgh'[d8() - 1] + d8();
+			const piece = sq === v.mamdani ? 'M' : v.board['abcdefgh'.indexOf(sq[0]) + (Number(sq[1]) - 1) * 8];
+			if (piece?.[1] === 'K') rerolls.push({ sq, reason: 'king' });
+			else if (v.potholes.some((h) => h.sq === sq)) rerolls.push({ sq, reason: 'pothole' });
+			else return { rerolls, target: sq };
+		}
+		return { rerolls, target };
+	}
+
 	function script(v: View): RollScript {
 		switch (rollKind) {
 			case 'odd':
 				return { pothole: 1 };
 			case 'empty':
 			case 'falls':
-				return { pothole: 2, target };
+				return { pothole: 2, ...placement(v) };
 			case 'saved':
-				return { pothole: 4, target, save: 3 };
+				return { pothole: 4, ...placement(v), save: 3 };
 			case 'notSaved':
-				return { pothole: 4, target, save: 6 };
-			case 'rerollFalls':
-				return { pothole: 6, rerolls: ['e1'], target };
+				return { pothole: 4, ...placement(v), save: 6 };
+			case 'rerollFalls': {
+				const p = placement(v);
+				return { pothole: 6, rerolls: ['e1', ...(p.rerolls ?? [])], target: p.target };
+			}
 			case 'mamdaniFalls':
 				return v.mamdani ? { pothole: 8, target: v.mamdani, save: 2 } : { pothole: 1 };
 			case 'random': {
@@ -121,10 +143,14 @@
 				{#each rollKinds as r (r.kind)}
 					<label class="choice"><input type="radio" name="roll" value={r.kind} bind:group={rollKind} /> {r.label}</label>
 				{/each}
-				<label class="field" class:disabled={!needsTarget}>
-					Target square
-					<input type="text" bind:value={target} maxlength="2" disabled={!needsTarget} />
-				</label>
+				<fieldset class="target" class:disabled={!needsTarget} disabled={!needsTarget}>
+					<legend>Target</legend>
+					<label class="choice"><input type="radio" name="target" value="random" bind:group={targetMode} /> Random</label>
+					<label class="field">
+						<span class="choice"><input type="radio" name="target" value="square" bind:group={targetMode} /> Square</span>
+						<input type="text" bind:value={target} maxlength="2" aria-label="Target square" onfocus={() => (targetMode = 'square')} />
+					</label>
+				</fieldset>
 			</section>
 
 			<section class="panel" aria-labelledby="view-heading">
@@ -159,6 +185,7 @@
 					<button onclick={() => load(positions.blockedLines())}>Blocked lines</button>
 					<button onclick={() => load(positions.promotion())}>Promotion</button>
 					<button onclick={() => load(positions.castling())}>Castling</button>
+					<button onclick={() => load(positions.repair())}>Repair</button>
 				</div>
 			</section>
 
@@ -182,6 +209,7 @@
 				interactive={!anim.animating && view.status === 'playing'}
 				dim={!!view.result && !anim.animating}
 				saved={savedSquare}
+				{repairs}
 				onmove={move}
 			/>
 			<PlayerBar color={bottom} you={you === bottom} lost={stage.lost[bottom]} pill={pillFor(view, bottom, anim.animating).text} pillTone={pillFor(view, bottom, anim.animating).tone} />
@@ -275,9 +303,20 @@
 	.field {
 		justify-content: space-between;
 	}
-	.field.disabled {
+	.target.disabled {
 		opacity: 0.5;
 	}
+	.target {
+		margin: 4px 0 0;
+		padding: 0;
+		border: 0;
+	}
+	.target legend {
+		padding: 0;
+		margin-bottom: 4px;
+		color: var(--text-body);
+		font-size: 14px;
+	}
 	input[type='text'],
 	select {
 		min-height: 36px;
diff --git a/web/src/routes/game/[code]/+page.svelte b/web/src/routes/game/[code]/+page.svelte
index 2c7e796..b81a552 100644
--- a/web/src/routes/game/[code]/+page.svelte
+++ b/web/src/routes/game/[code]/+page.svelte
@@ -13,7 +13,7 @@
 	import MovesSheet from '#lib/MovesSheet.svelte';
 	import PlayerBar from '#lib/PlayerBar.svelte';
 	import { Animator, STEP_MS } from '#lib/animator.svelte.ts';
-	import { checkSquare, pillFor, stageAt } from '#lib/board.ts';
+	import { checkSquare, matedByRoll, pillFor, repairsShown, stageAt } from '#lib/board.ts';
 	import { applyMove, settlesGuess } from '#lib/pieces.ts';
 	import { firstMoveLeft, paused, timeLeft } from '#lib/clock.ts';
 	import { notify } from '#lib/toast.ts';
@@ -173,6 +173,8 @@
 	});
 	let checked = $derived(view && stage ? checkSquare(view, stage, { animating, guessing: optimistic?.seq === view.seq }) : '');
 	let savedSquare = $derived(view?.last.find((e, i) => e.kind === 'saving_roll' && e.saved && i < shown)?.sq ?? '');
+	// The Mamdani's repairs, celebrated only on a turn that is playing out (never after a reload).
+	let repairs = $derived(view && anim.animated && !instant ? repairsShown(view, shown) : []);
 	let you = $derived(view?.you ?? 'spectator');
 	let bottom = $derived<Color>(you === 'black' ? 'black' : 'white');
 	let top = $derived<Color>(bottom === 'white' ? 'black' : 'white');
@@ -328,7 +330,7 @@
 		let detail = `By ${why}.`;
 		let title = r.draw ? 'Draw' : `${winner} wins`;
 		if (r.reason === 'resignation') detail = `${loser} resigned.`;
-		if (r.reason === 'checkmate') detail = `${winner} mated with ${lastSan}.`;
+		if (r.reason === 'checkmate') detail = matedByRoll(view) ? `${winner} mated by a pothole after ${lastSan}.` : `${winner} mated with ${lastSan}.`;
 		if (r.reason === 'timeout') detail = `${loser} ran out of time.`;
 		if (r.reason === 'timeout_vs_insufficient')
 			detail = `${toMove} ran out of time; ${toMove === 'White' ? 'Black' : 'White'} couldn’t mate.`;
@@ -341,7 +343,7 @@
 			detail = 'Nobody joined within a day.';
 		}
 		return {
-			kicker: `${why} · Move ${Math.max(1, Math.ceil(view.seq / 2))}`,
+			kicker: `${matedByRoll(view) ? `${why} · by a pothole` : why} · Move ${Math.max(1, Math.ceil(view.seq / 2))}`,
 			title,
 			detail
 		};
@@ -397,6 +399,7 @@
 					dim={!!resultCard || view.status === 'waiting'}
 					check={checked}
 					saved={savedSquare}
+					{repairs}
 					onmove={move}
 				/>
 			</div>
@@ -551,6 +554,7 @@
 						dim={!!resultCard}
 						check={checked}
 						saved={savedSquare}
+						{repairs}
 						onmove={move}
 					/>
 					{#if resultCard}
````

- [ ] **Step 3: Check the components**

Run for each of the four `.svelte` files: `npx @sveltejs/mcp svelte-autofixer <file>`
Expected: no issues (the game page's SvelteSet suggestion is pre-existing).
Run: `pnpm --dir web check && pnpm --dir web test && pnpm --dir web build`
Expected: `0 ERRORS`, `129 passed`, `✔ done`.

- [ ] **Step 4: Look at it in the sandbox**

With `pnpm --dir web dev` running, open `/dev/board` and check, at 1280 px and at 390 px wide:
- **Repair** preset, "Odd: nothing happens", move the Mamdani c3–e5: it glides, then the cone drops on f6, the hole shrinks, sparks burst, 👍 pops above the Mamdani; the log reads `repairs f6 · d8 1`.
- **Repair**, "Pothole opens", Target Square `d7`, Mamdani c3–c8: the cone drops on d7's ring and 👍 drops in *below* the Mamdani (top row).
- "Pothole opens", Target Random, play 10 or so moves: holes show 3, 2, 1 cones; the last blinks; a cone lifts away each round; at 5 holes the next one closes the oldest as it opens, and the tray says "At most 5 potholes: the oldest, on …, closes".
- Typed target `e1`: "Re-roll — Kings never fall"; the king stays.
- Hover a hole: the tooltip names its roller and rounds left.

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/Board.svelte web/src/lib/MiniBoard.svelte web/static/mamdani/piece.webp "web/src/routes/game/[code]/+page.svelte" web/src/routes/dev/board/+page.svelte
git commit -m "web: cones count holes down, the Mamdani's repair is celebrated, Mamdani art on the boards"
```

---

### Task 5: The playtest checks the holes drawn

**Files:**
- Modify: `web/scripts/playtest.js`

**Interfaces:**
- Consumes: the `.layer .slot > .hole` elements and the pothole squares' `aria-label`s (Task 4); the celebration's own hole is `.hole.patched` and is ignored.
- Produces: after every move that lands, `holesMatch(page)` compares holes drawn with pothole squares in the board's labels for both players, rechecks after 600 ms, and fails the game with "the board draws N potholes but has M".

This is the regression check for the Task 4 bug. In the scratch build, putting the 0 ms exit back made 2 of 4 animated games fail with "the board draws 3 potholes but has 1". Instant-mode games can't show it (every exit is 0 ms there), so Task 7 runs it with animations too.

- [ ] **Step 1: Add the check**

````diff
diff --git a/web/scripts/playtest.js b/web/scripts/playtest.js
index ed1efdb..360ba30 100644
--- a/web/scripts/playtest.js
+++ b/web/scripts/playtest.js
@@ -1,6 +1,7 @@
 // Plays whole games through the real UI, as two guests per game, with random
 // legal moves, and reports how each one ended. It fails (exit 1) on any page
-// error, console error, move that never lands, or board that gets stuck.
+// error, console error, move that never lands, board that gets stuck, or
+// pothole left drawn after it closed.
 //
 // Needs the Go server (:8080) and the dev server (:5173) running; it uses the
 // game page's dev-only ?instant mode, so a whole game takes seconds.
@@ -117,6 +118,25 @@ async function dragMove(page) {
 let pairing = Promise.resolve();
 
 /** Both guests tap Play online, the second once the first is queued. */
+/**
+ * Whether the potholes drawn match the board's own model (each pothole
+ * square's label says so). A hole can still be shrinking when the turn
+ * lands, so a mismatch is checked again after its exit has had time to end.
+ */
+async function holesMatch(page) {
+	const count = () =>
+		page.evaluate(() => [
+			document.querySelectorAll('.layer .slot > .hole:not(.patched)').length,
+			document.querySelectorAll('.square[aria-label*="pothole"]').length
+		]);
+	let [drawn, model] = await count();
+	if (drawn !== model) {
+		await sleep(600);
+		[drawn, model] = await count();
+	}
+	return drawn === model ? '' : `the board draws ${drawn} potholes but has ${model}`;
+}
+
 async function quickMatch(first, second) {
 	for (const p of [first, second]) {
 		await p.goto(`${opts.base}/`);
@@ -201,6 +221,11 @@ async function playGame(browser, n) {
 		}
 		if (move.dragged) drags++;
 		plies++;
+		const holes = (await holesMatch(w)) || (await holesMatch(b));
+		if (holes) {
+			errors.push(`ply ${plies}: ${holes}`);
+			break;
+		}
 		if (opts.phone) {
 			const bad = (await phoneLayout(w)) || (await phoneLayout(b)) || (plies === 10 ? await sheetOpensAndCloses(other) : '');
 			if (bad) {
````

- [ ] **Step 2: Run it**

With both servers running (`go run ./cmd/server`, `pnpm --dir web dev`):
Run: `pnpm --dir web playtest --games 4`
Expected: `chromium: 4/4 games finished cleanly`.

- [ ] **Step 3: Commit**

```bash
git add web/scripts/playtest.js
git commit -m "playtest: fail when the board still draws a pothole that has closed"
```

---

### Task 6: The rules written down

**Files:**
- Modify: `RULES.md`, `CLAUDE.md`
- Edit: the live rules doc, https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB (the source of truth; `RULES.md` is its copy)

**Interfaces:**
- Consumes: the rules as built (Tasks 1–2).
- Produces: the live doc and `RULES.md` agree with the engine: goals 2–3 reworded; decisions #4, #10, #11 and #16 rewritten with their reasons; turn order with "Count down"; setup with cone markers; "exposes" covering the cap; repetition with rounds left; two new "Watch in test games" items.

- [ ] **Step 1: Update `RULES.md` and `CLAUDE.md`**

````diff
diff --git a/CLAUDE.md b/CLAUDE.md
index ab4ccd4..65c48ef 100644
--- a/CLAUDE.md
+++ b/CLAUDE.md
@@ -13,10 +13,10 @@ A browser chess variant to play with friends: the original Pot-Hole Chess (Spice
 
 ## Decisions that are settled (don't reopen without asking)
 
-- Rules are the original Pot-Hole Chess plus only the Mamdani. Roll a d8 after every move; even opens a new pothole; it closes after the roller's next move. Kings never fall (re-roll). All dice are d8.
+- Rules are the original Pot-Hole Chess plus the Mamdani, with longer potholes (milestone 06c). Roll a d8 after every move; even opens a new pothole; it closes after 3 of the roller's moves; at most 5 are open (the oldest closes). A roll can checkmate, but kings never fall (re-roll). All dice are d8.
 - Stack: Go server (SSE out, JSON POST in, SQLite), SvelteKit static frontend with a custom board (no chessground). One Railway service.
 - Guests only, all games public, one clock (10+5), quick match only, emoji reactions instead of chat.
-- Visual design: "road works", dark only. Potholes are a dark hole with an orange ring.
+- Visual design: "road works", dark only. Potholes are a dark hole with an orange ring; small traffic cones on its front edge count the rounds left. The Mamdani's repair is celebrated (cone, sparks, 👍).
 
 ## Working rules
 
diff --git a/RULES.md b/RULES.md
index 9ed98b1..1ea074b 100644
--- a/RULES.md
+++ b/RULES.md
@@ -2,41 +2,41 @@
 
 Live doc: https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB (source of truth; this file is a copy)
 
-This is Pot-Hole Chess (Spicer and Chamberlain, 2001) with one addition. Potholes open at random and swallow pieces, exactly as in the original. The addition is the Mamdani: a neutral piece either player can move, which blocks lines, repairs potholes and saves pieces from falling. The rules are locked; see the decisions below.
+This is Pot-Hole Chess (Spicer and Chamberlain, 2001) with longer-lasting potholes and one addition. Potholes open at random and swallow pieces, as in the original, but each stays open for three rounds. The addition is the Mamdani: a neutral piece either player can move, which blocks lines, repairs potholes and saves pieces from falling. The rules are locked; see the decisions below.
 
 ## How it should feel
 
 Chess comes first; the dice bring chaos, and the Mamdani is how you fight back. Every rule below is judged against these goals.
 
 1. **The Mamdani is your answer to the dice.** The dice roll after every move and you can't control them. Where you put the Mamdani decides which pieces they can hurt.
-2. **Luck finishes, skill sets up.** A roll can decide a game only after good chess made it matter. No instant wins out of nowhere.
-3. **Every bad roll has an answer.** Block, capture, or send the Mamdani. The player hit should always have counterplay.
+2. **Luck finishes, skill sets up.** A roll can deliver mate, but kings never fall, so it only finishes a king that play has already cornered.
+3. **Most bad rolls have an answer.** Block, capture, or send the Mamdani.
 4. **Readable at a glance.** Players and spectators can see what's open, what's blocked and why. The portal game showed that hidden or moving hazards feel unfair.
 5. **Fun to watch.** Every roll is shown big, so a pothole opening or a piece being saved is an event everyone reacts to.
 6. **Simple to explain.** A new player learns the additions in one minute.
 
 ## Decisions
 
-All sixteen are agreed.
+All sixteen are agreed. Rows 4, 10, 11 and 16 changed on 2026-10-05 (milestone 06c): potholes closed too fast in sandbox play, and a simulation of 2,000 games per variant found about 2.5 holes open on average, the cap closing about one hole a game, and mates by roll in about 1 game in 1,000.
 
 | # | Decision | Rule |
 | --- | --- | --- |
 | 1 | When potholes happen | Automatic, after your move (original). Move first, then roll a d8. Even: a new pothole opens (#10). Odd: nothing. The dice never cost you a move. |
 | 2 | How many rolls | Unlimited (original). One roll after every move, for both players. |
 | 3 | Where it lands | Fully random (original). Two d8s pick the file and rank. |
-| 4 | How long it lasts | One round (original). A pothole closes when the player who rolled it finishes their next move. The Mamdani can repair it sooner. |
+| 4 | How long it lasts | Three rounds. A pothole closes when the player who rolled it finishes their third move after opening it. The Mamdani can repair it sooner. (Was one round, as in the original: holes closed before they mattered.) |
 | 5 | How the Mamdani repairs | Next to it. Any pothole on the 8 squares around the Mamdani is fixed at once. Saving rolls stay as in #7. |
 | 6 | Pothole lands on a king | Re-roll. Kings never fall, as in the original Pot-Hole Chess. Re-roll both d8s. |
 | 7 | Saving rolls | Keep. It's the Mamdani's defensive job and the answer to a bad roll (goal 3). |
 | 8 | Sliders and potholes | Can't cross, with blocked lines shown on the board (goal 4). Knights can jump over a pothole but can't land on it. |
 | 9 | Stalling with the Mamdani | No rule. Either player may move it anywhere it can reach, including straight back. Threefold repetition ends any back-and-forth as a draw. |
-| 10 | Number of potholes | New pothole on every even roll (original). Potholes never move; each one lasts as set in #4. |
-| 11 | Pothole would checkmate | Re-roll. If the result of the roll (a piece falling, or a hole opening on an empty square) would leave the next player checkmated, re-roll both d8s. A roll never wins on its own (goals 2, 3). |
+| 10 | Number of potholes | New pothole on every even roll (original), at most 5 open. When a pothole opens with 5 already open, the oldest closes first. Potholes never move. |
+| 11 | Pothole would checkmate | It stands: a roll can deliver checkmate. Kings still never fall (#6), so a roll can trap a king but never take it. (Was a re-roll; changed by choice, and it decides about 1 game in 1,000.) |
 | 12 | Saving-roll reach | Clear queen line. Nothing between the Mamdani and the square; nothing else is checked. |
 | 13 | Endless re-rolls | Cap at 64. After 64 re-rolls without a valid square, no pothole opens that turn. |
 | 14 | Pawns, castling, en passant | Blocks like a piece. No castling through or onto a pothole (rook's path included), no pawn double-step across one, and a pothole on the en passant square cancels that capture. |
 | 15 | Roll after a checkmating move | No roll. A move that checkmates ends the game at once, so the dice can't undo a mate made on the board (goal 2). |
-| 16 | Check held off only by your own pothole | Checkmate. Every move closes your own pothole, so if the king would then be in check and no move fixes it, it is checkmate. |
+| 16 | Check held off only by your own pothole | Checkmate, if that pothole is on its last round: your next move closes it, so if the king would then be in check and no move fixes it, it is checkmate. |
 
 ## Setup
 
@@ -47,7 +47,7 @@ Set up a normal chessboard, then put the Mamdani on a5. That square is empty at
 | Chess set | Standard starting position |
 | Mamdani token | Any distinct piece or coin, placed on a5 |
 | 3 eight-sided dice (d8) | All rolls: the pothole roll, the pothole's file (1 = a … 8 = h) and rank, and saving rolls |
-| A few checkers | Mark open potholes |
+| Up to 5 checkers, and 3 small markers each (cones or coins) | Mark open potholes and the rounds each has left |
 
 White moves first, as usual.
 
@@ -56,22 +56,21 @@ White moves first, as usual.
 Every turn is one move followed by one pothole roll. Play these steps in order:
 
 1. **Move.** Make one legal move: either one of your own pieces or the Mamdani.
-2. **Close.** If you opened a pothole on your previous turn, it closes now.
+2. **Count down.** Each pothole you rolled loses a round (take a marker off it). One with no rounds left closes now.
 3. **Repair.** Any open pothole next to the Mamdani (any of its 8 surrounding squares) is repaired and removed.
 4. **Roll for a pothole.** Roll a d8. Odd: nothing happens and the turn ends. Even: a pothole opens.
-5. **Place it.** Roll two d8s for file and rank. Resolve that square as described in Potholes and Saving rolls.
+5. **Place it.** Roll two d8s for file and rank. Resolve that square as described in Potholes and Saving rolls. If it opens with 5 already open, the oldest closes first. A new pothole has 3 rounds.
 
 ## Potholes
 
-A pothole destroys whatever stands on its square, unless a saving roll succeeds. It then blocks that square until the player who rolled it finishes their next move. Every even roll opens a new pothole; potholes never move. Two can be open at once, one from each player, each closing on its own schedule.
+A pothole destroys whatever stands on its square, unless a saving roll succeeds. It then blocks that square for three rounds: it closes when the player who rolled it finishes their third move after opening it. Every even roll opens a new pothole; potholes never move. Up to 5 can be open at once, each closing on its own schedule; when a pothole opens with 5 already open, the oldest closes first.
 
 When the d8s land on a square:
 
 - **King:** kings never fall. Re-roll both d8s.
 - **Next to the Mamdani:** the pothole is repaired the moment it opens. Nothing falls.
 - **Already a pothole:** re-roll both d8s.
-- **Would expose the roller's king:** if the piece falling would leave the player who just moved in check once the hole closes, re-roll both d8s.
-- **Would checkmate the next player:** if the result (a piece falling, or a hole on an empty square taking an escape or blocking square) would leave the next player checkmated, re-roll both d8s. A roll never wins the game on its own.
+- **Would expose the roller's king:** if the piece falling would leave the player who just moved in check once the hole closes, or if the cap closing the oldest pothole would leave them in check, re-roll both d8s.
 - **Too many re-rolls:** after 64 re-rolls without a valid square, no pothole opens this turn.
 - **Empty square:** the pothole opens. Mark it with a checker.
 - **Any other piece:** the piece falls in and is lost, unless it is saved (see Saving rolls).
@@ -83,7 +82,7 @@ While a pothole is open:
 - Bishops, rooks, queens and the Mamdani cannot slide across it.
 - Knights may jump over it but cannot land on it.
 - Because sliders can't cross it, a pothole blocks attacks and checks along that line, like a piece would.
-- Your move is illegal if your king would be in check after your own pothole closes and nearby potholes are repaired.
+- Your move is illegal if your king would be in check after your potholes on their last round close and nearby potholes are repaired.
 
 ## The Mamdani
 
@@ -112,12 +111,12 @@ A saving roll is one d8. An odd number saves the piece, and the pothole never op
 
 ## Winning and draws
 
-You win by checkmate, as in normal chess. Kings never fall into potholes, and a fall that would checkmate is re-rolled, so a pothole can't win the game directly.
+You win by checkmate, as in normal chess. Kings never fall into potholes, but a roll can deliver checkmate: a hole on a king's last escape square, or a blocking piece falling in.
 
-- **Checkmate:** check where no move of your pieces and no Mamdani move gets you out of check. A move that checkmates ends the game at once; no pothole roll follows. If your own open pothole is the only thing blocking a check and no move fixes it, that is also checkmate, because every move closes that pothole.
+- **Checkmate:** check where no move of your pieces and no Mamdani move gets you out of check. A move that checkmates ends the game at once; no pothole roll follows. If your own pothole on its last round is the only thing blocking a check and no move fixes it, that is also checkmate, because your next move closes that pothole.
 - **Stalemate:** a draw, but only if you also have no legal Mamdani move.
 - **50-move rule:** moving the Mamdani does not reset the count. Losing a piece to a pothole does reset it; the Mamdani falling does not.
-- **Repetition:** a position counts as repeated only if the pieces, the Mamdani and the open potholes are all the same.
+- **Repetition:** a position counts as repeated only if the pieces, the Mamdani, the open potholes and the rounds each has left are all the same.
 - Castling, en passant and promotion work as normal. A pothole blocks like a piece: you cannot castle through or onto one (including the rook's path over b1 or b8), a pawn cannot double-step across one, and a pothole on the en passant square cancels that capture.
 
 ## Watch in test games
@@ -126,7 +125,8 @@ The rules are locked. These are the things to watch when the group plays, in cas
 
 - [ ] **Stalling:** do players shuffle the Mamdani to waste time? If so, ban moving it two turns in a row.
 - [ ] **Chaos:** an even d8 opens a pothole about every other move, as in the original. If it feels too swingy, the house rules can make it rarer.
-- [ ] **Readability:** can everyone tell at a glance which lines a pothole blocks?
+- [ ] **Readability:** can everyone tell at a glance which lines a pothole blocks, and how long each has left?
+- [ ] **Petering out:** about 9 pieces fall a game. Do games end with too little material to mate?
 
 ## Sources
 
````

- [ ] **Step 2: Update the live rules doc**

Read the live doc (with the Artifact tool's `read`, or the docs tools if it is a Claude Doc), make the same edits as the `RULES.md` diff above in its own wording and layout, including its "Decisions to lock down" table rows #4, #10, #11 and #16 with their reasons. **Show the user the changed rows and ask before publishing**, since the doc is shared. Then check that the doc and `RULES.md` say the same thing.

- [ ] **Step 3: Commit**

```bash
git add RULES.md CLAUDE.md
git commit -m "Docs: RULES.md and CLAUDE.md for longer potholes"
```

---

### Task 7: Final checks and the roadmap

**Files:**
- Modify: `docs/superpowers/plans/2026-10-03-00-roadmap.md` (row 06c), `CLAUDE.md` ("Where things stand", "Next step")

- [ ] **Step 1: Every check**

```bash
go vet ./... && go test -race -short ./...
go test ./rules -count=1
pnpm --dir web check && pnpm --dir web test && pnpm --dir web build
```
Expected: all `ok`; `0 ERRORS`; `129 passed`; `✔ done`.

- [ ] **Step 2: Whole games, instant mode**

With both servers running:
```bash
pnpm --dir web playtest --games 8
pnpm --dir web playtest --browser webkit --games 8
pnpm --dir web playtest --phone --games 4
```
Expected: every game `ok`. (In the scratch build: Chromium 12/12, WebKit 12/12, phone 4/4; games run 150–350 plies and mostly end in insufficient material or the 50-move rule, since random movers lose about 15 pieces to potholes.)

- [ ] **Step 3: Whole games with full animations**

Instant mode turns every exit to 0 ms, so the Task 4 bug only shows with animations. Build, then run the Go server alone (it serves the build) on a spare port with a throwaway database:
```bash
pnpm --dir web build
PORT=8091 DB_PATH=/tmp/m06c-check.db go run ./cmd/server   # in another terminal; stop it with Ctrl-C afterwards
pnpm --dir web playtest --base http://localhost:8091 --games 3 --max-plies 60 --turn-ms 15000
pnpm --dir web playtest --base http://localhost:8091 --browser webkit --games 3 --max-plies 60 --turn-ms 15000
```
Expected: 3/3 each (games resign at the ply limit; each takes 75–90 s). Stop that server by its own process; never `pkill -f cmd/server`, which also stops the main dev server on :8080.

- [ ] **Step 4: The roadmap and CLAUDE.md**

- Roadmap row 06c: Status `Done ([plan](2026-10-05-06c-longer-potholes.md))`; row 07: Status `Next`.
- CLAUDE.md "Where things stand": add milestone 06c to **Done** (potholes last 3 rounds, at most 5, a roll can mate, cones, the repair celebration, old games retired). "Next step": milestone 07 (launch), whose brainstorm has started (audience a few friends, the Railway subdomain, the canvas rules page plus a collapsible full-rules section, one spec); the Mamdani art is already on the boards.

```bash
git add docs/superpowers/plans/2026-10-03-00-roadmap.md CLAUDE.md
git commit -m "Docs: 06c done, 07 next"
```

- [ ] **Step 5: Whole-branch review, then hand back**

Request a whole-branch review (superpowers:requesting-code-review) against `main`, with the spec and this plan's Review Focus. Fix what it finds. Then use superpowers:finishing-a-development-branch. **Don't push to `main` without asking:** the deploy ends every game in progress.

## Notes for the executor

- **The Go server on :8080 and Vite on :5173** may already be running in the main checkout, serving the old rules. In a worktree, run your own on other ports (`PORT=8091 go run ./cmd/server`, and for Vite point its `/api` proxy at that port locally without committing the change). Stop only the processes you started.
- **Two players in one browser:** `localhost`, `[::1]` and `127.0.0.1` keep separate cookies, so each origin is a different guest.
- **Random-game counts move:** the full rules suite plays 10,000 games; with more holes it takes about 4 s without `-race` (was 3.6 s), and `LegalMoves` is about 15% slower because `Position` is bigger. The rules-speed side plan still applies.
- **The sandbox's scripted dice** put the hole exactly where you type, except on a king (re-roll) or an open hole (re-roll). Random mirrors the server.
