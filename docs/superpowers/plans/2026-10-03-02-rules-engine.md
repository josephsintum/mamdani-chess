# Rules Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A pure Go package `rules` that generates legal moves and resolves full turns of Pothole Chess: Mamdani Edition. That means move, close, repair, pothole roll, placement and saving rolls, plus game-end detection and replay from a log.

**Architecture:** Mailbox board (`[64]Piece`) with direction tables. Legality works by copy-make: play the move plus the close and repair steps on a copy, then reject it if the mover's king is attacked. `Apply(pos, move, dice)` is a pure function that returns the new position and an ordered event list, which the server will stream for the dice animation. `Game` adds repetition history, result and a turn log of `{Move, Dice}` that `Replay` can rebuild exactly.

**Tech Stack:** Go 1.27.1 standard library only. Tests use `testing`, `math/rand/v2` (seeded PCG) and `slices`.

**Spec:** `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md` §3 (Rules engine) and §5 (Testing). The rules themselves are in `RULES.md`, copied from the live doc https://claude.ai/artifact/AvSPCQS42ggQGpTWQGoQRB, which wins if they disagree. Read RULES.md before starting.

## Global Constraints

- **No I/O and no randomness in `rules`.** No `time`, no randomness of its own; dice arrive through `rules.Dice`.
- **Module layout.** The module is `mamdani-chess` (go 1.27.1) and the package is at the repo root: `rules/`. This plan adds no dependencies, so `go.mod` is unchanged. Don't touch `names/`.
- **All dice are d8.** `D8()` returns 1..8. Odd means nothing (pothole roll) or saved (saving roll); even means a pothole opens or the piece falls.
- **Turn order is exactly RULES.md:** move → close (the mover's own pothole from their previous turn) → repair (potholes next to the Mamdani) → pothole d8 → two d8s for file and rank → resolve.
- **No Mamdani ping-pong rule.** Either player may move it straight back (decision #9).
- **Re-roll the placement when:**
  - the square holds a king,
  - it is already a pothole,
  - the fall would leave the roller in check,
  - or the result would leave the next player checkmated (decision #11).

  After 64 re-rolls, no pothole opens (decision #13).
- **Saving-roll reach is a clear queen line** from the Mamdani to the square, and nothing else is checked (decision #12).
- **A pothole blocks like a piece** for sliders, castling (the rook's path too), pawn double-steps and en passant (decision #14).
- **No house-rule `Settings` struct.** House rules are deferred (the live doc's "Watch in test games"). This is a deliberate deviation from spec §3 "Data"; perft simply uses positions with no Mamdani and no potholes.
- **Git:** stage explicit paths (`git add <paths>`), never `git add -A`. No Claude attribution in commit messages.

## Interpretations made here

Each of these is pinned by a test. If the rules doc decides otherwise, change the test first.

1. **"Would expose the roller's king" ignores the hole.** It is judged as if the fallen piece were gone and the hole weren't there. While the hole is open it blocks the same lines the piece did, so judging with the hole would make the rule do nothing. The real risk is the roller inheriting an unanswerable check when their own hole closes after their next move. (`TestRerollIfFallExposesRoller`)
2. **The checkmate re-roll covers any result of the roll,** including a hole on an empty square that takes away the last escape or block. "A roll never wins on its own." (`TestRerollIfResultWouldCheckmate`, plus the random-game invariant)
3. **Insufficient material:** a side can't mate with a bare king, or with king and one minor piece once the Mamdani has fallen. While the Mamdani is on the board it can block escape squares, so a minor piece might mate. (`TestInsufficientMaterial`)
4. **The Mamdani falling does not reset the 50-move count;** only a player's piece falling does. A Mamdani move increments the count like any quiet move. (`TestMamdaniMoveDoesNotResetFiftyMoveCount`)
5. **Repetition counts automatically:** the game is drawn the moment a position occurs for the third time, with no claim needed. The key covers pieces, Mamdani, potholes with their roller, side to move, castling rights and en passant square. (`TestThreefoldRepetition`)

## Review Focus

These are the most likely ways the engine could hurt real players. Each is pinned by a test in the task that owns the code.

1. **The dice deciding a game.** A roll that removes the only defender, or opens a hole on the last escape square, must re-roll instead of checkmating. Pinned by `TestRerollIfResultWouldCheckmate` (Task 3) and the "dice delivered checkmate" invariant over 10,000 games (Task 6). A mutation check confirmed the invariant fails when that re-roll is disabled.
2. **Replays drifting after a server restart.** The server rebuilds unfinished games from `{move, dice}` logs. A replay with too few or too many dice must fail loudly, not silently produce another game. Pinned by `TestReplay` (Task 4) and the replay check after every random game (Task 6).
3. **Hostile or buggy clients.** The server will pass client moves straight to `Apply`. Moving the opponent's piece, the Mamdani capturing, a fake promotion or a missing promotion must all return `ErrIllegalMove` and leave the position untouched. Pinned by `TestIllegalMovesRejected` (Task 3).
4. **A pothole where chess rules have special cases.** A hole on the castling path (including b1/b8), on the en passant square, or in front of a pawn must block like a piece. Pinned by `TestCastlingAndPotholes`, `TestPotholeCancelsEnPassant` and `TestPawnsAndPotholes` (Task 2).
5. **Saving rolls after the Mamdani has fallen.** None may happen, and the Mamdani must never come back. Pinned by `TestMamdaniFalls` (Task 3) and the "Mamdani came back" invariant (Task 6).

## File Structure

| File | Responsibility |
| --- | --- |
| `rules/board.go` | Package doc, `Color`, `Kind`, `Piece`, `Square` (+ named squares `A1`..`H8`), small helpers |
| `rules/position.go` | `Position`, `Castling`, `StartPosition`, `ParseFEN`, `Key`, blocking helpers |
| `rules/attack.go` | Direction tables, `Attacked`, `InCheck` |
| `rules/movegen.go` | `Move`, `ParseMove`, `LegalMoves` and per-piece generators |
| `rules/play.go` | `play`: make a move, close, repair, pass the turn |
| `rules/event.go` | `Event`, `EventKind`, `RerollReason` |
| `rules/turn.go` | `Dice`, `Apply`, pothole roll, placement, re-rolls, saving rolls |
| `rules/game.go` | `Game`, `Result`, end conditions, `Turn` log, `Replay`, `ScriptedDice` |
| `rules/san.go` | `SAN` for the move log |
| `rules/*_test.go` | One test file per source file, plus `helpers_test.go`, `perft_test.go` and `random_test.go` |

---

### Task 1: Board primitives and FEN

**Files:**
- Create: `rules/board.go`, `rules/position.go`
- Test: `rules/board_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `type Color uint8` (`White`, `Black`; `Other()`, `String()`)
  - `type Kind uint8` (`NoKind`, `Pawn`, `Knight`, `Bishop`, `Rook`, `Queen`, `King`, `Mamdani`)
  - `type Piece uint8`: `NoPiece`, `MamdaniPiece`, `NewPiece(Color, Kind)`, `Kind()`, `Color()`
  - `type Square int8`: `NoSquare`, `A1`..`H8`, `File()`, `Rank()`, `Offset(df, dr) Square`, `String()`, `ParseSquare(string) (Square, error)`, `Sq(string) Square`
  - `type Castling uint8`: `WhiteKingside`, `WhiteQueenside`, `BlackKingside`, `BlackQueenside`
  - `type Position struct { Board [64]Piece; Mamdani Square; Potholes [2]Square; Turn Color; Castling Castling; EP Square; Halfmove, Fullmove int }`. `Potholes` is indexed by the color that rolled the hole.
  - `StartPosition() Position`, `ParseFEN(string) (Position, error)`, `(*Position).IsPothole`, `Blocked`, `King(Color) Square`, `Key() Key`
  - unexported: `adjacent`, `abs`, `sign`, `startFEN`

- [ ] **Step 1: Write the failing test**

Create `rules/board_test.go`:

```go
package rules

import "testing"

func TestSquares(t *testing.T) {
	for _, c := range []struct {
		name       string
		sq         Square
		file, rank int
	}{
		{"a1", A1, 0, 0}, {"h1", H1, 7, 0}, {"e4", E4, 4, 3}, {"h8", H8, 7, 7},
	} {
		s, err := ParseSquare(c.name)
		if err != nil || s != c.sq || s.File() != c.file || s.Rank() != c.rank || s.String() != c.name {
			t.Errorf("%s: got %v (%d,%d) err %v", c.name, s, s.File(), s.Rank(), err)
		}
	}
	for _, bad := range []string{"", "i1", "a9", "a10", "A1"} {
		if _, err := ParseSquare(bad); err == nil {
			t.Errorf("ParseSquare(%q) should fail", bad)
		}
	}
	if H1.Offset(1, 0) != NoSquare || A1.Offset(0, -1) != NoSquare || E4.Offset(-1, 1) != D5 {
		t.Error("Offset is wrong at the edges or in the middle")
	}
}

func TestPieces(t *testing.T) {
	p := NewPiece(Black, Knight)
	if p.Color() != Black || p.Kind() != Knight || NewPiece(White, Pawn).Color() != White {
		t.Error("piece packing is wrong")
	}
	if White.Other() != Black || Black.Other() != White {
		t.Error("Other is wrong")
	}
}

func TestStartPosition(t *testing.T) {
	p := StartPosition()
	if p.Board[E1] != NewPiece(White, King) || p.Board[D8] != NewPiece(Black, Queen) || p.Board[E4] != NoPiece {
		t.Error("pieces are misplaced")
	}
	if p.Mamdani != A5 || p.Potholes != [2]Square{NoSquare, NoSquare} {
		t.Errorf("mamdani %v potholes %v", p.Mamdani, p.Potholes)
	}
	if p.Turn != White || p.Castling != WhiteKingside|WhiteQueenside|BlackKingside|BlackQueenside || p.EP != NoSquare || p.Fullmove != 1 {
		t.Errorf("state %+v", p)
	}
}

func TestParseFEN(t *testing.T) {
	p, err := ParseFEN("4k3/8/8/3Pp3/8/8/8/4K3 b - e6 3 40")
	if err != nil {
		t.Fatal(err)
	}
	if p.Turn != Black || p.EP != E6 || p.Halfmove != 3 || p.Fullmove != 40 || p.Castling != 0 || p.Mamdani != NoSquare {
		t.Errorf("state %+v", p)
	}
	for _, bad := range []string{
		"",
		"8/8/8/8/8/8/8 w - - 0 1",                 // 7 ranks
		"9/8/8/8/8/8/8/8 w - - 0 1",               // too wide
		"8/8/8/8/8/8/8/8 x - - 0 1",               // side to move
		"8/8/8/8/8/8/8/8 w X - 0 1",               // castling
		"8/8/8/8/8/8/8/8 w - z9 0 1",              // en passant
		"8/8/8/8/8/8/8/8 w - - zero 1",            // clocks
		"rnbqkbnr/ppppxppp/8/8/8/8/8/8 w - - 0 1", // bad piece
	} {
		if _, err := ParseFEN(bad); err == nil {
			t.Errorf("ParseFEN(%q) should fail", bad)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./rules/`
Expected: FAIL to build, with errors like `undefined: ParseSquare`.

- [ ] **Step 3: Write the board types**

Create `rules/board.go`:

```go
// Package rules is the Pothole Chess: Mamdani Edition rules engine.
// It is pure: no I/O, no clock, and no randomness of its own (dice come in
// through the Dice interface). RULES.md is the specification.
package rules

import "fmt"

// Color is a side. The Mamdani has no color.
type Color uint8

const (
	White Color = iota
	Black
)

// Other returns the opposing color.
func (c Color) Other() Color { return c ^ 1 }

func (c Color) String() string {
	if c == White {
		return "white"
	}
	return "black"
}

// Kind is a piece type. Mamdani never appears on Position.Board; it is only
// used to name the Mamdani in events.
type Kind uint8

const (
	NoKind Kind = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
	Mamdani
)

// Piece packs a Kind and a Color. The zero value is an empty square.
type Piece uint8

const NoPiece Piece = 0

// MamdaniPiece names the Mamdani in events (Fell, SavingRoll).
const MamdaniPiece = Piece(Mamdani)

func NewPiece(c Color, k Kind) Piece { return Piece(k) | Piece(c)<<3 }
func (p Piece) Kind() Kind           { return Kind(p & 7) }
func (p Piece) Color() Color         { return Color(p >> 3) }

// Square is 0..63: a1=0, b1=1, ..., h8=63.
type Square int8

const NoSquare Square = -1

// Named squares.
const (
	A1 Square = iota
	B1
	C1
	D1
	E1
	F1
	G1
	H1
	A2
	B2
	C2
	D2
	E2
	F2
	G2
	H2
	A3
	B3
	C3
	D3
	E3
	F3
	G3
	H3
	A4
	B4
	C4
	D4
	E4
	F4
	G4
	H4
	A5
	B5
	C5
	D5
	E5
	F5
	G5
	H5
	A6
	B6
	C6
	D6
	E6
	F6
	G6
	H6
	A7
	B7
	C7
	D7
	E7
	F7
	G7
	H7
	A8
	B8
	C8
	D8
	E8
	F8
	G8
	H8
)

func (s Square) File() int { return int(s) % 8 }
func (s Square) Rank() int { return int(s) / 8 }

// Offset returns the square df files and dr ranks away, or NoSquare if that
// is off the board.
func (s Square) Offset(df, dr int) Square {
	f, r := s.File()+df, s.Rank()+dr
	if f < 0 || f > 7 || r < 0 || r > 7 {
		return NoSquare
	}
	return Square(r*8 + f)
}

func (s Square) String() string {
	if s == NoSquare {
		return "-"
	}
	return string([]byte{byte('a' + s.File()), byte('1' + s.Rank())})
}

// ParseSquare parses "a1".."h8".
func ParseSquare(name string) (Square, error) {
	if len(name) != 2 || name[0] < 'a' || name[0] > 'h' || name[1] < '1' || name[1] > '8' {
		return NoSquare, fmt.Errorf("bad square %q", name)
	}
	return Square(int(name[1]-'1')*8 + int(name[0]-'a')), nil
}

// Sq is ParseSquare for literals; it panics on a bad name.
func Sq(name string) Square {
	s, err := ParseSquare(name)
	if err != nil {
		panic(err)
	}
	return s
}

// adjacent reports whether a and b are different squares that touch,
// including diagonally.
func adjacent(a, b Square) bool {
	df, dr := abs(a.File()-b.File()), abs(a.Rank()-b.Rank())
	return max(df, dr) == 1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}
```

- [ ] **Step 4: Write Position and FEN parsing**

Create `rules/position.go`:

```go
package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// Castling is a set of castling rights.
type Castling uint8

const (
	WhiteKingside Castling = 1 << iota
	WhiteQueenside
	BlackKingside
	BlackQueenside
)

// Position is everything needed to generate moves and resolve a turn.
type Position struct {
	Board [64]Piece
	// Mamdani is the Mamdani's square, or NoSquare once it has fallen
	// (and in plain-chess positions such as perft).
	Mamdani Square
	// Potholes holds the open pothole each color rolled, or NoSquare.
	// Each player has at most one: theirs closes on their next move,
	// before they can roll again.
	Potholes [2]Square
	Turn     Color
	Castling Castling
	EP       Square // en passant target square, or NoSquare
	Halfmove int    // plies since the last pawn move, capture or fall
	Fullmove int
}

const startFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// StartPosition is the standard chess setup with the Mamdani on a5.
func StartPosition() Position {
	p, err := ParseFEN(startFEN)
	if err != nil {
		panic(err)
	}
	p.Mamdani = A5
	return p
}

// IsPothole reports whether s holds an open pothole.
func (p *Position) IsPothole(s Square) bool {
	return s != NoSquare && (p.Potholes[White] == s || p.Potholes[Black] == s)
}

// Blocked reports whether s stops a slider: a piece, the Mamdani or a pothole.
func (p *Position) Blocked(s Square) bool {
	return p.Board[s] != NoPiece || s == p.Mamdani || p.IsPothole(s)
}

// King returns c's king square, or NoSquare if it has none (only in
// hand-built test positions).
func (p *Position) King(c Color) Square {
	k := NewPiece(c, King)
	for s := Square(0); s < 64; s++ {
		if p.Board[s] == k {
			return s
		}
	}
	return NoSquare
}

// Key identifies a position for repetition: pieces, Mamdani, open potholes,
// side to move, castling rights and en passant square.
type Key struct {
	Board    [64]Piece
	Mamdani  Square
	Potholes [2]Square
	Turn     Color
	Castling Castling
	EP       Square
}

func (p *Position) Key() Key {
	return Key{p.Board, p.Mamdani, p.Potholes, p.Turn, p.Castling, p.EP}
}

var fenPieces = map[byte]Piece{
	'P': NewPiece(White, Pawn), 'N': NewPiece(White, Knight), 'B': NewPiece(White, Bishop),
	'R': NewPiece(White, Rook), 'Q': NewPiece(White, Queen), 'K': NewPiece(White, King),
	'p': NewPiece(Black, Pawn), 'n': NewPiece(Black, Knight), 'b': NewPiece(Black, Bishop),
	'r': NewPiece(Black, Rook), 'q': NewPiece(Black, Queen), 'k': NewPiece(Black, King),
}

// ParseFEN reads standard FEN. The result has no Mamdani and no potholes;
// set those fields directly when a position needs them.
func ParseFEN(fen string) (Position, error) {
	p := Position{Mamdani: NoSquare, Potholes: [2]Square{NoSquare, NoSquare}, EP: NoSquare}
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return p, fmt.Errorf("fen %q: want 6 fields", fen)
	}
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return p, fmt.Errorf("fen %q: want 8 ranks", fen)
	}
	for i, row := range ranks {
		r, f := 7-i, 0
		for j := 0; j < len(row); j++ {
			c := row[j]
			if c >= '1' && c <= '8' {
				f += int(c - '0')
				continue
			}
			pc, ok := fenPieces[c]
			if !ok || f > 7 {
				return p, fmt.Errorf("fen %q: bad rank %q", fen, row)
			}
			p.Board[r*8+f] = pc
			f++
		}
		if f != 8 {
			return p, fmt.Errorf("fen %q: rank %q is not 8 squares", fen, row)
		}
	}
	switch fields[1] {
	case "w":
		p.Turn = White
	case "b":
		p.Turn = Black
	default:
		return p, fmt.Errorf("fen %q: bad side to move", fen)
	}
	for _, c := range fields[2] {
		switch c {
		case 'K':
			p.Castling |= WhiteKingside
		case 'Q':
			p.Castling |= WhiteQueenside
		case 'k':
			p.Castling |= BlackKingside
		case 'q':
			p.Castling |= BlackQueenside
		case '-':
		default:
			return p, fmt.Errorf("fen %q: bad castling", fen)
		}
	}
	if fields[3] != "-" {
		s, err := ParseSquare(fields[3])
		if err != nil {
			return p, fmt.Errorf("fen %q: %w", fen, err)
		}
		p.EP = s
	}
	var err error
	if p.Halfmove, err = strconv.Atoi(fields[4]); err != nil {
		return p, fmt.Errorf("fen %q: bad halfmove clock", fen)
	}
	if p.Fullmove, err = strconv.Atoi(fields[5]); err != nil {
		return p, fmt.Errorf("fen %q: bad fullmove number", fen)
	}
	return p, nil
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go vet ./rules/ && go test ./rules/`
Expected: `ok  	mamdani-chess/rules`

- [ ] **Step 6: Commit**

```bash
git add rules/board.go rules/position.go rules/board_test.go
git commit -m "rules: board, squares, pieces and FEN parsing"
```

---

### Task 2: Move generation, proven by perft

This task ships the whole move generator. Plain-chess perft proves standard chess. The movement tests prove the pothole and Mamdani additions: potholes block sliders, knights jump them, castling and en passant respect them, the Mamdani moves like a non-capturing queen, and a move is illegal if the close or repair step would expose your king.

**Files:**
- Create: `rules/attack.go`, `rules/movegen.go`, `rules/play.go`, `rules/event.go`
- Test: `rules/helpers_test.go`, `rules/perft_test.go`, `rules/movegen_test.go`

**Interfaces:**
- Consumes: everything from Task 1.
- Produces:
  - `type Move struct { From, To Square; Promo Kind }`. A Mamdani move has `From == pos.Mamdani`, and castling is the king's two-square move.
  - `(Move).String()` returns UCI (`"e7e8q"`), and `ParseMove(string) (Move, error)` parses it.
  - `(*Position).LegalMoves() []Move`, `(*Position).Attacked(Square, Color) bool`, `(*Position).InCheck(Color) bool`
  - unexported `(*Position).play(m Move, ev []Event) []Event`: makes the move, closes the mover's pothole, repairs, flips `Turn`. Task 3 and Task 5 call it.
  - `type Event struct { Kind EventKind; Move Move; Square Square; Piece Piece; Color Color; Roll int; Saved bool; Reason RerollReason }`, the `EventKind` constants and the `RerollReason` constants (used from Task 3 on).
  - Test helpers for later tasks: `setup(t, fen, mamdani, whiteHole, blackHole) Position`, `dice(rolls...) *ScriptedDice`, `mv(t, uci) Move`, `legal(t, p, uci) bool`, `apply(t, p, uci, d) (Position, []Event)`, `kinds([]Event) []EventKind`, `find([]Event, EventKind) (Event, bool)`. `dice` and `apply` refer to `ScriptedDice` and `Apply`, so this task adds a minimal `ScriptedDice` in Step 3. Task 4 moves it into `game.go`.

- [ ] **Step 1: Write the test helpers**

Create `rules/helpers_test.go`:

```go
package rules

import (
	"slices"
	"testing"
)

// setup parses fen and places the Mamdani (NoSquare for none) and the open
// potholes rolled by White and Black (NoSquare for none).
func setup(t *testing.T, fen string, mamdani, whiteHole, blackHole Square) Position {
	t.Helper()
	p, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	p.Mamdani = mamdani
	p.Potholes = [2]Square{whiteHole, blackHole}
	return p
}

func dice(rolls ...int) *ScriptedDice { return &ScriptedDice{Rolls: rolls} }

func mv(t *testing.T, uci string) Move {
	t.Helper()
	m, err := ParseMove(uci)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func legal(t *testing.T, p Position, uci string) bool {
	t.Helper()
	return slices.Contains(p.LegalMoves(), mv(t, uci))
}

// apply plays one turn and fails the test if the move is illegal or the
// dice script is not used up exactly.
func apply(t *testing.T, p Position, uci string, d *ScriptedDice) (Position, []Event) {
	t.Helper()
	next, ev, err := Apply(p, mv(t, uci), d)
	if err != nil {
		t.Fatalf("%s: %v", uci, err)
	}
	if d.Left() != 0 {
		t.Fatalf("%s: %d dice left over", uci, d.Left())
	}
	return next, ev
}

func kinds(ev []Event) []EventKind {
	var ks []EventKind
	for _, e := range ev {
		ks = append(ks, e.Kind)
	}
	return ks
}

func find(ev []Event, k EventKind) (Event, bool) {
	for _, e := range ev {
		if e.Kind == k {
			return e, true
		}
	}
	return Event{}, false
}
```

- [ ] **Step 2: Write the failing perft and movement tests**

Create `rules/perft_test.go`:

```go
package rules

import "testing"

func perft(p *Position, depth int) int {
	moves := p.LegalMoves()
	if depth == 1 {
		return len(moves)
	}
	n := 0
	for _, m := range moves {
		q := *p
		q.play(m, nil)
		n += perft(&q, depth-1)
	}
	return n
}

// Plain-chess perft (no Mamdani, no potholes) proves the move generator
// against published counts: https://www.chessprogramming.org/Perft_Results
func TestPerft(t *testing.T) {
	cases := []struct {
		name, fen string
		counts    []int // depth 1, 2, ...
	}{
		{"start", startFEN, []int{20, 400, 8902, 197281}},
		{"kiwipete", "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1", []int{48, 2039, 97862}},
		{"position 3", "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1", []int{14, 191, 2812, 43238}},
		{"position 4", "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1", []int{6, 264, 9467}},
		{"position 5", "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8", []int{44, 1486, 62379}},
	}
	for _, c := range cases {
		p, err := ParseFEN(c.fen)
		if err != nil {
			t.Fatal(err)
		}
		for i, want := range c.counts {
			if got := perft(&p, i+1); got != want {
				t.Errorf("%s depth %d: got %d, want %d", c.name, i+1, got, want)
			}
		}
	}
}
```

Create `rules/movegen_test.go`:

```go
package rules

import "testing"

func TestSlidersStopAtPotholes(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/R3K3 w - - 0 1", NoSquare, D1, NoSquare)
	for _, uci := range []string{"a1b1", "a1c1", "a1a8"} {
		if !legal(t, p, uci) {
			t.Errorf("%s should be legal", uci)
		}
	}
	if legal(t, p, "a1d1") {
		t.Error("rook landed on a pothole")
	}
}

func TestKnightsJumpPotholesButCannotLand(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/1N2K3 w - - 0 1", NoSquare, C3, C2)
	if !legal(t, p, "b1d2") || !legal(t, p, "b1a3") {
		t.Error("knight should jump over the pothole on c2")
	}
	if legal(t, p, "b1c3") {
		t.Error("knight landed on a pothole")
	}
}

func TestPawnsAndPotholes(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, NoSquare, E3)
	if legal(t, p, "e2e3") || legal(t, p, "e2e4") {
		t.Error("pawn moved into or across a pothole on e3")
	}
	p = setup(t, "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1", NoSquare, NoSquare, E4)
	if !legal(t, p, "e2e3") || legal(t, p, "e2e4") {
		t.Error("pothole on e4 should block only the double step")
	}
}

func TestPotholeBlocksCheck(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/4K2r w - - 0 1", NoSquare, NoSquare, F1)
	if p.InCheck(White) {
		t.Error("a pothole on f1 should block the rook's check")
	}
}

func TestCastlingAndPotholes(t *testing.T) {
	const fen = "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1"
	p := setup(t, fen, NoSquare, NoSquare, NoSquare)
	if !legal(t, p, "e1g1") || !legal(t, p, "e1c1") {
		t.Fatal("both castles should be legal")
	}
	p = setup(t, fen, NoSquare, NoSquare, B1)
	if legal(t, p, "e1c1") || !legal(t, p, "e1g1") {
		t.Error("a pothole on b1 should block only queenside castling")
	}
	p = setup(t, fen, NoSquare, NoSquare, F1)
	if legal(t, p, "e1g1") {
		t.Error("a pothole on f1 should block kingside castling")
	}
	p = setup(t, fen, B1, NoSquare, NoSquare)
	if legal(t, p, "e1c1") {
		t.Error("the Mamdani on b1 should block queenside castling")
	}
}

func TestPotholeCancelsEnPassant(t *testing.T) {
	const fen = "4k3/8/8/3Pp3/8/8/8/4K3 w - e6 0 1"
	p := setup(t, fen, NoSquare, NoSquare, NoSquare)
	if !legal(t, p, "d5e6") {
		t.Fatal("en passant should be legal")
	}
	p = setup(t, fen, NoSquare, NoSquare, E6)
	if legal(t, p, "d5e6") {
		t.Error("a pothole on e6 should cancel en passant")
	}
}

func TestMamdaniMovesLikeAQueenWithoutCapturing(t *testing.T) {
	p := StartPosition()
	// 20 normal moves + 13 Mamdani moves from a5: a6, a4, a3, b5-h5, b6, b4, c3.
	if got := len(p.LegalMoves()); got != 33 {
		t.Errorf("start position has %d legal moves, want 33", got)
	}
	for _, uci := range []string{"a5a7", "a5a2", "a5c7", "a5d2"} {
		if legal(t, p, uci) {
			t.Errorf("Mamdani captured with %s", uci)
		}
	}
}

func TestMamdaniCanGoStraightBack(t *testing.T) {
	p, _ := apply(t, StartPosition(), "a5b5", dice(1))
	if !legal(t, p, "b5a5") {
		t.Error("Black should be able to move the Mamdani straight back")
	}
}

func TestMamdaniCannotUnpinOwnKing(t *testing.T) {
	p := setup(t, "4k3/8/8/8/8/8/8/r3K3 w - - 0 1", C1, NoSquare, NoSquare)
	if !legal(t, p, "c1b1") || !legal(t, p, "c1d1") {
		t.Error("the Mamdani may slide along the pin line")
	}
	if legal(t, p, "c1c2") {
		t.Error("moving the Mamdani off the line would leave White in check")
	}
}

func TestMoveIllegalIfOwnPotholeClosingExposesKing(t *testing.T) {
	const fen = "k3r3/8/8/8/8/8/8/4K3 w - - 0 1"
	p := setup(t, fen, NoSquare, E4, NoSquare)
	if got := len(p.LegalMoves()); got != 4 || legal(t, p, "e1e2") {
		t.Errorf("White's own pothole closes after this move, so only king moves off the e-file are legal; got %v", p.LegalMoves())
	}
	p = setup(t, fen, NoSquare, NoSquare, E4)
	if !legal(t, p, "e1e2") {
		t.Error("Black's pothole stays open, so e1e2 is safe")
	}
}

func TestMoveIllegalIfRepairExposesKing(t *testing.T) {
	p := setup(t, "k3r3/8/8/8/8/8/8/4K3 w - - 0 1", H3, NoSquare, E4)
	if legal(t, p, "h3f3") || legal(t, p, "h3f5") {
		t.Error("moving the Mamdani next to e4 repairs it and exposes the king")
	}
	if !legal(t, p, "h3g3") {
		t.Error("h3g3 leaves the pothole alone")
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./rules/`
Expected: FAIL to build, with errors like `undefined: Move`, `undefined: ScriptedDice` and `undefined: Apply`.

- [ ] **Step 4: Write the event types**

Create `rules/event.go`:

```go
package rules

// EventKind names one step of a turn, in the order RULES.md plays them.
type EventKind string

const (
	Moved         EventKind = "moved"          // Move, Piece (MamdaniPiece for a Mamdani move), Color = mover
	Captured      EventKind = "captured"       // Square, Piece
	PotholeClosed EventKind = "pothole_closed" // Square
	Repaired      EventKind = "repaired"       // Square: by the repair step, or a new pothole next to the Mamdani
	RolledPothole EventKind = "rolled_pothole" // Roll: odd = nothing, even = a pothole opens
	Target        EventKind = "target"         // Square picked by the two placement d8s
	Reroll        EventKind = "reroll"         // Square, Reason
	SavingRoll    EventKind = "saving_roll"    // Square, Piece, Roll, Saved, Color = who rolls
	Fell          EventKind = "fell"           // Square, Piece
	PotholeOpened EventKind = "pothole_opened" // Square, Color = roller
	NoPothole     EventKind = "no_pothole"     // re-roll cap reached
)

// RerollReason says why placement dice were re-rolled.
type RerollReason string

const (
	ReasonKing      RerollReason = "king"      // kings never fall
	ReasonPothole   RerollReason = "pothole"   // already a pothole
	ReasonExposes   RerollReason = "exposes"   // the fall would leave the roller in check
	ReasonCheckmate RerollReason = "checkmate" // the result would checkmate the next player
)

// Event is one thing that happened during a turn. Fields not listed for a
// Kind are zero.
type Event struct {
	Kind   EventKind
	Move   Move
	Square Square
	Piece  Piece
	Color  Color
	Roll   int
	Saved  bool
	Reason RerollReason
}
```

- [ ] **Step 5: Write attack detection**

Create `rules/attack.go`:

```go
package rules

var (
	rookDirs    = [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}}
	bishopDirs  = [][2]int{{1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	queenDirs   = append(append([][2]int{}, rookDirs...), bishopDirs...)
	knightJumps = [][2]int{{1, 2}, {2, 1}, {2, -1}, {1, -2}, {-1, -2}, {-2, -1}, {-2, 1}, {-1, 2}}
)

// Attacked reports whether any piece of color by attacks s. Potholes and the
// Mamdani block sliders exactly like pieces do.
func (p *Position) Attacked(s Square, by Color) bool {
	// A pawn of color by attacks s from one rank behind s (from by's side).
	back := -1
	if by == Black {
		back = 1
	}
	for _, df := range []int{-1, 1} {
		if t := s.Offset(df, back); t != NoSquare && p.Board[t] == NewPiece(by, Pawn) {
			return true
		}
	}
	for _, j := range knightJumps {
		if t := s.Offset(j[0], j[1]); t != NoSquare && p.Board[t] == NewPiece(by, Knight) {
			return true
		}
	}
	for _, d := range queenDirs {
		if t := s.Offset(d[0], d[1]); t != NoSquare && p.Board[t] == NewPiece(by, King) {
			return true
		}
	}
	return p.slidingAttack(s, by, rookDirs, Rook) || p.slidingAttack(s, by, bishopDirs, Bishop)
}

func (p *Position) slidingAttack(s Square, by Color, dirs [][2]int, k Kind) bool {
	for _, d := range dirs {
		t := s.Offset(d[0], d[1])
		for t != NoSquare && !p.Blocked(t) {
			t = t.Offset(d[0], d[1])
		}
		if t == NoSquare {
			continue
		}
		if pc := p.Board[t]; pc == NewPiece(by, k) || pc == NewPiece(by, Queen) {
			return true
		}
	}
	return false
}

// InCheck reports whether c's king is attacked.
func (p *Position) InCheck(c Color) bool {
	k := p.King(c)
	return k != NoSquare && p.Attacked(k, c.Other())
}
```

- [ ] **Step 6: Write the move generator**

Create `rules/movegen.go`:

```go
package rules

import "fmt"

// Move is one move. A Mamdani move has From set to the Mamdani's square.
// Castling is the king's two-square move (e1g1). Promo is NoKind unless a
// pawn promotes.
type Move struct {
	From, To Square
	Promo    Kind
}

var promoLetters = map[Kind]byte{Knight: 'n', Bishop: 'b', Rook: 'r', Queen: 'q'}

// String returns the move in UCI form: "e2e4", "e7e8q".
func (m Move) String() string {
	s := m.From.String() + m.To.String()
	if c, ok := promoLetters[m.Promo]; ok {
		s += string(c)
	}
	return s
}

// ParseMove reads UCI form.
func ParseMove(s string) (Move, error) {
	if len(s) != 4 && len(s) != 5 {
		return Move{}, fmt.Errorf("bad move %q", s)
	}
	from, err := ParseSquare(s[0:2])
	if err != nil {
		return Move{}, err
	}
	to, err := ParseSquare(s[2:4])
	if err != nil {
		return Move{}, err
	}
	m := Move{From: from, To: to}
	if len(s) == 5 {
		for k, c := range promoLetters {
			if c == s[4] {
				m.Promo = k
			}
		}
		if m.Promo == NoKind {
			return Move{}, fmt.Errorf("bad promotion in %q", s)
		}
	}
	return m, nil
}

// LegalMoves returns every legal move for the side to move: its own pieces
// and the Mamdani. A move is legal only if the mover's king is safe after
// the move, the close step and the repair step.
func (p *Position) LegalMoves() []Move {
	var legal []Move
	for _, m := range p.pseudoMoves() {
		q := *p
		q.play(m, nil)
		if !q.InCheck(p.Turn) {
			legal = append(legal, m)
		}
	}
	return legal
}

// canLand reports whether a piece of color c may finish a move on s.
func (p *Position) canLand(s Square, c Color) bool {
	if p.IsPothole(s) || s == p.Mamdani {
		return false
	}
	pc := p.Board[s]
	return pc == NoPiece || pc.Color() != c
}

func (p *Position) pseudoMoves() []Move {
	c := p.Turn
	moves := make([]Move, 0, 64)
	if p.Mamdani != NoSquare {
		for _, d := range queenDirs {
			for t := p.Mamdani.Offset(d[0], d[1]); t != NoSquare && !p.Blocked(t); t = t.Offset(d[0], d[1]) {
				moves = append(moves, Move{From: p.Mamdani, To: t})
			}
		}
	}
	for s := Square(0); s < 64; s++ {
		pc := p.Board[s]
		if pc == NoPiece || pc.Color() != c {
			continue
		}
		switch pc.Kind() {
		case Pawn:
			moves = p.pawnMoves(moves, s)
		case Knight:
			moves = p.stepMoves(moves, s, knightJumps)
		case Bishop:
			moves = p.slideMoves(moves, s, bishopDirs)
		case Rook:
			moves = p.slideMoves(moves, s, rookDirs)
		case Queen:
			moves = p.slideMoves(moves, s, queenDirs)
		case King:
			moves = p.stepMoves(moves, s, queenDirs)
			moves = p.castlingMoves(moves, s)
		}
	}
	return moves
}

func (p *Position) stepMoves(moves []Move, from Square, steps [][2]int) []Move {
	c := p.Board[from].Color()
	for _, d := range steps {
		if t := from.Offset(d[0], d[1]); t != NoSquare && p.canLand(t, c) {
			moves = append(moves, Move{From: from, To: t})
		}
	}
	return moves
}

func (p *Position) slideMoves(moves []Move, from Square, dirs [][2]int) []Move {
	c := p.Board[from].Color()
	for _, d := range dirs {
		for t := from.Offset(d[0], d[1]); t != NoSquare; t = t.Offset(d[0], d[1]) {
			if p.canLand(t, c) {
				moves = append(moves, Move{From: from, To: t})
			}
			if p.Blocked(t) {
				break
			}
		}
	}
	return moves
}

func pawnDir(c Color) int {
	if c == White {
		return 1
	}
	return -1
}

func (p *Position) pawnMoves(moves []Move, from Square) []Move {
	c := p.Board[from].Color()
	dir := pawnDir(c)
	add := func(to Square) {
		if to.Rank() == 0 || to.Rank() == 7 {
			for _, k := range []Kind{Queen, Rook, Bishop, Knight} {
				moves = append(moves, Move{From: from, To: to, Promo: k})
			}
			return
		}
		moves = append(moves, Move{From: from, To: to})
	}
	if one := from.Offset(0, dir); one != NoSquare && !p.Blocked(one) {
		add(one)
		startRank := 1
		if c == Black {
			startRank = 6
		}
		if two := one.Offset(0, dir); from.Rank() == startRank && !p.Blocked(two) {
			add(two)
		}
	}
	for _, df := range []int{-1, 1} {
		t := from.Offset(df, dir)
		if t == NoSquare {
			continue
		}
		if pc := p.Board[t]; pc != NoPiece && pc.Color() != c {
			add(t)
		} else if t == p.EP && !p.Blocked(t) && p.Board[t.Offset(0, -dir)] == NewPiece(c.Other(), Pawn) {
			// En passant. A pothole on the target square cancels it.
			add(t)
		}
	}
	return moves
}

func (p *Position) castlingMoves(moves []Move, k Square) []Move {
	c := p.Board[k].Color()
	home, ks, qs := E1, WhiteKingside, WhiteQueenside
	if c == Black {
		home, ks, qs = E8, BlackKingside, BlackQueenside
	}
	if k != home || p.Attacked(k, c.Other()) {
		return moves
	}
	rook := NewPiece(c, Rook)
	// Every square between king and rook must be clear of pieces, the
	// Mamdani and potholes; the king may not pass through or land on an
	// attacked square.
	if p.Castling&ks != 0 && p.Board[k.Offset(3, 0)] == rook &&
		!p.Blocked(k.Offset(1, 0)) && !p.Blocked(k.Offset(2, 0)) &&
		!p.Attacked(k.Offset(1, 0), c.Other()) && !p.Attacked(k.Offset(2, 0), c.Other()) {
		moves = append(moves, Move{From: k, To: k.Offset(2, 0)})
	}
	if p.Castling&qs != 0 && p.Board[k.Offset(-4, 0)] == rook &&
		!p.Blocked(k.Offset(-1, 0)) && !p.Blocked(k.Offset(-2, 0)) && !p.Blocked(k.Offset(-3, 0)) &&
		!p.Attacked(k.Offset(-1, 0), c.Other()) && !p.Attacked(k.Offset(-2, 0), c.Other()) {
		moves = append(moves, Move{From: k, To: k.Offset(-2, 0)})
	}
	return moves
}
```

- [ ] **Step 7: Write move making, close and repair**

Create `rules/play.go`:

```go
package rules

// play makes move m for the side to move, then runs the close and repair
// steps and passes the turn. It assumes m is pseudo-legal. Events are
// appended to ev; pass nil when only the resulting position matters.
func (p *Position) play(m Move, ev []Event) []Event {
	mover := p.Turn
	if m.From == p.Mamdani {
		ev = append(ev, Event{Kind: Moved, Move: m, Piece: MamdaniPiece, Color: mover})
		p.Mamdani = m.To
		p.EP = NoSquare
		p.Halfmove++ // Mamdani moves never reset the 50-move count
	} else {
		ev = p.movePiece(m, ev)
	}
	if mover == Black {
		p.Fullmove++
	}
	// Close: the mover's own pothole from their previous turn.
	if s := p.Potholes[mover]; s != NoSquare {
		p.Potholes[mover] = NoSquare
		ev = append(ev, Event{Kind: PotholeClosed, Square: s})
	}
	ev = p.repair(ev)
	p.Turn = mover.Other()
	return ev
}

func (p *Position) movePiece(m Move, ev []Event) []Event {
	pc := p.Board[m.From]
	c := pc.Color()
	ev = append(ev, Event{Kind: Moved, Move: m, Piece: pc, Color: c})

	capSq := m.To
	if pc.Kind() == Pawn && m.To == p.EP && p.Board[m.To] == NoPiece {
		capSq = m.To.Offset(0, -pawnDir(c))
	}
	captured := p.Board[capSq]
	if captured != NoPiece {
		p.Board[capSq] = NoPiece
		ev = append(ev, Event{Kind: Captured, Square: capSq, Piece: captured})
	}

	p.Board[m.From] = NoPiece
	if m.Promo != NoKind {
		p.Board[m.To] = NewPiece(c, m.Promo)
	} else {
		p.Board[m.To] = pc
	}

	if pc.Kind() == King && abs(m.To.File()-m.From.File()) == 2 {
		rookFrom, rookTo := m.From.Offset(3, 0), m.From.Offset(1, 0)
		if m.To.File() < m.From.File() {
			rookFrom, rookTo = m.From.Offset(-4, 0), m.From.Offset(-1, 0)
		}
		p.Board[rookTo], p.Board[rookFrom] = p.Board[rookFrom], NoPiece
	}

	p.Castling &^= rightsLost(m.From) | rightsLost(m.To)

	p.EP = NoSquare
	if pc.Kind() == Pawn && abs(m.To.Rank()-m.From.Rank()) == 2 {
		p.EP = m.From.Offset(0, pawnDir(c))
	}

	if pc.Kind() == Pawn || captured != NoPiece {
		p.Halfmove = 0
	} else {
		p.Halfmove++
	}
	return ev
}

// rightsLost returns the castling rights that disappear when a piece moves
// from, or is captured or falls on, s.
func rightsLost(s Square) Castling {
	switch s {
	case E1:
		return WhiteKingside | WhiteQueenside
	case H1:
		return WhiteKingside
	case A1:
		return WhiteQueenside
	case E8:
		return BlackKingside | BlackQueenside
	case H8:
		return BlackKingside
	case A8:
		return BlackQueenside
	}
	return 0
}

// repair removes every open pothole next to the Mamdani.
func (p *Position) repair(ev []Event) []Event {
	if p.Mamdani == NoSquare {
		return ev
	}
	for c, s := range p.Potholes {
		if s != NoSquare && adjacent(s, p.Mamdani) {
			p.Potholes[c] = NoSquare
			ev = append(ev, Event{Kind: Repaired, Square: s})
		}
	}
	return ev
}
```

- [ ] **Step 8: Add temporary stubs so the helpers compile**

`helpers_test.go` uses `ScriptedDice` and `Apply`, which Tasks 3 and 4 implement properly. Create `rules/turn.go` with just enough for this task. Task 3 replaces the whole file.

```go
package rules

import (
	"errors"
	"slices"
)

// Dice rolls eight-sided dice. D8 returns 1..8.
type Dice interface {
	D8() int
}

// ErrIllegalMove is returned by Apply for a move not in LegalMoves.
var ErrIllegalMove = errors.New("illegal move")

// Apply is completed in Task 3; for now it only plays the move.
func Apply(p Position, m Move, dice Dice) (Position, []Event, error) {
	if !slices.Contains(p.LegalMoves(), m) {
		return p, nil, ErrIllegalMove
	}
	ev := p.play(m, nil)
	ev = append(ev, Event{Kind: RolledPothole, Roll: dice.D8()})
	return p, ev, nil
}
```

Create `rules/game.go` with just `ScriptedDice`. Task 4 replaces the whole file.

```go
package rules

// ScriptedDice returns preset rolls in order. It panics when it runs out:
// in tests that means the script is too short.
type ScriptedDice struct {
	Rolls []int
	next  int
}

func (s *ScriptedDice) D8() int {
	if s.next >= len(s.Rolls) {
		panic("scripted dice ran out")
	}
	r := s.Rolls[s.next]
	s.next++
	return r
}

// Left returns how many rolls have not been used.
func (s *ScriptedDice) Left() int { return len(s.Rolls) - s.next }
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go vet ./rules/ && go test -v -run 'Perft|Slider|Knight|Pawn|Pothole|Castling|EnPassant|Mamdani|MoveIllegal' ./rules/`
Expected: every test PASSes, `TestPerft` included. If perft fails, debug the generator with a divide (per-move counts at depth 1) against a known engine; don't change the expected numbers.

- [ ] **Step 10: Commit**

```bash
git add rules/attack.go rules/movegen.go rules/play.go rules/event.go rules/turn.go rules/game.go rules/helpers_test.go rules/perft_test.go rules/movegen_test.go
git commit -m "rules: legal move generation with potholes and the Mamdani, proven by perft"
```

---

### Task 3: Turn resolution

**Files:**
- Modify: `rules/turn.go` (replace the Task 2 stub entirely)
- Test: `rules/turn_test.go`

**Interfaces:**
- Consumes: `play`, `LegalMoves`, `InCheck`, `Event`, `ScriptedDice` (Task 2), named squares (Task 1).
- Produces:
  - `type Dice interface { D8() int }`, `var ErrIllegalMove`
  - `Apply(p Position, m Move, dice Dice) (Position, []Event, error)`. It is pure: `p` is a value and is not modified. Events come in RULES.md order. The dice are consumed in this order: pothole roll, then (file, rank) pairs until a valid target, then one saving roll if any.
  - unexported `maxRerolls = 64`, `mamdaniReaches(Square) bool`

- [ ] **Step 1: Write the failing tests**

Create `rules/turn_test.go`:

```go
package rules

import (
	"errors"
	"slices"
	"testing"
)

func TestOddRollNoPothole(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(3))
	if want := []EventKind{Moved, RolledPothole}; !slices.Equal(kinds(ev), want) {
		t.Errorf("events %v, want %v", kinds(ev), want)
	}
	if p.Potholes != [2]Square{NoSquare, NoSquare} || p.Turn != Black {
		t.Errorf("potholes %v turn %v", p.Potholes, p.Turn)
	}
}

func TestIllegalMovesRejected(t *testing.T) {
	// What a buggy or hostile client might send.
	for _, uci := range []string{
		"e2e5",  // too far
		"e7e5",  // opponent's piece
		"e2e4q", // promotion that isn't one
		"a5a7",  // the Mamdani capturing
		"e3e4",  // empty square
	} {
		if _, _, err := Apply(StartPosition(), mv(t, uci), dice()); !errors.Is(err, ErrIllegalMove) {
			t.Errorf("%s: got %v, want ErrIllegalMove", uci, err)
		}
	}
	p := setup(t, "4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare, NoSquare, NoSquare)
	if _, _, err := Apply(p, mv(t, "a7a8"), dice()); !errors.Is(err, ErrIllegalMove) {
		t.Errorf("promotion without a piece: got %v, want ErrIllegalMove", err)
	}
}

func TestPotholeOpensAndClosesAfterRollersNextMove(t *testing.T) {
	// Even roll, then file 4 rank 4: d4.
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 4, 4))
	if want := []EventKind{Moved, RolledPothole, Target, PotholeOpened}; !slices.Equal(kinds(ev), want) {
		t.Fatalf("events %v, want %v", kinds(ev), want)
	}
	if p.Potholes[White] != D4 {
		t.Fatalf("white pothole %v, want d4", p.Potholes[White])
	}
	p, _ = apply(t, p, "g8f6", dice(1))
	if p.Potholes[White] != D4 {
		t.Fatal("pothole closed before White's next move")
	}
	p, ev = apply(t, p, "g1f3", dice(1))
	if e, ok := find(ev, PotholeClosed); !ok || e.Square != D4 || p.Potholes[White] != NoSquare {
		t.Errorf("pothole should close after White's next move: %v", ev)
	}
}

func TestTwoPotholesAtOnce(t *testing.T) {
	p, _ := apply(t, StartPosition(), "e2e4", dice(2, 4, 4)) // White: d4
	p, _ = apply(t, p, "e7e5", dice(4, 8, 3))                // Black: h3
	if p.Potholes != [2]Square{D4, H3} {
		t.Errorf("potholes %v, want [d4 h3]", p.Potholes)
	}
}

func TestRerollKing(t *testing.T) {
	_, ev := apply(t, StartPosition(), "e2e4", dice(2, 5, 1, 4, 4)) // e1, then d4
	if e, ok := find(ev, Reroll); !ok || e.Square != E1 || e.Reason != ReasonKing {
		t.Errorf("want a king re-roll on e1: %v", ev)
	}
	if e, _ := find(ev, PotholeOpened); e.Square != D4 {
		t.Errorf("pothole should open on d4: %v", ev)
	}
}

func TestRerollExistingPothole(t *testing.T) {
	p := StartPosition()
	p.Potholes[Black] = D4
	_, ev := apply(t, p, "e2e4", dice(6, 4, 4, 8, 3)) // d4, then h3
	if e, ok := find(ev, Reroll); !ok || e.Reason != ReasonPothole {
		t.Errorf("want a pothole re-roll: %v", ev)
	}
}

func TestRerollCap(t *testing.T) {
	rolls := []int{2}
	for range maxRerolls + 1 {
		rolls = append(rolls, 5, 1) // e1, the king, every time
	}
	_, ev := apply(t, StartPosition(), "e2e4", dice(rolls...))
	if ev[len(ev)-1].Kind != NoPothole {
		t.Errorf("last event %v, want no_pothole", ev[len(ev)-1].Kind)
	}
}

func TestNextToMamdaniRepairedOnOpening(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 2, 4)) // b4, next to a5
	if e, ok := find(ev, Repaired); !ok || e.Square != B4 {
		t.Errorf("want b4 repaired at once: %v", ev)
	}
	if p.Potholes != [2]Square{NoSquare, NoSquare} {
		t.Errorf("no pothole should stay open: %v", p.Potholes)
	}
}

func TestPieceWithoutMamdaniLineFalls(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 7, 8)) // g8 knight; no line from a5
	if e, ok := find(ev, Fell); !ok || e.Square != G8 || e.Piece != NewPiece(Black, Knight) {
		t.Fatalf("want the g8 knight to fall: %v", ev)
	}
	if _, ok := find(ev, SavingRoll); ok {
		t.Error("no saving roll without a clear Mamdani line")
	}
	if p.Board[G8] != NoPiece || p.Potholes[White] != G8 || p.Halfmove != 0 {
		t.Errorf("board %v pothole %v halfmove %d", p.Board[G8], p.Potholes[White], p.Halfmove)
	}
}

func TestSavingRoll(t *testing.T) {
	// a5-b4-c3-d2 is a clear diagonal, so the d2 pawn gets a saving roll,
	// made by its owner.
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 4, 2, 5))
	e, ok := find(ev, SavingRoll)
	if !ok || !e.Saved || e.Color != White || e.Roll != 5 {
		t.Fatalf("want a successful white saving roll: %v", ev)
	}
	if p.Board[D2] != NewPiece(White, Pawn) || p.Potholes[White] != NoSquare {
		t.Error("a saved piece stays and no pothole opens")
	}

	p, ev = apply(t, StartPosition(), "e2e4", dice(2, 4, 2, 6))
	if e, _ := find(ev, SavingRoll); e.Saved {
		t.Fatal("even saving roll should fail")
	}
	if p.Board[D2] != NoPiece || p.Potholes[White] != D2 {
		t.Error("the pawn falls and the pothole opens")
	}
}

func TestMamdaniFalls(t *testing.T) {
	p, ev := apply(t, StartPosition(), "e2e4", dice(2, 1, 5, 3)) // a5, saved
	if e, ok := find(ev, SavingRoll); !ok || !e.Saved || e.Piece != MamdaniPiece || e.Color != White {
		t.Fatalf("the roller makes the Mamdani's saving roll: %v", ev)
	}
	if p.Mamdani != A5 {
		t.Fatal("saved Mamdani stays")
	}

	p, ev = apply(t, StartPosition(), "e2e4", dice(2, 1, 5, 4)) // a5, falls
	if e, ok := find(ev, Fell); !ok || e.Piece != MamdaniPiece {
		t.Fatalf("Mamdani should fall: %v", ev)
	}
	if p.Mamdani != NoSquare || p.Potholes[White] != A5 {
		t.Fatalf("mamdani %v pothole %v", p.Mamdani, p.Potholes[White])
	}
	// With the Mamdani gone there are no more saving rolls: a7 falls at once.
	_, ev = apply(t, p, "e7e5", dice(2, 1, 7))
	if _, ok := find(ev, SavingRoll); ok {
		t.Error("no saving rolls after the Mamdani has fallen")
	}
}

func TestRerollIfFallExposesRoller(t *testing.T) {
	// The e2 bishop shields the white king from the e8 rook.
	p := setup(t, "k3r3/8/8/8/8/8/4B2P/4K3 w - - 0 1", NoSquare, NoSquare, NoSquare)
	_, ev := apply(t, p, "h2h3", dice(2, 5, 2, 8, 8)) // e2, then h8
	if e, ok := find(ev, Reroll); !ok || e.Square != E2 || e.Reason != ReasonExposes {
		t.Errorf("want an exposes re-roll on e2: %v", ev)
	}
}

func TestRerollIfResultWouldCheckmate(t *testing.T) {
	// Ra8+ is answered only by the c7 knight (Nxa8 or Ne8). If it fell,
	// Black would be mated by the roll, so the dice go again.
	p := setup(t, "7k/2n3pp/8/8/8/8/8/R5K1 w - - 0 1", NoSquare, NoSquare, NoSquare)
	p, ev := apply(t, p, "a1a8", dice(2, 3, 7, 1, 1)) // c7, then a1
	if e, ok := find(ev, Reroll); !ok || e.Square != C7 || e.Reason != ReasonCheckmate {
		t.Errorf("want a checkmate re-roll on c7: %v", ev)
	}
	if p.Board[C7] != NewPiece(Black, Knight) {
		t.Error("the knight must survive")
	}
}

func TestRepairStepAfterMove(t *testing.T) {
	p := StartPosition()
	p.Potholes[Black] = D4
	p, ev := apply(t, p, "a5c5", dice(1)) // c5 touches d4
	if e, ok := find(ev, Repaired); !ok || e.Square != D4 || p.Potholes[Black] != NoSquare {
		t.Errorf("moving next to d4 should repair it: %v", ev)
	}
}

func TestMamdaniMoveDoesNotResetFiftyMoveCount(t *testing.T) {
	p := StartPosition()
	p.Halfmove = 10
	p, _ = apply(t, p, "a5b5", dice(1))
	if p.Halfmove != 11 {
		t.Errorf("halfmove %d, want 11", p.Halfmove)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./rules/`
Expected: FAIL to build, with `undefined: maxRerolls`. The stub `Apply` from Task 2 doesn't roll for placement yet.

- [ ] **Step 3: Write turn resolution**

Replace `rules/turn.go` entirely:

```go
package rules

import (
	"errors"
	"slices"
)

// Dice rolls eight-sided dice. D8 returns 1..8.
type Dice interface {
	D8() int
}

// ErrIllegalMove is returned by Apply for a move not in LegalMoves.
var ErrIllegalMove = errors.New("illegal move")

// maxRerolls caps placement re-rolls; past it no pothole opens this turn.
const maxRerolls = 64

// Apply plays one full turn: move, close, repair, pothole roll, placement
// and resolution. It returns the new position and what happened, in order.
// p is not modified.
func Apply(p Position, m Move, dice Dice) (Position, []Event, error) {
	if !slices.Contains(p.LegalMoves(), m) {
		return p, nil, ErrIllegalMove
	}
	mover := p.Turn
	ev := p.play(m, nil)
	ev = p.rollPothole(mover, dice, ev)
	return p, ev, nil
}

func (p *Position) rollPothole(mover Color, dice Dice, ev []Event) []Event {
	r := dice.D8()
	ev = append(ev, Event{Kind: RolledPothole, Roll: r, Color: mover})
	if r%2 == 1 {
		return ev
	}
	for range maxRerolls + 1 {
		file, rank := dice.D8(), dice.D8()
		s := Square((rank-1)*8 + file - 1)
		ev = append(ev, Event{Kind: Target, Square: s})
		if reason := p.rerollReason(s, mover); reason != "" {
			ev = append(ev, Event{Kind: Reroll, Square: s, Reason: reason})
			continue
		}
		return p.resolve(s, mover, dice, ev)
	}
	return append(ev, Event{Kind: NoPothole})
}

// rerollReason returns why the placement dice must be rolled again for s,
// or "" if s is a valid target.
func (p *Position) rerollReason(s Square, mover Color) RerollReason {
	if p.Board[s].Kind() == King {
		return ReasonKing
	}
	if p.IsPothole(s) {
		return ReasonPothole
	}
	if p.nextToMamdani(s) {
		return "" // repaired the moment it opens; nothing changes
	}
	// Would the fall leave the roller in check once the hole is gone?
	// While open, the hole blocks the same lines the piece did, so the test
	// is made without it: the roller must not inherit an unanswerable check
	// when their own pothole closes after their next move.
	gone := *p
	gone.remove(s)
	if gone.InCheck(mover) {
		return ReasonExposes
	}
	// Would the outcome checkmate the next player? A roll never wins.
	opened := gone
	opened.Potholes[mover] = s
	if opened.InCheck(opened.Turn) && len(opened.LegalMoves()) == 0 {
		return ReasonCheckmate
	}
	return ""
}

func (p *Position) nextToMamdani(s Square) bool {
	return p.Mamdani != NoSquare && adjacent(s, p.Mamdani)
}

// remove takes whatever stands on s off the board.
func (p *Position) remove(s Square) {
	if s == p.Mamdani {
		p.Mamdani = NoSquare
		return
	}
	p.Board[s] = NoPiece
}

func (p *Position) resolve(s Square, mover Color, dice Dice, ev []Event) []Event {
	if p.nextToMamdani(s) {
		return append(ev, Event{Kind: Repaired, Square: s})
	}
	switch {
	case s == p.Mamdani:
		// The Mamdani always gets a saving roll, made by the roller.
		r := dice.D8()
		saved := r%2 == 1
		ev = append(ev, Event{Kind: SavingRoll, Square: s, Piece: MamdaniPiece, Roll: r, Saved: saved, Color: mover})
		if saved {
			return ev
		}
		ev = append(ev, Event{Kind: Fell, Square: s, Piece: MamdaniPiece})
		p.Mamdani = NoSquare
	case p.Board[s] != NoPiece:
		pc := p.Board[s]
		if p.mamdaniReaches(s) {
			r := dice.D8()
			saved := r%2 == 1
			ev = append(ev, Event{Kind: SavingRoll, Square: s, Piece: pc, Roll: r, Saved: saved, Color: pc.Color()})
			if saved {
				return ev
			}
		}
		ev = append(ev, Event{Kind: Fell, Square: s, Piece: pc})
		p.Board[s] = NoPiece
		p.Halfmove = 0
		p.Castling &^= rightsLost(s)
	}
	p.Potholes[mover] = s
	return append(ev, Event{Kind: PotholeOpened, Square: s, Color: mover})
}

// mamdaniReaches reports whether the Mamdani has a clear queen line to s:
// same rank, file or diagonal, nothing in between. That is all a saving
// roll needs.
func (p *Position) mamdaniReaches(s Square) bool {
	m := p.Mamdani
	if m == NoSquare || m == s {
		return false
	}
	df, dr := s.File()-m.File(), s.Rank()-m.Rank()
	if df != 0 && dr != 0 && abs(df) != abs(dr) {
		return false
	}
	for t := m.Offset(sign(df), sign(dr)); t != s; t = t.Offset(sign(df), sign(dr)) {
		if p.Blocked(t) {
			return false
		}
	}
	return true
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./rules/ && go test ./rules/`
Expected: `ok  	mamdani-chess/rules`

- [ ] **Step 5: Commit**

```bash
git add rules/turn.go rules/turn_test.go
git commit -m "rules: pothole roll, placement, re-rolls and saving rolls"
```

---

### Task 4: Game end, history and replay

**Files:**
- Modify: `rules/game.go` (replace the Task 2 stub entirely)
- Test: `rules/game_test.go`

**Interfaces:**
- Consumes: `Apply`, `Dice`, `ErrIllegalMove` (Task 3), `Key` (Task 1).
- Produces (the game server builds on these):
  - `type Reason string` with `Checkmate`, `Stalemate`, `FiftyMoves`, `Repetition`, `InsufficientMaterial`. The server adds its own (timeout, resignation, forfeit).
  - `type Result struct { Over, Draw bool; Winner Color; Reason Reason }`. The zero value means the game is on.
  - `type Turn struct { Move Move; Dice []int }` is what the store persists per turn.
  - `type Game struct { Pos Position; Turns []Turn; Result Result; /* unexported seen */ }`
  - `NewGame() *Game`, `NewGameFrom(Position) *Game`, `(*Game).Play(Move, Dice) ([]Event, error)`, `var ErrGameOver`
  - `(*Position).CannotMate(Color) bool` decides whether a flag fall is a draw.
  - `Replay(start Position, turns []Turn) (*Game, error)`
  - `ScriptedDice` (unchanged from Task 2), with an exported `Rolls` field and `Left()`.

- [ ] **Step 1: Write the failing tests**

Create `rules/game_test.go`:

```go
package rules

import (
	"errors"
	"testing"
)

func play(t *testing.T, g *Game, ucis ...string) {
	t.Helper()
	for _, uci := range ucis {
		if _, err := g.Play(mv(t, uci), dice(1)); err != nil {
			t.Fatalf("%s: %v", uci, err)
		}
	}
}

func TestFoolsMate(t *testing.T) {
	g := NewGame()
	play(t, g, "f2f3", "e7e5", "g2g4", "d8h4")
	if want := (Result{Over: true, Winner: Black, Reason: Checkmate}); g.Result != want {
		t.Errorf("result %+v, want %+v", g.Result, want)
	}
	if _, err := g.Play(mv(t, "a2a3"), dice(1)); !errors.Is(err, ErrGameOver) {
		t.Errorf("got %v, want ErrGameOver", err)
	}
}

func TestMamdaniCanBlockMate(t *testing.T) {
	const fen = "7k/8/8/8/8/8/6PP/r6K w - - 0 1" // back-rank check from a1
	if g := NewGameFrom(setup(t, fen, D4, NoSquare, NoSquare)); g.Result.Over {
		t.Error("Md1 blocks the check, so it is not mate")
	}
	g := NewGameFrom(setup(t, fen, NoSquare, NoSquare, NoSquare))
	if g.Result.Reason != Checkmate || g.Result.Winner != Black {
		t.Errorf("without the Mamdani it is mate: %+v", g.Result)
	}
}

func TestStalemateCountsMamdaniMoves(t *testing.T) {
	const fen = "k7/8/1Q6/8/8/8/8/7K b - - 0 1"
	if g := NewGameFrom(setup(t, fen, H4, NoSquare, NoSquare)); g.Result.Over {
		t.Error("Black can still move the Mamdani, so it is not stalemate")
	}
	g := NewGameFrom(setup(t, fen, NoSquare, NoSquare, NoSquare))
	if g.Result.Reason != Stalemate || !g.Result.Draw {
		t.Errorf("want stalemate, got %+v", g.Result)
	}
}

func TestThreefoldRepetition(t *testing.T) {
	g := NewGame()
	play(t, g, "g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1")
	if g.Result.Over {
		t.Fatal("only two repetitions so far")
	}
	play(t, g, "f6g8")
	if g.Result.Reason != Repetition {
		t.Errorf("want repetition, got %+v", g.Result)
	}
}

func TestFiftyMoveRule(t *testing.T) {
	p := StartPosition()
	p.Halfmove = 99
	g := NewGameFrom(p)
	play(t, g, "a5b5") // a Mamdani move counts toward the 50
	if g.Result.Reason != FiftyMoves {
		t.Errorf("want fifty-move draw, got %+v", g.Result)
	}
}

func TestInsufficientMaterial(t *testing.T) {
	cases := []struct {
		fen     string
		mamdani Square
		over    bool
	}{
		{"4k3/8/8/8/8/8/8/4K3 w - - 0 1", A5, true},
		{"4k3/8/8/8/8/8/8/3NK3 w - - 0 1", NoSquare, true},
		{"4k3/8/8/8/8/8/8/3NK3 w - - 0 1", A5, false}, // the Mamdani can help a knight mate
		{"4k3/8/8/8/8/8/8/3RK3 w - - 0 1", NoSquare, false},
	}
	for _, c := range cases {
		g := NewGameFrom(setup(t, c.fen, c.mamdani, NoSquare, NoSquare))
		if over := g.Result.Reason == InsufficientMaterial; over != c.over {
			t.Errorf("%s mamdani %v: insufficient=%v, want %v", c.fen, c.mamdani, over, c.over)
		}
	}
}

func TestReplay(t *testing.T) {
	g := NewGame()
	for _, step := range []struct {
		uci   string
		rolls []int
	}{
		{"e2e4", []int{2, 4, 4}},
		{"e7e5", []int{2, 4, 2, 6}},
		{"a5b5", []int{1}},
	} {
		if _, err := g.Play(mv(t, step.uci), dice(step.rolls...)); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Replay(StartPosition(), g.Turns)
	if err != nil {
		t.Fatal(err)
	}
	if r.Pos != g.Pos || r.Result != g.Result {
		t.Error("replay did not rebuild the same game")
	}
	short := []Turn{{Move: g.Turns[0].Move, Dice: []int{2}}}
	if _, err := Replay(StartPosition(), short); err == nil {
		t.Error("replay with missing dice should fail")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./rules/`
Expected: FAIL to build, with errors like `undefined: NewGame`.

- [ ] **Step 3: Write the game layer**

Replace `rules/game.go` entirely:

```go
package rules

import (
	"errors"
	"fmt"
)

// Reason says why a game ended.
type Reason string

const (
	Checkmate            Reason = "checkmate"
	Stalemate            Reason = "stalemate"
	FiftyMoves           Reason = "fifty_moves"
	Repetition           Reason = "repetition"
	InsufficientMaterial Reason = "insufficient_material"
)

// Result is a game's outcome. The zero value means the game is still on.
type Result struct {
	Over   bool
	Draw   bool
	Winner Color // set when Over && !Draw
	Reason Reason
}

// Turn is one recorded turn: the move and every d8 rolled, in order.
// Replaying turns through Replay rebuilds the game exactly.
type Turn struct {
	Move Move
	Dice []int
}

// Game is a position plus the history needed for repetition and replay.
type Game struct {
	Pos    Position
	Turns  []Turn
	Result Result
	seen   map[Key]int
}

var ErrGameOver = errors.New("game is over")

// NewGame starts from StartPosition.
func NewGame() *Game { return NewGameFrom(StartPosition()) }

// NewGameFrom starts from any position (tests, replays).
func NewGameFrom(p Position) *Game {
	g := &Game{Pos: p, seen: map[Key]int{p.Key(): 1}}
	g.Result = g.status()
	return g
}

// Play applies one turn and updates the result.
func (g *Game) Play(m Move, dice Dice) ([]Event, error) {
	if g.Result.Over {
		return nil, ErrGameOver
	}
	rec := &recordingDice{dice: dice}
	next, ev, err := Apply(g.Pos, m, rec)
	if err != nil {
		return nil, err
	}
	g.Pos = next
	g.Turns = append(g.Turns, Turn{Move: m, Dice: rec.rolls})
	g.seen[next.Key()]++
	g.Result = g.status()
	return ev, nil
}

func (g *Game) status() Result {
	p := &g.Pos
	if len(p.LegalMoves()) == 0 {
		if p.InCheck(p.Turn) {
			return Result{Over: true, Winner: p.Turn.Other(), Reason: Checkmate}
		}
		return Result{Over: true, Draw: true, Reason: Stalemate}
	}
	if p.Halfmove >= 100 {
		return Result{Over: true, Draw: true, Reason: FiftyMoves}
	}
	if g.seen[p.Key()] >= 3 {
		return Result{Over: true, Draw: true, Reason: Repetition}
	}
	if p.CannotMate(White) && p.CannotMate(Black) {
		return Result{Over: true, Draw: true, Reason: InsufficientMaterial}
	}
	return Result{}
}

// CannotMate reports whether c can never deliver checkmate: a bare king,
// or king and one knight or bishop once the Mamdani has fallen. (While the
// Mamdani is on the board it can block escape squares, so a minor piece
// might mate.) The game server also uses this for timeouts.
func (p *Position) CannotMate(c Color) bool {
	minors := 0
	for _, pc := range p.Board {
		if pc == NoPiece || pc.Color() != c {
			continue
		}
		switch pc.Kind() {
		case King:
		case Knight, Bishop:
			minors++
		default:
			return false
		}
	}
	return minors == 0 || (minors == 1 && p.Mamdani == NoSquare)
}

// Replay rebuilds a game from start by playing turns with their recorded
// dice. It fails if a move is illegal or the dice don't match exactly.
func Replay(start Position, turns []Turn) (*Game, error) {
	g := NewGameFrom(start)
	for i, t := range turns {
		if err := g.replayTurn(t); err != nil {
			return nil, fmt.Errorf("turn %d (%s): %w", i+1, t.Move, err)
		}
	}
	return g, nil
}

func (g *Game) replayTurn(t Turn) (err error) {
	script := &ScriptedDice{Rolls: t.Dice}
	defer func() {
		if r := recover(); r != nil { // the script ran out
			err = fmt.Errorf("%v", r)
		}
	}()
	if _, err := g.Play(t.Move, script); err != nil {
		return err
	}
	if script.Left() != 0 {
		return fmt.Errorf("%d unused dice", script.Left())
	}
	return nil
}

// ScriptedDice returns preset rolls in order. It panics when it runs out:
// in tests that means the script is too short.
type ScriptedDice struct {
	Rolls []int
	next  int
}

func (s *ScriptedDice) D8() int {
	if s.next >= len(s.Rolls) {
		panic("scripted dice ran out")
	}
	r := s.Rolls[s.next]
	s.next++
	return r
}

// Left returns how many rolls have not been used.
func (s *ScriptedDice) Left() int { return len(s.Rolls) - s.next }

type recordingDice struct {
	dice  Dice
	rolls []int
}

func (r *recordingDice) D8() int {
	v := r.dice.D8()
	r.rolls = append(r.rolls, v)
	return v
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./rules/ && go test ./rules/`
Expected: `ok  	mamdani-chess/rules`

- [ ] **Step 5: Commit**

```bash
git add rules/game.go rules/game_test.go
git commit -m "rules: game results, repetition and replay from the turn log"
```

---

### Task 5: Move notation for the log

**Files:**
- Create: `rules/san.go`
- Test: `rules/san_test.go`

**Interfaces:**
- Consumes: `LegalMoves`, `play`, `InCheck`.
- Produces: `(*Position).SAN(m Move) string`. It is called on the position *before* the move, and `m` must be legal there. A Mamdani move is `"M" + square` (`"Mb5"`). `+` marks check before the pothole roll. There is no `#`, because the dice resolve after the notation is written.

- [ ] **Step 1: Write the failing test**

Create `rules/san_test.go`:

```go
package rules

import "testing"

func TestSAN(t *testing.T) {
	cases := []struct {
		fen     string
		mamdani Square
		uci     string
		want    string
	}{
		{startFEN, A5, "e2e4", "e4"},
		{startFEN, A5, "g1f3", "Nf3"},
		{startFEN, A5, "a5b5", "Mb5"},
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", NoSquare, "e1g1", "O-O"},
		{"r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1", NoSquare, "e1c1", "O-O-O"},
		{"4k3/P7/8/8/8/8/8/4K3 w - - 0 1", NoSquare, "a7a8q", "a8=Q+"},
		{"4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", NoSquare, "e4d5", "exd5"},
		{"4k3/8/8/8/8/8/8/R4RK1 w - - 0 1", NoSquare, "a1d1", "Rad1"},
		{"4k3/8/8/8/8/R7/8/R3K3 w - - 0 1", NoSquare, "a1a2", "R1a2"},
		{"4k3/8/8/8/8/8/8/4K2Q w - - 0 1", NoSquare, "h1h5", "Qh5+"},
	}
	for _, c := range cases {
		p := setup(t, c.fen, c.mamdani, NoSquare, NoSquare)
		if got := p.SAN(mv(t, c.uci)); got != c.want {
			t.Errorf("%s %s: got %q, want %q", c.fen, c.uci, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./rules/`
Expected: FAIL to build, with `p.SAN undefined`.

- [ ] **Step 3: Write SAN**

Create `rules/san.go`:

```go
package rules

import "strings"

var sanLetters = map[Kind]string{Knight: "N", Bishop: "B", Rook: "R", Queen: "Q", King: "K"}

// SAN returns m in standard algebraic notation for the log: "e4", "Nbd2",
// "exd5", "e8=Q", "O-O", "Qh5+". A Mamdani move is "M" plus its square:
// "Mb5". The check mark reflects the position before the pothole roll.
// m must be legal in p.
func (p *Position) SAN(m Move) string {
	var b strings.Builder
	pc := p.Board[m.From]
	switch {
	case m.From == p.Mamdani:
		b.WriteString("M" + m.To.String())
	case pc.Kind() == King && m.To.File()-m.From.File() == 2:
		b.WriteString("O-O")
	case pc.Kind() == King && m.From.File()-m.To.File() == 2:
		b.WriteString("O-O-O")
	default:
		capture := p.Board[m.To] != NoPiece || (pc.Kind() == Pawn && m.From.File() != m.To.File())
		if pc.Kind() == Pawn {
			if capture {
				b.WriteByte(byte('a' + m.From.File()))
			}
		} else {
			b.WriteString(sanLetters[pc.Kind()])
			b.WriteString(p.disambiguate(m))
		}
		if capture {
			b.WriteByte('x')
		}
		b.WriteString(m.To.String())
		if m.Promo != NoKind {
			b.WriteString("=" + sanLetters[m.Promo])
		}
	}
	q := *p
	q.play(m, nil)
	if q.InCheck(q.Turn) {
		b.WriteByte('+')
	}
	return b.String()
}

// disambiguate returns the file, rank or square needed to tell m apart from
// other legal moves of the same piece kind to the same square.
func (p *Position) disambiguate(m Move) string {
	kind := p.Board[m.From].Kind()
	var sameFile, sameRank, other bool
	for _, o := range p.LegalMoves() {
		if o.To != m.To || o.From == m.From || o.From == p.Mamdani || p.Board[o.From].Kind() != kind {
			continue
		}
		other = true
		sameFile = sameFile || o.From.File() == m.From.File()
		sameRank = sameRank || o.From.Rank() == m.From.Rank()
	}
	switch {
	case !other:
		return ""
	case !sameFile:
		return m.From.String()[:1]
	case !sameRank:
		return m.From.String()[1:]
	}
	return m.From.String()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go vet ./rules/ && go test ./rules/`
Expected: `ok  	mamdani-chess/rules`

- [ ] **Step 5: Commit**

```bash
git add rules/san.go rules/san_test.go
git commit -m "rules: standard algebraic notation, with M for the Mamdani"
```

---

### Task 6: Random-game invariants

This test plays 10,000 seeded random games with real-looking dice, checks every promise in RULES.md after every turn, and replays each finished game from its log. It runs in about 20 s on 8 cores (`-short` runs 500 games in about 2 s). In a sample of 1,000 games, every event kind and every re-roll reason occurred: about 260 checkmate re-rolls, 110 exposes re-rolls and 690 Mamdani falls.

**Files:**
- Test: `rules/random_test.go`

**Interfaces:**
- Consumes: `NewGame`, `Play`, `Replay`, `StartPosition`, `play`, `InCheck`, `LegalMoves`, `King`.
- Produces: nothing new.

- [ ] **Step 1: Write the test**

Create `rules/random_test.go`:

```go
package rules

import (
	"fmt"
	"math/rand/v2"
	"testing"
)

type randDice struct{ r *rand.Rand }

func (d randDice) D8() int { return d.r.IntN(8) + 1 }

// TestRandomGames plays thousands of seeded random games and checks the
// invariants RULES.md promises after every turn.
func TestRandomGames(t *testing.T) {
	games := 10000
	if testing.Short() {
		games = 500
	}
	const batch = 500
	for first := 0; first < games; first += batch {
		t.Run(fmt.Sprintf("seeds %d-%d", first, first+batch-1), func(t *testing.T) {
			t.Parallel()
			for seed := first; seed < first+batch; seed++ {
				playRandomGame(t, seed)
			}
		})
	}
}

func playRandomGame(t *testing.T, seed int) {
	r := rand.New(rand.NewPCG(uint64(seed), 1))
	d := randDice{r}
	g := NewGame()
	for ply := 0; ply < 300 && !g.Result.Over; ply++ {
		moves := g.Pos.LegalMoves()
		m := moves[r.IntN(len(moves))]
		before := g.Pos
		if _, err := g.Play(m, d); err != nil {
			t.Fatalf("seed %d ply %d %s: %v", seed, ply, m, err)
		}
		checkInvariants(t, seed, ply, before, m, g)
	}
	replayed, err := Replay(StartPosition(), g.Turns)
	if err != nil {
		t.Fatalf("seed %d: replay: %v", seed, err)
	}
	if replayed.Pos != g.Pos || replayed.Result != g.Result {
		t.Fatalf("seed %d: replay differs", seed)
	}
}

func checkInvariants(t *testing.T, seed, ply int, before Position, m Move, g *Game) {
	t.Helper()
	p := &g.Pos
	fail := func(format string, args ...any) {
		t.Helper()
		t.Fatalf("seed %d ply %d after %s: "+format, append([]any{seed, ply, m}, args...)...)
	}
	if p.King(White) == NoSquare || p.King(Black) == NoSquare {
		fail("a king fell")
	}
	for _, s := range p.Potholes {
		if s != NoSquare && (p.Board[s] != NoPiece || s == p.Mamdani) {
			fail("something stands on the pothole at %v", s)
		}
	}
	if p.Mamdani != NoSquare && p.Board[p.Mamdani] != NoPiece {
		fail("the Mamdani shares %v with a piece", p.Mamdani)
	}
	if before.Mamdani == NoSquare && p.Mamdani != NoSquare {
		fail("the Mamdani came back")
	}
	if p.InCheck(p.Turn.Other()) {
		fail("the player who just moved is in check")
	}
	if g.Result.Reason == Checkmate {
		// A roll never wins: the move alone must already have been mate.
		q := before
		q.play(m, nil)
		if !q.InCheck(q.Turn) || len(q.LegalMoves()) != 0 {
			fail("the dice delivered checkmate")
		}
	}
}
```

- [ ] **Step 2: Run it**

Run: `go test -count=1 ./rules/`
Expected: `ok  	mamdani-chess/rules` in roughly 20 s. If an invariant fails, the message names the seed and ply. Reproduce it with `go test -run 'TestRandomGames' ./rules/`, then fix the engine, not the invariant.

- [ ] **Step 3: Prove the checkmate invariant has teeth**

Temporarily change `return ReasonCheckmate` in `rules/turn.go` to `return ""`, then run `go test -count=1 -short ./rules/`.
Expected: FAIL, including `the dice delivered checkmate` from `TestRandomGames` and a failure from `TestRerollIfResultWouldCheckmate`. Revert the change with `git checkout rules/turn.go` and re-run to see `ok`.

- [ ] **Step 4: Check coverage**

Run: `go test -short -cover ./rules/`
Expected: coverage at or above 95%.

- [ ] **Step 5: Commit**

```bash
git add rules/random_test.go
git commit -m "rules: random-game invariants and replay over 10,000 seeded games"
```

---

## Done when

- `go vet ./...` is clean, and `go test ./rules/` passes, including perft and 10,000 random games.
- `rules` imports nothing outside the standard library and has no `time` or `crypto/rand`.
- The game-server plan can be written against the Task 4 interfaces without reading `rules` internals.

## Later: speed (2026-10-04)

The engine moved to bitboards plus the mailbox, with allocation-free legality checks ([plan](2026-10-04-go-rules-speed.md)). No rule changed: the tests above still pass, and the Rust engine in `server_rs/` agrees on 300 recorded games.

Measured with `go test -bench . -benchmem ./rules` (benchstat, 6 runs each, Apple M4 Pro):
- one random game: 2.15 ms and 35,700 allocations → 0.45 ms and 2,000 (−79% time, −94% allocations);
- perft from the start to depth 4: 15.2 ms → 6.8 ms (−56%);
- `LegalMoves` at the start: 1.59 µs and 36 allocations → 0.91 µs and 1;
- the 10,000-game test: about 20 s → about 3.6 s.

`LegalMoves()` order changed; nothing depends on it.
