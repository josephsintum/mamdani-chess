# Playable Game (Rough) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Two people in two browsers can play a full game of Pothole Chess: Mamdani Edition through a shared link. The board is plain and the dice results appear as text.

**Architecture:**
- **`game` package:** each live game is one goroutine that owns a `rules.Game`, the two seats and the open streams, as spec §2 describes. Callers hand the goroutine a function to run and wait for it, so no command types are needed.
- **Hub:** a `game.Hub` maps 6-character codes to games, all in memory.
- **HTTP:** `server` gains a guest cookie and three routes: create a game, stream its state, and post a move.
- **Frontend:** the SvelteKit app gains a home page that creates a game and a game page with a tap-to-move board.
- **Honk demo:** deleted.

**Tech Stack:** Go 1.27.1 standard library, plus the merged `rules` package. SvelteKit 3, Svelte 5 runes, TypeScript 6.

**Spec:** `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md` §1 (Game screen, friend game), §2 (packages, concurrency model, API, SSE `state`), §4 (frontend). Rules: `RULES.md`. Roadmap: `docs/superpowers/plans/2026-10-03-00-roadmap.md`, milestone 03.

## Global Constraints

- **Playable first; no scale work.**
- **Deferred to later milestones, so not in this plan:**
  - clocks and timeouts;
  - persistence (games live in memory and vanish on restart);
  - guest names, quick match and join-by-code;
  - reactions and rematch;
  - dice animation, and showing which lines a pothole blocks;
  - Railway deploy, which moves to milestone 07.
- **Concurrency:** one goroutine per game owns all of its state (spec §2). Nothing outside that goroutine reads or writes a game's `rules.Game`, seats or subscribers.
- **Seats:** the creator plays White, the first other guest to open the game takes Black, and everyone else watches. A guest who reconnects keeps their seat.
- **Wire format:** the SSE event is `state`, and its payload is the whole `game.View`, so a reconnecting browser needs nothing else.
  - Moves are `POST {from, to, promo?, seq}`, where `seq` is the number of turns played. A stale `seq` is rejected.
  - Responses: `204` on success, `403` for a non-player, `409` for illegal, out-of-turn, stale or game-over moves, `400` for a bad body, and `404` for an unknown game.
  - Bodies are capped at 4 KB (spec §2).
- **Game codes:** 6 characters from `ABCDEFGHJKMNPQRSTUVWXYZ23456789`, which has no `0 O 1 I L` (spec §1). Dice use `crypto/rand` (spec §2).
- **Frontend rules:**
  - Svelte 5 runes on SvelteKit 3. `#lib/…` imports need the file extension (`#lib/game.ts`, `#lib/Board.svelte`), and there is no `$lib`.
  - Colors come only from CSS variables in `web/src/lib/theme/tokens.css`. Touch targets are at least 44 px.
  - Run `npx @sveltejs/mcp svelte-autofixer <file>` on every `.svelte` file. It must report no issues and no suggestions.
- **Migrations are append-only.** The honk table is dropped by a new migration 2; migration 1 is not edited.
- **Git:** stage explicit paths, never `git add -A`. No Claude attribution in commits.

## Review Focus

1. **A move from an out-of-date board.** If two tabs of the same player both try to move, or a move crosses with an incoming update, the second must be rejected as stale, never applied to a position the player didn't see. Pinned by `TestMoveErrors` (Task 2, `ErrStale`) and the `seq` 409 case in the server `TestMoveErrors` (Task 3).
2. **Someone who isn't seated tries to move.** That covers a spectator, a third visitor, or a second guest racing for Black. Only the two seated guests may move, and only on their turn. Pinned by `TestSeats` and `TestMoveErrors` (Task 2) and the `403` case (Task 3).
3. **A slow or frozen tab.** One stream that stops reading must never block the game for the other player. `Sub.C` holds only the newest view, and `send` replaces it without blocking. Pinned by `TestLeaveStopsUpdates`, and by how `send` and the stream loop are written (Task 2).
4. **Reconnecting mid-game.** A refreshed or reconnected tab must get the whole current position, its seat and its legal moves immediately. Pinned by `TestSeats` (the reconnect case) and by `View` being the whole game (Task 1).
5. **Promotion and the Mamdani in the UI.** A pawn reaching the last rank must offer the four pieces. Tapping the Mamdani must show its moves, because Mamdani moves start from its square. Pinned by the manual two-browser check in Task 5; the server side is covered by `rules` tests.

## File Structure

| File | Responsibility |
| --- | --- |
| `game/view.go` | `View` and its JSON shapes, mapping from `rules` types, `ParseMove`, one-line `describe` for the log |
| `game/dice.go` | `NewCode` (share codes) and `CryptoDice` |
| `game/game.go` | `Game`: the per-game goroutine, seats, `Join`, `Leave`, `Move`, broadcasting views |
| `game/hub.go` | `Hub`: create and look up games by code |
| `server/guest.go` | Guest ID cookie |
| `server/games.go` | `POST /api/games`, `GET /api/games/{code}/stream`, `POST /api/games/{code}/move` |
| `server/server.go` | Routes; `New` now takes the hub |
| `server/sse.go` | SSE helpers (the honk broadcaster is removed) |
| `store/migrate.go` | Migration 2 drops the honk table |
| `cmd/server/main.go` | Builds the hub with `CryptoDice` |
| `web/src/lib/game.ts` | TypeScript mirror of `View`, API calls, plain-English event text |
| `web/src/lib/Board.svelte` | Tap-to-move board: legal-move dots, potholes, Mamdani, promotion picker, flipped for Black |
| `web/src/routes/+page.svelte` | Home: "Play a friend" |
| `web/src/routes/game/[code]/+page.svelte` | Game page: status, share link, board, last turn, move log |

---

### Task 1: Wire format, game codes and dice

**Files:**
- Create: `game/view.go`, `game/dice.go`
- Test: `game/view_test.go`, `game/dice_test.go`

**Interfaces:**
- Consumes: `rules` (merged): `Piece`, `Kind`, `Color`, `Square`, `Move`, `Event`, `EventKind`, `RerollReason`, `Reason`, `ParseSquare`.
- Produces:
  - `type Status string` (`Waiting`, `Playing`, `Over`)
  - `type View struct{…}`, `Pothole`, `MoveJSON`, `EventJSON`, `ResultJSON`, with JSON field names as in the code below. The frontend mirrors them.
  - `ParseMove(MoveJSON) (rules.Move, bool)`
  - `NewCode() string`, `type CryptoDice struct{}` (implements `rules.Dice`)
  - unexported, used by Task 2: `pieceCode`, `colorName`, `squareName`, `moveJSON`, `eventJSON`, `describe(san string, events []rules.Event) string`

- [ ] **Step 1: Write the failing tests**

Create `game/view_test.go`:

```go
package game

import (
	"encoding/json"
	"testing"

	"mamdani-chess/rules"
)

func TestEventJSONOmitsFieldsThatDontApply(t *testing.T) {
	cases := []struct {
		e    rules.Event
		want string
	}{
		{rules.Event{Kind: rules.RolledPothole, Roll: 3, Color: rules.Black},
			`{"kind":"rolled_pothole","color":"black","roll":3}`},
		{rules.Event{Kind: rules.Target, Square: rules.A1},
			`{"kind":"target","sq":"a1"}`},
		{rules.Event{Kind: rules.SavingRoll, Square: rules.D2, Piece: rules.NewPiece(rules.White, rules.Pawn), Roll: 6, Saved: false, Color: rules.White},
			`{"kind":"saving_roll","sq":"d2","piece":"wP","color":"white","roll":6,"saved":false}`},
		{rules.Event{Kind: rules.Fell, Square: rules.A5, Piece: rules.MamdaniPiece},
			`{"kind":"fell","sq":"a5","piece":"M"}`},
		{rules.Event{Kind: rules.Moved, Move: rules.Move{From: rules.E7, To: rules.E8, Promo: rules.Queen}, Piece: rules.NewPiece(rules.White, rules.Pawn), Color: rules.White},
			`{"kind":"moved","from":"e7","to":"e8","promo":"q","piece":"wP","color":"white"}`},
		{rules.Event{Kind: rules.NoPothole}, `{"kind":"no_pothole"}`},
	}
	for _, c := range cases {
		b, err := json.Marshal(eventJSON(c.e))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != c.want {
			t.Errorf("got  %s\nwant %s", b, c.want)
		}
	}
}

func TestParseMove(t *testing.T) {
	m, ok := ParseMove(MoveJSON{From: "e7", To: "e8", Promo: "n"})
	if !ok || m != (rules.Move{From: rules.E7, To: rules.E8, Promo: rules.Knight}) {
		t.Errorf("got %v %v", m, ok)
	}
	for _, bad := range []MoveJSON{{From: "e9", To: "e8"}, {From: "e7", To: ""}, {From: "e7", To: "e8", Promo: "k"}} {
		if _, ok := ParseMove(bad); ok {
			t.Errorf("%+v should not parse", bad)
		}
	}
}

func TestDescribe(t *testing.T) {
	ev := []rules.Event{
		{Kind: rules.Moved},
		{Kind: rules.RolledPothole, Roll: 4},
		{Kind: rules.Target, Square: rules.E1},
		{Kind: rules.Reroll, Square: rules.E1, Reason: rules.ReasonKing},
		{Kind: rules.Target, Square: rules.D2},
		{Kind: rules.SavingRoll, Roll: 5, Saved: true},
	}
	if got, want := describe("e4", ev), "e4 · d8 4 → e1 (re-roll: king) → d2 · save 5 ✓"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	ev = []rules.Event{
		{Kind: rules.Moved},
		{Kind: rules.Repaired, Square: rules.D4},
		{Kind: rules.RolledPothole, Roll: 2},
		{Kind: rules.Target, Square: rules.G8},
		{Kind: rules.Fell, Piece: rules.NewPiece(rules.Black, rules.Knight)},
	}
	if got, want := describe("Mc5", ev), "Mc5 · repairs d4 · d8 2 → g8 · bN falls"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
```

