# Go Rules Engine Speed Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the Go `rules` package about 4.5× faster per game, with 18× fewer allocations, without changing a single rule:
- move generation stops allocating;
- the board gains bitboards, as in the Rust engine.

**Architecture:**
- **Step one, allocation-free move generation.**
  - Move lists are built in a stack buffer.
  - "Is there any legal move?" stops at the first one.
  - `play`'s event list becomes an optional pointer, so legality checks record nothing.
- **Step two, bitboards.** `Position` keeps its exported `Board [64]Piece`. It adds bitboards (`byColor`, `byKind`) that `put` and `take` keep in step. Attack detection and move generation then use precomputed attack tables and classical ray attacks over one "blocked" set: pieces, the Mamdani and potholes. That design is ported from `server_rs/rules/src/bitboard.rs`.
- The public API stays the same, except that the order of `LegalMoves()` changes.

**Tech Stack:** Go 1.27 standard library (`math/bits`).

**Spec:** the rules are `RULES.md` and the Plan 02 interpretations (`docs/superpowers/plans/2026-10-03-02-rules-engine.md`, "Interpretations made here"). None change. The bitboard design is in `docs/superpowers/specs/2026-10-04-rust-server-design.md` ("Rules crate").

**Priority:** optional. The roadmap says "no scale work until people are playing", and a human game spends microseconds in the engine. Do this when bulk speed starts to matter: a bigger random-game suite, a computer opponent, or analysing saved games. It is independent of the server improvements plan (`2026-10-04-go-server-improvements.md`), and either can go first.

**Prototyped:** every block here was run in a throwaway worktree first. All Go tests passed, including 10,000 random games with the new invariants. The Rust parity test matched 300 games recorded from the bitboard Go engine. Measured on the author's machine (`-count 3`):

| Benchmark | Today | After Task 2 | After Task 5 | Rust |
| --- | --- | --- | --- | --- |
| `LegalMoves`, start position | 1.64 µs, 36 allocs | 1.29 µs, 1 alloc | 0.91 µs, 1 alloc | — |
| Perft, start, depth 4 | 15.5 ms, 249k allocs | 13.7 ms, 18.6k allocs | 7.0 ms, 18.6k allocs | 3.4 ms |
| One random game | 2.2 ms, 35.7k allocs | 0.85 ms, 2.0k allocs | 0.47 ms, 2.0k allocs | 0.48 ms |
| `go test ./rules` (10,000 random games) | ~20 s | ~4 s | ~3 s | — |

## Global Constraints

- **No rule changes.** Every existing rules test passes unchanged: perft, the 10,000 random games, and the turn, SAN and game tests. Where they disagree with the new code, the code is wrong.
- **Public API stays:** `LegalMoves() []Move`, `Attacked`, `InCheck`, `Blocked`, `King`, `Mated`, `CannotMate`, `Apply`, `Play`, `Replay`, `SAN`, `Position.Board`. Only the *order* of `LegalMoves()` changes. Nothing in the repo depends on it: the frontend shows moves as dots, and the Rust parity test sorts.
- Go version stays as `go.mod` says. No new dependencies.
- Commits: stage only the files you changed; no Claude attribution (CLAUDE.md).
- After every task, `go vet ./...` and `go test ./...` pass.

## Review Focus

- **Positions built by hand.** Code that writes `p.Board[s] = ...` directly leaves the bitboards stale, and the engine quietly misjudges the position. Expected: only `put`/`take` write pieces. Today nothing outside `rules/play.go`, `position.go` and `turn.go` writes to `Board`. Task 4 adds `checkBitboards` to the random-game invariants, and documents the rule on the `Board` field.
- **Very long move lists.** Positions with more pseudo-legal moves than the stack buffer holds (320) must still be complete. Go's `append` just moves to the heap. Task 2 adds the record 218-move position to perft.
- **The early-exit helpers drifting from `LegalMoves`.** `hasLegalMove` and `isLegal` are separate code paths, and any disagreement would change game results. Task 2 adds two random-game invariants: `hasLegalMove() == (len(LegalMoves()) > 0)`, and `isLegal(m)` for every move played.
- **Sliders and potholes.** A rook's attack ray must stop *at* a pothole or the Mamdani (blocked, not landable), exactly as at a piece. Task 3 compares every square and 400 blocker patterns against a square-by-square walk, and the existing `TestSlidersStopAtPotholes` covers landing.
- **The legacy dice path.** `Apply` uses `checkedDice`, and its events must still come out in order. The events from `play` now go through a pointer. Expected: identical event lists. The existing turn tests compare event kinds in order, and the parity test compares every `last` list against Rust.

