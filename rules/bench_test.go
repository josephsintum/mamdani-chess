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