Create `game/dice_test.go`:

```go
package game

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	for range 1000 {
		c := NewCode()
		if len(c) != 6 || strings.ContainsAny(c, "0O1IL") {
			t.Fatalf("bad code %q", c)
		}
	}
}

func TestCryptoDice(t *testing.T) {
	seen := map[int]bool{}
	for range 2000 {
		r := CryptoDice{}.D8()
		if r < 1 || r > 8 {
			t.Fatalf("roll %d", r)
		}
		seen[r] = true
	}
	if len(seen) != 8 {
		t.Errorf("only saw %v in 2000 rolls", seen)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./game/`
Expected: FAIL to build, with `undefined: eventJSON`, `undefined: ParseMove`, `undefined: NewCode` and similar.

- [ ] **Step 3: Write the wire format**

Create `game/view.go`:

```go
package game

import (
	"strconv"
	"strings"

	"mamdani-chess/rules"
)

// Status is where a game is in its life.
type Status string

const (
	Waiting Status = "waiting" // Black's seat is still empty
	Playing Status = "playing"
	Over    Status = "over"
)

// View is one subscriber's picture of the game, sent as the SSE "state"
// event after every change. It is the whole game, so a reconnecting browser
// needs nothing else.
type View struct {
	Code     string      `json:"code"`
	Status   Status      `json:"status"`
	You      string      `json:"you"`     // "white", "black" or "spectator"
	Board    [64]string  `json:"board"`   // index 0 = a1; "" or color+kind: "wP", "bQ"
	Mamdani  string      `json:"mamdani"` // its square, "" once it has fallen
	Potholes []Pothole   `json:"potholes"`
	Turn     string      `json:"turn"` // "white" or "black"
	Check    bool        `json:"check"`
	Legal    []MoveJSON  `json:"legal"` // only for the player to move
	Last     []EventJSON `json:"last"`  // what happened on the latest turn, in order
	Log      []string    `json:"log"`   // one line per turn
	Result   *ResultJSON `json:"result"`
	Seq      int         `json:"seq"` // turns played; a move must quote it
}

// Pothole is an open pothole and the color that rolled it.
type Pothole struct {
	Sq string `json:"sq"`
	By string `json:"by"`
}

// MoveJSON is a move on the wire. Promo is "", "q", "r", "b" or "n".
type MoveJSON struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Promo string `json:"promo,omitempty"`
}

// EventJSON is one step of a turn. Only the fields that apply to Kind are set.
type EventJSON struct {
	Kind   rules.EventKind    `json:"kind"`
	Sq     string             `json:"sq,omitempty"`
	From   string             `json:"from,omitempty"`
	To     string             `json:"to,omitempty"`
	Promo  string             `json:"promo,omitempty"`
	Piece  string             `json:"piece,omitempty"`
	Color  string             `json:"color,omitempty"`
	Roll   int                `json:"roll,omitempty"`
	Saved  *bool              `json:"saved,omitempty"`
	Reason rules.RerollReason `json:"reason,omitempty"`
}

// ResultJSON is how the game ended.
type ResultJSON struct {
	Winner string       `json:"winner,omitempty"` // "" for a draw
	Draw   bool         `json:"draw"`
	Reason rules.Reason `json:"reason"`
}

var kindLetters = map[rules.Kind]string{
	rules.Pawn: "P", rules.Knight: "N", rules.Bishop: "B",
	rules.Rook: "R", rules.Queen: "Q", rules.King: "K",
}

// pieceCode returns "wP", "bK" and so on, "M" for the Mamdani, "" for none.
func pieceCode(p rules.Piece) string {
	switch {
	case p == rules.NoPiece:
		return ""
	case p == rules.MamdaniPiece:
		return "M"
	}
	return colorName(p.Color())[:1] + kindLetters[p.Kind()]
}

func colorName(c rules.Color) string { return c.String() }

func squareName(s rules.Square) string {
	if s == rules.NoSquare {
		return ""
	}
	return s.String()
}

var promoLetters = map[rules.Kind]string{
	rules.Queen: "q", rules.Rook: "r", rules.Bishop: "b", rules.Knight: "n",
}

func moveJSON(m rules.Move) MoveJSON {
	return MoveJSON{From: m.From.String(), To: m.To.String(), Promo: promoLetters[m.Promo]}
}

// ParseMove turns a wire move back into a rules.Move.
func ParseMove(m MoveJSON) (rules.Move, bool) {
	from, err1 := rules.ParseSquare(m.From)
	to, err2 := rules.ParseSquare(m.To)
	if err1 != nil || err2 != nil {
		return rules.Move{}, false
	}
	mv := rules.Move{From: from, To: to}
	if m.Promo != "" {
		for k, l := range promoLetters {
			if l == m.Promo {
				mv.Promo = k
			}
		}
		if mv.Promo == rules.NoKind {
			return rules.Move{}, false
		}
	}
	return mv, true
}

func eventJSON(e rules.Event) EventJSON {
	j := EventJSON{Kind: e.Kind}
	switch e.Kind {
	case rules.Moved:
		mv := moveJSON(e.Move)
		j.From, j.To, j.Promo = mv.From, mv.To, mv.Promo
		j.Piece, j.Color = pieceCode(e.Piece), colorName(e.Color)
	case rules.Captured, rules.Fell:
		j.Sq, j.Piece = squareName(e.Square), pieceCode(e.Piece)
	case rules.PotholeClosed, rules.Repaired, rules.Target:
		j.Sq = squareName(e.Square)
	case rules.RolledPothole:
		j.Roll, j.Color = e.Roll, colorName(e.Color)
	case rules.Reroll:
		j.Sq, j.Reason = squareName(e.Square), e.Reason
	case rules.SavingRoll:
		saved := e.Saved
		j.Sq, j.Piece, j.Roll, j.Saved, j.Color = squareName(e.Square), pieceCode(e.Piece), e.Roll, &saved, colorName(e.Color)
	case rules.PotholeOpened:
		j.Sq, j.Color = squareName(e.Square), colorName(e.Color)
	}
	return j
}

// describe writes one log line for a turn, e.g. "e4 · d8 4 → d3 · save 5 ✓".
func describe(san string, events []rules.Event) string {
	parts := []string{san}
	rolled := false
	for _, e := range events {
		last := len(parts) - 1
		switch e.Kind {
		case rules.Repaired:
			if rolled {
				parts[last] += " repaired"
			} else {
				parts = append(parts, "repairs "+e.Square.String())
			}
		case rules.RolledPothole:
			rolled = true
			parts = append(parts, "d8 "+strconv.Itoa(e.Roll))
		case rules.Target:
			parts[last] += " → " + e.Square.String()
		case rules.Reroll:
			parts[last] += " (re-roll: " + string(e.Reason) + ")"
		case rules.SavingRoll:
			mark := "✗"
			if e.Saved {
				mark = "✓"
			}
			parts = append(parts, "save "+strconv.Itoa(e.Roll)+" "+mark)
		case rules.Fell:
			parts = append(parts, pieceCode(e.Piece)+" falls")
		case rules.NoPothole:
			parts = append(parts, "no pothole")
		}
	}
	return strings.Join(parts, " · ")
}
```

- [ ] **Step 4: Write codes and dice**

Create `game/dice.go`:

```go
package game

import (
	"crypto/rand"
	"math/big"
)

// codeAlphabet has no lookalikes: no 0, O, 1, I or L.
const codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// NewCode returns a random 6-character game code such as "K7F3QZ".
func NewCode() string {
	b := make([]byte, 6)
	for i := range b {
		b[i] = codeAlphabet[randN(len(codeAlphabet))]
	}
	return string(b)
}

// CryptoDice rolls fair d8s from crypto/rand.
type CryptoDice struct{}

func (CryptoDice) D8() int { return randN(8) + 1 }

func randN(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return int(v.Int64())
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go vet ./game/ && go test ./game/`
Expected: `ok  	mamdani-chess/game`

