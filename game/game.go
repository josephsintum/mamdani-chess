// Package game runs live games. Each Game is one goroutine that owns its
// state; every method hands it a function to run and waits for it, so the
// game's state is only ever touched by that goroutine and needs no locks.
package game

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"slices"
	"sync/atomic"
	"time"

	"mamdani-chess/names"
	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// Errors returned by a game's methods.
var (
	ErrNotPlayer   = errors.New("you are not playing in this game")
	ErrWaiting     = errors.New("waiting for an opponent")
	ErrNotYourTurn = errors.New("not your turn")
	ErrStale       = errors.New("the game has moved on; reload the position")
	ErrGameOver    = rules.ErrGameOver
	ErrIllegalMove = rules.ErrIllegalMove
	ErrInternal    = errors.New("internal error in this game")
	ErrNotOver     = errors.New("the game isn't over")
	ErrPractice    = errors.New("a practice game has no rematch")
	// ErrGone is returned once a game has stopped: it sat idle and was
	// evicted, or a panic retired it.
	ErrGone = errors.New("game not found")
)

// Resignation is the result reason when a player resigns. The other
// reasons come from the rules package.
const Resignation rules.Reason = "resignation"

// Game is one live game.
type Game struct {
	code    string
	hub     *Hub // creates rematches
	dice    rules.Dice
	store   Store
	idle    time.Duration
	calls   chan call
	done    chan struct{} // closed when the game stops
	onExit  func()        // tells the hub to forget the game
	created time.Time     // orders the live games list
	// live is the game as the live games list shows it, replaced after
	// every call so the hub can list games without asking each one.
	live atomic.Pointer[Live]

	// Owned by the loop goroutine.
	g       *rules.Game
	seats   [2]string // guest ID per color; "" while empty
	names   [2]string // each player's name when they sat down; "" if unknown
	subs    map[*Sub]struct{}
	last    []rules.Event
	log     []LogEntry  // only ever grows: views share it (see shared)
	lost    [2][]string // piece codes lost to potholes, by color; only ever grow
	taken   [2][]string // piece codes captured, by the capturer's color; only ever grow
	stats   StatsJSON
	road    road
	clock   clock
	rematch rematch
	// failed is set when a save fails. The loop then retires the game
	// before anyone sees the change that wasn't saved.
	failed error
	// streams counts each guest's open streams, so who is online and
	// how many are watching never means looking through every stream.
	streams map[string]int
	// views is the latest encoded view for each role, which streams that
	// join soon after reuse (see sharedView).
	views [3]builtView
	// practice: one guest holds both seats and moves for the side to move;
	// there's no clock (see CreatePractice).
	practice bool
	// kind is how the game was made: friend or quick (rematches inherit it).
	kind string
	// playtest: a playtest started the game (rematches inherit it).
	playtest bool
	// quit stops the game after the current call: the guest started
	// another practice game.
	quit bool
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

// newGame returns the game sg describes (code, seats, names), not yet
// running: the caller starts it with go g.loop() once its state is set.
// After idle with no calls, the game stops if it is over or nobody is
// watching; onExit runs when it stops.
func newGame(h *Hub, sg store.Game, onExit func()) *Game {
	return &Game{
		code:     sg.Code,
		kind:     sg.Kind,
		playtest: sg.Playtest,
		hub:      h,
		dice:     h.dice,
		store:    h.store,
		idle:     h.Idle,
		calls:    make(chan call),
		done:     make(chan struct{}),
		onExit:   onExit,
		created:  sg.CreatedAt,
		g:        rules.NewGame(),
		seats:    [2]string{sg.White, sg.Black},
		names:    [2]string{sg.WhiteName, sg.BlackName},
		subs:     map[*Sub]struct{}{},
		streams:  map[string]int{},
		clock:    newClock(),
	}
}

func (g *Game) loop() {
	idle := time.NewTimer(g.idle)
	defer idle.Stop()
	// deadline fires when the side to move runs out of time. Every change
	// re-arms it, which drops a firing not yet received; flagIfDue checks
	// the clock again all the same.
	deadline := time.NewTimer(0)
	deadline.Stop()
	defer deadline.Stop()
	arm := func() {
		if g.clock.deadline.IsZero() {
			deadline.Stop()
			return
		}
		deadline.Reset(time.Until(g.clock.deadline))
	}
	arm()
	for {
		select {
		case c := <-g.calls:
			if err := g.check(g.run(c.f)); err != nil {
				// A panic may have left the game half-updated (the position
				// moved but the log didn't, say), and a failed save means
				// the database is behind, so the game can't go on. Stop
				// before replying, so the caller never sees it listed.
				g.stop()
				c.reply <- err
				return
			}
			if g.quit {
				slog.Info("practice game ended", "code", g.code, "moves", len(g.g.Turns))
				g.stop()
				c.reply <- nil
				return
			}
			g.publish() // before the reply, so the caller's next List sees it
			c.reply <- nil
			idle.Reset(g.idle)
			arm()
		case <-deadline.C:
			if g.check(g.run(func() { g.flagIfDue(time.Now()) })) != nil {
				g.stop()
				return
			}
			arm()
			g.publish()
		case <-idle.C:
			if g.g.Result.Over || len(g.subs) == 0 {
				if !g.g.Result.Over { // still waiting for Black: nobody came
					g.end(time.Now(), rules.Result{Over: true, Reason: Expired}, nil)
				}
				slog.Info("game evicted", "code", g.code, "result", g.g.Result.Reason, "moves", len(g.g.Turns))
				g.stop()
				return
			}
			idle.Reset(g.idle)
		}
	}
}

// flagIfDue ends the game on time if the side to move's deadline has
// passed. A call can arrive before the timer fires, so every call that acts
// on the game checks the clock first.
func (g *Game) flagIfDue(now time.Time) {
	if !g.g.Result.Over && g.expired(now) {
		g.flag(now)
	}
}

// end finishes the game with r and stops the clock, saving the result
// (with final, the move that ended it, if any) before anyone sees it.
// Without a final move, nothing happened on the board, so no dice replay.
func (g *Game) end(now time.Time, r rules.Result, final *store.Turn) {
	g.g.Result = r
	if final == nil {
		g.last = nil
	}
	g.clock.deadline = time.Time{}
	saved := savedResult(now, r)
	var st *store.GameStats
	if !g.practice && isFinished(r.Reason) {
		s := g.road.stats(r, g.last, g.lost)
		st = &s
	}
	g.save("result", func(ctx context.Context) error { return g.store.EndGame(ctx, g.code, saved, final, st) })
	if g.failed != nil {
		return
	}
	if r.Reason == Aborted {
		slog.Info("game aborted", "code", g.code, "missed", colorName(g.g.Pos.Turn))
	} else {
		slog.Info("game ended", "code", g.code, "result", r.Reason, "winner", winnerName(r), "moves", len(g.g.Turns))
	}
	g.broadcast()
}

// check returns err, or the save failure that f left behind.
func (g *Game) check(err error) error {
	if err == nil && g.failed != nil {
		slog.Error("game retired: save failed", "code", g.code, "err", g.failed)
		err = g.failed
	}
	return err
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
	g.subs, g.streams, g.views = nil, nil, [3]builtView{}
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

// try is do for a function that can refuse: it returns do's error, or f's.
func (g *Game) try(f func() error) error {
	var err error
	if perr := g.do(func() { err = f() }); perr != nil {
		return perr
	}
	return err
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
		if g.seats[rules.Black] == "" && guest != g.seats[rules.White] && !g.g.Result.Over {
			var name string
			g.save("black", func(ctx context.Context) error {
				var err error
				if name, err = g.store.EnsureGuest(ctx, guest, names.Random); err != nil {
					return err
				}
				return g.store.SeatBlack(ctx, g.code, guest, name, time.Now())
			})
			if g.failed != nil {
				return
			}
			g.seats[rules.Black] = guest
			g.names[rules.Black] = name
			g.startCounting(time.Now()) // White's first-move deadline
			slog.Info("black joined", "code", g.code, "black", guestTag(guest))
			g.addSub(sub)
			g.broadcast() // White's view changes from waiting to playing
			return
		}
		c, seated := g.seatOf(guest)
		returning := seated && !g.connected(c)
		g.addSub(sub)
		if returning {
			g.broadcast() // the opponent sees them connected again
			return
		}
		send(sub, g.sharedView(g.roleOf(guest)))
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}

// Leave closes a stream. Seats are kept, so the player can come back. A
// player's rematch offer lasts until their last stream closes.
func (g *Game) Leave(sub *Sub) {
	g.do(func() {
		g.removeSub(sub)
		c, seated := g.seatOf(sub.guest)
		if !seated || g.connected(c) {
			return
		}
		if g.rematch.offered && g.rematch.from == c {
			g.rematch.offered = false
		}
		g.broadcast() // the opponent sees them disconnected
	})
}

// addSub and removeSub keep subs and the streams count in step.
func (g *Game) addSub(sub *Sub) {
	g.subs[sub] = struct{}{}
	g.streams[sub.guest]++
}

func (g *Game) removeSub(sub *Sub) {
	if _, ok := g.subs[sub]; !ok {
		return
	}
	delete(g.subs, sub)
	if n := g.streams[sub.guest] - 1; n > 0 {
		g.streams[sub.guest] = n
	} else {
		delete(g.streams, sub.guest)
	}
	if len(g.subs) == 0 {
		g.views = [3]builtView{} // nobody left to share them with
	}
}

// connected reports whether c's player has a stream open.
func (g *Game) connected(c rules.Color) bool {
	id := g.seats[c]
	return id != "" && g.streams[id] > 0
}

// Move plays guest's move. seq must equal the number of turns played so far,
// which rejects a move made from an out-of-date board.
func (g *Game) Move(guest string, m rules.Move, seq int) error {
	return g.try(func() error { return g.move(guest, m, seq) })
}

func (g *Game) move(guest string, m rules.Move, seq int) error {
	now := time.Now()
	color, err := g.player(guest, now)
	switch {
	case err != nil:
		return err
	case color != g.g.Pos.Turn:
		return ErrNotYourTurn
	case seq != len(g.g.Turns):
		return ErrStale
	}
	running := g.running()
	if err := g.apply(m, g.dice); err != nil { // an illegal move is refused here, unchanged
		return err
	}
	if running {
		g.charge(color, now)
		g.clock.remaining[color] += Increment
	}
	turn := g.savedTurn(now)
	if g.g.Result.Over { // the move ended the game: save both at once
		g.end(now, g.g.Result, &turn)
		return nil
	}
	g.save("turn", func(ctx context.Context) error { return g.store.AddTurn(ctx, g.code, turn) })
	if g.failed != nil {
		return nil // the loop reports g.failed and retires the game
	}
	g.startCounting(now.Add(pauseFor(g.last)))
	g.broadcast()
	return nil
}

// apply plays m and adds it to the log, lost pieces and stats.
func (g *Game) apply(m rules.Move, dice rules.Dice) error {
	color, before := g.g.Pos.Turn, g.g.Pos // SAN reads the position the move was made in
	ev, err := g.g.Play(m, dice)
	if err != nil {
		return err
	}
	g.last = ev
	g.log = append(g.log, logEntry(before.SAN(m), color, ev))
	g.tally(ev)
	g.road.add(ev, &g.g.Pos)
	return nil
}

// logEntry is the log's line for a turn played as san by color.
func logEntry(san string, color rules.Color, ev []rules.Event) LogEntry {
	l := LogEntry{SAN: san, Color: colorName(color), Dice: describe(ev)}
	for _, e := range ev {
		switch e.Kind {
		case rules.Moved:
			l.Piece = pieceCode(e.Piece)
		case rules.PotholeOpened:
			l.Opened = e.Square.String()
		case rules.Fell:
			l.Fell = append(l.Fell, pieceCode(e.Piece))
		case rules.Repaired:
			l.Repaired = true
		}
	}
	return l
}

// tally adds a turn's events to the game's stats.
func (g *Game) tally(ev []rules.Event) {
	for _, e := range ev {
		switch e.Kind {
		case rules.Captured:
			c := e.Piece.Color().Other()
			g.taken[c] = append(g.taken[c], pieceCode(e.Piece))
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
	return g.try(func() error { return g.resign(guest) })
}

func (g *Game) resign(guest string) error {
	now := time.Now()
	color, err := g.player(guest, now)
	if err != nil {
		return err
	}
	if g.running() && g.g.Pos.Turn == color {
		g.charge(color, now) // the clock shows what they had left
	}
	g.end(now, rules.Result{Over: true, Winner: color.Other(), Reason: Resignation}, nil)
	return nil
}

// player returns guest's color in a game being played. It checks the clock
// first: a flag that fell before the call ends the game.
func (g *Game) player(guest string, now time.Time) (rules.Color, error) {
	g.flagIfDue(now)
	color, seated := g.seatOf(guest)
	switch {
	case !seated:
		return 0, ErrNotPlayer
	case g.g.Result.Over:
		return 0, ErrGameOver
	case g.status() == Waiting:
		return 0, ErrWaiting
	}
	return color, nil
}

func (g *Game) seatOf(guest string) (rules.Color, bool) {
	if g.practice && guest == g.seats[rules.White] {
		return g.g.Pos.Turn, true // the one guest plays whoever is to move
	}
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
	g.views = [3]builtView{} // the game changed: no view built before is current
	for sub := range g.subs {
		send(sub, g.sharedView(g.roleOf(sub.guest)))
	}
}

// viewReuse is how long a built view is handed to streams that join. A
// view carries the server's time (clock.now), which the browser sets its
// clocks by, so it can't be reused for long; but a crowd arriving at once
// costs a few dozen views a second instead of one each.
const viewReuse = 25 * time.Millisecond

// builtView is an encoded view and when it was built.
type builtView struct {
	v  *View
	at time.Time
}

// sharedView returns r's encoded view, reusing the latest if it was built
// within viewReuse. broadcast clears them, so a change is never hidden.
func (g *Game) sharedView(r role) *View {
	now := time.Now()
	if b := g.views[r]; b.v != nil && now.Sub(b.at) < viewReuse {
		return b.v
	}
	v := sealed(g.viewFor(r))
	g.views[r] = builtView{v: v, at: now}
	return v
}

// sealed encodes v for the streams it is about to be shared with.
func sealed(v *View) *View {
	v.data = v.JSON()
	return v
}

// View returns guest's current view of the game.
func (g *Game) View(guest string) (*View, error) {
	var v *View
	err := g.do(func() { v = g.viewFor(g.roleOf(guest)) })
	return v, err
}

// shared returns s for a view to hold instead of a copy. The game only
// appends to its log and lost pieces, never changes an entry, and the
// capped slice can't reach what is appended later. Never nil, so it
// encodes as [].
func shared[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return slices.Clip(s)
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
	now := time.Now()
	v := &View{
		Code:     g.code,
		Status:   g.status(),
		You:      "spectator",
		Board:    boardJSON(p),
		Mamdani:  squareName(p.Mamdani),
		Potholes: potholesJSON(p),
		Turn:     colorName(p.Turn),
		Check:    p.InCheck(p.Turn),
		Legal:    []MoveJSON{},
		Last:     make([]EventJSON, 0, len(g.last)),
		Log:      shared(g.log),
		Lost:     LostJSON{White: shared(g.lost[rules.White]), Black: shared(g.lost[rules.Black])},
		Taken:    LostJSON{White: shared(g.taken[rules.White]), Black: shared(g.taken[rules.Black])},
		Stats:    g.stats,
		Seq:      len(g.g.Turns),
		Clock:    g.clockJSON(now),
		Online:   OnlineJSON{White: g.connected(rules.White), Black: g.connected(rules.Black)},
		Players:  PlayersJSON{White: g.names[rules.White], Black: g.names[rules.Black]},
		Rematch:  g.rematchJSON(),
		Result:   resultJSON(g.g.Result),
		Practice: g.practice,
	}
	color, seated := rules.Color(r), r != roleSpectator
	if seated {
		v.You = colorName(color)
	}
	if v.Status == Playing && seated && color == p.Turn {
		moves := p.LegalMoves()
		v.Legal = make([]MoveJSON, len(moves))
		for i, m := range moves {
			v.Legal[i] = moveJSON(m)
		}
	}
	for _, e := range g.last {
		v.Last = append(v.Last, eventJSON(e))
	}
	return v
}
