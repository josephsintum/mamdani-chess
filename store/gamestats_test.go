package store

import (
	"testing"
	"time"
)

func TestGameStatsSavedWithTheResult(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.CreateGame(ctx, Game{Code: "G00001", White: "w", WhiteName: "w", Kind: "quick", CreatedAt: t0}))
	must(t, s.SeatBlack(ctx, "G00001", "b", "b", t0.Add(90*time.Second)))
	must(t, s.AddTurn(ctx, "G00001", Turn{Ply: 0, Move: "e2e4", At: t0.Add(2 * time.Minute)}))
	st := GameStats{Moves: 2, Opened: 1, Fell: 1, BlackLost: 1, SavingRolls: 1, OpenHist: [6]int{1, 1}}
	must(t, s.EndGame(ctx, "G00001", Result{EndedAt: t0.Add(3 * time.Minute), Reason: "checkmate", Winner: "white"},
		&Turn{Ply: 1, Move: "e7e5", At: t0.Add(3 * time.Minute)}, &st))

	var kind string
	var joined int64
	var moves, fell, blackLost int
	var hist string
	must(t, s.db.QueryRowContext(ctx, `SELECT kind, joined_at FROM games WHERE code = 'G00001'`).Scan(&kind, &joined))
	must(t, s.db.QueryRowContext(ctx, `SELECT moves, fell, black_lost, open_hist FROM game_stats WHERE game = 'G00001'`).Scan(&moves, &fell, &blackLost, &hist))
	if kind != "quick" || joined != t0.Add(90*time.Second).UnixMilli() || moves != 2 || fell != 1 || blackLost != 1 || hist != "1,1,0,0,0,0" {
		t.Fatalf("saved %q %d %d %d %d %q", kind, joined, moves, fell, blackLost, hist)
	}
	// A default kind, and a game ended without stats, both fine.
	must(t, s.CreateGame(ctx, Game{Code: "G00002", White: "w", WhiteName: "w", CreatedAt: t0}))
	must(t, s.EndGame(ctx, "G00002", Result{EndedAt: t0.Add(time.Hour), Reason: "expired"}, nil, nil))
	var n int
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM game_stats`).Scan(&n))
	must(t, s.db.QueryRowContext(ctx, `SELECT kind FROM games WHERE code = 'G00002'`).Scan(&kind))
	if n != 1 || kind != "friend" {
		t.Fatalf("rows %d, kind %q", n, kind)
	}
}

func TestGamesWithoutStatsAndBackfillRow(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	// Three games: one finished without stats (needs a backfill), one with
	// stats, one that never got going.
	for _, g := range []struct{ code, reason string }{{"G00001", "resignation"}, {"G00002", "checkmate"}, {"G00003", "aborted"}} {
		must(t, s.CreateGame(ctx, Game{Code: g.code, White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: t0}))
		must(t, s.AddTurn(ctx, g.code, Turn{Ply: 0, Move: "e2e4", Dice: []int{1}, At: t0}))
		must(t, s.EndGame(ctx, g.code, Result{EndedAt: t0.Add(time.Minute), Reason: g.reason, Winner: "white"}, nil, nil))
	}
	must(t, s.AddGameStats(ctx, "G00002", GameStats{Moves: 1}))
	todo, err := s.GamesWithoutStats(ctx)
	must(t, err)
	if len(todo) != 1 || todo[0].Code != "G00001" || len(todo[0].Turns) != 1 || todo[0].Result == nil || todo[0].Result.Reason != "resignation" {
		t.Fatalf("to backfill: %+v", todo)
	}
	must(t, s.AddGameStats(ctx, "G00001", GameStats{Moves: 1}))
	must(t, s.AddGameStats(ctx, "G00001", GameStats{Moves: 99})) // a second write is ignored
	var moves int
	must(t, s.db.QueryRowContext(ctx, `SELECT moves FROM game_stats WHERE game = 'G00001'`).Scan(&moves))
	if moves != 1 {
		t.Fatalf("moves = %d after a repeat, want 1", moves)
	}
	todo, err = s.GamesWithoutStats(ctx)
	must(t, err)
	if len(todo) != 0 {
		t.Fatalf("still to backfill: %d", len(todo))
	}
}

func TestSearchesAndEvents(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(19 * time.Second), Guest: "ann000000000", Matched: true, Others: 1}))
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(65 * time.Second), Guest: "bob000000000", Matched: false, Others: 0}))
	must(t, s.AddEvent(ctx, "practice", t0))
	must(t, s.AddEvent(ctx, "restart", t0.Add(time.Hour)))
	var matched, others, events int
	must(t, s.db.QueryRowContext(ctx, `SELECT SUM(matched), SUM(others) FROM searches`).Scan(&matched, &others))
	must(t, s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE kind IN ('practice', 'restart')`).Scan(&events))
	if matched != 1 || others != 1 || events != 2 {
		t.Fatalf("matched %d others %d events %d", matched, others, events)
	}
}

func TestSearchesAndCountEventsByRange(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	must(t, s.AddSearch(ctx, Search{StartedAt: t0.Add(time.Hour), EndedAt: t0.Add(time.Hour + time.Second), Guest: "bob000000000", Others: 2}))
	must(t, s.AddSearch(ctx, Search{StartedAt: t0, EndedAt: t0.Add(19 * time.Second), Guest: "ann000000000", Matched: true}))
	rows, err := s.Searches(ctx, t0.Add(-time.Minute), t0.Add(30*time.Minute))
	must(t, err)
	if len(rows) != 1 || rows[0].Guest != "ann000000000" || !rows[0].Matched || !rows[0].StartedAt.Equal(t0) || !rows[0].EndedAt.Equal(t0.Add(19*time.Second)) {
		t.Fatalf("one in range: %+v", rows)
	}
	rows, err = s.Searches(ctx, time.Time{}, t0.Add(2*time.Hour))
	must(t, err)
	if len(rows) != 2 || rows[0].Guest != "ann000000000" || rows[1].Others != 2 {
		t.Fatalf("both, oldest first: %+v", rows)
	}

	must(t, s.AddEvent(ctx, "practice", t0))
	must(t, s.AddEvent(ctx, "practice", t0.Add(time.Hour)))
	must(t, s.AddEvent(ctx, "restart", t0))
	n, err := s.CountEvents(ctx, "practice", t0.Add(-time.Minute), t0.Add(time.Minute))
	must(t, err)
	if n != 1 {
		t.Fatalf("practice in range: %d", n)
	}
}
