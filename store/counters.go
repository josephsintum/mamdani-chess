package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// upsertBatch is how many counter rows one statement writes (3 variables
// each, well under SQLite's limit).
const upsertBatch = 300

// addCounts adds c to the daily counters: one multi-row upsert, in the
// caller's transaction.
func addCounts(ctx context.Context, db execer, c counts) error {
	rows := make([]dayKey, 0, len(c))
	for k := range c {
		rows = append(rows, k)
	}
	// A fixed order keeps the statement's page writes in key order.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].day != rows[j].day {
			return rows[i].day < rows[j].day
		}
		return rows[i].key < rows[j].key
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
	rows, err := s.db.QueryContext(ctx, `SELECT day, key, n FROM stats_days WHERE day >= ? AND day < ? ORDER BY day, key`, fromDay, toDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DayCounts
	for rows.Next() {
		var day, key string
		var n int
		if err := rows.Scan(&day, &key, &n); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].Day != day {
			out = append(out, DayCounts{Day: day, N: map[string]int{}})
		}
		out[len(out)-1].N[key] = n
	}
	return out, rows.Err()
}

// CountersStale reports that the counters were written under another
// version or zone and the rebuild that should have fixed them failed.
func (s *Store) CountersStale() bool { return s.stale.Load() }

// Zone is the zone the counters cut days in.
func (s *Store) Zone() *time.Location { return s.zone }

// errorsKeep is how long browser errors' raw rows are kept: about thirteen
// months. Their daily counts are kept for good.
const errorsKeep = 400

// errorCutoff is the first day whose browser errors are all still kept:
// a midnight in the store's zone, so no day is ever half pruned.
func (s *Store) errorCutoff(now time.Time) time.Time {
	y, m, d := now.In(s.zone).AddDate(0, 0, -errorsKeep).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, s.zone)
}

// PruneErrors deletes the browser errors from before the cutoff day and
// returns how many went.
func (s *Store) PruneErrors(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM browser_errors WHERE at < ?`, s.errorCutoff(now).UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RebuildReport is what a rebuild did, for the log.
type RebuildReport struct {
	Days, Rows int
	Took       time.Duration
}

// EnsureCounters rebuilds the counters when they were written under
// another version or zone, or never (a new database, or the first start
// with counters). It returns nil and a zero report when nothing was
// needed. On an error the old counters stay and CountersStale reports it.
// Run it before anything else uses the store: it holds the one connection.
func (s *Store) EnsureCounters(ctx context.Context, now time.Time) (RebuildReport, error) {
	var version int
	var zone string
	err := s.db.QueryRowContext(ctx, `SELECT version, zone FROM stats_state WHERE id = 1`).Scan(&version, &zone)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		s.stale.Store(true)
		return RebuildReport{}, err
	}
	if err == nil && version == countersVersion && zone == s.zone.String() {
		s.stale.Store(false)
		return RebuildReport{}, nil
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
// Errors' counts before the cutoff day are kept, since their rows are gone.
func (s *Store) rebuild(ctx context.Context, now time.Time) (RebuildReport, error) {
	c := counts{}
	if err := s.recountGames(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("games: %w", err)
	}
	if err := s.recountSearches(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("searches: %w", err)
	}
	if err := s.recountEvents(ctx, c); err != nil {
		return RebuildReport{}, fmt.Errorf("events: %w", err)
	}
	cutoff := s.errorCutoff(now)
	if err := s.recountErrors(ctx, c, cutoff); err != nil {
		return RebuildReport{}, fmt.Errorf("errors: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return RebuildReport{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM stats_days WHERE NOT (key = 'errors' AND day < ?)`, dayOf(cutoff, s.zone)); err != nil {
		return RebuildReport{}, err
	}
	if err := addCounts(ctx, tx, c); err != nil {
		return RebuildReport{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO stats_state (id, version, zone) VALUES (1, ?, ?)
		ON CONFLICT (id) DO UPDATE SET version = excluded.version, zone = excluded.zone`, countersVersion, s.zone.String()); err != nil {
		return RebuildReport{}, err
	}
	if err := tx.Commit(); err != nil {
		return RebuildReport{}, err
	}
	days := map[string]bool{}
	for k := range c {
		days[k.day] = true
	}
	return RebuildReport{Days: len(days), Rows: len(c)}, nil
}

// recountGames streams every game with its stats row and its last turn's
// clocks (two primary-key seeks), never its turns.
func (s *Store) recountGames(ctx context.Context, c counts) error {
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.created_at, g.kind, g.rematch_of IS NOT NULL, g.black IS NOT NULL, g.joined_at, g.result, g.winner,
		       (SELECT white_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       (SELECT black_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       s.moves, s.opened, s.reset, s.closed_rounds, s.closed_cap, s.repaired, s.fell, s.saving_rolls, s.saved, s.white_lost, s.black_lost, s.mate_by_roll, s.open_hist
		FROM games g LEFT JOIN game_stats s ON s.game = g.code
		WHERE g.playtest = 0`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var f gameFacts
		var created int64
		var joined, whiteMS, blackMS sql.NullInt64
		var result, winner, hist sql.NullString
		var st [11]sql.NullInt64
		var mate sql.NullBool
		if err := rows.Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &joined, &result, &winner, &whiteMS, &blackMS,
			&st[0], &st[1], &st[2], &st[3], &st[4], &st[5], &st[6], &st[7], &st[8], &st[9], &st[10], &mate, &hist); err != nil {
			return err
		}
		f.Created, f.Result, f.Winner = time.UnixMilli(created), result.String, winner.String
		if joined.Valid {
			f.Joined = time.UnixMilli(joined.Int64)
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
		// Retired games were ended by a migration, which counts nothing.
		if f.Result != "" && f.Result != "retired" {
			countEnded(c, s.zone, f)
		}
	}
	return rows.Err()
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

func (s *Store) recountSearches(ctx context.Context, c counts) error {
	rows, err := s.db.QueryContext(ctx, `SELECT started_at, ended_at, matched, others FROM searches`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sr Search
		var started, ended int64
		if err := rows.Scan(&started, &ended, &sr.Matched, &sr.Others); err != nil {
			return err
		}
		sr.StartedAt, sr.EndedAt = time.UnixMilli(started), time.UnixMilli(ended)
		countSearch(c, s.zone, sr)
	}
	return rows.Err()
}

func (s *Store) recountEvents(ctx context.Context, c counts) error {
	rows, err := s.db.QueryContext(ctx, `SELECT at, kind FROM events`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var at int64
		var kind string
		if err := rows.Scan(&at, &kind); err != nil {
			return err
		}
		countEvent(c, s.zone, kind, time.UnixMilli(at))
	}
	return rows.Err()
}

// recountErrors counts the browser errors from the cutoff day on; the
// counts before it are kept as they are.
func (s *Store) recountErrors(ctx context.Context, c counts, cutoff time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT at FROM browser_errors WHERE at >= ?`, cutoff.UnixMilli())
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var at int64
		if err := rows.Scan(&at); err != nil {
			return err
		}
		countError(c, s.zone, time.UnixMilli(at))
	}
	return rows.Err()
}
