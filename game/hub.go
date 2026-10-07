package game

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"mamdani-chess/names"
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

// Create starts a friend game with creator in White's seat, giving them a
// name first if they have none.
func (h *Hub) Create(creator string) (*Game, error) {
	name, err := h.name(creator)
	if err != nil {
		return nil, err
	}
	return h.create(store.Game{White: creator, WhiteName: name})
}

// CreatePair starts a game between two guests, both seated (quick match),
// giving each a name first if they have none. White's first-move deadline
// starts at once.
func (h *Hub) CreatePair(white, black string) (*Game, error) {
	whiteName, err := h.name(white)
	if err != nil {
		return nil, err
	}
	blackName, err := h.name(black)
	if err != nil {
		return nil, err
	}
	return h.create(store.Game{White: white, WhiteName: whiteName, Black: black, BlackName: blackName})
}

// name returns the guest's name, creating it if they have none.
func (h *Hub) name(guest string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
	defer cancel()
	name, err := h.store.EnsureGuest(ctx, guest, names.Random)
	if err != nil {
		return "", fmt.Errorf("guest name: %w", err)
	}
	return name, nil
}

// create saves and starts a game with sg's seats under a fresh code. A
// game with both seats filled (quick match or a rematch) starts with
// White's first-move deadline running.
func (h *Hub) create(sg store.Game) (*Game, error) {
	now := time.Now()
	sg.CreatedAt = now
	// The database write happens without holding mu, so lookups (every
	// request) don't wait for it; the code is reserved meanwhile.
	for tries := 0; ; tries++ {
		sg.Code = h.reserve()
		ctx, cancel := context.WithTimeout(context.Background(), storeTimeout)
		err := h.store.CreateGame(ctx, sg)
		cancel()
		if err == nil {
			break
		}
		h.unreserve(sg.Code)
		if !errors.Is(err, store.ErrCodeTaken) || tries >= 10 {
			return nil, fmt.Errorf("create game: %w", err)
		}
		// an old saved game has this code: draw another
	}
	h.mu.Lock()
	delete(h.reserved, sg.Code)
	g := h.add(sg)
	h.mu.Unlock()
	if sg.Black != "" {
		g.startCounting(now)
	}
	slog.Info("game created", "code", sg.Code, "white", guestTag(sg.White), "rematch_of", sg.RematchOf)
	g.publish() // listed as soon as Create returns
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
func (h *Hub) add(sg store.Game) *Game {
	g := newGame(h, sg, h.forget(sg.Code))
	h.games[sg.Code] = g
	return g
}

// forget returns a game's onExit: it takes code out of the hub.
func (h *Hub) forget(code string) func() {
	return func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.games, code)
	}
}

// guestTag is a short one-way tag for a guest ID, for logs. The ID itself
// is the guest's cookie, so it must never be logged: anyone who read it
// could take that guest's seat.
func guestTag(id string) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:4])
}

// Get returns the game with code, if any. Codes are upper case; a code
// typed in lower case finds the same game.
func (h *Hub) Get(code string) (*Game, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	g, ok := h.games[strings.ToUpper(code)]
	return g, ok
}
