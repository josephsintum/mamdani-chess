package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"time"
)

// GameStats is what the road did to one game, counted from its dice:
// potholes opened, reset by a roll landing on them, closed by their rounds
// or by the cap, repaired by the Mamdani; pieces that fell, saving rolls
// and saves, pieces each side lost to a pothole; whether the dice
// delivered the mate; and OpenHist[k], the turns that ended with k holes
// open.
type GameStats struct {
	Moves, Opened, Reset, ClosedRounds, ClosedCap, Repaired int
	Fell, SavingRolls, Saved, WhiteLost, BlackLost          int
	MateByRoll                                              bool
	OpenHist                                                [6]int
}

// Search is one quick-match search: how long it lasted, whether it found
// an opponent, and how many others were looking when it started. Nothing
// says who searched.
type Search struct {
	StartedAt, EndedAt time.Time
	Matched            bool
	Others             int
}

// AddGameStats writes a game's stats on their own (the backfill) and
// moves its counters from "ended, road unknown" to what the stats say. A
// game that has a row keeps it and counts nothing.
func (s *Store) AddGameStats(ctx context.Context, code string, st GameStats) error {
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		added, err := addGameStats(ctx, tx, code, st)
		if err != nil || !added {
			return err
		}
		var f gameFacts
		var result, winner sql.NullString
		playtest, err := scanFacts(tx.QueryRowContext(ctx, `SELECT `+factsColumns+`, result, winner FROM games WHERE code = ?`, code),
			&f, &result, &winner)
		if errors.Is(err, sql.ErrNoRows) {
			return nil // a stats row for no game: nothing to count
		}
		if err != nil {
			return err
		}
		// An ended game that counts at all (countedGame).
		if playtest || !result.Valid || result.String == "retired" {
			return nil
		}
		f.Result, f.Winner = result.String, winner.String
		if err := lastTurn(ctx, tx, code, &f); err != nil {
			return err
		}
		before := counts{}
		countEnded(before, s.zone, f)
		f.Stats = &st
		after := counts{}
		countEnded(after, s.zone, f)
		maps.Copy(c, after.minus(before))
		return nil
	})
}

// addGameStats writes a stats row unless the game has one, and reports
// whether it wrote.
func addGameStats(ctx context.Context, db execer, code string, st GameStats) (bool, error) {
	res, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		code, st.Moves, st.Opened, st.Reset, st.ClosedRounds, st.ClosedCap, st.Repaired, st.Fell, st.SavingRolls, st.Saved, st.WhiteLost, st.BlackLost, st.MateByRoll, formatInts(st.OpenHist[:]))
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// GamesWithoutStats returns every game under the current rules that got
// going and has no stats row yet, with its turns, oldest first. Playtests'
// games are included: their rows are kept, just not shown.
func (s *Store) GamesWithoutStats(ctx context.Context) ([]SavedGame, error) {
	return s.savedGames(ctx, `rules = ? AND result IN `+finishedReasons+` AND code NOT IN (SELECT game FROM game_stats)`, rulesVersion)
}

// GameKind returns a saved game's kind and whether a playtest started it.
func (s *Store) GameKind(ctx context.Context, code string) (kind string, playtest bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT kind, playtest FROM games WHERE code = ?`, code).Scan(&kind, &playtest)
	return kind, playtest, err
}

// HasGameStats reports whether a game has a stats row.
func (s *Store) HasGameStats(ctx context.Context, code string) (bool, error) {
	var has bool
	err := s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM game_stats WHERE game = ?)`, code).Scan(&has)
	return has, err
}

// AddSearch logs one quick-match search and counts it.
func (s *Store) AddSearch(ctx context.Context, sr Search) error {
	// No cap yet: milestone 07's per-IP limits will bound searches.
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		countSearch(c, s.zone, sr)
		_, err := tx.ExecContext(ctx, `INSERT INTO searches (started_at, ended_at, matched, others) VALUES (?, ?, ?, ?)`,
			sr.StartedAt.UnixMilli(), sr.EndedAt.UnixMilli(), sr.Matched, sr.Others)
		return err
	})
}

// AddEvent logs one event of kind ("practice", "reconnect", "restart") and
// counts it.
func (s *Store) AddEvent(ctx context.Context, kind string, at time.Time) error {
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		countEvent(c, s.zone, kind, at, 1)
		return insertFolded(ctx, tx, DayOf(at, s.zone), "event."+kind, eventRowsPerDay,
			`INSERT INTO events (at, kind) VALUES (?, ?)`,
			`UPDATE events SET at = ?, n = n + 1 WHERE rowid = (SELECT rowid FROM events WHERE kind = ?2 ORDER BY rowid DESC LIMIT 1)`,
			at.UnixMilli(), kind)
	})
}

// parseHist reads back a [6]int that formatInts wrote, "9,14,19,22,18,18".
func parseHist(s string) ([6]int, error) {
	var h [6]int
	ns, err := parseInts(s)
	if err != nil || len(ns) != len(h) {
		return h, fmt.Errorf("bad open_hist %q", s)
	}
	copy(h[:], ns)
	return h, nil
}