## Files

| File | Change |
| --- | --- |
| `rules/bench_test.go` | New: benchmarks (Task 1) and allocation tests (Task 2). |
| `rules/play.go` | `play`/`movePiece`/`repair` take `ev *[]Event`; `emit` (Task 2). `put`/`take` (Task 4). |
| `rules/movegen.go` | `maxMoves`, `safe`, `hasLegalMove`, `isLegal`, `pseudoMoves(moves)` (Task 2). Bitboard generation (Task 5). |
| `rules/turn.go` | `Apply` uses `isLegal`; `Mated` uses `hasLegalMove` (Task 2). `take` (Task 4). Bitboard `mamdaniReaches` (Task 5). |
| `rules/game.go` | `status` uses `hasLegalMove` (Task 2). Bitboard `CannotMate` (Task 5). |
| `rules/bitboard.go`, `rules/bitboard_test.go` | New: type, tables, tests (Task 3). `checkBitboards` (Task 4). |
| `rules/position.go` | Bitboard fields, `put`, `take`, `pieces`, `potholes`, `blocked`; `King` and `Blocked` use them (Task 4). |
| `rules/attack.go` | Bitboard `Attacked` (Task 5). |
| `rules/random_test.go`, `rules/perft_test.go` | New invariants (Tasks 2 and 4); the 218-move case (Task 2). |

---

### Task 1: Benchmarks to measure against

**Files:**
- Create: `rules/bench_test.go`

- [ ] **Step 1: Write the benchmarks**

```go
package rules

import (
	"math/rand/v2"
	"testing"
)

// Benchmarks for the engine's hot paths. Compare before and after a change:
//
//	go test -run '^$' -bench . -benchmem -count 6 ./rules > old.txt
//	go run golang.org/x/perf/cmd/benchstat@latest old.txt new.txt

func BenchmarkLegalMovesStart(b *testing.B) {
	p := StartPosition()
	b.ReportAllocs()
	for b.Loop() {
		_ = p.LegalMoves()
	}
}

func BenchmarkPerftStartDepth4(b *testing.B) {
	p, err := ParseFEN(startFEN)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		perft(&p, 4)
	}
}

// BenchmarkRandomGame plays whole seeded games of up to 300 plies, as the
// random-game test does, without its invariant checks.
func BenchmarkRandomGame(b *testing.B) {
	b.ReportAllocs()
	seed := 0
	for b.Loop() {
		r := rand.New(rand.NewPCG(uint64(seed), 1))
		g := NewGame()
		for len(g.Turns) < 300 && !g.Result.Over {
			moves := g.Pos.LegalMoves()
			if _, err := g.Play(moves[r.IntN(len(moves))], randDice{r}); err != nil {
				b.Fatal(err)
			}
		}
		seed++
	}
}
```

`perft` comes from `perft_test.go` and `randDice` from `random_test.go`; both are in the same package.

- [ ] **Step 2: Record the baseline**

Run: `go test -run '^$' -bench . -benchmem -count 6 ./rules | tee /tmp/bench-old.txt`
Expected: numbers close to the "Today" column above.

- [ ] **Step 3: Commit**

```bash
git add rules/bench_test.go
git commit -m "rules: benchmarks for move generation and whole games"
```

---

### Task 2: Move generation without allocations

Today `LegalMoves` grows slices with `append`, and so does every legality check, because `Mated`, `status` and `Apply` all build the full move list. On top of that, `play(m, nil)` still appends a `Moved` event to its nil slice, which allocates once per move tried.

**Files:**
- Modify: `rules/play.go`, `rules/movegen.go`, `rules/turn.go`, `rules/game.go`, `rules/random_test.go`, `rules/perft_test.go`, `rules/bench_test.go`

**Interfaces:**
- Produces:
  - `const maxMoves = 320`.
  - `func (p *Position) pseudoMoves(moves []Move) []Move`, which appends.
  - `func (p *Position) safe(m Move) bool`.
  - `func (p *Position) hasLegalMove() bool`.
  - `func (p *Position) isLegal(m Move) bool`.
  - `func (p *Position) play(m Move, ev *[]Event)`, where `ev` may be nil.
  - `func emit(ev *[]Event, e Event)`.

- [ ] **Step 1: Write the failing tests**

Append to `rules/bench_test.go`:

