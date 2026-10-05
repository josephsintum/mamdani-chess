package game

import (
	"sync"
	"time"

	"mamdani-chess/rules"
)

// DefaultIdle is how long a game may go without a call before it is
// evicted, if it is over or nobody is watching it.
const DefaultIdle = 24 * time.Hour

// Hub maps game codes to live games. A game leaves the hub when it stops:
// evicted after Idle, or retired by a panic.
type Hub struct {
	dice rules.Dice
	// Idle is the eviction delay for games created from now on.
	Idle  time.Duration
	mu    sync.Mutex
	games map[string]*Game
}

// NewHub returns a hub whose games roll with dice and are evicted after
// DefaultIdle.
func NewHub(dice rules.Dice) *Hub {
	return &Hub{dice: dice, Idle: DefaultIdle, games: map[string]*Game{}}
}

// Create starts a game with creator in White's seat.
func (h *Hub) Create(creator string) *Game {
	h.mu.Lock()
	defer h.mu.Unlock()
	code := NewCode()
	for h.games[code] != nil {
		code = NewCode()
	}
	g := newGame(code, creator, h.dice, h.Idle, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.games, code)
	})
	h.games[code] = g
	return g
}

// Get returns the game with code, if any.
func (h *Hub) Get(code string) (*Game, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.games[code]
	return g, ok
}
