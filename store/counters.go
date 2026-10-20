package store

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// upsertBatch is how many counter rows one statement writes (3 variables
// each, well under SQLite's limit).
const upsertBatch = 300

// counted runs fn in one transaction and adds the counts it collects to
// the daily counters in the same one, so a write and its counts are saved
// together or not at all. Counts left empty write nothing.
func (s *Store) counted(ctx context.Context, fn func(tx *sql.Tx, c counts) error) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		c := counts{}
		if err := fn(tx, c); err != nil {
			return err
		}
		return addCounts(ctx, tx, c)
	})
}

// addCounts adds c to the daily counters: one multi-row upsert, in the
// caller's transaction.
func addCounts(ctx context.Context, db execer, c counts) error {
	rows := make([]dayKey, 0, len(c))
	for k := range c {
		rows = append(rows, k)
	}
	// A fixed order keeps the statement's page writes in key order.
	slices.SortFunc(rows, func(a, b dayKey) int {
		return cmp.Or(strings.Compare(a.day, b.day), strings.Compare(a.key, b.key))
	})
	for len(rows) > 0 {
		n := min(len(rows), upsertBatch)
		args := make([]any, 0, 3*n)
		for _, k := range rows[:n] {
			args = append(args, k.day, k.key, c[k])
		}
		q := `INSERT INTO stats_days (day, key, n) VALUES ` + strings.TrimSuffix(strings.Repeat("(?, ?, ?), ", n), ", ") +
			` ON CONFLICT (day, key) DO UPDATE SET n = n + excluded.n`
		if _, err := db.ExecContext(ctx, q, args...); err != nil {
			return err
		}
		rows = rows[n:]
	}
	return nil
}

// DayCounts is one day's counters, key → amount.
type DayCounts struct {
	Day string // 2006-01-02, in the store's zone
	N   map[string]int
}

// Counts returns the counters for the days with fromDay <= day < toDay,
// in day order; an empty fromDay has no lower bound. Days with no counters
// are missing.
func (s *Store) Counts(ctx context.Context, fromDay, toDay string) ([]DayCounts, error) {
	var out []DayCounts
	err := eachRow(ctx, s.db, `SELECT day, key, n FROM stats_days WHERE day >= ? AND day < ? ORDER BY day, key`, []any{fromDay, toDay}, func(rows *sql.Rows) error {
		var day, key string
		var n int
		if err := rows.Scan(&day, &key, &n); err != nil {
			return err
		}
		if len(out) == 0 || out[len(out)-1].Day != day {
			out = append(out, DayCounts{Day: day, N: map[string]int{}})
		}
		out[len(out)-1].N[key] = n
		return nil
	})
	return out, err
}

// CountersStale reports that the counters were written under another
// version or zone and the rebuild that should have fixed them failed.
func (s *Store) CountersStale() bool { return s.stale.Load() }

// Zone is the zone the counters cut days in.
func (s *Store) Zone() *time.Location { return s.zone }

// rawKeep is how long the raw rows of browser errors, searches and events
// are kept: about thirteen months. Their daily counts are kept for good.
const rawKeep = 400

// rawKeys are the counters those rows feed, as SQL on stats_days.key.
const rawKeys = `(key = 'errors' OR key = 'search' OR key LIKE 'search.%' OR key LIKE 'wait.%' OR key LIKE 'gaveup.%' OR key LIKE 'event.%')`

// cutoff is the first day whose raw rows are all still kept: a midnight
// in the store's zone, so no day is ever half pruned.
func (s *Store) cutoff(now time.Time) time.Time {
	y, m, d := now.In(s.zone).AddDate(0, 0, -rawKeep).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, s.zone)
}

