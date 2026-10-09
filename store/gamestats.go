package store

import (
	"context"
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

// Search is one quick-match search: who looked (the 12-hex visitor key),
// for how long, whether it found an opponent, and how many others were
// looking when it started.
type Search struct {
	StartedAt, EndedAt time.Time
	Guest              string
	Matched            bool
	Others             int
}

// AddGameStats writes a game's stats on their own (the backfill). A game
// that has a row keeps it.
func (s *Store) AddGameStats(ctx context.Context, code string, st GameStats) error {
	return addGameStats(ctx, s.db, code, st)
}

func addGameStats(ctx context.Context, db execer, code string, st GameStats) error {
	_, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO game_stats (game, moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost, mate_by_roll, open_hist)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		code, st.Moves, st.Opened, st.Reset, st.ClosedRounds, st.ClosedCap, st.Repaired, st.Fell, st.SavingRolls, st.Saved, st.WhiteLost, st.BlackLost, st.MateByRoll, formatHist(st.OpenHist))
	return err
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

// AddSearch logs one quick-match search.
func (s *Store) AddSearch(ctx context.Context, sr Search) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO searches (started_at, ended_at, guest, matched, others) VALUES (?, ?, ?, ?, ?)`,
		sr.StartedAt.UnixMilli(), sr.EndedAt.UnixMilli(), sr.Guest, sr.Matched, sr.Others)
	return err
}

// AddEvent counts one event of kind ("practice", "reconnect", "restart").
func (s *Store) AddEvent(ctx context.Context, kind string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO events (at, kind) VALUES (?, ?)`, at.UnixMilli(), kind)
	return err
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

// Searches returns the searches that started in [from, to), oldest first.
func (s *Store) Searches(ctx context.Context, from, to time.Time) ([]Search, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT started_at, ended_at, guest, matched, others FROM searches
		 WHERE started_at >= ? AND started_at < ? ORDER BY started_at, rowid`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Search
	for rows.Next() {
		var sr Search
		var started, ended int64
		if err := rows.Scan(&started, &ended, &sr.Guest, &sr.Matched, &sr.Others); err != nil {
			return nil, err
		}
		sr.StartedAt, sr.EndedAt = time.UnixMilli(started), time.UnixMilli(ended)
		out = append(out, sr)
	}
	return out, rows.Err()
}

// CountEvents counts the events of kind that happened in [from, to).
func (s *Store) CountEvents(ctx context.Context, kind string, from, to time.Time) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE kind = ? AND at >= ? AND at < ?`,
		kind, from.UnixMilli(), to.UnixMilli()).Scan(&n)
	return n, err
}
