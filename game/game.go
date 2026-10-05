// Package game runs live games. Each Game is one goroutine that owns its
// state; every method hands it a function to run and waits for it, so the
// game's state is only ever touched by that goroutine and needs no locks.
package game

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"mamdani-chess/rules"
)

// Errors returned by Move.
var (
	ErrNotPlayer   = errors.New("you are not playing in this game")
	ErrWaiting     = errors.New("waiting for an opponent")
	ErrNotYourTurn = errors.New("not your turn")
	ErrStale       = errors.New("the game has moved on; reload the position")
	ErrGameOver    = rules.ErrGameOver
	ErrIllegalMove = rules.ErrIllegalMove
	ErrInternal    = errors.New("internal error in this game")
	// ErrGone is returned once a game has stopped: it sat idle and was
	// evicted, or a panic retired it.
	ErrGone = errors.New("game not found")
)

// Resignation is the result reason when a player resigns. The other
// reasons come from the rules package.
const Resignation rules.Reason = "resignation"

// Game is one live game.
type Game struct {
	code   string
	dice   rules.Dice
	idle   time.Duration
	calls  chan call
	done   chan struct{} // closed when the game stops
	onExit func()        // tells the hub to forget the game

	// Owned by the loop goroutine.
	g     *rules.Game
	seats [2]string // guest ID per color; "" while empty
	subs  map[*Sub]struct{}
	last  []rules.Event
	log   []LogEntry
	lost  [2][]string // piece codes lost to potholes, by color
	stats StatsJSON
}

// Sub is one open stream. C always holds the newest View: a reader that
// falls behind skips stale views instead of blocking the game.
type Sub struct {
	C     <-chan *View
	ch    chan *View
	guest string
}

// call is one function for the loop to run, and where to send the outcome.
type call struct {
	f     func()
	reply chan error
}

// newGame starts a game with White's seat taken by creator. After idle with
// no calls, the game stops if it is over or nobody is watching; onExit runs
// when it stops.
func newGame(code, creator string, dice rules.Dice, idle time.Duration, onExit func()) *Game {
	g := &Game{
		code:   code,
		dice:   dice,
		idle:   idle,
		calls:  make(chan call),
		done:   make(chan struct{}),
		onExit: onExit,
		g:      rules.NewGame(),
		seats:  [2]string{creator, ""},
		subs:   map[*Sub]struct{}{},
	}
	go g.loop()
	return g
}

func (g *Game) loop() {
	idle := time.NewTimer(g.idle)
	defer idle.Stop()
	for {
		select {
		case c := <-g.calls:
			if err := g.run(c.f); err != nil {
				// A panic may have left the game half-updated (the position
				// moved but the log didn't, say), so the game can't go on.
				// Stop before replying, so the caller never sees it listed.
				g.stop()
				c.reply <- err
				return
			}
			c.reply <- nil
			idle.Reset(g.idle)
		case <-idle.C:
			if g.g.Result.Over || len(g.subs) == 0 {
				slog.Info("evicting idle game", "code", g.code)
				g.stop()
				return
			}
			idle.Reset(g.idle)
		}
	}
}

// run calls f, turning a panic into ErrInternal so one bad game can't take
// the whole server down.
func (g *Game) run(f func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic in game", "code", g.code, "panic", r, "turns", g.g.Turns, "stack", string(debug.Stack()))
			err = fmt.Errorf("%w: %v", ErrInternal, r)
		}
	}()
	f()
	return nil
}

// stop ends the game: open streams close, later calls get ErrGone, and the
// hub forgets the game.
func (g *Game) stop() {
	close(g.done)
	for sub := range g.subs {
		close(sub.ch)
	}
	g.subs = nil
	if g.onExit != nil {
		g.onExit()
	}
}

// do runs f on the game's goroutine and waits for it to finish. It returns
// ErrGone if the game has stopped, and ErrInternal if f panicked.
func (g *Game) do(f func()) error {
	c := call{f: f, reply: make(chan error, 1)}
	select {
	case g.calls <- c:
		return <-c.reply
	case <-g.done:
		return ErrGone
	}
}

// Code returns the game's share code.
func (g *Game) Code() string { return g.code }

