package game

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// DefaultIdle is how long a game may go without a call before it is
// evicted, if it is over or nobody is watching it.
const DefaultIdle = 24 * time.Hour

// Hub maps game codes to live games. A game leaves the hub when it stops:
// evicted after Idle, or retired by a panic or a failed save.
type Hub struct {
	dice  rules.Dice
	store Store
	// newCode draws a game code; tests replace it to force a clash.
	newCode func() string
	// Idle is the eviction delay for games created from now on.
	Idle  time.Duration
	mu    sync.Mutex
	games map[string]*Game
	// reserved holds codes being saved by create, so two creates can't
	// pick the same one while neither holds mu.
	reserved map[string]bool
}

// NewHub returns a hub whose games roll with dice, are saved to st (nil
// keeps them in memory only), and are evicted after DefaultIdle.
func NewHub(dice rules.Dice, st Store) *Hub {
	if st == nil {
		st = nopStore{}
	}
	return &Hub{dice: dice, store: st, newCode: NewCode, Idle: DefaultIdle, games: map[string]*Game{}, reserved: map[string]bool{}}
}

// Create starts a game with creator in White's seat.
func (h *Hub) Create(creator string) (*Game, error) {
	return h.create(store.Game{White: creator})
}

// create saves and starts a game with sg's seats under a fresh code. A
// game with both seats filled (a rematch) starts with White's first-move
// deadline running.
func (h *Hub) create(sg store.Game) (*Game, error) {
	now := time.Now()
	sg.CreatedAt = now
	// The database write happens without holding mu, so lookups (every
	// request) don't wait for it; the code is reserved meanwhile.
	for tries := 0; ; tries++ {
		sg.Code = h.reserve()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := h.store.CreateGame(ctx, sg)
		cancel()
		if err != nil {
			h.unreserve(sg.Code)
		}
		if errors.Is(err, store.ErrCodeTaken) && tries < 10 {
			continue // an old saved game has this code
		}
		if err != nil {
			return nil, fmt.Errorf("create game: %w", err)
		}
		break
	}
	h.mu.Lock()
	delete(h.reserved, sg.Code)
	g := h.add(sg.Code, [2]string{sg.White, sg.Black})
	h.mu.Unlock()
	if sg.Black != "" {
		g.startCounting(now)
	}
	slog.Info("game created", "code", sg.Code, "white", guestTag(sg.White), "rematch_of", sg.RematchOf)
	go g.loop()
	return g, nil
}

// reserve draws a code no live game has and no other create is saving.
func (h *Hub) reserve() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	for {
		code := h.newCode()
		if h.games[code] == nil && !h.reserved[code] {
			h.reserved[code] = true
			return code
		}
	}
}

// unreserve frees a code whose save failed.
func (h *Hub) unreserve(code string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.reserved, code)
}

// add puts a new, not yet running game in the hub. The caller holds h.mu.
func (h *Hub) add(code string, seats [2]string) *Game {
	g := newGame(h, code, seats, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.games, code)
	})
	h.games[code] = g
	return g
}

// guestTag is a short one-way tag for a guest ID, for logs. The ID itself
// is the guest's cookie, so it must never be logged: anyone who read it
// could take that guest's seat.
func guestTag(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:4])
}

// Get returns the game with code, if any.
func (h *Hub) Get(code string) (*Game, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.games[code]
	return g, ok
}