```go
func TestLegalityChecksDoNotAllocate(t *testing.T) {
	p := StartPosition()
	for name, f := range map[string]func(){
		"LegalMoves":   func() { _ = p.LegalMoves() },
		"hasLegalMove": func() { _ = p.hasLegalMove() },
		"isLegal":      func() { _ = p.isLegal(Move{From: E2, To: E4}) },
		"Mated":        func() { _ = p.Mated() },
	} {
		want := 0.0
		if name == "LegalMoves" {
			want = 1 // the returned slice
		}
		if got := testing.AllocsPerRun(100, f); got > want {
			t.Errorf("%s: %v allocations, want at most %v", name, got, want)
		}
	}
}
```

In `rules/random_test.go`, at the top of `checkInvariants` (after `fail` is defined), add:

```go
	if !before.isLegal(m) {
		fail("isLegal rejects a move LegalMoves offered")
	}
	if p.hasLegalMove() != (len(p.LegalMoves()) > 0) {
		fail("hasLegalMove disagrees with LegalMoves")
	}
```

In `rules/perft_test.go`, add a case after "position 5":

```go
		// The most legal moves known in a legal chess position: a long move
		// list must still come out whole.
		{"218 moves", "R6R/3Q4/1Q4Q1/4Q3/2Q4Q/Q4Q2/pp1Q4/kBNN1KB1 w - - 0 1", []int{218}},
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./rules`
Expected: compile errors (`p.hasLegalMove undefined`, `p.isLegal undefined`).

- [ ] **Step 3: Make `play`'s events optional (`rules/play.go`)**

Change the signatures and doc comment:

```go
// play makes move m for the side to move, then runs the close and repair
// steps and passes the turn. It assumes m is pseudo-legal. Events are
// appended to *ev; pass nil when only the resulting position matters, and
// nothing is recorded or allocated.
func (p *Position) play(m Move, ev *[]Event) {
```

```go
func (p *Position) movePiece(m Move, ev *[]Event) {
```

```go
func (p *Position) repair(ev *[]Event) {
```

In those three functions:
- replace every `ev = append(ev, X)` with `emit(ev, X)`;
- replace `ev = p.movePiece(m, ev)` with `p.movePiece(m, ev)`, and `ev = p.repair(ev)` with `p.repair(ev)`;
- delete the `return ev` lines, and change `repair`'s early `return ev` to `return`.

Then add:

```go
// emit records e if events are being kept.
func emit(ev *[]Event, e Event) {
	if ev != nil {
		*ev = append(*ev, e)
	}
}
```

The other callers pass `nil` already (`san.go`, `perft_test.go`, `random_test.go`), and that still compiles: it's now a nil pointer.

- [ ] **Step 4: Stack-buffered, early-exit legality (`rules/movegen.go`)**

Replace `import "fmt"` with:

```go
import (
	"fmt"
	"slices"
)
```

Replace `LegalMoves` with:

```go
func (p *Position) LegalMoves() []Move {
	var buf [maxMoves]Move
	pseudo := p.pseudoMoves(buf[:0])
	legal := make([]Move, 0, len(pseudo))
	for _, m := range pseudo {
		if p.safe(m) {
			legal = append(legal, m)
		}
	}
	return legal
}

// maxMoves is room for every pseudo-legal move in any reachable position:
// chess tops out at 218 legal moves and the Mamdani adds at most 27. If a
// position ever had more, append would just move to the heap.
const maxMoves = 320

// safe reports whether pseudo-legal m leaves the mover's king unattacked
// once the close and repair steps have run.
func (p *Position) safe(m Move) bool {
	q := *p
	q.play(m, nil)
	return !q.InCheck(p.Turn)
}

// hasLegalMove reports whether the side to move has any legal move. It
// stops at the first one, which is all mate and stalemate checks need.
func (p *Position) hasLegalMove() bool {
	var buf [maxMoves]Move
	for _, m := range p.pseudoMoves(buf[:0]) {
		if p.safe(m) {
			return true
		}
	}
	return false
}

// isLegal reports whether m is one of LegalMoves, without building them all.
func (p *Position) isLegal(m Move) bool {
	var buf [maxMoves]Move
	return slices.Contains(p.pseudoMoves(buf[:0]), m) && p.safe(m)
}
```

Change `pseudoMoves` to append to its argument:

```go
// pseudoMoves appends every pseudo-legal move to moves and returns it.
func (p *Position) pseudoMoves(moves []Move) []Move {
	c := p.Turn
```

Delete the line `moves := make([]Move, 0, 64)` that followed `c := p.Turn`. The rest of its body is unchanged.

- [ ] **Step 5: Use the cheap checks (`rules/turn.go`, `rules/game.go`)**