// Prune deletes the browser errors, searches and events from before the
// cutoff day and returns how many rows went.
func (s *Store) Prune(ctx context.Context, now time.Time) (int64, error) {
	cut := s.cutoff(now).UnixMilli()
	var total int64
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		for _, q := range []string{`DELETE FROM browser_errors WHERE at < ?`, `DELETE FROM searches WHERE started_at < ?`, `DELETE FROM events WHERE at < ?`} {
			res, err := tx.ExecContext(ctx, q, cut)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			total += n
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

// RebuildReport is what a rebuild did, for the log.
type RebuildReport struct {
	Days, Rows int
	Took       time.Duration
}

// EnsureCounters rebuilds the counters when they are from another version
// or zone, missing (a new database), or disagree with the rows (after a
// rollback to a build without counters; a check that fails counts as
// disagreeing). It returns a zero report when nothing was needed. On an
// error the old counters stay and CountersStale reports it. Run it before
// anything else uses the store: it holds the one connection.
func (s *Store) EnsureCounters(ctx context.Context, now time.Time) (RebuildReport, error) {
	var version int
	var zone string
	err := s.db.QueryRowContext(ctx, `SELECT version, zone FROM stats_state WHERE id = 1`).Scan(&version, &zone)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.stale.Store(true)
		return RebuildReport{}, err
	}
	if err == nil && version == countersVersion && zone == s.zone.String() {
		if agree, err := s.totalsAgree(ctx, now); err == nil && agree {
			s.stale.Store(false)
			return RebuildReport{}, nil
		}
	}
	start := time.Now()
	rep, err := s.rebuild(ctx, now)
	if err != nil {
		s.stale.Store(true)
		return RebuildReport{}, fmt.Errorf("rebuild counters: %w", err)
	}
	s.stale.Store(false)
	rep.Took = time.Since(start)
	return rep, nil
}

// rebuild recounts every raw row through the counting functions: each
// table is read once, summed in memory, and written in one transaction.
// Errors, searches and events are recounted from the cutoff day; the days
// before it keep their counters, since their rows were pruned.
func (s *Store) rebuild(ctx context.Context, now time.Time) (RebuildReport, error) {
	c := counts{}
	cut := s.cutoff(now)
	if err := s.recountGames(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("games: %w", err)
	}
	if err := s.recountSearches(ctx, c, cut); err != nil {
		return RebuildReport{}, fmt.Errorf("searches: %w", err)
	}
	if err := s.recountEvents(ctx, c, cut); err != nil {
		return RebuildReport{}, fmt.Errorf("events: %w", err)
	}
	if err := s.recountErrors(ctx, c, cut); err != nil {
		return RebuildReport{}, fmt.Errorf("errors: %w", err)
	}
	if err := s.keepPrunedCounts(ctx, c, DayOf(cut, s.zone)); err != nil {
		return RebuildReport{}, fmt.Errorf("pruned days: %w", err)
	}
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM stats_days`); err != nil {
			return err
		}
		if err := addCounts(ctx, tx, c); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO stats_state (id, version, zone) VALUES (1, ?, ?)
			ON CONFLICT (id) DO UPDATE SET version = excluded.version, zone = excluded.zone`, countersVersion, s.zone.String())
		return err
	})
	if err != nil {
		return RebuildReport{}, err
	}
	days := map[string]bool{}
	for k := range c {
		days[k.day] = true
	}
	return RebuildReport{Days: len(days), Rows: len(c)}, nil
}

// countedGame is the SQL condition for a game the counters count at all:
// playtests and retired games count nowhere. AddGameStats checks the same
// in Go.
const countedGame = `playtest = 0 AND COALESCE(result, '') != 'retired'`

// recountGames streams every counted game with its stats row and its last
// turn (one seek), never its other turns. A migration that retires games
// bumps countersVersion.
func (s *Store) recountGames(ctx context.Context, c counts) error {
	return eachRow(ctx, s.db, `
		SELECT g.created_at, g.kind, g.rematch_of IS NOT NULL, g.black IS NOT NULL, g.joined_at, g.result, g.winner,
		       t.ply, t.white_ms, t.black_ms,
		       s.moves, s.opened, s.reset, s.closed_rounds, s.closed_cap, s.repaired, s.fell, s.saving_rolls, s.saved, s.white_lost, s.black_lost, s.mate_by_roll, s.open_hist
		FROM games g
		LEFT JOIN turns t ON t.game = g.code AND t.ply = (SELECT MAX(ply) FROM turns WHERE game = g.code)
		LEFT JOIN game_stats s ON s.game = g.code
		WHERE `+countedGame, nil, func(rows *sql.Rows) error {
		var f gameFacts
		var created int64
		var joined, ply, whiteMS, blackMS sql.NullInt64
		var result, winner, hist sql.NullString
		var st [11]sql.NullInt64
		var mate sql.NullBool
		if err := rows.Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &joined, &result, &winner, &ply, &whiteMS, &blackMS,
			&st[0], &st[1], &st[2], &st[3], &st[4], &st[5], &st[6], &st[7], &st[8], &st[9], &st[10], &mate, &hist); err != nil {
			return err
		}
		f.Created, f.Result, f.Winner = time.UnixMilli(created), result.String, winner.String
		if joined.Valid {
			f.Joined = time.UnixMilli(joined.Int64)
		}
		if ply.Valid {
			f.Moves = int(ply.Int64) + 1
		}
		f.WinnerMS, f.HasClock = winnerClock(f.Winner, whiteMS, blackMS)
		if st[0].Valid {
			h, err := parseHist(hist.String)
			if err != nil {
				return err
			}
			n := func(i int) int { return int(st[i].Int64) }
			f.Stats = &GameStats{Moves: n(0), Opened: n(1), Reset: n(2), ClosedRounds: n(3), ClosedCap: n(4), Repaired: n(5),
				Fell: n(6), SavingRolls: n(7), Saved: n(8), WhiteLost: n(9), BlackLost: n(10), MateByRoll: mate.Bool, OpenHist: h}
		}
		countCreated(c, s.zone, f)
		countJoined(c, s.zone, f)
		if f.Result != "" {
			countEnded(c, s.zone, f)
		}
		return nil
	})
}

