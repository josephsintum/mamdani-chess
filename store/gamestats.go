package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
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
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	added, err := addGameStats(ctx, tx, code, st)
	if err != nil || !added {
		return err
	}
	var created int64
	var result, winner sql.NullString
	var playtest bool
	f := gameFacts{}
	err = tx.QueryRowContext(ctx, `SELECT created_at, kind, rematch_of IS NOT NULL, black IS NOT NULL, result, winner, playtest FROM games WHERE code = ?`, code).
		Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &result, &winner, &playtest)
	if errors.Is(err, sql.ErrNoRows) {
		return tx.Commit() // a stats row for no game: nothing to count
	}
	if err != nil {
		return err
	}
	if !playtest && result.Valid && result.String != "retired" {
		f.Created, f.Result, f.Winner = time.UnixMilli(created), result.String, winner.String
		if err := lastTurn(ctx, tx, code, &f); err != nil {
			return err
		}
		before := counts{}
		countEnded(before, s.zone, f)
		f.Stats = &st
		after := counts{}
		countEnded(after, s.zone, f)
		if err := addCounts(ctx, tx, after.minus(before)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// addGameStats writes a stats row unless the game has one, and reports
// whether it wrote.
func addGameStats(ctx context.Context, db execer, code string, st GameStats) (bool, error) {
	res, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		code, st.Moves, st.Opened, st.Reset, st.ClosedRounds, st.ClosedCap, st.Repaired, st.Fell, st.SavingRolls, st.Saved, st.WhiteLost, st.BlackLost, st.MateByRoll, formatHist(st.OpenHist))
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
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+savedGameColumns+`
		 FROM games WHERE rules = ? AND result IN `+finishedReasons+` AND code NOT IN (SELECT game FROM game_stats) ORDER BY created_at`,
		rulesVersion)
	if err != nil {
		return nil, err
	}
	var games []SavedGame
	for rows.Next() {
		g, err := scanSavedGame(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		games = append(games, g)
	}
	rows.Close() // one connection: close before the next query
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range games {
		if games[i].Turns, err = s.turns(ctx, games[i].Code); err != nil {
			return nil, err
		}
	}
	return games, nil
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
	c := counts{}
	countSearch(c, s.zone, sr)
	return s.insertCounted(ctx, c,
		`INSERT INTO searches (started_at, ended_at, matched, others) VALUES (?, ?, ?, ?)`,
		sr.StartedAt.UnixMilli(), sr.EndedAt.UnixMilli(), sr.Matched, sr.Others)
}

// AddEvent logs one event of kind ("practice", "reconnect", "restart") and
// counts it.
func (s *Store) AddEvent(ctx context.Context, kind string, at time.Time) error {
	c := counts{}
	countEvent(c, s.zone, kind, at, 1)
	return s.insertFolded(ctx, c, DayOf(at, s.zone), "event."+kind, eventRowsPerDay,
		`INSERT INTO events (at, kind) VALUES (?, ?)`,
		`UPDATE events SET at = ?, n = n + 1 WHERE rowid = (SELECT rowid FROM events WHERE kind = ?2 ORDER BY rowid DESC LIMIT 1)`,
		at.UnixMilli(), kind)
}

// insertCounted runs one raw insert and adds its counts, in one
// transaction (one commit, as the insert alone was).
func (s *Store) insertCounted(ctx context.Context, c counts, query string, args ...any) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	if err := addCounts(ctx, tx, c); err != nil {
		return err
	}
	return tx.Commit()
}

// formatHist writes [6]int as "9,14,19,22,18,18"; parseHist reads it back.
func formatHist(h [6]int) string {
	parts := make([]string, len(h))
	for i, n := range h {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func parseHist(s string) ([6]int, error) {
	var h [6]int
	parts := strings.Split(s, ",")
	if len(parts) != len(h) {
		return h, fmt.Errorf("bad open_hist %q", s)
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return h, fmt.Errorf("bad open_hist %q", s)
		}
		h[i] = n
	}
	return h, nil
}