In `Apply`, replace the legality check and first `play`:

```go
	if !p.isLegal(m) {
		return p, nil, ErrIllegalMove
	}
	mover := p.Turn
	next := p
	var ev []Event
	next.play(m, &ev)
```

Remove the now-unused `"slices"` import from `turn.go`. In `Mated`, use `return p.threatened() && !p.hasLegalMove()`. In `game.go`'s `status`, use `if !p.hasLegalMove() {`.

- [ ] **Step 6: Run the tests**

Run: `go vet ./... && go test -count=1 ./...`
Expected: PASS. The full 10,000 random games take about 4 s now, down from about 20 s.

- [ ] **Step 7: Benchmark**

Run: `go test -run '^$' -bench . -benchmem -count 6 ./rules | tee /tmp/bench-noalloc.txt`
Expected: close to the "After Task 2" column. `BenchmarkRandomGame` should show about 2,000 allocs/op, down from about 35,700.

- [ ] **Step 8: Commit**

```bash
git add rules/play.go rules/movegen.go rules/turn.go rules/game.go rules/random_test.go rules/perft_test.go rules/bench_test.go
git commit -m "rules: legality checks without allocations; stop at the first legal move"
```

---

### Task 3: The bitboard type and attack tables

**Files:**
- Create: `rules/bitboard.go`, `rules/bitboard_test.go`

**Interfaces:**
- Produces:
  - `type Bitboard uint64`, with `Has(Square) bool`, `First() Square`, `Last() Square`, `Count() int` and the unexported `pop() Square`.
  - `bit(s Square) Bitboard`.
  - Tables: `knightAttacks`, `kingAttacks`, `pawnAttacks[color]`.
  - `rookAttacks(s, blocked)`, `bishopAttacks(s, blocked)`, `queenAttacks(s, blocked) Bitboard`.
  - `between(a, b Square) (Bitboard, bool)`.

- [ ] **Step 1: Write the failing tests (`rules/bitboard_test.go`)**

```go
package rules

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

// naiveSlide walks one square at a time, the way the mailbox engine did:
// the reference the attack tables must match.
func naiveSlide(s Square, dirs [][2]int, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range dirs {
		for t := s.Offset(d[0], d[1]); t != NoSquare; t = t.Offset(d[0], d[1]) {
			b |= bit(t)
			if blocked.Has(t) {
				break
			}
		}
	}
	return b
}

func TestSliderAttacksMatchANaiveWalk(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for i := range 400 {
		// Sparse to dense blocker patterns.
		blocked := Bitboard(r.Uint64())
		for range i % 4 {
			blocked &= Bitboard(r.Uint64())
		}
		for s := range Square(64) {
			if got, want := rookAttacks(s, blocked), naiveSlide(s, rookDirs, blocked); got != want {
				t.Fatalf("rook on %v, blocked %x: got %x want %x", s, uint64(blocked), uint64(got), uint64(want))
			}
			if got, want := bishopAttacks(s, blocked), naiveSlide(s, bishopDirs, blocked); got != want {
				t.Fatalf("bishop on %v, blocked %x: got %x want %x", s, uint64(blocked), uint64(got), uint64(want))
			}
		}
	}
}

func squares(b Bitboard) []Square {
	var out []Square
	for b != 0 {
		out = append(out, b.pop())
	}
	return out
}

func TestLeaperTables(t *testing.T) {
	for _, c := range []struct {
		name string
		got  Bitboard
		want []Square
	}{
		{"knight a1", knightAttacks[A1], []Square{C2, B3}},
		{"king a1", kingAttacks[A1], []Square{B1, A2, B2}},
		{"white pawn e4", pawnAttacks[White][E4], []Square{D5, F5}},
		{"black pawn e4", pawnAttacks[Black][E4], []Square{D3, F3}},
	} {
		if got := squares(c.got); fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestBetween(t *testing.T) {
	if b, ok := between(A5, D2); !ok || fmt.Sprint(squares(b)) != fmt.Sprint([]Square{C3, B4}) {
		t.Errorf("a5-d2: %v %v", squares(b), ok)
	}
	if b, ok := between(A1, A2); !ok || b != 0 {
		t.Errorf("a1-a2 should be aligned with nothing between: %v %v", squares(b), ok)
	}
	for _, pair := range [][2]Square{{A1, B3}, {A1, A1}} {
		if _, ok := between(pair[0], pair[1]); ok {
			t.Errorf("%v-%v should not be aligned", pair[0], pair[1])
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./rules -run 'Slider|Leaper|Between'`
Expected: compile errors (`undefined: Bitboard`).