- [ ] **Step 6: Commit**

```bash
git add game/view.go game/dice.go game/view_test.go game/dice_test.go
git commit -m "game: wire format for game state, share codes and crypto dice"
```

---

### Task 2: The game goroutine and hub

**Files:**
- Create: `game/game.go`, `game/hub.go`
- Test: `game/game_test.go`

**Interfaces:**
- Consumes: Task 1's types and helpers; `rules.NewGame`, `(*rules.Game).Play`, `(*rules.Position).LegalMoves`, `SAN`, `InCheck`.
- Produces (Task 3 builds on these):
  - `NewHub(dice rules.Dice) *Hub`, `(*Hub).Create(creator string) *Game`, `(*Hub).Get(code string) (*Game, bool)`
  - `(*Game).Code() string`
  - `(*Game).Join(guest string) *Sub`: the current view is already waiting on `sub.C`.
  - `(*Game).Leave(*Sub)`
  - `(*Game).Move(guest string, m rules.Move, seq int) error`
  - `type Sub struct { C <-chan *View; … }`
  - Errors: `ErrNotPlayer`, `ErrWaiting`, `ErrNotYourTurn`, `ErrStale`, `ErrGameOver` (= `rules.ErrGameOver`), `ErrIllegalMove` (= `rules.ErrIllegalMove`). `Move` can also return `rules.ErrBadDie`.

- [ ] **Step 1: Write the failing tests**

Create `game/game_test.go`:

```go
package game

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"mamdani-chess/rules"
)

// odd always rolls 1: no potholes, so moves are plain chess.
type odd struct{}

func (odd) D8() int { return 1 }

// script rolls the given values in order, then 1 forever.
type script struct{ rolls []int }

func (s *script) D8() int {
	if len(s.rolls) == 0 {
		return 1
	}
	r := s.rolls[0]
	s.rolls = s.rolls[1:]
	return r
}

func recv(t *testing.T, sub *Sub) *View {
	t.Helper()
	select {
	case v := <-sub.C:
		return v
	case <-time.After(time.Second):
		t.Fatal("no view within 1s")
		return nil
	}
}

func mv(t *testing.T, uci string) rules.Move {
	t.Helper()
	m, err := rules.ParseMove(uci)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestSeats(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := g.Join("alice")
	if v := recv(t, a); v.You != "white" || v.Status != Waiting || len(v.Legal) != 0 {
		t.Fatalf("creator: you=%s status=%s legal=%d", v.You, v.Status, len(v.Legal))
	}
	b := g.Join("bob")
	if v := recv(t, b); v.You != "black" || v.Status != Playing || len(v.Legal) != 0 {
		t.Fatalf("second guest: you=%s status=%s legal=%d", v.You, v.Status, len(v.Legal))
	}
	if v := recv(t, a); v.Status != Playing || len(v.Legal) != 33 {
		t.Fatalf("white after black joins: status=%s legal=%d, want playing and 33", v.Status, len(v.Legal))
	}
	c := g.Join("carol")
	if v := recv(t, c); v.You != "spectator" || len(v.Legal) != 0 {
		t.Fatalf("third guest: you=%s legal=%d", v.You, len(v.Legal))
	}
	a2 := g.Join("alice") // a second tab, or a reconnect
	if v := recv(t, a2); v.You != "white" {
		t.Fatalf("reconnect: you=%s, want white", v.You)
	}
}

func TestMoveBroadcastsToEveryone(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a, b, c := g.Join("alice"), g.Join("bob"), g.Join("carol")
	recv(t, a)
	recv(t, b)
	recv(t, c)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	for name, sub := range map[string]*Sub{"alice": a, "bob": b, "carol": c} {
		v := recv(t, sub)
		if v.Seq != 1 || v.Turn != "black" || v.Board[rules.E4] != "wP" || v.Board[rules.E2] != "" {
			t.Errorf("%s: seq=%d turn=%s e4=%q e2=%q", name, v.Seq, v.Turn, v.Board[rules.E4], v.Board[rules.E2])
		}
		if !slices.Equal(v.Log, []string{"e4 · d8 1"}) {
			t.Errorf("%s: log %q", name, v.Log)
		}
		// Black: 19 normal moves (a7a5 is blocked by the Mamdani) + 13 Mamdani moves.
		wantLegal := map[string]int{"alice": 0, "bob": 32, "carol": 0}[name]
		if len(v.Legal) != wantLegal {
			t.Errorf("%s: %d legal moves, want %d", name, len(v.Legal), wantLegal)
		}
	}
}

func TestMoveErrors(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrWaiting) {
		t.Errorf("before black joins: %v", err)
	}
	g.Join("bob")
	cases := []struct {
		guest, uci string
		seq        int
		want       error
	}{
		{"carol", "e2e4", 0, ErrNotPlayer},
		{"bob", "e7e5", 0, ErrNotYourTurn},
		{"alice", "e2e4", 3, ErrStale},
		{"alice", "e2e5", 0, ErrIllegalMove},
	}
	for _, c := range cases {
		if err := g.Move(c.guest, mv(t, c.uci), c.seq); !errors.Is(err, c.want) {
			t.Errorf("%s %s seq %d: got %v, want %v", c.guest, c.uci, c.seq, err, c.want)
		}
	}
}

func TestPotholeShowsInView(t *testing.T) {
	// Even roll, then file 4 rank 4: a pothole opens on d4.
	g := NewHub(&script{rolls: []int{2, 4, 4}}).Create("alice")
	a := g.Join("alice")
	g.Join("bob")
	recv(t, a)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	v := recv(t, a)
	if !slices.Equal(v.Potholes, []Pothole{{Sq: "d4", By: "white"}}) {
		t.Errorf("potholes %v", v.Potholes)
	}
	kinds := []rules.EventKind{}
	for _, e := range v.Last {
		kinds = append(kinds, e.Kind)
	}
	want := []rules.EventKind{rules.Moved, rules.RolledPothole, rules.Target, rules.PotholeOpened}
	if !slices.Equal(kinds, want) {
		t.Errorf("last %v, want %v", kinds, want)
	}
	if !strings.HasPrefix(v.Log[0], "e4 · d8 2 → d4") {
		t.Errorf("log %q", v.Log[0])
	}
}

func TestCheckmateEndsTheGame(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := g.Join("alice")
	g.Join("bob")
	for i, m := range []string{"f2f3", "e7e5", "g2g4", "d8h4"} {
		guest := []string{"alice", "bob"}[i%2]
		if err := g.Move(guest, mv(t, m), i); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
	}
	var v *View
	for range 5 { // drain to the newest view
		v = recv(t, a)
		if v.Seq == 4 {
			break
		}
	}
	if v.Status != Over || v.Result == nil || v.Result.Winner != "black" || v.Result.Reason != rules.Checkmate {
		t.Fatalf("status %s result %+v", v.Status, v.Result)
	}
	if err := g.Move("alice", mv(t, "a2a3"), 4); !errors.Is(err, ErrGameOver) {
		t.Errorf("move after mate: %v", err)
	}
}

func TestLeaveStopsUpdates(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := g.Join("alice")
	recv(t, a)
	g.Leave(a)
	g.Join("bob") // would broadcast to alice if she were still subscribed
	select {
	case v := <-a.C:
		t.Fatalf("got a view after Leave: seq %d", v.Seq)
	case <-time.After(50 * time.Millisecond):
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./game/`
Expected: FAIL to build, with `undefined: NewHub` and `undefined: Sub`.

- [ ] **Step 3: Write the game goroutine**

Create `game/game.go`:

