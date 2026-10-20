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
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		_, err := tx.ExecContext(ctx,
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
			countCreated(c, s.zone, gameFacts{Created: g.CreatedAt, Kind: kind, Rematch: g.RematchOf != ""})
		}
		return nil
	})
}

// SeatBlack records the guest who took Black's seat, their name, and when,
// and counts the join. A seat already taken changes nothing and is no error.
func (s *Store) SeatBlack(ctx context.Context, code, guest, name string, now time.Time) error {
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		f := gameFacts{Joined: now}
		playtest, err := scanFacts(tx.QueryRowContext(ctx,
			`UPDATE games SET black = ?, black_name = ?, joined_at = ? WHERE code = ? AND black IS NULL
			 RETURNING `+factsColumns,
			guest, nullable(name), now.UnixMilli(), code), &f)
		if errors.Is(err, sql.ErrNoRows) {
			return noChange(ctx, tx, code)
		}
		if err != nil {
			return err
		}
		if !playtest {
			countJoined(c, s.zone, f)
		}
		return nil
	})
}

// AddTurn saves one move.
func (s *Store) AddTurn(ctx context.Context, code string, t Turn) error {
	return addTurn(ctx, s.db, code, t)
}

// EndGame records how a game ended and counts it. final, if not nil, is
// the move that ended it, saved in the same transaction. A game that has
// already ended changes nothing and is no error.
func (s *Store) EndGame(ctx context.Context, code string, r Result, final *Turn, st *GameStats) error {
	return s.counted(ctx, func(tx *sql.Tx, c counts) error {
		f := gameFacts{Result: r.Reason, Winner: r.Winner, Stats: st}
		playtest, err := scanFacts(tx.QueryRowContext(ctx,
			`UPDATE games SET ended_at = ?, result = ?, winner = ? WHERE code = ? AND ended_at IS NULL
			 RETURNING `+factsColumns,
			r.EndedAt.UnixMilli(), r.Reason, nullable(r.Winner), code), &f)
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
		if playtest {
			return nil
		}
		if err := lastTurn(ctx, tx, code, &f); err != nil {
			return err
		}
		countEnded(c, s.zone, f)
		return nil
	})
}

// factsColumns are the games columns counting reads, as scanFacts reads
// them.
const factsColumns = `created_at, kind, rematch_of IS NOT NULL, black IS NOT NULL, playtest`

// scanFacts reads a row of factsColumns, then any more columns, into f and
// more, and reports whether a playtest started the game.
func scanFacts(row interface{ Scan(...any) error }, f *gameFacts, more ...any) (playtest bool, err error) {
	var created int64
	if err := row.Scan(append([]any{&created, &f.Kind, &f.Rematch, &f.HasBlack, &playtest}, more...)...); err != nil {
		return false, err
	}
	f.Created = time.UnixMilli(created)
	return playtest, nil
}

// noChange explains a guarded update that changed no row: nil when the
// game exists (it had already ended, or its seat was taken); an error when
// it was never saved.
func noChange(ctx context.Context, tx *sql.Tx, code string) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM games WHERE code = ?)`, code).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("no saved game %s", code)
	}
	return nil
}

// lastTurn reads a game's last turn into f, by one primary-key seek: the
// moves played (its ply + 1) and the winner's clock after it. A game with
// no turn keeps zero moves and no clock.
func lastTurn(ctx context.Context, tx *sql.Tx, code string, f *gameFacts) error {
	var ply int
	var white, black sql.NullInt64
	err := tx.QueryRowContext(ctx, `SELECT ply, white_ms, black_ms FROM turns WHERE game = ? ORDER BY ply DESC LIMIT 1`, code).Scan(&ply, &white, &black)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	f.Moves = ply + 1
	f.WinnerMS, f.HasClock = winnerClock(f.Winner, white, black)
	return nil
}

// ExpireWaiting ends every game still waiting for Black that was created
// before cutoff, with result "expired", and counts them. It returns how
// many it ended.
func (s *Store) ExpireWaiting(ctx context.Context, cutoff, now time.Time) (int64, error) {
	var n int64
	err := s.counted(ctx, func(tx *sql.Tx, c counts) error {
		return eachRow(ctx, tx,
			`UPDATE games SET ended_at = ?, result = 'expired'
			 WHERE ended_at IS NULL AND black IS NULL AND created_at < ?
			 RETURNING `+factsColumns,
			[]any{now.UnixMilli(), cutoff.UnixMilli()}, func(rows *sql.Rows) error {
				f := gameFacts{Result: "expired"}
				playtest, err := scanFacts(rows, &f)
				if err != nil {
					return err
				}
				n++
				if !playtest {
					countEnded(c, s.zone, f)
				}
				return nil
			})
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

// LoadForRestore returns every unfinished game and every game that ended
// after endedAfter, oldest first, each with its turns in order. Only games
// saved under the current rules are returned.
func (s *Store) LoadForRestore(ctx context.Context, endedAfter time.Time) ([]SavedGame, error) {
	return s.savedGames(ctx, `rules = ? AND (ended_at IS NULL OR ended_at > ?)`, rulesVersion, endedAfter.UnixMilli())
}

// savedGames returns the games that match where, oldest first, each with
// its turns in order.
func (s *Store) savedGames(ctx context.Context, where string, args ...any) ([]SavedGame, error) {
	var games []SavedGame
	err := eachRow(ctx, s.db, `SELECT `+savedGameColumns+` FROM games WHERE `+where+` ORDER BY created_at`, args, func(rows *sql.Rows) error {
		g, err := scanSavedGame(rows)
		if err != nil {
			return err
		}
		games = append(games, g)
		return nil
	})
	if err != nil {
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
	var turns []Turn
	err := eachRow(ctx, s.db, `SELECT ply, move, dice, white_ms, black_ms, at FROM turns WHERE game = ? ORDER BY ply`, []any{code}, func(rows *sql.Rows) error {
		var t Turn
		var dice string
		var at int64
		if err := rows.Scan(&t.Ply, &t.Move, &dice, &t.WhiteMS, &t.BlackMS, &at); err != nil {
			return err
		}
		rolls, err := parseInts(dice)
		if err != nil {
			return fmt.Errorf("game %s ply %d: bad dice %q", code, t.Ply, dice)
		}
		t.Dice, t.At = rolls, time.UnixMilli(at)
		turns = append(turns, t)
		return nil
	})
	return turns, err
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func addTurn(ctx context.Context, db execer, code string, t Turn) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO turns (game, ply, move, dice, white_ms, black_ms, at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		code, t.Ply, t.Move, formatInts(t.Dice), t.WhiteMS, t.BlackMS, t.At.UnixMilli())
	return err
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// formatInts writes a list as "3,6,2" (a turn's dice, a game's open_hist);
// an empty list is "". parseInts reads it back.
func formatInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = strconv.Itoa(n)
	}
	return strings.Join(parts, ",")
}

func parseInts(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	ns := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		ns[i] = n
	}
	return ns, nil
}
