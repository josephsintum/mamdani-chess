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
		Potholes: []Pothole{},
		Watching: g.watching(),
		status:   g.status(),
		created:  g.created,
	}
	for s, pc := range p.Board {
		l.Board[s] = pieceCode(pc)
	}
	for c, s := range p.Potholes {
		if s != rules.NoSquare {
			l.Potholes = append(l.Potholes, Pothole{Sq: s.String(), By: colorName(rules.Color(c))})
		}
	}
	if n := len(g.g.Turns); n > 0 {
		m := moveJSON(g.g.Turns[n-1].Move)
		l.Last = &m
	}
	g.live.Store(l)
}

// watching counts the guests with the game open who aren't seated: two
// tabs are one guest.
func (g *Game) watching() int {
	seen := map[string]bool{}
	for sub := range g.subs {
		if _, seated := g.seatOf(sub.guest); !seated {
			seen[sub.guest] = true
		}
	}
	return len(seen)
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