```go
// Package game runs live games. Each Game is one goroutine that owns its
// state; every method hands it a function to run and waits for it, so the
// game's state is only ever touched by that goroutine and needs no locks.
package game

import (
	"errors"
	"slices"

	"mamdani-chess/rules"
)

// Errors returned by Move.
var (
	ErrNotPlayer   = errors.New("you are not playing in this game")
	ErrWaiting     = errors.New("waiting for an opponent")
	ErrNotYourTurn = errors.New("not your turn")
	ErrStale       = errors.New("the game has moved on; reload the position")
	ErrGameOver    = rules.ErrGameOver
	ErrIllegalMove = rules.ErrIllegalMove
)

// Game is one live game.
type Game struct {
	code  string
	dice  rules.Dice
	calls chan func()

	// Owned by the loop goroutine.
	g     *rules.Game
	seats [2]string // guest ID per color; "" while empty
	subs  map[*Sub]struct{}
	last  []rules.Event
	log   []string
}

// Sub is one open stream. C always holds the newest View: a reader that
// falls behind skips stale views instead of blocking the game.
type Sub struct {
	C     <-chan *View
	ch    chan *View
	guest string
}

// newGame starts a game with White's seat taken by creator.
func newGame(code, creator string, dice rules.Dice) *Game {
	g := &Game{
		code:  code,
		dice:  dice,
		calls: make(chan func()),
		g:     rules.NewGame(),
		seats: [2]string{creator, ""},
		subs:  map[*Sub]struct{}{},
	}
	go g.loop()
	return g
}

func (g *Game) loop() {
	for f := range g.calls {
		f()
	}
}

// do runs f on the game's goroutine and waits for it to finish.
func (g *Game) do(f func()) {
	done := make(chan struct{})
	g.calls <- func() {
		defer close(done)
		f()
	}
	<-done
}

// Code returns the game's share code.
func (g *Game) Code() string { return g.code }

// Join opens a stream for guest. The creator plays White, the first other
// guest takes Black, and everyone after that watches. A guest who reconnects
// keeps their seat. The current view is already waiting on the Sub.
func (g *Game) Join(guest string) *Sub {
	ch := make(chan *View, 1)
	sub := &Sub{C: ch, ch: ch, guest: guest}
	g.do(func() {
		if g.seats[rules.Black] == "" && guest != g.seats[rules.White] {
			g.seats[rules.Black] = guest
			g.subs[sub] = struct{}{}
			g.broadcast() // White's view changes from waiting to playing
			return
		}
		g.subs[sub] = struct{}{}
		send(sub, g.view(guest))
	})
	return sub
}

// Leave closes a stream. Seats are kept, so the player can come back.
func (g *Game) Leave(sub *Sub) {
	g.do(func() { delete(g.subs, sub) })
}

// Move plays guest's move. seq must equal the number of turns played so far,
// which rejects a move made from an out-of-date board.
func (g *Game) Move(guest string, m rules.Move, seq int) error {
	var err error
	g.do(func() { err = g.move(guest, m, seq) })
	return err
}

func (g *Game) move(guest string, m rules.Move, seq int) error {
	color, seated := g.seatOf(guest)
	switch {
	case !seated:
		return ErrNotPlayer
	case g.g.Result.Over:
		return ErrGameOver
	case g.status() == Waiting:
		return ErrWaiting
	case color != g.g.Pos.Turn:
		return ErrNotYourTurn
	case seq != len(g.g.Turns):
		return ErrStale
	case !slices.Contains(g.g.Pos.LegalMoves(), m):
		return ErrIllegalMove
	}
	san := g.g.Pos.SAN(m) // before the move: SAN reads the old position
	ev, err := g.g.Play(m, g.dice)
	if err != nil {
		return err
	}
	g.last = ev
	g.log = append(g.log, describe(san, ev))
	g.broadcast()
	return nil
}

func (g *Game) seatOf(guest string) (rules.Color, bool) {
	for c, id := range g.seats {
		if id != "" && id == guest {
			return rules.Color(c), true
		}
	}
	return 0, false
}

func (g *Game) status() Status {
	switch {
	case g.g.Result.Over:
		return Over
	case g.seats[rules.Black] == "":
		return Waiting
	}
	return Playing
}

func (g *Game) broadcast() {
	for sub := range g.subs {
		send(sub, g.view(sub.guest))
	}
}

// send replaces whatever is waiting on the sub with v. Only the game's
// goroutine sends, so this never blocks.
func send(sub *Sub, v *View) {
	select {
	case <-sub.ch:
	default:
	}
	sub.ch <- v
}

func (g *Game) view(guest string) *View {
	p := &g.g.Pos
	v := &View{
		Code:     g.code,
		Status:   g.status(),
		You:      "spectator",
		Mamdani:  squareName(p.Mamdani),
		Potholes: []Pothole{},
		Turn:     colorName(p.Turn),
		Check:    p.InCheck(p.Turn),
		Legal:    []MoveJSON{},
		Last:     []EventJSON{},
		Log:      append([]string{}, g.log...),
		Seq:      len(g.g.Turns),
	}
	color, seated := g.seatOf(guest)
	if seated {
		v.You = colorName(color)
	}
	for s, pc := range p.Board {
		v.Board[s] = pieceCode(pc)
	}
	for c, s := range p.Potholes {
		if s != rules.NoSquare {
			v.Potholes = append(v.Potholes, Pothole{Sq: s.String(), By: colorName(rules.Color(c))})
		}
	}
	if v.Status == Playing && seated && color == p.Turn {
		for _, m := range p.LegalMoves() {
			v.Legal = append(v.Legal, moveJSON(m))
		}
	}
	for _, e := range g.last {
		v.Last = append(v.Last, eventJSON(e))
	}
	if r := g.g.Result; r.Over {
		v.Result = &ResultJSON{Draw: r.Draw, Reason: r.Reason}
		if !r.Draw {
			v.Result.Winner = colorName(r.Winner)
		}
	}
	return v
}
```

- [ ] **Step 4: Write the hub**

Create `game/hub.go`:

```go
package game

import (
	"sync"

	"mamdani-chess/rules"
)

// Hub maps game codes to live games. Games stay in memory until the server
// restarts; saving and resuming them comes in a later milestone.
type Hub struct {
	dice  rules.Dice
	mu    sync.Mutex
	games map[string]*Game
}

// NewHub returns a hub whose games roll with dice.
func NewHub(dice rules.Dice) *Hub {
	return &Hub{dice: dice, games: map[string]*Game{}}
}

// Create starts a game with creator in White's seat.
func (h *Hub) Create(creator string) *Game {
	h.mu.Lock()
	defer h.mu.Unlock()
	code := NewCode()
	for h.games[code] != nil {
		code = NewCode()
	}
	g := newGame(code, creator, h.dice)
	h.games[code] = g
	return g
}

// Get returns the game with code, if any.
func (h *Hub) Get(code string) (*Game, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.games[code]
	return g, ok
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go vet ./game/ && go test -race ./game/`
Expected: `ok  	mamdani-chess/game`. Note that after `e4`, Black has 32 legal moves, not 33: `a7a5` is blocked by the Mamdani on a5.

- [ ] **Step 6: Commit**

```bash
git add game/game.go game/hub.go game/game_test.go
git commit -m "game: one goroutine per game with seats, moves and live views"
```

---

### Task 3: Game API, guest cookie, honk removed

**Files:**
- Create: `server/guest.go`, `server/games.go`
- Modify (replace entirely): `server/server.go`, `server/sse.go`, `server/server_test.go`, `store/migrate.go`, `store/store_test.go`, `cmd/server/main.go`
- Delete: `server/honk.go`, `server/sse_test.go` (broadcaster tests), `store/honk.go`

**Interfaces:**
- Consumes: `game.NewHub`, `Hub.Create/Get`, `Game.Join/Leave/Move`, `game.ParseMove`, `game.MoveJSON`, the `game.Err*` errors, `rules.ErrBadDie`, `game.CryptoDice`. Also the existing `startSSE`, `writeEvent`, `writeHeartbeat` and `writeJSON`.
- Produces:
  - `server.New(st *store.Store, hub *game.Hub, assets fs.FS) *Server`
  - `POST /api/games` returns `201 {"code"}`.
  - `GET /api/games/{code}/stream` streams `state` events, or returns `404`.
  - `POST /api/games/{code}/move` with `{from,to,promo?,seq}` returns `204`, `400`, `403`, `404`, `409` or `500`.
  - A `guest` cookie: 32 hex characters, `HttpOnly`, `SameSite=Lax`, `Path=/`, one year, and `Secure` behind HTTPS.

- [ ] **Step 1: Write the failing tests**

Replace `server/server_test.go`:

