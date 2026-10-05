package rules

import (
	"fmt"
	"math/rand/v2"
	"slices"
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
		ev, err := g.Play(m, d)
		if err != nil {
			t.Fatalf("seed %d ply %d %s: %v", seed, ply, m, err)
		}
		checkInvariants(t, seed, ply, before, m, ev, g)
	}
	replayed, err := Replay(StartPosition(), g.Turns)
	if err != nil {
		t.Fatalf("seed %d: replay: %v", seed, err)
	}
	if replayed.Pos != g.Pos || replayed.Result != g.Result {
		t.Fatalf("seed %d: replay differs", seed)
	}
}

func checkInvariants(t *testing.T, seed, ply int, before Position, m Move, ev []Event, g *Game) {
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
	for _, h := range p.Potholes {
		if h.Sq == NoSquare {
			continue
		}
		if p.Board[h.Sq] != NoPiece || h.Sq == p.Mamdani {
			fail("something stands on the pothole at %v", h.Sq)
		}
		if h.Left < 1 || h.Left > HoleRounds {
			fail("the pothole at %v has %d rounds left", h.Sq, h.Left)
		}
	}
	if err := checkHoles(before, ev, p); err != nil {
		fail("%v", err)
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
}

// checkHoles accounts for every hole across one turn: each of the mover's
// holes loses a round and closes at zero, the other player's stay as they
// were, and any other close is a repair or the cap pushing out the oldest
// to make room for a new hole. At most one hole opens, the mover's, with
// every round to go.
func checkHoles(before Position, ev []Event, after *Position) error {
	mover := before.Turn
	rolled := false
	var counted, repaired, capped, opened []Square
	for _, e := range ev {
		switch e.Kind {
		case RolledPothole:
			rolled = true
		case PotholeClosed:
			if rolled {
				capped = append(capped, e.Square)
			} else {
				counted = append(counted, e.Square)
			}
		case Repaired:
			repaired = append(repaired, e.Square)
		case PotholeOpened:
			opened = append(opened, e.Square)
		}
	}
	if len(opened) > 1 || len(capped) > 1 {
		return fmt.Errorf("opened %v and capped %v in one turn", opened, capped)
	}
	if len(capped) == 1 && len(opened) == 0 {
		return fmt.Errorf("the cap closed %v but nothing opened", capped[0])
	}
	oldest, count := before.oldestHole()
	for _, h := range before.Potholes {
		if h.Sq == NoSquare {
			continue
		}
		want := h
		if h.By == mover {
			want.Left--
		}
		switch {
		case want.Left == 0:
			if !slices.Contains(counted, h.Sq) {
				return fmt.Errorf("%v reached its last round but did not close", h.Sq)
			}
		case slices.Contains(counted, h.Sq):
			return fmt.Errorf("%v closed with %d rounds left", h.Sq, want.Left)
		case slices.Contains(repaired, h.Sq):
		case slices.Contains(capped, h.Sq):
			if count < HoleCap || h != before.Potholes[oldest] {
				return fmt.Errorf("the cap closed %v, not the oldest of %d", h.Sq, count)
			}
		case !slices.Contains(after.Potholes[:], want):
			return fmt.Errorf("%v went from %+v to missing or changed", h.Sq, h)
		}
	}
	for _, s := range opened {
		i := slices.IndexFunc(after.Potholes[:], func(h Hole) bool { return h.Sq == s })
		if i < 0 {
			return fmt.Errorf("%v opened but is not in the list", s)
		}
		h := after.Potholes[i]
		if h.By != mover || h.Left != HoleRounds {
			return fmt.Errorf("the new hole %+v is not the mover's with every round", h)
		}
		for _, o := range after.Potholes {
			if o.Sq != NoSquare && o.Sq != s && o.Seq >= h.Seq {
				return fmt.Errorf("the new hole %+v is not the newest: %+v", h, o)
			}
		}
	}
	return nil
}
