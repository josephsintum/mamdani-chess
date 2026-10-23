package game

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"mamdani-chess/store"
)

// Close stops every game where it stands: streams close, the hub forgets
// them, and a clock that runs out afterwards saves nothing, since the store
// is about to close. The next start restores them from what was saved.
func TestCloseStopsEveryGame(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := &ended{}
		h := NewHub(odd{}, st)
		g, a, b := seated(t, h) // White's first-move deadline is running
		waiting := create(t, h, "carol")
		if n := h.Close(); n != 2 {
			t.Errorf("Close stopped %d games, want 2", n)
		}
		waitClosed(t, a)
		waitClosed(t, b)
		for _, x := range []*Game{g, waiting} {
			if _, ok := h.Get(x.Code()); ok {
				t.Errorf("%s still listed after Close", x.Code())
			}
		}
		if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrGone) {
			t.Errorf("move after Close: %v, want ErrGone", err)
		}
		time.Sleep(DefaultIdle + time.Minute) // past every deadline
		synctest.Wait()
		st.mu.Lock()
		defer st.mu.Unlock()
		if len(st.results) != 0 {
			t.Errorf("saved %+v after Close, want nothing", st.results)
		}
	})
}

// slowTurn holds every AddTurn until release is closed.
type slowTurn struct {
	nopStore
	entered chan struct{}
	release chan struct{}
}

func (s *slowTurn) AddTurn(context.Context, string, store.Turn) error {
	close(s.entered)
	<-s.release
	return nil
}

// Close waits for a save that has started, so the store never closes
// under it.
func TestCloseWaitsForASave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := &slowTurn{entered: make(chan struct{}), release: make(chan struct{})}
		h := NewHub(odd{}, st)
		g, _, _ := seated(t, h)
		moved := make(chan error)
		go func() { moved <- g.Move("alice", mv(t, "e2e4"), 0) }()
		<-st.entered
		closed := make(chan struct{})
		go func() {
			h.Close()
			close(closed)
		}()
		synctest.Wait()
		select {
		case <-closed:
			t.Fatal("Close returned while a move was being saved")
		default:
		}
		close(st.release)
		if err := <-moved; err != nil {
			t.Errorf("move: %v", err)
		}
		<-closed
	})
}