// winnerClock is the winner's clock from the last turn's two clocks.
func winnerClock(winner string, whiteMS, blackMS sql.NullInt64) (int64, bool) {
	switch {
	case winner == "white" && whiteMS.Valid:
		return whiteMS.Int64, true
	case winner == "black" && blackMS.Valid:
		return blackMS.Int64, true
	}
	return 0, false
}

// totalsAgree checks the counters against the rows: games made, friend
// links joined, games ended, and from the cutoff day searches, events and
// browser errors.
func (s *Store) totalsAgree(ctx context.Context, now time.Time) (bool, error) {
	cut := s.cutoff(now)
	var agree bool
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM games WHERE `+countedGame+`)
		  = (SELECT COALESCE(SUM(n), 0) FROM stats_days WHERE key IN ('games.quick', 'games.friend'))
		AND (SELECT COUNT(*) FROM games WHERE `+countedGame+` AND kind = 'friend' AND rematch_of IS NULL AND black IS NOT NULL)
		  = (SELECT COALESCE(SUM(n), 0) FROM stats_days WHERE key = 'friend.joined')
		AND (SELECT COUNT(*) FROM games WHERE `+countedGame+` AND result IS NOT NULL)
		  = (SELECT COALESCE(SUM(n), 0) FROM stats_days WHERE key LIKE 'end.%')
		AND (SELECT COUNT(*) FROM searches WHERE started_at >= ?1)
		  = (SELECT COALESCE(SUM(n), 0) FROM stats_days WHERE key = 'search' AND day >= ?2)
		AND (SELECT COALESCE(SUM(n), 0) FROM events WHERE at >= ?1)
		  = (SELECT COALESCE(SUM(n), 0) FROM stats_days WHERE key LIKE 'event.%' AND day >= ?2)
		AND (SELECT COALESCE(SUM(n), 0) FROM browser_errors WHERE at >= ?1)
		  = (SELECT COALESCE(SUM(n), 0) FROM stats_days WHERE key = 'errors' AND day >= ?2)`,
		cut.UnixMilli(), DayOf(cut, s.zone)).Scan(&agree)
	return agree, err
}

func (s *Store) recountSearches(ctx context.Context, c counts, cutoff time.Time) error {
	return eachRow(ctx, s.db, `SELECT started_at, ended_at, matched, others FROM searches WHERE started_at >= ?`, []any{cutoff.UnixMilli()}, func(rows *sql.Rows) error {
		var sr Search
		var started, ended int64
		if err := rows.Scan(&started, &ended, &sr.Matched, &sr.Others); err != nil {
			return err
		}
		sr.StartedAt, sr.EndedAt = time.UnixMilli(started), time.UnixMilli(ended)
		countSearch(c, s.zone, sr)
		return nil
	})
}

func (s *Store) recountEvents(ctx context.Context, c counts, cutoff time.Time) error {
	return eachRow(ctx, s.db, `SELECT at, kind, n FROM events WHERE at >= ?`, []any{cutoff.UnixMilli()}, func(rows *sql.Rows) error {
		var at int64
		var kind string
		var n int
		if err := rows.Scan(&at, &kind, &n); err != nil {
			return err
		}
		countEvent(c, s.zone, kind, time.UnixMilli(at), n)
		return nil
	})
}

// keepPrunedCounts keeps the counters that pruned rows fed, for the days
// before the cutoff day.
func (s *Store) keepPrunedCounts(ctx context.Context, c counts, cutoffDay string) error {
	return eachRow(ctx, s.db, `SELECT day, key, n FROM stats_days WHERE day < ? AND `+rawKeys, []any{cutoffDay}, func(rows *sql.Rows) error {
		var day, key string
		var n int
		if err := rows.Scan(&day, &key, &n); err != nil {
			return err
		}
		c.add(day, key, n)
		return nil
	})
}

// recountErrors counts the browser errors from the cutoff day on.
func (s *Store) recountErrors(ctx context.Context, c counts, cutoff time.Time) error {
	return eachRow(ctx, s.db, `SELECT at, n FROM browser_errors WHERE at >= ?`, []any{cutoff.UnixMilli()}, func(rows *sql.Rows) error {
		var at int64
		var n int
		if err := rows.Scan(&at, &n); err != nil {
			return err
		}
		countError(c, s.zone, time.UnixMilli(at), n)
		return nil
	})
}