- [ ] **Step 3: Implement (`rules/bitboard.go`)**

```go
package rules

import "math/bits"

// Bitboard is a set of squares, one bit per square: bit 0 is a1, bit 63 is
// h8. Pieces, the Mamdani and potholes all stop sliders the same way, so
// move generation works on one "blocked" set and the tables here never need
// to know which is which.
type Bitboard uint64

// bit is the set holding just s. s must be on the board.
func bit(s Square) Bitboard { return 1 << uint(s) }

// Has reports whether s is in the set. NoSquare never is.
func (b Bitboard) Has(s Square) bool { return s != NoSquare && b&bit(s) != 0 }

// First returns the lowest square in the set, or NoSquare if it is empty.
func (b Bitboard) First() Square {
	if b == 0 {
		return NoSquare
	}
	return Square(bits.TrailingZeros64(uint64(b)))
}

// Last returns the highest square in the set, or NoSquare if it is empty.
func (b Bitboard) Last() Square {
	if b == 0 {
		return NoSquare
	}
	return Square(63 - bits.LeadingZeros64(uint64(b)))
}

// Count returns how many squares are in the set.
func (b Bitboard) Count() int { return bits.OnesCount64(uint64(b)) }

// pop removes and returns the lowest square. b must not be empty.
func (b *Bitboard) pop() Square {
	s := b.First()
	*b &= *b - 1
	return s
}

// The eight queen directions. The first four run towards higher squares,
// so the nearest blocker on them is the lowest set bit; the last four run
// the other way.
var dirSteps = [8][2]int{{0, 1}, {1, 0}, {1, 1}, {-1, 1}, {0, -1}, {-1, 0}, {-1, -1}, {1, -1}}

var (
	rookDirIdx   = [4]int{0, 1, 4, 5}
	bishopDirIdx = [4]int{2, 3, 6, 7}
)

// Attack tables, filled once at startup.
var (
	rays          [8][64]Bitboard // squares beyond each square in each direction
	knightAttacks [64]Bitboard
	kingAttacks   [64]Bitboard
	pawnAttacks   [2][64]Bitboard // by the pawn's color
)

func init() {
	for s := range Square(64) {
		for d, step := range dirSteps {
			for t := s.Offset(step[0], step[1]); t != NoSquare; t = t.Offset(step[0], step[1]) {
				rays[d][s] |= bit(t)
			}
		}
		knightAttacks[s] = leaper(s, knightJumps)
		kingAttacks[s] = leaper(s, queenDirs)
		pawnAttacks[White][s] = leaper(s, [][2]int{{-1, 1}, {1, 1}})
		pawnAttacks[Black][s] = leaper(s, [][2]int{{-1, -1}, {1, -1}})
	}
}

func leaper(s Square, steps [][2]int) Bitboard {
	var b Bitboard
	for _, st := range steps {
		if t := s.Offset(st[0], st[1]); t != NoSquare {
			b |= bit(t)
		}
	}
	return b
}

// rayAttack returns the squares a slider on s reaches in direction d: every
// empty square up to and including the first blocked one.
func rayAttack(d int, s Square, blocked Bitboard) Bitboard {
	ray := rays[d][s]
	hits := ray & blocked
	if hits == 0 {
		return ray
	}
	first := hits.First()
	if d >= 4 {
		first = hits.Last()
	}
	return ray ^ rays[d][first]
}

func rookAttacks(s Square, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range rookDirIdx {
		b |= rayAttack(d, s, blocked)
	}
	return b
}

func bishopAttacks(s Square, blocked Bitboard) Bitboard {
	var b Bitboard
	for _, d := range bishopDirIdx {
		b |= rayAttack(d, s, blocked)
	}
	return b
}

func queenAttacks(s Square, blocked Bitboard) Bitboard {
	return rookAttacks(s, blocked) | bishopAttacks(s, blocked)
}

// between returns the squares strictly between a and b, and false if they
// don't share a rank, file or diagonal.
func between(a, b Square) (Bitboard, bool) {
	df, dr := sign(b.File()-a.File()), sign(b.Rank()-a.Rank())
	if a == b || (b.File() != a.File() && b.Rank() != a.Rank() && abs(b.File()-a.File()) != abs(b.Rank()-a.Rank())) {
		return 0, false
	}
	for d, step := range dirSteps {
		if step[0] == df && step[1] == dr {
			return (rays[d][a] ^ rays[d][b]) &^ bit(b), true
		}
	}
	return 0, false
}
```