```go
package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/store"
)

// odd always rolls 1, so no potholes open and moves are plain chess.
type odd struct{}

func (odd) D8() int { return 1 }

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	assets := fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html>app shell")},
		"_app/immutable/app.js": {Data: []byte("console.log(1)")},
		"favicon.svg":           {Data: []byte("<svg/>")},
	}
	s := New(st, game.NewHub(odd{}), assets)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return s, ts
}

// player is one browser: its own cookie jar, so its own guest ID.
type player struct {
	t   *testing.T
	c   *http.Client
	url string
}

func newPlayer(t *testing.T, ts *httptest.Server) *player {
	jar, _ := cookiejar.New(nil)
	return &player{t: t, c: &http.Client{Jar: jar}, url: ts.URL}
}

func (p *player) post(path, body string) (int, string) {
	p.t.Helper()
	resp, err := p.c.Post(p.url+path, "application/json", strings.NewReader(body))
	if err != nil {
		p.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func (p *player) create() string {
	p.t.Helper()
	status, body := p.post("/api/games", "")
	var out struct{ Code string }
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &out) != nil || len(out.Code) != 6 {
		p.t.Fatalf("create: %d %s", status, body)
	}
	return out.Code
}

// sseReader reads "event:"/"data:" pairs and comments from a stream.
type sseReader struct {
	t  *testing.T
	sc *bufio.Scanner
}

// next returns the next event name and data, or ("comment", text) for a heartbeat.
func (r sseReader) next() (string, string) {
	r.t.Helper()
	var event, data string
	for r.sc.Scan() {
		line := r.sc.Text()
		switch {
		case line == "":
			if event != "" || data != "" {
				return event, data
			}
		case strings.HasPrefix(line, ":"):
			return "comment", strings.TrimSpace(line[1:])
		case strings.HasPrefix(line, "event: "):
			event = line[len("event: "):]
		case strings.HasPrefix(line, "data: "):
			data = line[len("data: "):]
		}
	}
	r.t.Fatalf("stream ended: %v", r.sc.Err())
	return "", ""
}

// state reads the next "state" event as a View.
func (r sseReader) state() game.View {
	r.t.Helper()
	ev, data := r.next()
	var v game.View
	if ev != "state" || json.Unmarshal([]byte(data), &v) != nil {
		r.t.Fatalf("got %q %q, want a state event", ev, data)
	}
	return v
}

func (p *player) stream(code string) sseReader {
	p.t.Helper()
	resp, err := p.c.Get(p.url + "/api/games/" + code + "/stream")
	if err != nil {
		p.t.Fatal(err)
	}
	p.t.Cleanup(func() { resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		p.t.Fatalf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" {
		p.t.Fatal("missing X-Accel-Buffering: no")
	}
	return sseReader{t: p.t, sc: bufio.NewScanner(resp.Body)}
}

func TestFriendGameOverHTTP(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)

	code := alice.create()
	a := alice.stream(code)
	if v := a.state(); v.You != "white" || v.Status != game.Waiting {
		t.Fatalf("alice: you=%s status=%s", v.You, v.Status)
	}
	b := bob.stream(code)
	if v := b.state(); v.You != "black" || v.Status != game.Playing {
		t.Fatalf("bob: you=%s status=%s", v.You, v.Status)
	}
	if v := a.state(); v.Status != game.Playing || len(v.Legal) != 33 {
		t.Fatalf("alice after bob joins: status=%s legal=%d", v.Status, len(v.Legal))
	}
	if v := carol.stream(code).state(); v.You != "spectator" {
		t.Fatalf("carol: you=%s", v.You)
	}

	if status, body := alice.post("/api/games/"+code+"/move", `{"from":"e2","to":"e4","seq":0}`); status != http.StatusNoContent {
		t.Fatalf("alice e2e4: %d %s", status, body)
	}
	if v := b.state(); v.Seq != 1 || v.Board[28] != "wP" || len(v.Legal) != 32 || v.Log[0] != "e4 · d8 1" {
		t.Fatalf("bob after e4: seq=%d e4=%q legal=%d log=%q", v.Seq, v.Board[28], len(v.Legal), v.Log)
	}
	if v := a.state(); v.Seq != 1 || len(v.Legal) != 0 {
		t.Fatalf("alice after e4: seq=%d legal=%d", v.Seq, len(v.Legal))
	}
}

func TestMoveErrors(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	bob.stream(code).state()
	move := "/api/games/" + code + "/move"
	cases := []struct {
		who        *player
		path, body string
		want       int
	}{
		{carol, move, `{"from":"e2","to":"e4","seq":0}`, http.StatusForbidden},
		{bob, move, `{"from":"e7","to":"e5","seq":0}`, http.StatusConflict},   // not your turn
		{alice, move, `{"from":"e2","to":"e4","seq":5}`, http.StatusConflict}, // stale
		{alice, move, `{"from":"e2","to":"e5","seq":0}`, http.StatusConflict}, // illegal
		{alice, move, `{"from":"e9","to":"e4","seq":0}`, http.StatusBadRequest},
		{alice, move, `not json`, http.StatusBadRequest},
		{alice, move, `{"from":"` + strings.Repeat("x", 5000) + `"}`, http.StatusBadRequest},
		{alice, "/api/games/NOPE99/move", `{"from":"e2","to":"e4","seq":0}`, http.StatusNotFound},
	}
	for _, c := range cases {
		if status, body := c.who.post(c.path, c.body); status != c.want {
			t.Errorf("%s %.40s: %d %s, want %d", c.path, c.body, status, body, c.want)
		}
	}
	resp, err := http.Get(ts.URL + "/api/games/NOPE99/stream")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown game stream: %d, want 404", resp.StatusCode)
	}
}

func TestGuestCookie(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/games", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var c *http.Cookie
	for _, k := range resp.Cookies() {
		if k.Name == "guest" {
			c = k
		}
	}
	if c == nil || len(c.Value) != 32 || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("guest cookie %+v", c)
	}
}

func TestGameStreamHeartbeat(t *testing.T) {
	s, ts := newTestServer(t)
	s.heartbeat = 10 * time.Millisecond
	alice := newPlayer(t, ts)
	r := alice.stream(alice.create())
	r.state()
	if ev, text := r.next(); ev != "comment" || text != "ping" {
		t.Fatalf("got %q %q, want heartbeat", ev, text)
	}
}

func TestStaticAndFallback(t *testing.T) {
	_, ts := newTestServer(t)
	cases := []struct {
		path, wantBody, wantCache string
		wantStatus                int
	}{
		{"/", "app shell", "no-cache", 200},
		{"/game/K7F3QZ", "app shell", "no-cache", 200},
		{"/favicon.svg", "<svg/>", "no-cache", 200},
		{"/_app/immutable/app.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/api/nope", `{"error":"not found"}`, "", 404},
		{"/healthz", `{"status":"ok"}`, "", 200},
	}
	for _, c := range cases {
		resp, err := http.Get(ts.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != c.wantStatus {
			t.Errorf("%s: status %d, want %d", c.path, resp.StatusCode, c.wantStatus)
		}
		if !strings.Contains(string(body), c.wantBody) {
			t.Errorf("%s: body %q, want it to contain %q", c.path, body, c.wantBody)
		}
		if c.wantCache != "" && resp.Header.Get("Cache-Control") != c.wantCache {
			t.Errorf("%s: Cache-Control %q, want %q", c.path, resp.Header.Get("Cache-Control"), c.wantCache)
		}
	}
}

func TestCloseEndsStreamsButNotRequests(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	r := alice.stream(alice.create())
	r.state()

	s.Close()
	s.Close() // idempotent

	ended := make(chan struct{})
	go func() {
		for r.sc.Scan() {
		}
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("stream still open 1s after Close")
	}
	alice.create() // other requests still work
}
```

Replace `store/store_test.go`:

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, path
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	s.Close()

	s, err := Open(path) // second open must not re-run migrations
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	v, err := s.Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != len(migrations) {
		t.Fatalf("version = %d, want %d", v, len(migrations))
	}
	var rows int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_version`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != len(migrations) {
		t.Fatalf("schema_version has %d rows, want %d", rows, len(migrations))
	}
}

func TestHonksTableDropped(t *testing.T) {
	s, _ := openTemp(t)
	defer s.Close()
	var n int
	err := s.db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'honks'`).Scan(&n)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("the honks table should be dropped by migration 2")
	}
}
```

Delete the broadcaster tests and the honk code:

```bash
git rm -q server/sse_test.go server/honk.go store/honk.go
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./server/ ./store/`
Expected:
- `server` fails to build: `New` has the wrong number of arguments, `game` is unused, and `honks`/`broadcaster` are undefined.
- `store` fails `TestHonksTableDropped` with "the honks table should be dropped by migration 2".

- [ ] **Step 3: Drop the honk table**

Replace `store/migrate.go`:

```go
package store

import (
	"context"
	"fmt"
)

// migrations run in order; each index is a schema version. Append only:
// never edit or reorder a migration that has shipped.
var migrations = []string{
	// 1: walking-skeleton honk counter (dropped by the game-server plan).
	`CREATE TABLE honks (id INTEGER PRIMARY KEY CHECK (id = 1), count INTEGER NOT NULL);
	 INSERT INTO honks (id, count) VALUES (1, 0);`,
	// 2: the honk demo is gone.
	`DROP TABLE honks;`,
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}
	return nil
}

// Version returns the applied schema version.
func (s *Store) Version(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	return v, err
}
```

- [ ] **Step 4: Trim the SSE helpers**

Replace `server/sse.go`. The broadcaster is gone, because each game's goroutine now fans out its own views:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// startSSE sets the event-stream headers and returns the flusher.
func startSSE(w http.ResponseWriter) (http.Flusher, bool) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // stop proxies buffering the stream
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	return fl, true
}

// writeEvent writes one SSE event with a JSON payload and flushes it.
func writeEvent(w http.ResponseWriter, fl http.Flusher, event string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// writeHeartbeat writes an SSE comment so idle connections stay open.
func writeHeartbeat(w http.ResponseWriter, fl http.Flusher) error {
	if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
		return err
	}
	fl.Flush()
	return nil
}
```

- [ ] **Step 5: Add the guest cookie and game routes**

Create `server/guest.go`:

