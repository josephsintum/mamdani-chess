package store

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

// Run with: go test ./store -run XXX -bench . -benchtime 5x
// The numbers go in the PR; Railway's vCPU is about 3–5× slower.

// BenchmarkEndGame is one counted game end: the update, the final turn,
// the stats row, the clock seek and the counters, in one transaction.
func BenchmarkEndGame(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	ctx := b.Context()
	st := &GameStats{Moves: 40, Opened: 12, Reset: 1, ClosedRounds: 9, ClosedCap: 2, Repaired: 1, Fell: 3, SavingRolls: 2, Saved: 1, WhiteLost: 1, BlackLost: 2, OpenHist: [6]int{4, 8, 10, 9, 6, 3}}
	for i := 0; b.Loop(); i++ {
		code := fmt.Sprintf("B%05d", i)
		b.StopTimer()
		must(b, s.CreateGame(ctx, Game{Code: code, White: "w", Black: "b", Kind: "quick", CreatedAt: t0}))
		b.StartTimer()
		final := &Turn{Ply: 0, Move: "e2e4", WhiteMS: 300_000, BlackMS: 200_000, At: t0}
		must(b, s.EndGame(ctx, code, Result{EndedAt: t0, Reason: "checkmate", Winner: "white"}, final, st))
	}
}

// seedGames writes n finished quick games with stats and two turns each,
// spread over days, straight in SQL (fast enough for 100,000).
func seedGames(b *testing.B, s *Store, n, days int) {
	b.Helper()
	ctx := b.Context()
	start := t0.AddDate(0, 0, -days).UnixMilli()
	step := int64(days) * int64(24*time.Hour/time.Millisecond) / int64(n)
	for _, q := range []string{
		`WITH RECURSIVE i(x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM i WHERE x < ? - 1)
		 INSERT INTO games (code, white, black, created_at, ended_at, result, winner, rules, kind, joined_at)
		 SELECT printf('S%07d', x), 'w', 'b', ? + x * ?, ? + x * ? + 600000, 'checkmate', 'white', 3, 'quick', ? + x * ? FROM i`,
	} {
		if _, err := s.db.ExecContext(ctx, q, n, start, step, start, step, start, step); err != nil {
			b.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO game_stats SELECT code, 40, 12, 1, 9, 2, 1, 3, 2, 1, 1, 2, 0, '4,8,10,9,6,3' FROM games`,
		`INSERT INTO turns SELECT code, 0, 'e2e4', '', 600000, 600000, created_at FROM games`,
		`INSERT INTO turns SELECT code, 1, 'e7e5', '', 590000, 595000, created_at FROM games`,
	} {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRebuild recounts 100,000 games over 400 days. Budget: under 3 s
// on Railway, so under about 0.6–1 s here.
func BenchmarkRebuild(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	seedGames(b, s, 100_000, 400)
	for b.Loop() {
		if _, err := s.rebuild(b.Context(), t0); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkStartChecks is what every start does when the counters are
// current: prune the raw rows and check the totals. Over 100,000 games and
// 400 days of 1,000 searches and 1,000 events a day, the oldest day's past
// the cutoff.
func BenchmarkStartChecks(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	ctx := b.Context()
	seedGames(b, s, 100_000, 400)
	start := t0.AddDate(0, 0, -401).UnixMilli()
	step := int64(401*24*time.Hour/time.Millisecond) / 401_000
	for _, q := range []string{
		`WITH RECURSIVE i(x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM i WHERE x < 401000 - 1)
		 INSERT INTO searches (started_at, ended_at, matched, others) SELECT ? + x * ?, ? + x * ? + 20000, x % 2, x % 3 FROM i`,
		`WITH RECURSIVE i(x) AS (SELECT 0 UNION ALL SELECT x + 1 FROM i WHERE x < 401000 - 1)
		 INSERT INTO events (at, kind) SELECT ? + x * ?, 'reconnect' FROM i`,
	} {
		if _, err := s.db.ExecContext(ctx, q, start, step, start, step); err != nil {
			b.Fatal(err)
		}
	}
	if _, err := s.rebuild(ctx, t0); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := s.Prune(ctx, t0); err != nil {
			b.Fatal(err)
		}
		if agree, err := s.totalsAgree(ctx, t0); err != nil || !agree {
			b.Fatalf("agree %v, %v", agree, err)
		}
	}
}

// BenchmarkCounts reads a 90-day page and its range before (180 days) out
// of 400 days of counters, the way the page does. It doesn't depend on how
// many games there were.
func BenchmarkCounts(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	seedGames(b, s, 20_000, 400)
	if _, err := s.rebuild(b.Context(), t0); err != nil {
		b.Fatal(err)
	}
	to := DayOf(t0.AddDate(0, 0, 1), time.UTC)
	from := DayOf(t0.AddDate(0, 0, -179), time.UTC)
	for b.Loop() {
		days, err := s.Counts(b.Context(), from, to)
		if err != nil {
			b.Fatal(err)
		}
		_ = Summarize(days, from, to)
	}
}
