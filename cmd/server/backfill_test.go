package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"mamdani-chess/store"
)

func TestBackfillSkipsGamesWithStats(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now()
	// A finished game saved without stats, as games were before this plan.
	if err := st.CreateGame(ctx, store.Game{Code: "G00001", White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	for i, tn := range []store.Turn{{Ply: 0, Move: "e2e4", Dice: []int{1}}, {Ply: 1, Move: "e7e5", Dice: []int{1}}} {
		tn.At = now.Add(time.Duration(i) * time.Second)
		if err := st.AddTurn(ctx, "G00001", tn); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.EndGame(ctx, "G00001", store.Result{EndedAt: now.Add(time.Minute), Reason: "resignation", Winner: "white"}, nil, nil); err != nil {
		t.Fatal(err)
	}
	done, failed, err := backfillStats(ctx, st)
	if err != nil || done != 1 || failed != 0 {
		t.Fatalf("first run: %d %d %v", done, failed, err)
	}
	done, failed, err = backfillStats(ctx, st)
	if err != nil || done != 0 || failed != 0 {
		t.Fatalf("second run: %d %d %v", done, failed, err)
	}
	todo, err := st.GamesWithoutStats(ctx)
	if err != nil || len(todo) != 0 {
		t.Fatalf("left: %d %v", len(todo), err)
	}
}