```go
package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const guestCookie = "guest"

// guestID returns the caller's guest ID from its cookie, setting a new
// random one first if there is none. Call it before writing the response.
func guestID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(guestCookie); err == nil && len(c.Value) == 32 {
		return c.Value
	}
	b := make([]byte, 16)
	rand.Read(b)
	id := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     guestCookie,
		Value:    id,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
	})
	return id
}
```

Create `server/games.go`:

```go
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/rules"
)

// createGame starts a friend game with the caller as White.
func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	g := s.games.Create(guestID(w, r))
	writeJSON(w, http.StatusCreated, map[string]string{"code": g.Code()})
}

// gameStream sends the caller's view of the game on connect and after every
// change. Opening it takes Black's seat if that is still free.
func (s *Server) gameStream(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.games.Get(r.PathValue("code"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	sub := g.Join(guest)
	defer g.Leave(sub)
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case v := <-sub.C:
			if writeEvent(w, fl, "state", v) != nil {
				return
			}
		case <-ticker.C:
			if writeHeartbeat(w, fl) != nil {
				return
			}
		}
	}
}

type moveRequest struct {
	game.MoveJSON
	Seq int `json:"seq"`
}

// gameMove plays the caller's move. The new state arrives on the stream.
func (s *Server) gameMove(w http.ResponseWriter, r *http.Request) {
	guest := guestID(w, r)
	g, ok := s.games.Get(r.PathValue("code"))
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	var req moveRequest
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request body"})
		return
	}
	m, ok := game.ParseMove(req.MoveJSON)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad move"})
		return
	}
	switch err := g.Move(guest, m, req.Seq); {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, game.ErrNotPlayer):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, rules.ErrBadDie):
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "dice failed"})
	default:
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	}
}
```

Replace `server/server.go`:

```go
// Package server is the HTTP layer: routes, SSE and the static frontend.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/store"
)

// Server routes API and frontend requests.
type Server struct {
	store     *store.Store
	games     *game.Hub
	assets    fs.FS
	heartbeat time.Duration
	mux       *http.ServeMux
	done      chan struct{} // closed by Close to end SSE streams
	closeOnce sync.Once
}

// New returns a Server for the games in hub that serves the frontend from
// assets. st is unused until games are saved (a later milestone).
func New(st *store.Store, hub *game.Hub, assets fs.FS) *Server {
	s := &Server{
		store:     st,
		games:     hub,
		assets:    assets,
		heartbeat: 15 * time.Second,
		mux:       http.NewServeMux(),
		done:      make(chan struct{}),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/games", s.createGame)
	s.mux.HandleFunc("GET /api/games/{code}/stream", s.gameStream)
	s.mux.HandleFunc("POST /api/games/{code}/move", s.gameMove)
	s.mux.HandleFunc("/api/", s.apiNotFound)
	s.mux.HandleFunc("/", s.static)
	return s
}

// Close ends every open SSE stream. Other in-flight requests are unaffected.
// Safe to call more than once.
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.done) }) }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

Replace `cmd/server/main.go`:

```go
// Command server runs Pothole Chess: Mamdani Edition.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/server"
	"mamdani-chess/store"
	"mamdani-chess/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data/mamdani.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	handler := server.New(st, game.NewHub(game.CryptoDice{}), web.Assets())
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Ends SSE streams when Shutdown starts; other in-flight requests finish.
	srv.RegisterOnShutdown(handler.Close)

	sig, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "db", dbPath)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-sig.Done():
	}
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go vet ./... && go test -race ./...`
Expected: `ok` for `game`, `names`, `rules`, `server` and `store`. The rules package's 10,000 random games take about 2 minutes under `-race`, so `go test -race -short ./...` is enough while iterating.

- [ ] **Step 7: Commit**

```bash
git add server/guest.go server/games.go server/server.go server/sse.go server/server_test.go store/migrate.go store/store_test.go cmd/server/main.go
git commit -m "server: friend games over HTTP with a guest cookie; remove the honk demo"
```

---

### Task 4: Board and game pages

**Files:**
- Modify (replace entirely): `web/src/lib/theme/tokens.css`, `web/src/routes/+page.svelte`
- Create: `web/src/lib/game.ts`, `web/src/lib/Board.svelte`, `web/src/routes/game/[code]/+page.svelte`

**Interfaces:**
- Consumes: Task 3's routes and `game.View` JSON.
- Produces: `/` (home, "Play a friend") and `/game/[code]` (the game).

- [ ] **Step 1: Add three tokens**

Replace `web/src/lib/theme/tokens.css`. This adds `--hole`, `--piece-light` and `--piece-dark`, so components need no color literals:

```css
/* Road-works theme tokens (spec §6). Dark only for now; components use
   these variables, never color literals, so a light theme can be added. */
:root {
	--bg: #141518;
	--surface: #1e2024;
	--surface-2: #2a2d32;
	--line: #3a3e44;
	--text: #f1eee6;
	--text-body: #c9c5bb;
	--text-muted: #a8a49b;
	--accent: #f2c230;
	--accent-text: #141518;
	--hazard: #ff7a3d;
	--board-light: #d9d3c4;
	--board-dark: #857e72;
	--board-last-light: #e9d98b;
	--board-last-dark: #b5a24f;
	--target-ring: #f2c230;
	--hole: #08090a; /* inside of a pothole */
	--piece-light: #fbf8f0;
	--piece-dark: #141518;

	--font-display: 'Barlow Condensed', 'Arial Narrow', sans-serif;
	--font-body: 'IBM Plex Sans', system-ui, sans-serif;
	--font-mono: 'IBM Plex Mono', ui-monospace, monospace;

	color-scheme: dark;
}

*,
*::before,
*::after {
	box-sizing: border-box;
}

body {
	margin: 0;
	background: var(--bg);
	color: var(--text-body);
	font-family: var(--font-body);
	-webkit-font-smoothing: antialiased;
}
```

- [ ] **Step 2: Write the API types and helpers**

Create `web/src/lib/game.ts`:

```ts
// Types and calls for the game API. Mirrors game/view.go on the server.

export type Color = 'white' | 'black';

export interface MoveJSON {
	from: string;
	to: string;
	promo?: string;
}

export interface EventJSON {
	kind: string;
	sq?: string;
	from?: string;
	to?: string;
	promo?: string;
	piece?: string;
	color?: Color;
	roll?: number;
	saved?: boolean;
	reason?: string;
}

export interface View {
	code: string;
	status: 'waiting' | 'playing' | 'over';
	you: Color | 'spectator';
	board: string[]; // 64 entries, index 0 = a1; "" or "wP", "bQ", ...
	mamdani: string; // "" once it has fallen
	potholes: { sq: string; by: Color }[];
	turn: Color;
	check: boolean;
	legal: MoveJSON[];
	last: EventJSON[];
	log: string[];
	result: { winner?: Color; draw: boolean; reason: string } | null;
	seq: number;
}

export function squareName(index: number): string {
	return 'abcdefgh'[index % 8] + (Math.floor(index / 8) + 1);
}

/** Starts a friend game with the caller as White; returns its code. */
export async function createGame(): Promise<string> {
	const res = await fetch('/api/games', { method: 'POST' });
	if (!res.ok) throw new Error(`could not create a game (${res.status})`);
	return (await res.json()).code;
}

/** Sends a move. Returns null on success, or the server's error message. */
export async function sendMove(code: string, move: MoveJSON, seq: number): Promise<string | null> {
	const res = await fetch(`/api/games/${code}/move`, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ ...move, seq })
	});
	if (res.ok) return null;
	try {
		return (await res.json()).error ?? res.statusText;
	} catch {
		return res.statusText;
	}
}

const pieceNames: Record<string, string> = {
	P: 'pawn',
	N: 'knight',
	B: 'bishop',
	R: 'rook',
	Q: 'queen',
	K: 'king'
};

/** "white knight", "the Mamdani". */
export function pieceName(code = ''): string {
	if (code === 'M') return 'the Mamdani';
	const color = code[0] === 'w' ? 'white' : 'black';
	return `${color} ${pieceNames[code[1]] ?? 'piece'}`;
}

const rerollReasons: Record<string, string> = {
	king: 'kings never fall',
	pothole: 'already a pothole',
	exposes: 'would expose the roller’s king',
	checkmate: 'would decide the game'
};

/** One plain-English line per turn event. */
export function eventText(e: EventJSON): string {
	switch (e.kind) {
		case 'moved':
			return `${e.color} moves ${pieceName(e.piece)} ${e.from}–${e.to}`;
		case 'captured':
			return `${pieceName(e.piece)} captured`;
		case 'pothole_closed':
			return `pothole on ${e.sq} closes`;
		case 'repaired':
			return `the Mamdani repairs ${e.sq}`;
		case 'rolled_pothole':
			return `pothole roll: ${e.roll} — ${(e.roll ?? 1) % 2 === 0 ? 'a pothole opens' : 'nothing happens'}`;
		case 'target':
			return `the dice pick ${e.sq}`;
		case 'reroll':
			return `re-roll: ${rerollReasons[e.reason ?? ''] ?? e.reason}`;
		case 'saving_roll':
			return `saving roll for ${pieceName(e.piece)}: ${e.roll} — ${e.saved ? 'saved' : 'lost'}`;
		case 'fell':
			return `${pieceName(e.piece)} falls into ${e.sq}`;
		case 'pothole_opened':
			return `pothole opens on ${e.sq}`;
		case 'no_pothole':
			return 'no valid square: no pothole this turn';
	}
	return e.kind;
}

