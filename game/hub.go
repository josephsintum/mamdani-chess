package game

import (
	"sync"

	"mamdani-chess/rules"
)

// Hub maps game codes to live games. Games stay in memory until the server
// restarts; saving and resuming them comes in a later milestone.
type Hub struct {
	dice  rules.Dice
	mu    sync.Mutex
	games map[string]*Game
}

// NewHub returns a hub whose games roll with dice.
func NewHub(dice rules.Dice) *Hub {
	return &Hub{dice: dice, games: map[string]*Game{}}
}

// Create starts a game with creator in White's seat.
func (h *Hub) Create(creator string) *Game {
	h.mu.Lock()
	defer h.mu.Unlock()
	code := NewCode()
	for h.games[code] != nil {
		code = NewCode()
	}
	g := newGame(code, creator, h.dice)
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