// Join opens a stream for guest. The creator plays White, the first other
// guest takes Black, and everyone after that watches. A guest who reconnects
// keeps their seat. The current view is already waiting on the Sub. C is
// closed when the game stops.
func (g *Game) Join(guest string) (*Sub, error) {
	ch := make(chan *View, 1)
	sub := &Sub{C: ch, ch: ch, guest: guest}
	err := g.do(func() {
		if g.seats[rules.Black] == "" && guest != g.seats[rules.White] {
			g.seats[rules.Black] = guest
			g.subs[sub] = struct{}{}
			g.broadcast() // White's view changes from waiting to playing
			return
		}
		g.subs[sub] = struct{}{}
		send(sub, g.viewFor(g.roleOf(guest)))
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// Leave closes a stream. Seats are kept, so the player can come back.
func (g *Game) Leave(sub *Sub) {
	g.do(func() { delete(g.subs, sub) })
}

// Move plays guest's move. seq must equal the number of turns played so far,
// which rejects a move made from an out-of-date board.
func (g *Game) Move(guest string, m rules.Move, seq int) error {
	var err error
	if perr := g.do(func() { err = g.move(guest, m, seq) }); perr != nil {
		return perr
	}
	return err
}

func (g *Game) move(guest string, m rules.Move, seq int) error {
	color, seated := g.seatOf(guest)
	switch {
	case !seated:
		return ErrNotPlayer
	case g.g.Result.Over:
		return ErrGameOver
	case g.status() == Waiting:
		return ErrWaiting
	case color != g.g.Pos.Turn:
		return ErrNotYourTurn
	case seq != len(g.g.Turns):
		return ErrStale
	}
	before := g.g.Pos              // SAN reads the position the move was made in
	ev, err := g.g.Play(m, g.dice) // an illegal move is refused here, unchanged
	if err != nil {
		return err
	}
	san := before.SAN(m)
	g.last = ev
	g.log = append(g.log, LogEntry{SAN: san, Color: colorName(color), Dice: describe(ev)})
	g.tally(ev)
	g.broadcast()
	return nil
}

// tally adds a turn's events to the game's stats.
func (g *Game) tally(ev []rules.Event) {
	for _, e := range ev {
		switch e.Kind {
		case rules.Fell:
			if e.Piece == rules.MamdaniPiece {
				g.stats.MamdaniFell = true
			} else {
				c := e.Piece.Color()
				g.lost[c] = append(g.lost[c], pieceCode(e.Piece))
			}
		case rules.SavingRoll:
			g.stats.SavingRolls++
			if e.Saved {
				g.stats.Saved++
			}
		case rules.Repaired:
			g.stats.Repaired++
		}
	}
}

// Resign ends the game with guest's opponent as the winner.
func (g *Game) Resign(guest string) error {
	var err error
	if perr := g.do(func() { err = g.resign(guest) }); perr != nil {
		return perr
	}
	return err
}

func (g *Game) resign(guest string) error {
	color, seated := g.seatOf(guest)
	switch {
	case !seated:
		return ErrNotPlayer
	case g.g.Result.Over:
		return ErrGameOver
	case g.status() == Waiting:
		return ErrWaiting
	}
	g.g.Result = rules.Result{Over: true, Winner: color.Other(), Reason: Resignation}
	g.last = nil
	g.broadcast()
	return nil
}

func (g *Game) seatOf(guest string) (rules.Color, bool) {
	for c, id := range g.seats {
		if id != "" && id == guest {
			return rules.Color(c), true
		}
	}
	return 0, false
}

func (g *Game) status() Status {
	switch {
	case g.g.Result.Over:
		return Over
	case g.seats[rules.Black] == "":
		return Waiting
	}
	return Playing
}

// role is who a view is for: a seated color, or roleSpectator. Everyone in
// the same role sees the same view, so broadcast builds at most one per
// role, not one per stream.
type role int

const roleSpectator = role(2)

func (g *Game) roleOf(guest string) role {
	if c, ok := g.seatOf(guest); ok {
		return role(c)
	}
	return roleSpectator
}

// broadcast sends every stream its role's view. Streams in the same role
// share one *View, so a View must never be changed once sent.
func (g *Game) broadcast() {
	var views [3]*View
	for sub := range g.subs {
		r := g.roleOf(sub.guest)
		if views[r] == nil {
			views[r] = g.viewFor(r)
		}
		send(sub, views[r])
	}
}

// View returns guest's current view of the game.
func (g *Game) View(guest string) (*View, error) {
	var v *View
	err := g.do(func() { v = g.viewFor(g.roleOf(guest)) })
	return v, err
}

// send replaces whatever is waiting on the sub with v. Only the game's
// goroutine sends, so this never blocks.
func send(sub *Sub, v *View) {
	select {
	case <-sub.ch:
	default:
	}
	sub.ch <- v
}

func (g *Game) viewFor(r role) *View {
	p := &g.g.Pos
	v := &View{
		Code:     g.code,
		Status:   g.status(),
		You:      "spectator",
		Mamdani:  squareName(p.Mamdani),
		Potholes: []Pothole{},
		Turn:     colorName(p.Turn),
		Check:    p.InCheck(p.Turn),
		Legal:    []MoveJSON{},
		Last:     []EventJSON{},
		Log:      append([]LogEntry{}, g.log...),
		Lost:     LostJSON{White: append([]string{}, g.lost[rules.White]...), Black: append([]string{}, g.lost[rules.Black]...)},
		Stats:    g.stats,
		Seq:      len(g.g.Turns),
	}
	color, seated := rules.Color(r), r != roleSpectator
	if seated {
		v.You = colorName(color)
	}
	for s, pc := range p.Board {
		v.Board[s] = pieceCode(pc)
	}
	for c, s := range p.Potholes {
		if s != rules.NoSquare {
			v.Potholes = append(v.Potholes, Pothole{Sq: s.String(), By: colorName(rules.Color(c))})
		}
	}
	if v.Status == Playing && seated && color == p.Turn {
		for _, m := range p.LegalMoves() {
			v.Legal = append(v.Legal, moveJSON(m))
		}
	}
	for _, e := range g.last {
		v.Last = append(v.Last, eventJSON(e))
	}
	if r := g.g.Result; r.Over {
		v.Result = &ResultJSON{Draw: r.Draw, Reason: r.Reason}
		if !r.Draw {
			v.Result.Winner = colorName(r.Winner)
		}
	}
	return v
}
