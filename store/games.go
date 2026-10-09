package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// ErrCodeTaken is returned by CreateGame when a saved game already has the code.
var ErrCodeTaken = errors.New("game code already taken")

// rulesVersion is the rules new games are saved under: 3 since a roll
// onto an open pothole resets it (2 from milestone 06c's three-round
// potholes). Games saved under other rules are never loaded, since their
// turns would not replay the same.
const rulesVersion = 3

// Game is a saved game's seats. Black is "" until someone joins. The names
// are the players' names when they sat down ("" for games saved before
// names existed).
type Game struct {
	Code      string
	White     string
	Black     string
	WhiteName string
	BlackName string
	CreatedAt time.Time
	RematchOf string // "" unless the game is a rematch
	// Kind is "friend" (a link) or "quick" (quick match); a rematch keeps
	// the kind of the game it follows. "" saves as friend.
	Kind string
	// Playtest marks a game a playtest started (the X-Playtest header or a
	// headless browser); the stats leave it out. A rematch keeps it.
	Playtest bool
}

// Turn is one saved move: the move in UCI ("e2e4"), every d8 rolled that
// turn in order, and both clocks after it (increment included).
type Turn struct {
	Ply     int
	Move    string
	Dice    []int
	WhiteMS int64
	BlackMS int64
	At      time.Time
}

// Result is how a game ended. Winner is "white", "black", or "" for a draw
// or a game nobody won (aborted, expired, retired).
type Result struct {
	EndedAt time.Time
	Reason  string
	Winner  string
}

// SavedGame is a game as loaded at startup. Result is nil while unfinished.
type SavedGame struct {
	Game
	Turns    []Turn
	Result   *Result
	JoinedAt time.Time // when Black sat down; zero when unknown
}

// CreateGame saves a new game.
func (s *Store) CreateGame(ctx context.Context, g Game) error {
	kind := g.Kind
	if kind == "" {
		kind = "friend"
	}
	var joined any
	if g.Black != "" {
		joined = g.CreatedAt.UnixMilli() // both seated from the start: quick match or a rematch
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO games (code, white, black, white_name, black_name, created_at, rematch_of, rules, kind, joined_at, playtest) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.Code, g.White, nullable(g.Black), nullable(g.WhiteName), nullable(g.BlackName), g.CreatedAt.UnixMilli(), nullable(g.RematchOf), rulesVersion, kind, joined, g.Playtest)
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
		return ErrCodeTaken
	}
	return err
}

// SeatBlack records the guest who took Black's seat, their name, and when.
func (s *Store) SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error {
	return execOne(ctx, s.db, `UPDATE games SET black = ?, black_name = ?, joined_at = ? WHERE code = ?`, guest, nullable(name), now.UnixMilli(), code)
}

// AddTurn saves one move.
func (s *Store) AddTurn(ctx context.Context, code string, t Turn) error {
	return addTurn(ctx, s.db, code, t)
}

// EndGame records how a game ended. final, if not nil, is the move that
// ended it, saved in the same transaction.
func (s *Store) EndGame(ctx context.Context, code string, r Result, final *Turn, st *GameStats) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if final != nil {
		if err := addTurn(ctx, tx, code, *final); err != nil {
			return err
		}
	}
	if err := execOne(ctx, tx,
		`UPDATE games SET ended_at = ?, result = ?, winner = ? WHERE code = ?`,
		r.EndedAt.UnixMilli(), r.Reason, nullable(r.Winner), code); err != nil {
		return err
	}
	if st != nil {
		if err := addGameStats(ctx, tx, code, *st); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ExpireWaiting ends every game still waiting for Black that was created
// before cutoff, with result "expired". It returns how many it ended.
func (s *Store) ExpireWaiting(ctx context.Context, cutoff, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE games SET ended_at = ?, result = 'expired'
		 WHERE ended_at IS NULL AND black IS NULL AND created_at < ?`,
		now.UnixMilli(), cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// LoadForRestore returns every unfinished game and every game that ended
// after endedAfter, oldest first, each with its turns in order. Only games
// saved under the current rules are returned.
func (s *Store) LoadForRestore(ctx context.Context, endedAfter time.Time) ([]SavedGame, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+savedGameColumns+`
		 FROM games WHERE rules = ? AND (ended_at IS NULL OR ended_at > ?) ORDER BY created_at`,
		rulesVersion, endedAfter.UnixMilli())
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

// savedGameColumns are the games table's restore columns, as scanSavedGame
// reads them.
const savedGameColumns = `code, white, black, white_name, black_name, created_at, rematch_of, ended_at, result, winner, kind, joined_at, playtest`

// scanSavedGame reads one row of savedGameColumns.
func scanSavedGame(rows *sql.Rows) (SavedGame, error) {
	var g SavedGame
	var black, whiteName, blackName, rematchOf, result, winner sql.NullString
	var created int64
	var ended, joined sql.NullInt64
	if err := rows.Scan(&g.Code, &g.White, &black, &whiteName, &blackName, &created, &rematchOf, &ended, &result, &winner, &g.Kind, &joined, &g.Playtest); err != nil {
		return g, err
	}
	g.Black, g.RematchOf, g.CreatedAt = black.String, rematchOf.String, time.UnixMilli(created)
	g.WhiteName, g.BlackName = whiteName.String, blackName.String
	if joined.Valid {
		g.JoinedAt = time.UnixMilli(joined.Int64)
	}
	if ended.Valid {
		g.Result = &Result{EndedAt: time.UnixMilli(ended.Int64), Reason: result.String, Winner: winner.String}
	}
	return g, nil
}

func (s *Store) turns(ctx context.Context, code string) ([]Turn, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT ply, move, dice, white_ms, black_ms, at FROM turns WHERE game = ? ORDER BY ply`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []Turn
	for rows.Next() {
		var t Turn
		var dice string
		var at int64
		if err := rows.Scan(&t.Ply, &t.Move, &dice, &t.WhiteMS, &t.BlackMS, &at); err != nil {
			return nil, err
		}
		if t.Dice, err = parseDice(dice); err != nil {
			return nil, fmt.Errorf("game %s ply %d: %w", code, t.Ply, err)
		}
		t.At = time.UnixMilli(at)
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func addTurn(ctx context.Context, db execer, code string, t Turn) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO turns (game, ply, move, dice, white_ms, black_ms, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		code, t.Ply, t.Move, formatDice(t.Dice), t.WhiteMS, t.BlackMS, t.At.UnixMilli())
	return err
}

// execOne runs an UPDATE that must change exactly one row.
func execOne(ctx context.Context, db execer, query string, args ...any) error {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n != 1 {
		return fmt.Errorf("updated %d games, want 1", n)
	}
	return nil
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// formatDice writes rolls as "3,6,2"; no rolls is "".
func formatDice(rolls []int) string {
	parts := make([]string, len(rolls))
	for i, r := range rolls {
		parts[i] = strconv.Itoa(r)
	}
	return strings.Join(parts, ",")
}

func parseDice(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	rolls := make([]int, len(parts))
	for i, p := range parts {
		r, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("bad dice %q", s)
		}
		rolls[i] = r
	}
	return rolls, nil
}