`knightJumps`, `queenDirs`, `rookDirs`, `bishopDirs`, `sign` and `abs` already exist (`attack.go`, `board.go`). Package variables are set before `init` runs.

- [ ] **Step 4: Run the tests**

Run: `go test ./rules -run 'Slider|Leaper|Between' -v`
Expected: PASS. Nothing uses the tables yet.

- [ ] **Step 5: Commit**

```bash
git add rules/bitboard.go rules/bitboard_test.go
git commit -m "rules: bitboard type and attack tables, checked against a naive walk"
```

---

### Task 4: Keep bitboards in step with the board

**Files:**
- Modify: `rules/position.go`, `rules/play.go`, `rules/turn.go`, `rules/random_test.go`, `rules/bitboard_test.go`

**Interfaces:**
- Consumes: `Bitboard` and `bit` (Task 3).
- Produces: `Position.byColor [2]Bitboard` and `Position.byKind [King + 1]Bitboard`; `put(s, pc)`, `take(s) Piece`, `pieces(c, k) Bitboard`, `potholes() Bitboard`, `blocked() Bitboard`. Also `checkBitboards(p *Position) error`, for tests.

- [ ] **Step 1: Write the failing check**

Append to `rules/bitboard_test.go`:

```go
// checkBitboards fails if p's bitboards disagree with its Board.
func checkBitboards(p *Position) error {
	var want Position
	for s, pc := range p.Board {
		if pc != NoPiece {
			want.byColor[pc.Color()] |= bit(Square(s))
			want.byKind[pc.Kind()] |= bit(Square(s))
		}
	}
	if want.byColor != p.byColor || want.byKind != p.byKind {
		return fmt.Errorf("bitboards out of step with the board")
	}
	return nil
}
```

In `rules/random_test.go`'s `checkInvariants`, right after the Task 2 checks, add:

```go
	if err := checkBitboards(p); err != nil {
		fail("%v", err)
	}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./rules`
Expected: compile error (`want.byColor undefined`).

- [ ] **Step 3: Add the fields and helpers (`rules/position.go`)**

In `Position`, give `Board` this comment:

```go
	// Board is what stands on each square. Read it freely, but change
	// pieces only through put and take, which keep the bitboards in step.
	Board [64]Piece
```

After `Fullmove int`, add:

```go
	// The same pieces as Board, as sets: by color and by kind (indexed by
	// Kind, so byKind[NoKind] stays empty).
	byColor [2]Bitboard
	byKind  [King + 1]Bitboard
```

After the struct, add:

```go
// put places pc on s, replacing whatever was there.
func (p *Position) put(s Square, pc Piece) {
	p.take(s)
	p.Board[s] = pc
	p.byColor[pc.Color()] |= bit(s)
	p.byKind[pc.Kind()] |= bit(s)
}

// take removes and returns the piece on s, or NoPiece.
func (p *Position) take(s Square) Piece {
	pc := p.Board[s]
	if pc == NoPiece {
		return NoPiece
	}
	p.Board[s] = NoPiece
	p.byColor[pc.Color()] &^= bit(s)
	p.byKind[pc.Kind()] &^= bit(s)
	return pc
}

// pieces returns c's pieces of kind k.
func (p *Position) pieces(c Color, k Kind) Bitboard { return p.byColor[c] & p.byKind[k] }

// potholes returns the open potholes as a set.
func (p *Position) potholes() Bitboard {
	var b Bitboard
	for _, s := range p.Potholes {
		if s != NoSquare {
			b |= bit(s)
		}
	}
	return b
}

// blocked returns every square that stops a slider: pieces, the Mamdani
// and potholes.
func (p *Position) blocked() Bitboard {
	b := p.byColor[White] | p.byColor[Black] | p.potholes()
	if p.Mamdani != NoSquare {
		b |= bit(p.Mamdani)
	}
	return b
}
```

Replace `Blocked` and `King`:

```go
func (p *Position) Blocked(s Square) bool {
	return p.blocked().Has(s)
}
```

```go
func (p *Position) King(c Color) Square {
	return p.pieces(c, King).First()
}
```

In `ParseFEN`, replace `p.Board[r*8+f] = pc` with `p.put(Square(r*8+f), pc)`.

- [ ] **Step 4: Route every board write through `put`/`take`**

In `rules/play.go`'s `movePiece`, replace the capture and landing block with:

