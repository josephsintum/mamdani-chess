// Package match is the quick-match queue: first come, first served, with
// random colors. A guest is in the queue while they have a ticket open; two
// tabs share one place in line.
package match

import (
	"log/slog"
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
)

// CreateFunc starts a game between two guests and returns its code.
// playtest marks a game a playtest joined (the stats leave it out).
type CreateFunc func(white, black string, playtest bool) (string, error)

// Queue pairs guests in the order they arrived.
type Queue struct {
	create CreateFunc
	// flip picks colors: true puts the guest who waited longer on White.
	// Tests replace it.
	flip func() bool

	mu      sync.Mutex
	line    []*entry // oldest first
	byGuest map[string]*entry
	looking atomic.Int64
}

// entry is one guest's place in line and their open tickets. playtest is
// set when any of their tickets came from a playtest.
type entry struct {
	guest    string
	playtest bool
	tickets  map[*Ticket]struct{}
}

// Ticket is one open stream in the queue. C receives the game's code once
// its guest is matched.
type Ticket struct {
	C     <-chan string
	c     chan string
	q     *Queue
	guest string
}

// New returns an empty queue that starts games with create.
func New(create CreateFunc) *Queue {
	return &Queue{
		create:  create,
		flip:    func() bool { return rand.N(2) == 0 },
		byGuest: map[string]*entry{},
	}
}

// Looking is how many guests are waiting.
func (q *Queue) Looking() int { return int(q.looking.Load()) }

// LookingFor is how many guests other than guest are waiting, so a guest
// never sees themselves counted (say, just after cancelling).
func (q *Queue) LookingFor(guest string) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	n := len(q.line)
	if q.byGuest[guest] != nil {
		n--
	}
	return n
}

// Join puts guest in line, or adds a ticket to their place if another tab
// already holds one, then pairs whoever can be paired. playtest says the
// ticket comes from a playtest.
func (q *Queue) Join(guest string, playtest bool) *Ticket {
	c := make(chan string, 1)
	t := &Ticket{C: c, c: c, q: q, guest: guest}
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.byGuest[guest]
	if e == nil {
		e = &entry{guest: guest, tickets: map[*Ticket]struct{}{}}
		q.byGuest[guest] = e
		q.line = append(q.line, e)
	}
	e.tickets[t] = struct{}{}
	e.playtest = e.playtest || playtest
	q.pair()
	return t
}

// Leave closes the ticket. The guest leaves the line with their last
// ticket. Safe to call more than once, and after a match.
func (t *Ticket) Leave() {
	q := t.q
	q.mu.Lock()
	defer q.mu.Unlock()
	e := q.byGuest[t.guest]
	if e == nil {
		return
	}
	delete(e.tickets, t)
	if len(e.tickets) == 0 {
		q.remove(e)
	}
}

// pair starts games for the two longest-waiting guests while there are
// two. If a game can't be created, both keep their places and the next
// arrival tries again. The caller holds q.mu.
func (q *Queue) pair() {
	defer func() { q.looking.Store(int64(len(q.line))) }()
	for len(q.line) >= 2 {
		a, b := q.line[0], q.line[1]
		white, black := a, b
		if !q.flip() {
			white, black = b, a
		}
		code, err := q.create(white.guest, black.guest, a.playtest || b.playtest)
		if err != nil {
			slog.Error("quick match: create game", "err", err)
			return
		}
		for _, e := range []*entry{a, b} {
			for t := range e.tickets {
				t.c <- code // buffered, and each ticket hears once
			}
			q.remove(e)
		}
		slog.Info("quick match", "code", code)
	}
}

// remove takes e out of line. The caller holds q.mu.
func (q *Queue) remove(e *entry) {
	delete(q.byGuest, e.guest)
	if i := slices.Index(q.line, e); i >= 0 {
		q.line = slices.Delete(q.line, i, i+1)
	}
	q.looking.Store(int64(len(q.line)))
}
