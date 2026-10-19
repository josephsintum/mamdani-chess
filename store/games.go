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

// CreateGame saves a new game and counts it.
func (s *Store) CreateGame(ctx context.Context, g Game) error {
	kind := g.Kind
	if kind == "" {
		kind = "friend"
	}
	var joined any
	if g.Black != "" {
		joined = g.CreatedAt.UnixMilli() // both seated from the start: quick match or a rematch
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx,
		`INSERT INTO games (code, white, black, white_name, black_name, created_at, rematch_of, rules, kind, joined_at, playtest) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		g.Code, g.White, nullable(g.Black), nullable(g.WhiteName), nullable(g.BlackName), g.CreatedAt.UnixMilli(), nullable(g.RematchOf), rulesVersion, kind, joined, g.Playtest)
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code() == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY {
		return ErrCodeTaken
	}
	if err != nil {
		return err
	}
	if !g.Playtest {
		c := counts{}
		countCreated(c, s.zone, gameFacts{Created: g.CreatedAt, Kind: kind, Rematch: g.RematchOf != ""})
		if err := addCounts(ctx, tx, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SeatBlack records the guest who took Black's seat, their name, and when,
// and counts the join. A seat already taken changes nothing and is no error.
func (s *Store) SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var created int64
	var playtest bool
	f := gameFacts{HasBlack: true, Joined: now}
	err = tx.QueryRowContext(ctx,
		`UPDATE games SET black = ?, black_name = ?, joined_at = ? WHERE code = ? AND black IS NULL
		 RETURNING created_at, kind, rematch_of IS NOT NULL, playtest`,
		guest, nullable(name), now.UnixMilli(), code).Scan(&created, &f.Kind, &f.Rematch, &playtest)
	if errors.Is(err, sql.ErrNoRows) {
		return noChange(ctx, tx, code)
	}
	if err != nil {
		return err
	}
	if !playtest {
		f.Created = time.UnixMilli(created)
		c := counts{}
		countJoined(c, s.zone, f)
		if err := addCounts(ctx, tx, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AddTurn saves one move.
func (s *Store) AddTurn(ctx context.Context, code string, t Turn) error {
	return addTurn(ctx, s.db, code, t)
}

// EndGame records how a game ended and counts it. final, if not nil, is
// the move that ended it, saved in the same transaction. A game that has
// already ended changes nothing and is no error.
func (s *Store) EndGame(ctx context.Context, code string, r Result, final *Turn, st *GameStats) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var created int64
	var playtest bool
	f := gameFacts{Result: r.Reason, Winner: r.Winner, Stats: st}
	err = tx.QueryRowContext(ctx,
		`UPDATE games SET ended_at = ?, result = ?, winner = ? WHERE code = ? AND ended_at IS NULL
		 RETURNING created_at, kind, rematch_of IS NOT NULL, black IS NOT NULL, playtest`,
		r.EndedAt.UnixMilli(), r.Reason, nullable(r.Winner), code).Scan(&created, &f.Kind, &f.Rematch, &f.HasBlack, &playtest)
	if errors.Is(err, sql.ErrNoRows) {
		return noChange(ctx, tx, code)
	}
	if err != nil {
		return err
	}
	if final != nil {
		if err := addTurn(ctx, tx, code, *final); err != nil {
			return err
		}
	}
	if st != nil {
		if _, err := addGameStats(ctx, tx, code, *st); err != nil {
			return err
		}
	}
	if !playtest {
		f.Created = time.UnixMilli(created)
		if f.WinnerMS, f.HasClock, err = lastClock(ctx, tx, code, r.Winner); err != nil {
			return err
		}
		c := counts{}
		countEnded(c, s.zone, f)
		if err := addCounts(ctx, tx, c); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// noChange explains a guarded update that changed no row: nil when the
// game exists (it had already ended, or its seat was taken); an error when
// it was never saved.
func noChange(ctx context.Context, db querier, code string) error {
	var exists bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE code = ?)`, code).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no saved game %s", code)
	}
	return nil
}

// lastClock is the winner's clock after a game's last turn: one
// primary-key seek. It reports false with no winner or no turn.
func lastClock(ctx context.Context, db querier, code, winner string) (int64, bool, error) {
	if winner == "" {
		return 0, false, nil
	}
	var white, black sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT white_ms, black_ms FROM turns WHERE game = ? ORDER BY ply DESC LIMIT 1`, code).Scan(&white, &black)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	ms, ok := winnerClock(winner, white, black)
	return ms, ok, nil
}

// ExpireWaiting ends every game still waiting for Black that was created
// before cutoff, with result "expired", and counts them. It returns how
// many it ended.
func (s *Store) ExpireWaiting(ctx context.Context, cutoff, now time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx,
		`UPDATE games SET ended_at = ?, result = 'expired'
		 WHERE ended_at IS NULL AND black IS NULL AND created_at < ?
		 RETURNING created_at, kind, rematch_of IS NOT NULL, playtest`,
		now.UnixMilli(), cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	var n int64
	c := counts{}
	for rows.Next() {
		var created int64
		var playtest bool
		f := gameFacts{Result: "expired"}
		if err := rows.Scan(&created, &f.Kind, &f.Rematch, &playtest); err != nil {
			rows.Close()
			return 0, err
		}
		n++
		if !playtest {
			f.Created = time.UnixMilli(created)
			countEnded(c, s.zone, f)
		}
	}
	rows.Close() // one connection: close before the next statement
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if err := addCounts(ctx, tx, c); err != nil {
		return 0, err
	}
	return n, tx.Commit()
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

type querier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func addTurn(ctx context.Context, db execer, code string, t Turn) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO turns (game, ply, move, dice, white_ms, black_ms, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		code, t.Ply, t.Move, formatDice(t.Dice), t.WhiteMS, t.BlackMS, t.At.UnixMilli())
	return err
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
