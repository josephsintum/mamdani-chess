// Package game runs live games. Each Game is one goroutine that owns its
// state; every method hands it a function to run and waits for it, so the
// game's state is only ever touched by that goroutine and needs no locks.
package game

import (
	"errors"
	"slices"

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
)

// Game is one live game.
type Game struct {
	code  string
	dice  rules.Dice
	calls chan func()

	// Owned by the loop goroutine.
	g     *rules.Game
	seats [2]string // guest ID per color; "" while empty
	subs  map[*Sub]struct{}
	last  []rules.Event
	log   []string
}

// Sub is one open stream. C always holds the newest View: a reader that
// falls behind skips stale views instead of blocking the game.
type Sub struct {
	C     <-chan *View
	ch    chan *View
	guest string
}

// newGame starts a game with White's seat taken by creator.
func newGame(code, creator string, dice rules.Dice) *Game {
	g := &Game{
		code:  code,
		dice:  dice,
		calls: make(chan func()),
		g:     rules.NewGame(),
		seats: [2]string{creator, ""},
		subs:  map[*Sub]struct{}{},
	}
	go g.loop()
	return g
}

func (g *Game) loop() {
	for f := range g.calls {
		f()
	}
}

// do runs f on the game's goroutine and waits for it to finish.
func (g *Game) do(f func()) {
	done := make(chan struct{})
	g.calls <- func() {
		defer close(done)
		f()
	}
	<-done
}

// Code returns the game's share code.
func (g *Game) Code() string { return g.code }

// Join opens a stream for guest. The creator plays White, the first other
// guest takes Black, and everyone after that watches. A guest who reconnects
// keeps their seat. The current view is already waiting on the Sub.
func (g *Game) Join(guest string) *Sub {
	ch := make(chan *View, 1)
	sub := &Sub{C: ch, ch: ch, guest: guest}
	g.do(func() {
		if g.seats[rules.Black] == "" && guest != g.seats[rules.White] {
			g.seats[rules.Black] = guest
			g.subs[sub] = struct{}{}
			g.broadcast() // White's view changes from waiting to playing
			return
		}
		g.subs[sub] = struct{}{}
		send(sub, g.view(guest))
	})
	return sub
}

// Leave closes a stream. Seats are kept, so the player can come back.
func (g *Game) Leave(sub *Sub) {
	g.do(func() { delete(g.subs, sub) })
}

// Move plays guest's move. seq must equal the number of turns played so far,
// which rejects a move made from an out-of-date board.
func (g *Game) Move(guest string, m rules.Move, seq int) error {
	var err error
	g.do(func() { err = g.move(guest, m, seq) })
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
	case !slices.Contains(g.g.Pos.LegalMoves(), m):
		return ErrIllegalMove
	}
	san := g.g.Pos.SAN(m) // before the move: SAN reads the old position
	ev, err := g.g.Play(m, g.dice)
	if err != nil {
		return err
	}
	g.last = ev
	g.log = append(g.log, describe(san, ev))
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

func (g *Game) broadcast() {
	for sub := range g.subs {
		send(sub, g.view(sub.guest))
	}
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

func (g *Game) view(guest string) *View {
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
		Log:      append([]string{}, g.log...),
		Seq:      len(g.g.Turns),
	}
	color, seated := g.seatOf(guest)
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