const reasons: Record<string, string> = {
	checkmate: 'checkmate',
	stalemate: 'stalemate',
	fifty_moves: 'the 50-move rule',
	repetition: 'threefold repetition',
	insufficient_material: 'insufficient material'
};

export function resultText(r: NonNullable<View['result']>): string {
	const why = reasons[r.reason] ?? r.reason;
	return r.draw ? `Draw by ${why}` : `${r.winner === 'white' ? 'White' : 'Black'} wins by ${why}`;
}
```

- [ ] **Step 3: Write the board**

Create `web/src/lib/Board.svelte`. A selection remembers the `seq` it was made on, so a new position clears it without an `$effect`:

```svelte
<script lang="ts">
	import { squareName, type MoveJSON, type View } from './game.ts';

	let { view, onmove }: { view: View; onmove: (move: MoveJSON) => void } = $props();

	const glyphs: Record<string, string> = { K: '♚', Q: '♛', R: '♜', B: '♝', N: '♞', P: '♟' };
	const promoOptions = [
		{ promo: 'q', kind: 'Q' },
		{ promo: 'r', kind: 'R' },
		{ promo: 'b', kind: 'B' },
		{ promo: 'n', kind: 'N' }
	];

	// Selections remember the position they were made on (seq), so a new
	// position from the server clears any half-made move.
	let picked = $state<{ seq: number; sq: string } | null>(null);
	let pending = $state<{ seq: number; from: string; to: string } | null>(null);
	let selected = $derived(picked?.seq === view.seq ? picked.sq : null);
	let promoting = $derived(pending?.seq === view.seq ? pending : null);

	// White sees rank 8 at the top; Black sees the board flipped.
	let flipped = $derived(view.you === 'black');
	let order = $derived.by(() => {
		const squares: number[] = [];
		for (let row = 0; row < 8; row++) {
			for (let col = 0; col < 8; col++) {
				const rank = flipped ? row : 7 - row;
				const file = flipped ? 7 - col : col;
				squares.push(rank * 8 + file);
			}
		}
		return squares;
	});
	let targets = $derived(new Set(view.legal.filter((m) => m.from === selected).map((m) => m.to)));
	let movable = $derived(new Set(view.legal.map((m) => m.from)));
	let holes = $derived(new Set(view.potholes.map((p) => p.sq)));
	let lastMove = $derived(view.last.find((e) => e.kind === 'moved'));

	function tap(sq: string) {
		if (promoting) return;
		if (selected && targets.has(sq)) {
			const moves = view.legal.filter((m) => m.from === selected && m.to === sq);
			if (moves.length > 1) {
				pending = { seq: view.seq, from: selected, to: sq };
				return;
			}
			onmove(moves[0]);
			picked = null;
			return;
		}
		picked = movable.has(sq) && selected !== sq ? { seq: view.seq, sq } : null;
	}

	function promote(promo: string) {
		if (!promoting) return;
		onmove({ from: promoting.from, to: promoting.to, promo });
		pending = null;
		picked = null;
	}
</script>

