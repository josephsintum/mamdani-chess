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
	if !before.isLegal(m) {
		fail("isLegal rejects a move LegalMoves offered")
	}
	if p.hasLegalMove() != (len(p.LegalMoves()) > 0) {
		fail("hasLegalMove disagrees with LegalMoves")
	}
	if err := checkBitboards(p); err != nil {
		fail("%v", err)
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
	q := before
	q.play(m, nil)
	if q.Mated() && g.Result.Reason != Checkmate {
		fail("the dice undid a checkmate made on the board")
	}
	if g.Result.Reason == Checkmate {
		// A roll never wins: the move alone must already have been mate.
		if !q.Mated() {
			fail("the dice delivered checkmate")
		}
	}
}
