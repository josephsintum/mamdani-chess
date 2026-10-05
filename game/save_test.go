package game

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"mamdani-chess/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "games.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func load(t *testing.T, st *store.Store) []store.SavedGame {
	t.Helper()
	saved, err := st.LoadForRestore(context.Background(), time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return saved
}

// withCodes makes h draw the given codes in order.
func withCodes(h *Hub, codes ...string) *Hub {
	h.newCode = func() string {
		c := codes[0]
		codes = codes[1:]
		return c
	}
	return h
}

// failing is a store whose every write fails once fail is set. Set it
// before a call into the game: the game's goroutine reads it after.
type failing struct{ fail bool }

var errDiskFull = errors.New("disk full")

func (f *failing) err() error {
	if f.fail {
		return errDiskFull
	}
	return nil
}
func (f *failing) CreateGame(context.Context, store.Game) error    { return f.err() }
func (f *failing) SeatBlack(context.Context, string, string) error { return f.err() }
func (f *failing) AddTurn(context.Context, string, store.Turn) error {
	return f.err()
}
func (f *failing) EndGame(context.Context, string, store.Result, *store.Turn) error {
	return f.err()
}

func TestCreateAndJoinAreSaved(t *testing.T) {
	st := openStore(t)
	g := create(t, NewHub(odd{}, st), "alice")
	join(t, g, "bob")
	saved := load(t, st)
	if len(saved) != 1 || saved[0].Code != g.Code() || saved[0].White != "alice" || saved[0].Black != "bob" {
		t.Fatalf("saved %+v", saved)
	}
}

func TestCreateRetriesATakenCode(t *testing.T) {
	st := openStore(t)
	create(t, withCodes(NewHub(odd{}, st), "TAKEN1"), "alice")
	h2 := withCodes(NewHub(odd{}, st), "TAKEN1", "FRESH1") // TAKEN1 is saved, but not in h2's memory
	if g := create(t, h2, "bob"); g.Code() != "FRESH1" {
		t.Fatalf("code %s, want FRESH1", g.Code())
	}
}

func TestCreateFailsWhenTheSaveFails(t *testing.T) {
	h := NewHub(odd{}, &failing{fail: true})
	if _, err := h.Create("alice"); !errors.Is(err, errDiskFull) {
		t.Fatalf("create: %v, want the store's error", err)
	}
}

func TestAFailedSaveRetiresTheGame(t *testing.T) {
	st := &failing{}
	h := NewHub(odd{}, st)
	g := create(t, h, "alice")
	a := join(t, g, "alice")
	recv(t, a)
	st.fail = true
	if _, err := g.Join("bob"); !errors.Is(err, ErrInternal) {
		t.Fatalf("join: %v, want ErrInternal", err)
	}
	for v := range a.C { // the stream closes without showing Black seated
		if v.Status != Waiting {
			t.Fatalf("alice saw status %s, a seat that was never saved", v.Status)
		}
	}
	if _, ok := h.Get(g.Code()); ok {
		t.Error("the game is still listed")
	}
	if _, err := g.View("alice"); !errors.Is(err, ErrGone) {
		t.Errorf("view after retiring: %v, want ErrGone", err)
	}
}

// Guest IDs are cookies: a log line must show a tag, never the ID.
func TestLogsDontShowGuestIDs(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.DiscardHandler)) })
	white, black := strings.Repeat("a", 32), strings.Repeat("b", 32)
	g := create(t, NewHub(odd{}, nil), white)
	join(t, g, black)
	out := buf.String()
	if strings.Contains(out, white) || strings.Contains(out, black) {
		t.Fatalf("a guest ID was logged:\n%s", out)
	}
	if !strings.Contains(out, "white="+guestTag(white)) || !strings.Contains(out, "black="+guestTag(black)) {
		t.Fatalf("want the guests' tags in the log:\n%s", out)
	}
}

func TestMovesAndResultsAreSaved(t *testing.T) {
	st := openStore(t)
	// The first roll opens a pothole (2, then b4), so the saved dice matter.
	g, _, _ := seated(t, NewHub(&script{rolls: []int{2, 2, 4}}, st))
	openings(t, g)
	if err := g.Resign("bob"); err != nil {
		t.Fatal(err)
	}
	saved := load(t, st)
	if len(saved) != 1 || len(saved[0].Turns) != 2 {
		t.Fatalf("saved %+v", saved)
	}
	first := saved[0].Turns[0]
	if first.Ply != 0 || first.Move != "e2e4" || len(first.Dice) != 3 || first.WhiteMS != InitialTime.Milliseconds() {
		t.Errorf("first turn %+v", first)
	}
	if r := saved[0].Result; r == nil || r.Reason != string(Resignation) || r.Winner != "white" {
		t.Errorf("result %+v, want white wins by resignation", r)
	}
}

func TestAFailedTurnSaveRetiresTheGame(t *testing.T) {
	st := &failing{}
	h := NewHub(odd{}, st)
	g, a, b := seated(t, h)
	recv(t, a)
	recv(t, b)
	st.fail = true
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrInternal) {
		t.Fatalf("move: %v, want ErrInternal", err)
	}
	for v := range b.C { // the stream closes without showing the unsaved move
		if v.Seq != 0 {
			t.Fatalf("bob saw seq %d, a move that was never saved", v.Seq)
		}
	}
	if _, ok := h.Get(g.Code()); ok {
		t.Error("the game is still listed")
	}
}

// ended records the results a hub saves.
type ended struct {
	nopStore
	mu      sync.Mutex
	results []store.Result
}

func (e *ended) EndGame(_ context.Context, _ string, r store.Result, _ *store.Turn) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.results = append(e.results, r)
	return nil
}

func TestAGameNobodyJoinedExpiresWhenEvicted(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st := &ended{}
		create(t, NewHub(odd{}, st), "alice") // nobody ever opens the link
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		st.mu.Lock()
		defer st.mu.Unlock()
		if len(st.results) != 1 || st.results[0].Reason != string(Expired) || st.results[0].Winner != "" {
			t.Fatalf("saved results %+v, want one expired game", st.results)
		}
	})
}