```go
	captured := p.take(capSq)
	if captured != NoPiece {
		emit(ev, Event{Kind: Captured, Square: capSq, Piece: captured})
	}

	p.take(m.From)
	if m.Promo != NoKind {
		p.put(m.To, NewPiece(c, m.Promo))
	} else {
		p.put(m.To, pc)
	}
```

Replace the castling rook line with `p.put(rookTo, p.take(rookFrom))`.

In `rules/turn.go`, replace `p.Board[s] = NoPiece` with `p.take(s)` in both `remove` and `resolve`.

Run `grep -n 'Board\[.*\] *=' rules/*.go | grep -v _test`. Expected: no writes left outside `put`/`take`. The `==` comparisons are fine.

- [ ] **Step 5: Run the tests**

Run: `go test -count=1 ./rules`
Expected: PASS. Every ply of 10,000 random games now checks the bitboards against the board.

- [ ] **Step 6: Commit**

```bash
git add rules/position.go rules/play.go rules/turn.go rules/random_test.go rules/bitboard_test.go
git commit -m "rules: keep bitboards in step with the board"
```

---

### Task 5: Attacks and move generation on bitboards

**Files:**
- Modify: `rules/attack.go`, `rules/movegen.go`, `rules/turn.go`, `rules/game.go`

**Interfaces:**
- Consumes: everything from Tasks 2–4.

- [ ] **Step 1: The safety net is already there**

There's no new test. Perft (now with the 218-move case), the slider and pothole tests, 10,000 random games with the Task 2 and Task 4 invariants, and the Rust parity check below all pin this change.

- [ ] **Step 2: `Attacked` (`rules/attack.go`)**

Replace `Attacked`, and delete `slidingAttack`. Keep the direction variables at the top, because the tables and tests use them.

```go
// Attacked reports whether any piece of color by attacks s. Potholes and the
// Mamdani block sliders exactly like pieces do.
func (p *Position) Attacked(s Square, by Color) bool {
	them := p.byColor[by]
	blocked := p.blocked()
	straight := p.byKind[Rook] | p.byKind[Queen]
	diagonal := p.byKind[Bishop] | p.byKind[Queen]
	// A pawn of color by attacks s from the squares a pawn of the other
	// color on s would attack.
	return pawnAttacks[by.Other()][s]&them&p.byKind[Pawn] != 0 ||
		knightAttacks[s]&them&p.byKind[Knight] != 0 ||
		kingAttacks[s]&them&p.byKind[King] != 0 ||
		rookAttacks(s, blocked)&them&straight != 0 ||
		bishopAttacks(s, blocked)&them&diagonal != 0
}
```

- [ ] **Step 3: Move generation (`rules/movegen.go`)**

Delete `canLand`, `stepMoves`, `slideMoves`, `pseudoMoves`, `pawnDir` and `pawnMoves`. Keep `castlingMoves`. Put in their place:

```go
// pseudoMoves appends every pseudo-legal move to moves and returns it.
func (p *Position) pseudoMoves(moves []Move) []Move {
	us := p.Turn
	blocked := p.blocked()
	// A piece may land on an empty square or an enemy piece, never on a
	// pothole or the Mamdani.
	landable := ^(p.byColor[us] | p.potholes())
	if p.Mamdani != NoSquare {
		landable &^= bit(p.Mamdani)
		// The Mamdani moves like a queen but never captures.
		moves = appendTargets(moves, p.Mamdani, queenAttacks(p.Mamdani, blocked)&^blocked)
	}
	for b := p.pieces(us, Knight); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, knightAttacks[from]&landable)
	}
	for b := p.pieces(us, Bishop); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, bishopAttacks(from, blocked)&landable)
	}
	for b := p.pieces(us, Rook); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, rookAttacks(from, blocked)&landable)
	}
	for b := p.pieces(us, Queen); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, queenAttacks(from, blocked)&landable)
	}
	for b := p.pieces(us, King); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, kingAttacks[from]&landable)
		moves = p.castlingMoves(moves, from)
	}
	for b := p.pieces(us, Pawn); b != 0; {
		moves = p.pawnMoves(moves, b.pop(), blocked)
	}
	return moves
}

// appendTargets appends a move from from to each square in targets.
func appendTargets(moves []Move, from Square, targets Bitboard) []Move {
	for targets != 0 {
		moves = append(moves, Move{From: from, To: targets.pop()})
	}
	return moves
}

func pawnDir(c Color) int {
	if c == White {
		return 1
	}
	return -1
}

func (p *Position) pawnMoves(moves []Move, from Square, blocked Bitboard) []Move {
	c := p.Turn
	dir := pawnDir(c)
	if one := from.Offset(0, dir); one != NoSquare && !blocked.Has(one) {
		moves = appendPawnMove(moves, from, one)
		startRank := 1
		if c == Black {
			startRank = 6
		}
		if two := one.Offset(0, dir); from.Rank() == startRank && !blocked.Has(two) {
			moves = appendPawnMove(moves, from, two)
		}
	}
	enemyPawn := NewPiece(c.Other(), Pawn)
	for t := pawnAttacks[c][from]; t != 0; {
		to := t.pop()
		capture := p.byColor[c.Other()].Has(to)
		// En passant. A pothole on the target square cancels it.
		enPassant := to == p.EP && !blocked.Has(to) && p.Board[to.Offset(0, -dir)] == enemyPawn
		if capture || enPassant {
			moves = appendPawnMove(moves, from, to)
		}
	}
	return moves
}

var promoKinds = [...]Kind{Queen, Rook, Bishop, Knight}

// appendPawnMove appends a pawn move, as the four promotions on the last rank.
func appendPawnMove(moves []Move, from, to Square) []Move {
	if to.Rank() == 0 || to.Rank() == 7 {
		for _, k := range promoKinds {
			moves = append(moves, Move{From: from, To: to, Promo: k})
		}
		return moves
	}
	return append(moves, Move{From: from, To: to})
}
```

