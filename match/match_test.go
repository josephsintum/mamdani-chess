package match

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

// games records the games a queue starts, and can be made to fail.
type games struct {
	started  [][2]string
	playtest []bool // each started game's mark
	fail     bool
}

func (g *games) create(white, black string, playtest bool) (string, error) {
	if g.fail {
		return "", errors.New("database is down")
	}
	g.started = append(g.started, [2]string{white, black})
	g.playtest = append(g.playtest, playtest)
	return fmt.Sprintf("GAME%02d", len(g.started)), nil
}

func newQueue(g *games) *Queue {
	q := New(g.create)
	q.flip = func() bool { return true } // the guest who waited longer plays White
	return q
}

// matched returns the code waiting on t, or "" if it hasn't been matched.
func matched(t *Ticket) string {
	select {
	case code := <-t.C:
		return code
	default:
		return ""
	}
}

func TestTwoGuestsArePairedInOrder(t *testing.T) {
	g := &games{}
	q := newQueue(g)
	a := q.Join("alice", false)
	if matched(a) != "" || q.Looking() != 1 {
		t.Fatalf("alone: matched %q, looking %d", matched(a), q.Looking())
	}
	b := q.Join("bob", false)
	c := q.Join("carol", false)
	if ca, cb := matched(a), matched(b); ca != "GAME01" || cb != "GAME01" {
		t.Fatalf("alice %q bob %q, want both GAME01", ca, cb)
	}
	if g.started[0] != [2]string{"alice", "bob"} {
		t.Fatalf("started %v, want alice (White) vs bob", g.started)
	}
	if matched(c) != "" || q.Looking() != 1 {
		t.Fatalf("carol: matched %q, looking %d; want still waiting", matched(c), q.Looking())
	}
}

func TestColorsComeFromFlip(t *testing.T) {
	g := &games{}
	q := newQueue(g)
	q.flip = func() bool { return false }
	q.Join("alice", false)
	q.Join("bob", false)
	if g.started[0] != [2]string{"bob", "alice"} {
		t.Fatalf("started %v, want bob (White) vs alice", g.started)
	}
}

// A game is a playtest's when either guest is one.
func TestAPlaytestMarksTheGame(t *testing.T) {
	for _, c := range []struct{ alice, bob, want bool }{{false, false, false}, {true, false, true}, {false, true, true}, {true, true, true}} {
		g := &games{}
		q := newQueue(g)
		q.Join("alice", c.alice)
		q.Join("bob", c.bob)
		if len(g.playtest) != 1 || g.playtest[0] != c.want {
			t.Errorf("alice %v, bob %v: marks %v, want [%v]", c.alice, c.bob, g.playtest, c.want)
		}
	}
}

func TestTwoTabsShareOnePlace(t *testing.T) {
	g := &games{}
	q := newQueue(g)
	a1 := q.Join("alice", false)
	a2 := q.Join("alice", false) // a second tab: not a match with herself
	if len(g.started) != 0 || q.Looking() != 1 {
		t.Fatalf("one guest, two tabs: started %v, looking %d", g.started, q.Looking())
	}
	q.Join("bob", false)
	if c1, c2 := matched(a1), matched(a2); c1 != "GAME01" || c2 != "GAME01" {
		t.Fatalf("alice's tabs got %q and %q, want GAME01 on both", c1, c2)
	}
}

func TestLeavingWithTheLastTabLeavesTheLine(t *testing.T) {
	g := &games{}
	q := newQueue(g)
	a1 := q.Join("alice", false)
	a2 := q.Join("alice", false)
	a1.Leave()
	if q.Looking() != 1 {
		t.Fatalf("one tab left: looking %d, want 1", q.Looking())
	}
	a2.Leave()
	a2.Leave() // twice is fine
	if q.Looking() != 0 {
		t.Fatalf("no tabs left: looking %d, want 0", q.Looking())
	}
	q.Join("bob", false)
	if len(g.started) != 0 {
		t.Fatalf("started %v with a guest who left", g.started)
	}
}

func TestAFailedGameKeepsBothInLine(t *testing.T) {
	g := &games{fail: true}
	q := newQueue(g)
	a := q.Join("alice", false)
	b := q.Join("bob", false)
	if matched(a) != "" || matched(b) != "" || q.Looking() != 2 {
		t.Fatalf("after a failure: looking %d", q.Looking())
	}
	g.fail = false
	q.Join("carol", false) // the next arrival tries again, oldest first
	if matched(a) != "GAME01" || matched(b) != "GAME01" || q.Looking() != 1 {
		t.Fatalf("retry: started %v, looking %d", g.started, q.Looking())
	}
}

func TestLeaveAfterAMatchIsHarmless(t *testing.T) {
	q := newQueue(&games{})
	a := q.Join("alice", false)
	b := q.Join("bob", false)
	a.Leave()
	b.Leave()
	if q.Looking() != 0 {
		t.Fatalf("looking %d", q.Looking())
	}
}

func TestLookingForSomeoneCountsTheOthers(t *testing.T) {
	q := newQueue(&games{})
	q.Join("alice", false)
	if q.LookingFor("alice") != 0 || q.LookingFor("bob") != 1 {
		t.Fatalf("alice sees %d, bob sees %d; want 0 and 1", q.LookingFor("alice"), q.LookingFor("bob"))
	}
}
