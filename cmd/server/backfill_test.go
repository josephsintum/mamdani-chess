package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

// With the database gone, the prune, the counters, the backfill and the
// restart count each log at ERROR and startup goes on.
func TestPrepareStatsNeverStopsTheServer(t *testing.T) {
	st := openStore(t, filepath.Join(t.TempDir(), "test.db"))
	st.Close()
	buf := captureLog(t)
	prepareStats(context.Background(), st, time.Now())
	out := buf.String()
	if strings.Count(out, "level=ERROR") != 4 || !strings.Contains(out, "old stats rows not pruned") || !strings.Contains(out, "restart not counted") ||
		!strings.Contains(out, "game stats backfill stopped") || !strings.Contains(out, "stats counters not rebuilt") {
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
// add to rebuilt counters instead of being thrown away by the rebuild. (A
// second backfill finds nothing: store's TestGamesWithoutStatsAndBackfillRow.)
func TestPrepareStatsRebuildsBeforeBackfilling(t *testing.T) {
	st := openStore(t, filepath.Join(t.TempDir(), "test.db"))
	savedWithoutStats(t, st, time.Now())
	buf := captureLog(t)
	prepareStats(t.Context(), st, time.Now())
	out := buf.String()
	rebuilt, backfilled := strings.Index(out, "stats rebuilt"), strings.Index(out, "game stats backfilled")
	if rebuilt < 0 || backfilled < 0 || rebuilt > backfilled || !strings.Contains(out, "games=1 failed=0") {
		t.Fatalf("want the rebuild logged before the backfill of one game:\n%s", out)
	}
}