- [ ] **Step 4: Saving-roll line and insufficient material**

In `rules/turn.go`, replace `mamdaniReaches` with:

```go
func (p *Position) mamdaniReaches(s Square) bool {
	if p.Mamdani == NoSquare || p.Mamdani == s {
		return false
	}
	path, aligned := between(p.Mamdani, s)
	return aligned && path&p.blocked() == 0
}
```

Keep its doc comment. In `rules/game.go`, replace the body of `CannotMate` (keep its comment):

```go
func (p *Position) CannotMate(c Color) bool {
	if p.byColor[c]&(p.byKind[Pawn]|p.byKind[Rook]|p.byKind[Queen]) != 0 {
		return false
	}
	minors := (p.byColor[c] & (p.byKind[Knight] | p.byKind[Bishop])).Count()
	return minors == 0 || (minors == 1 && p.Mamdani == NoSquare)
}
```

- [ ] **Step 5: Run everything**

Run: `gofmt -l rules; go vet ./... && go test -count=1 ./...`
Expected: no gofmt output, and PASS everywhere. That includes `game` and `server`: their counts (33 legal moves at the start, 32 for Black after e4) don't depend on move order.

- [ ] **Step 6: Check against the Rust engine**

Run:

```bash
go run ./server_rs/parity -n 300 -seed 4242 > /tmp/parity.jsonl
(cd server_rs && PARITY_FILE=/tmp/parity.jsonl cargo test --release -p server --test parity)
```

Expected: `parity: 300 games match`. Two engines written separately now agree turn by turn. CI runs this too, with seed 1000.

- [ ] **Step 7: Benchmark**

Run: `go test -run '^$' -bench . -benchmem -count 6 ./rules | tee /tmp/bench-new.txt`, then `go run golang.org/x/perf/cmd/benchstat@latest /tmp/bench-old.txt /tmp/bench-new.txt`.
Expected: close to the "After Task 5" column.

- [ ] **Step 8: Commit**

```bash
git add rules/attack.go rules/movegen.go rules/turn.go rules/game.go
git commit -m "rules: attacks and move generation on bitboards"
```

---

### Task 6: Record the numbers

**Files:**
- Modify: `docs/superpowers/plans/2026-10-03-02-rules-engine.md`

- [ ] **Step 1: Add a note at the end of the Plan 02 doc**

```markdown
## Later: speed (2026-10-04)

The engine moved to bitboards plus the mailbox, with allocation-free legality checks ([plan](2026-10-04-go-rules-speed.md)). No rule changed: the tests above still pass, and the Rust engine in `server_rs/` agrees on 300 recorded games.

Measured with `go test -bench . -benchmem ./rules`:
- one random game: 2.2 ms and 35,700 allocations → 0.47 ms and 2,000;
- perft from the start to depth 4: 15.5 ms → 7.0 ms;
- the 10,000-game test: about 20 s → about 3 s.

`LegalMoves()` order changed; nothing depends on it.
```

If your benchstat numbers from Task 5 Step 7 differ by more than about 20%, use yours instead.

- [ ] **Step 2: Commit**

```bash
git add docs/superpowers/plans/2026-10-03-02-rules-engine.md
git commit -m "Docs: rules engine speed numbers"
```
