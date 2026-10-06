package game

import (
	"cmp"
	"slices"
	"time"

	"mamdani-chess/rules"
)

// Live is a game as the home page's live games list shows it. A game
// replaces its Live after every call; a published Live is never changed.
type Live struct {
	Code     string     `json:"code"`
	White    string     `json:"white"` // names
	Black    string     `json:"black"`
	Move     int        `json:"move"` // the full-move number being played
	Board    [64]string `json:"board"`
	Mamdani  string     `json:"mamdani"`
	Potholes []Pothole  `json:"potholes"`
	Last     *MoveJSON  `json:"last"`     // the latest move, or null before the first
	Watching int        `json:"watching"` // guests watching who aren't playing
	status   Status
	created  time.Time
	seats    [2]string   // guest IDs, for Active
	plies    int         // turns played, for Summary
	result   *ResultJSON // how it ended, or nil
}

// publish replaces the game's Live with its current state.
func (g *Game) publish() {
	p := &g.g.Pos
	l := &Live{
		Code:     g.code,
		White:    g.names[rules.White],
		Black:    g.names[rules.Black],
		Move:     len(g.g.Turns)/2 + 1,
		Mamdani:  squareName(p.Mamdani),
		Potholes: potholesJSON(p),
		Watching: g.watching(),
		status:   g.status(),
		created:  g.created,
		seats:    g.seats,
		plies:    len(g.g.Turns),
	}
	if r := g.g.Result; r.Over {
		l.result = &ResultJSON{Winner: winnerName(r), Draw: r.Draw, Reason: r.Reason}
	}
	for s, pc := range p.Board {
		l.Board[s] = pieceCode(pc)
	}
	if n := len(g.g.Turns); n > 0 {
		m := moveJSON(g.g.Turns[n-1].Move)
		l.Last = &m
	}
	g.live.Store(l)
}

// Summary is a game at a glance, as a shared link's preview describes it.
type Summary struct {
	White, Black string // names; "" for an empty seat
	Status       Status
	Plies        int         // turns played
	Result       *ResultJSON // how it ended, once over
}

// Summary reads the game's last published state, so it never waits on the
// game.
func (g *Game) Summary() Summary {
	l := g.live.Load()
	if l == nil {
		return Summary{Status: Waiting}
	}
	return Summary{White: l.White, Black: l.Black, Status: l.status, Plies: l.plies, Result: l.result}
}

// watching counts the guests with the game open who aren't seated: two
// tabs are one guest.
func (g *Game) watching() int {
	n := len(g.streams)
	for c := range g.seats {
		if g.connected(rules.Color(c)) {
			n--
		}
	}
	return n
}

// Active returns the code of the newest game being played with guest in a
// seat, or "". A game waiting for a friend, a finished game, and games the
// guest only watches don't count.
func (h *Hub) Active(guest string) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var newest *Live
	for _, g := range h.games {
		l := g.live.Load()
		if l == nil || l.status != Playing || (l.seats[0] != guest && l.seats[1] != guest) {
			continue
		}
		if newest == nil || l.created.After(newest.created) {
			newest = l
		}
	}
	if newest == nil {
		return ""
	}
	return newest.Code
}

// List returns up to max games being played, most watched first, then
// newest. It reads each game's last published Live, so it never waits on
// a game.
func (h *Hub) List(max int) []*Live {
	h.mu.Lock()
	games := make([]*Live, 0, len(h.games))
	for _, g := range h.games {
		if l := g.live.Load(); l != nil && l.status == Playing {
			games = append(games, l)
		}
	}
	h.mu.Unlock()
	slices.SortFunc(games, func(a, b *Live) int {
		if c := cmp.Compare(b.Watching, a.Watching); c != 0 {
			return c
		}
		return b.created.Compare(a.created)
	})
	return games[:min(max, len(games))]
}
