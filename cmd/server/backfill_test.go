package main

import (
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

// With the database gone, the backfill, the counters and the restart count
// each log at ERROR and startup goes on.
func TestPrepareStatsNeverStopsTheServer(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	var buf bytes.Buffer
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	prepareStats(context.Background(), st, time.Now())
	out := buf.String()
	if strings.Count(out, "level=ERROR") != 3 || !strings.Contains(out, "restart not counted") || !strings.Contains(out, "game stats backfill stopped") ||
		!strings.Contains(out, "stats counters not rebuilt") {
		t.Fatalf("logged %q", out)
	}
}

// savedWithoutStats saves a finished game without stats, as games were
// before stats were kept.
func savedWithoutStats(t *testing.T, st *store.Store, now time.Time) {
	t.Helper()
	ctx := t.Context()
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
}

// The counters are rebuilt before the backfill, so the backfill's counts
// add to rebuilt counters instead of being thrown away by the rebuild.
func TestPrepareStatsRebuildsBeforeBackfilling(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	savedWithoutStats(t, st, time.Now())
	var buf bytes.Buffer
	defer slog.SetDefault(slog.Default())
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	prepareStats(t.Context(), st, time.Now())
	out := buf.String()
	rebuilt, backfilled := strings.Index(out, "stats rebuilt"), strings.Index(out, "game stats backfilled")
	if rebuilt < 0 || backfilled < 0 || rebuilt > backfilled {
		t.Fatalf("want the rebuild logged before the backfill:\n%s", out)
	}
}

func TestBackfillSkipsGamesWithStats(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	savedWithoutStats(t, st, time.Now())
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
