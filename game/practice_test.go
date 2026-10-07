package game

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

func TestPracticePlaysBothSides(t *testing.T) {
	st := openStore(t)
	h := NewHub(odd{}, st)
	g := h.CreatePractice("alice")

	v := recvView(t, g, "alice")
	if v.Status != Playing || !v.Practice || v.You != "white" || len(v.Legal) == 0 {
		t.Fatalf("new practice game: status %s, practice %v, you %q, %d legal moves", v.Status, v.Practice, v.You, len(v.Legal))
	}
	play(t, g, "alice", "e2e4", 0)
	if v := recvView(t, g, "alice"); v.You != "black" || len(v.Legal) == 0 {
		t.Fatalf("after White's move: you %q with %d legal moves, want black's", v.You, len(v.Legal))
	}
	play(t, g, "alice", "e7e5", 1)
	if err := g.Move("bob", mv(t, "g1f3"), 2); !errors.Is(err, ErrNotPlayer) {
		t.Errorf("another guest's move: got %v, want ErrNotPlayer", err)
	}
	if v := recvView(t, g, "bob"); v.You != "spectator" || len(v.Legal) != 0 {
		t.Errorf("another guest sees you %q and %d legal moves, want a spectator's view", v.You, len(v.Legal))
	}
	if saved := load(t, st); len(saved) != 0 {
		t.Errorf("practice game saved: %+v", saved)
	}
	if err := g.Rematch("alice", false); !errors.Is(err, ErrPractice) {
		t.Errorf("rematch: got %v, want ErrPractice", err)
	}
}

func TestPracticeHasNoClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{}, nil)
		g := h.CreatePractice("alice")
		a := join(t, g, "alice")
		time.Sleep(2 * FirstMoveTime) // a real game would be aborted by now
		play(t, g, "alice", "e2e4", 0)
		play(t, g, "alice", "e7e5", 1)
		time.Sleep(InitialTime + time.Minute) // and lost on time by now
		synctest.Wait()
		v := recvView(t, g, "alice")
		if v.Result != nil || v.Clock.Running != "" || v.Clock.FirstMoveDeadline != 0 || v.Clock.WhiteMS != InitialTime.Milliseconds() {
			t.Errorf("practice clock: result %+v, clock %+v", v.Result, v.Clock)
		}
		g.Leave(a)
		time.Sleep(PracticeIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Error("practice game kept after PracticeIdle with nobody watching")
		}
	})
}

func TestPracticeIsNeitherListedNorActive(t *testing.T) {
	h := NewHub(odd{}, nil)
	h.CreatePractice("alice")
	real := playing(t, h, "alice", "bob")
	if got := codes(h.List(10)); len(got) != 1 || got[0] != real.Code() {
		t.Errorf("listed %v, want only the real game %s", got, real.Code())
	}
	h2 := NewHub(odd{}, nil)
	h2.CreatePractice("alice")
	if code := h2.Active("alice"); code != "" {
		t.Errorf("Active = %q for a guest who is only practising", code)
	}
}

func TestNewPracticeEndsTheLast(t *testing.T) {
	h := NewHub(odd{}, nil)
	first := h.CreatePractice("alice")
	sub := join(t, first, "alice")
	other := h.CreatePractice("bob")
	second := h.CreatePractice("alice")
	waitClosed(t, sub)
	if _, ok := h.Get(first.Code()); ok {
		t.Error("alice's first practice game is still there")
	}
	for _, g := range []*Game{second, other} {
		if _, ok := h.Get(g.Code()); !ok {
			t.Errorf("practice game %s is gone", g.Code())
		}
	}
}

// Two creates at once (a double click, two tabs) still leave one game.
func TestPracticeCreatesAtOnceLeaveOne(t *testing.T) {
	h := NewHub(odd{}, nil)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() { h.CreatePractice("alice") })
	}
	wg.Wait()
	h.mu.Lock()
	n := len(h.games)
	h.mu.Unlock()
	if n != 1 {
		t.Errorf("%d practice games for one guest, want 1", n)
	}
}

func TestPracticeOwnerIsNotWatching(t *testing.T) {
	h := NewHub(odd{}, nil)
	g := h.CreatePractice("alice")
	join(t, g, "alice")
	join(t, g, "bob")
	if err := g.do(func() {}); err != nil { // the joins have been handled
		t.Fatal(err)
	}
	if got := g.live.Load().Watching; got != 1 {
		t.Errorf("Watching = %d with the owner and one onlooker, want 1", got)
	}
}