<div class="board" class:flipped>
	{#each order as index (index)}
		{@const sq = squareName(index)}
		{@const piece = view.board[index]}
		{@const dark = (Math.floor(index / 8) + (index % 8)) % 2 === 0}
		<button
			class="square"
			class:dark
			class:last={lastMove?.from === sq || lastMove?.to === sq}
			class:selected={selected === sq}
			class:movable={movable.has(sq)}
			aria-label={sq}
			onclick={() => tap(sq)}
		>
			{#if holes.has(sq)}
				<span class="hole" aria-label="pothole"></span>
			{/if}
			{#if view.mamdani === sq}
				<span class="mamdani" aria-label="the Mamdani">M</span>
			{:else if piece}
				<span class="piece" class:white={piece[0] === 'w'}>{glyphs[piece[1]]}</span>
			{/if}
			{#if targets.has(sq)}
				<span class="dot" class:capture={!!piece}></span>
			{/if}
		</button>
	{/each}

	{#if promoting}
		<div class="promote" role="dialog" aria-label="Promote to">
			{#each promoOptions as option (option.promo)}
				<button onclick={() => promote(option.promo)}>
					<span class="piece" class:white={view.you === 'white'}>{glyphs[option.kind]}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>

<style>
	.board {
		position: relative;
		display: grid;
		grid-template-columns: repeat(8, 1fr);
		width: min(100%, 560px);
		aspect-ratio: 1;
		border: 2px solid var(--line);
	}
	.square {
		position: relative;
		display: grid;
		place-items: center;
		padding: 0;
		border: 0;
		background: var(--board-light);
		cursor: default;
		aspect-ratio: 1;
	}
	.square.dark {
		background: var(--board-dark);
	}
	.square.last {
		background: var(--board-last-light);
	}
	.square.dark.last {
		background: var(--board-last-dark);
	}
	.square.movable {
		cursor: pointer;
	}
	.square.selected {
		outline: 3px solid var(--accent);
		outline-offset: -3px;
	}
	.piece {
		position: relative;
		font-size: clamp(24px, 8vmin, 52px);
		line-height: 1;
		color: var(--piece-dark);
		user-select: none;
	}
	.piece.white {
		color: var(--piece-light);
		-webkit-text-stroke: 1px var(--piece-dark);
	}
	.mamdani {
		position: relative;
		display: grid;
		place-items: center;
		width: 70%;
		aspect-ratio: 1;
		border-radius: 50%;
		background: var(--surface);
		border: 3px solid var(--accent);
		color: var(--accent);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: clamp(14px, 4vmin, 28px);
	}
	.hole {
		position: absolute;
		inset: 12%;
		border-radius: 46% 54% 50% 48%;
		background: var(--hole);
		box-shadow:
			0 0 0 3px var(--hazard),
			inset 0 4px 10px var(--bg);
	}
	.dot {
		position: absolute;
		width: 28%;
		aspect-ratio: 1;
		border-radius: 50%;
		background: var(--target-ring);
		opacity: 0.8;
		pointer-events: none;
	}
	.dot.capture {
		width: 86%;
		background: none;
		border: 4px solid var(--target-ring);
	}
	.promote {
		position: absolute;
		inset: 35% 10%;
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 8px;
		padding: 8px;
		background: var(--surface);
		border: 2px solid var(--accent);
	}
	.promote button {
		min-height: 44px;
		border: 0;
		background: var(--surface-2);
		cursor: pointer;
	}
</style>
```

- [ ] **Step 4: Write the home page**

Replace `web/src/routes/+page.svelte`:

```svelte
<script lang="ts">
	import { goto } from '$app/navigation';
	import { createGame } from '#lib/game.ts';

	let busy = $state(false);
	let error = $state('');

	async function newGame() {
		busy = true;
		error = '';
		try {
			await goto(`/game/${await createGame()}`);
		} catch (e) {
			error = e instanceof Error ? e.message : String(e);
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Pothole Chess</title>
</svelte:head>

<main>
	<h1>Pothole Chess</h1>
	<p>
		Chess, but the road is falling apart. After every move a d8 may open a pothole that swallows
		a piece. The Mamdani — a neutral piece either player can move — repairs them.
	</p>
	<button onclick={newGame} disabled={busy}>{busy ? 'Starting…' : 'Play a friend'}</button>
	{#if error}<p class="error" role="alert">{error}</p>{/if}
	<p class="hint">You play White. Send the link to a friend; they play Black.</p>
</main>

<style>
	main {
		min-height: 100dvh;
		display: grid;
		place-content: center;
		justify-items: center;
		gap: 16px;
		padding: 16px;
		max-width: 560px;
		margin: 0 auto;
		text-align: center;
	}
	h1 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 56px;
		text-transform: uppercase;
		color: var(--text);
	}
	p {
		margin: 0;
	}
	button {
		min-height: 44px;
		padding: 0 32px;
		border: 0;
		border-radius: 4px;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 24px;
		text-transform: uppercase;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.6;
		cursor: default;
	}
	.hint {
		color: var(--text-muted);
		font-size: 14px;
	}
	.error {
		color: var(--hazard);
	}
</style>
```

- [ ] **Step 5: Write the game page**

Create `web/src/routes/game/[code]/+page.svelte`:

```svelte
<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import Board from '#lib/Board.svelte';
	import { eventText, resultText, sendMove, type MoveJSON, type View } from '#lib/game.ts';

	const code = page.params.code ?? '';

	let view = $state<View | null>(null);
	let connected = $state(false);
	let notFound = $state(false);
	let error = $state('');
	let copied = $state(false);

	onMount(() => {
		const source = new EventSource(`/api/games/${code}/stream`);
		source.onopen = () => (connected = true);
		source.onerror = () => {
			connected = false;
			// A 404 closes the stream for good; other errors reconnect by themselves.
			if (source.readyState === EventSource.CLOSED && !view) notFound = true;
		};
		source.addEventListener('state', (e) => {
			view = JSON.parse((e as MessageEvent<string>).data);
			error = '';
		});
		return () => source.close();
	});

	async function move(m: MoveJSON) {
		if (!view) return;
		error = (await sendMove(code, m, view.seq)) ?? '';
	}

	async function copyLink() {
		await navigator.clipboard.writeText(location.href);
		copied = true;
	}

	let status = $derived.by(() => {
		if (!view) return '';
		if (view.result) return resultText(view.result);
		if (view.status === 'waiting') return 'Waiting for your friend to open the link…';
		const side = view.turn === 'white' ? 'White' : 'Black';
		const check = view.check ? ' — check!' : '';
		if (view.you === view.turn) return `Your move${check}`;
		return `${side} to move${check}`;
	});
</script>

<svelte:head>
	<title>Game {code} · Pothole Chess</title>
</svelte:head>

<main>
	<header>
		<a href="/" class="logo">Pothole Chess</a>
		<span class="code">{code}</span>
		{#if view && !connected}<span class="warn">Reconnecting…</span>{/if}
	</header>

	{#if notFound}
		<p>Game not found. <a href="/">Start a new one</a>.</p>
	{:else if !view}
		<p>Connecting…</p>
	{:else}
		<p class="status" aria-live="polite">
			{status}
			{#if view.you === 'spectator'}<span class="muted">(watching)</span>{/if}
		</p>

		{#if view.status === 'waiting' && view.you === 'white'}
			<button class="share" onclick={copyLink}>{copied ? 'Link copied' : 'Copy link for your friend'}</button>
		{/if}

		<div class="layout">
			<Board {view} onmove={move} />
			<aside>
				{#if error}<p class="error" role="alert">{error}</p>{/if}
				<h2>Last turn</h2>
				{#if view.last.length === 0}
					<p class="muted">No moves yet.</p>
				{:else}
					<ul>
						{#each view.last as e, i (i)}
							<li class:hazard={['pothole_opened', 'fell'].includes(e.kind)}>{eventText(e)}</li>
						{/each}
					</ul>
				{/if}
				<h2>Moves</h2>
				<ol class="log">
					{#each view.log as line, i (i)}
						<li>{line}</li>
					{/each}
				</ol>
			</aside>
		</div>
	{/if}
</main>

<style>
	main {
		max-width: 960px;
		margin: 0 auto;
		padding: 16px;
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
	.share {
		justify-self: start;
		min-height: 44px;
		padding: 0 20px;
		border: 0;
		border-radius: 4px;
		background: var(--accent);
		color: var(--accent-text);
		font-weight: 600;
		cursor: pointer;
	}
	.layout {
		display: grid;
		grid-template-columns: minmax(0, 560px) 1fr;
		gap: 16px;
		align-items: start;
	}
	@media (max-width: 760px) {
		.layout {
			grid-template-columns: 1fr;
		}
	}
	aside {
		display: grid;
		gap: 8px;
	}
	h2 {
		margin: 8px 0 0;
		font-family: var(--font-display);
		text-transform: uppercase;
		font-size: 18px;
		color: var(--text);
	}
	ul,
	ol {
		margin: 0;
		padding-left: 20px;
	}
	.hazard {
		color: var(--hazard);
	}
	.log {
		padding-left: 36px; /* room for two-digit move numbers */
		font-family: var(--font-mono);
		font-size: 14px;
		max-height: 320px;
		overflow-y: auto;
	}
	.error {
		color: var(--hazard);
	}
	a {
		color: var(--accent);
	}
</style>
```

- [ ] **Step 6: Check, autofix and build**

Run:

```bash
for f in web/src/lib/Board.svelte web/src/routes/+page.svelte 'web/src/routes/game/[code]/+page.svelte'; do npx -y @sveltejs/mcp svelte-autofixer "$f"; done
pnpm --dir web check && pnpm --dir web build
```

Expected: each autofixer run prints `issues: []` and `suggestions: []`. `svelte-check` reports `0 ERRORS 0 WARNINGS`, and the build ends with `Wrote site to "build"`.

- [ ] **Step 7: Commit**

```bash
git add web/src/lib/theme/tokens.css web/src/lib/game.ts web/src/lib/Board.svelte web/src/routes/+page.svelte 'web/src/routes/game/[code]/+page.svelte'
git commit -m "web: home page and a tap-to-move game page"
```

---

### Task 5: End-to-end check and docs

**Files:**
- Modify: `README.md` (Status section), `docs/superpowers/plans/2026-10-03-00-roadmap.md` (milestone 03 status)

- [ ] **Step 1: Scripted game over HTTP with the embedded build**

Run:

```bash
pnpm --dir web build && go build -tags embedweb -o bin/server ./cmd/server
T=$(mktemp -d); PORT=18090 DB_PATH=$T/m.db ./bin/server > $T/log 2>&1 & PID=$!; sleep 1
CODE=$(curl -s -c $T/a -XPOST localhost:18090/api/games | python3 -c 'import sys,json;print(json.load(sys.stdin)["code"])')
curl -sN -b $T/a --max-time 4 localhost:18090/api/games/$CODE/stream > $T/a.txt &
sleep 0.3; curl -sN -c $T/b --max-time 0.5 localhost:18090/api/games/$CODE/stream > /dev/null  # Bob takes Black; jar written on exit
curl -s -o /dev/null -w "white e4: %{http_code}\n" -b $T/a -XPOST -d '{"from":"e2","to":"e4","seq":0}' localhost:18090/api/games/$CODE/move
curl -s -o /dev/null -w "black e5: %{http_code}\n" -b $T/b -XPOST -d '{"from":"e7","to":"e5","seq":1}' localhost:18090/api/games/$CODE/move
sleep 4; grep -c '^data: ' $T/a.txt; tail -c 300 $T/a.txt; echo
curl -s localhost:18090/game/$CODE | head -c 15; echo
kill $PID
```

Expected:
- `white e4: 204` and `black e5: 204`. The dice are real, so a pothole from White's roll can occasionally block e7–e5 and give `409`. If that happens, rerun the step.
- White's stream holds 4 `data:` lines: waiting, playing, after e4, after e5.
- The last view has `"seq":2` and two `log` entries. A real d8 may have opened a pothole.
- `/game/$CODE` serves `<!doctype html>`.

- [ ] **Step 2: Two-browser check (manual)**

Run `go run ./cmd/server` and `pnpm --dir web dev`. Open `http://localhost:5173`, press **Play a friend**, then open the same game URL with `127.0.0.1` in place of `localhost`. The two hosts keep separate cookies, so that window plays Black.

Expected:
- **Seats:** the White window says "Waiting…" until the Black window opens, and Black's board is flipped.
- **Moving:** tapping a piece shows yellow dots on its legal squares, and tapping a dot moves it. The other window updates within a second, and both move logs grow.
- **The Mamdani:** tapping it (`M` on a5) shows queen-line dots and moves it.
- **Potholes:** after an even pothole roll, a dark hole with an orange ring appears, and "Last turn" lists the rolls in words.
- **Promotion:** a pawn reaching the last rank offers Q, R, B and N.
- **Checkmate:** after Fool's Mate (f3, e5, g4, Qh4), both windows say "Black wins by checkmate" and neither can move. With real dice, a pothole may change the line; just play on.

- [ ] **Step 3: Update the docs**

In `README.md`, replace the line "Early development. Nothing is playable yet." with:

```markdown
Early development. You can play a rough game: press **Play a friend**, send the link, and play in two browsers. There is no clock yet, and games live in memory, so they vanish when the server restarts.
```

Also in `README.md`, replace the Status table's first "Next" cell, "A rough playable game: create a game, share the link, play it in two browsers", with "Dice animation, a move log and a result screen; then a playtest". Then replace the first "Done" cell's "App skeleton: … The frontend is currently a placeholder "honk" counter." with "App: Go server, live updates over Server-Sent Events, SvelteKit frontend with a tap-to-move board, CI and a Dockerfile."

In the roadmap, set milestone 03's Status to `Done` and milestone 04's Status to `Next`.

- [ ] **Step 4: Commit**

```bash
git add README.md docs/superpowers/plans/2026-10-03-00-roadmap.md
git commit -m "Docs: a rough game is playable"
```

---

## Done when

- `go vet ./... && go test -race ./...` and `pnpm --dir web check && pnpm --dir web build` pass.
- Two browsers finish a game through a shared link, with live updates and potholes shown.
- No honk code remains: `grep -ri honk server store web/src cmd` finds only migrations 1 and 2.
