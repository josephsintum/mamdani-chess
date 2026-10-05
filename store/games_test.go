package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

var t0 = time.UnixMilli(1_791_000_000_000)

func TestGameRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	must(t, s.CreateGame(ctx, Game{Code: "ABC123", White: "alice", WhiteName: "pigeon-astoria", CreatedAt: t0}))
	must(t, s.SeatBlack(ctx, "ABC123", "bob", "bagel-soho"))
	turns := []Turn{
		{Ply: 0, Move: "e2e4", Dice: []int{4, 3, 5}, WhiteMS: 605000, BlackMS: 600000, At: t0.Add(5 * time.Second)},
		{Ply: 1, Move: "e7e8q", Dice: nil, WhiteMS: 605000, BlackMS: 597000, At: t0.Add(10 * time.Second)},
	}
	for _, tn := range turns {
		must(t, s.AddTurn(ctx, "ABC123", tn))
	}
	s.Close()

	s, err := Open(path) // a restart
	must(t, err)
	defer s.Close()
	got, err := s.LoadForRestore(ctx, t0)
	must(t, err)
	want := []SavedGame{{Game: Game{Code: "ABC123", White: "alice", Black: "bob", WhiteName: "pigeon-astoria", BlackName: "bagel-soho", CreatedAt: t0}, Turns: turns}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loaded\n%+v\nwant\n%+v", got, want)
	}
}

func TestEndGameSavesTheFinalTurnAndResult(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	must(t, s.CreateGame(ctx, Game{Code: "MATE01", White: "a", Black: "b", CreatedAt: t0, RematchOf: ""}))
	final := Turn{Ply: 0, Move: "f2f3", Dice: []int{1}, WhiteMS: 1, BlackMS: 2, At: t0.Add(time.Second)}
	r := Result{EndedAt: t0.Add(time.Second), Reason: "checkmate", Winner: "white"}
	must(t, s.EndGame(ctx, "MATE01", r, &final))
	got, err := s.LoadForRestore(ctx, t0)
	must(t, err)
	if len(got) != 1 || !reflect.DeepEqual(got[0].Result, &r) || !reflect.DeepEqual(got[0].Turns, []Turn{final}) {
		t.Fatalf("loaded %+v", got)
	}
}

func TestEndGameWithoutAFinalTurn(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	must(t, s.CreateGame(ctx, Game{Code: "RESIGN", White: "a", Black: "b", CreatedAt: t0}))
	must(t, s.EndGame(ctx, "RESIGN", Result{EndedAt: t0, Reason: "aborted"}, nil))
	got, err := s.LoadForRestore(ctx, t0.Add(-time.Second))
	must(t, err)
	if len(got) != 1 || got[0].Result == nil || got[0].Result.Winner != "" || got[0].Turns != nil {
		t.Fatalf("loaded %+v", got)
	}
}

func TestEndGameFailsForAnUnknownGame(t *testing.T) {
	s, _ := openTemp(t)
	defer s.Close()
	if err := s.EndGame(context.Background(), "NOPE00", Result{EndedAt: t0, Reason: "timeout"}, nil); err == nil {
		t.Fatal("ending a game that was never saved should fail")
	}
}

func TestAddTurnRejectsADuplicatePly(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	must(t, s.CreateGame(ctx, Game{Code: "DUP001", White: "a", CreatedAt: t0}))
	tn := Turn{Ply: 0, Move: "e2e4", At: t0}
	must(t, s.AddTurn(ctx, "DUP001", tn))
	if err := s.AddTurn(ctx, "DUP001", tn); err == nil {
		t.Fatal("a second turn with the same ply should fail")
	}
}

func TestCreateGameCodeTaken(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	must(t, s.CreateGame(ctx, Game{Code: "SAME00", White: "a", CreatedAt: t0}))
	err := s.CreateGame(ctx, Game{Code: "SAME00", White: "b", CreatedAt: t0})
	if !errors.Is(err, ErrCodeTaken) {
		t.Fatalf("got %v, want ErrCodeTaken", err)
	}
}

func TestLoadForRestoreSkipsOldFinishedGames(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	for _, c := range []string{"OLD000", "RECENT", "OPEN00"} {
		must(t, s.CreateGame(ctx, Game{Code: c, White: "a", Black: "b", CreatedAt: t0}))
	}
	must(t, s.EndGame(ctx, "OLD000", Result{EndedAt: t0.Add(time.Hour), Reason: "timeout", Winner: "black"}, nil))
	must(t, s.EndGame(ctx, "RECENT", Result{EndedAt: t0.Add(3 * time.Hour), Reason: "timeout", Winner: "black"}, nil))
	got, err := s.LoadForRestore(ctx, t0.Add(2*time.Hour))
	must(t, err)
	var codes []string
	for _, g := range got {
		codes = append(codes, g.Code)
	}
	if !reflect.DeepEqual(codes, []string{"RECENT", "OPEN00"}) && !reflect.DeepEqual(codes, []string{"OPEN00", "RECENT"}) {
		t.Fatalf("loaded %v, want RECENT and OPEN00", codes)
	}
}

func TestExpireWaiting(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	must(t, s.CreateGame(ctx, Game{Code: "STALE0", White: "a", CreatedAt: t0}))
	must(t, s.CreateGame(ctx, Game{Code: "FRESH0", White: "a", CreatedAt: t0.Add(2 * time.Hour)}))
	must(t, s.CreateGame(ctx, Game{Code: "SEATED", White: "a", Black: "b", CreatedAt: t0}))
	now := t0.Add(3 * time.Hour)
	n, err := s.ExpireWaiting(ctx, t0.Add(time.Hour), now)
	must(t, err)
	if n != 1 {
		t.Fatalf("expired %d games, want 1", n)
	}
	got, err := s.LoadForRestore(ctx, now) // only games still unfinished
	must(t, err)
	if len(got) != 2 || got[0].Code == "STALE0" || got[1].Code == "STALE0" {
		t.Fatalf("loaded %+v; STALE0 should have expired", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
