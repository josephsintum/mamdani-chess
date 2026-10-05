// Command loadgen plays many games at once against a running server, Go or
// Rust (server_rs), through the public API, and reports throughput and
// latency. Each game has two players and some spectators, all on SSE
// streams. Players pick a random legal move as soon as it is their turn,
// or after a random thinking time.
//
// Burst (how fast can it go):
//
//	go run ./tools/loadgen -games 200 -spectators 10 -plies 100
//
// Many viewers at a human pace (about 10,000 open streams):
//
//	go run ./tools/loadgen -games 500 -spectators 20 -plies 40 -ramp 25ms -think 300ms
//
// The tool measures the client side. For the server's CPU and memory, start
// the server yourself and read them with ps while the run goes, for example
// `ps -o rss=,time= -p <pid>` before and after.
//
// On macOS the listen backlog is 128 (kern.ipc.somaxconn), so opening
// thousands of streams in the same instant makes the kernel reset
// connections. -ramp spreads game starts out; for bigger bursts raise the
// backlog with `sudo sysctl kern.ipc.somaxconn=4096`. Linux defaults to 4096.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// config is one load run.
type config struct {
	url        string
	games      int
	spectators int           // per game
	plies      int           // moves per game, at most
	ramp       time.Duration // delay between game starts
	think      time.Duration // average thinking time before a move
	stall      time.Duration // give up on a game that makes no progress
}

// result is what a run measured.
type result struct {
	wall                  time.Duration
	moves, events, errors int64
	eventBytes            int64
	dialRetries           int64
	peakStreams           int64
	moveLatency           []time.Duration // move POST round trip
	viewLatency           []time.Duration // move POST sent → state seen by a viewer
}

// view is the part of a state event a player needs.
type view struct {
	Status string `json:"status"`
	Turn   string `json:"turn"`
	Seq    int    `json:"seq"`
	Legal  []struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Promo string `json:"promo,omitempty"`
	} `json:"legal"`
}

// run holds the shared state of one load run.
type run struct {
	cfg       config
	transport *http.Transport
	errShown  atomic.Int64

	moves, events, errors, eventBytes, dialRetries atomic.Int64
	open, peak                                     atomic.Int64

	mu                       sync.Mutex
	moveLatency, viewLatency []time.Duration
}

func main() {
	var c config
	flag.StringVar(&c.url, "url", "http://localhost:8080", "server to load")
	flag.IntVar(&c.games, "games", 100, "games to play at once")
	flag.IntVar(&c.spectators, "spectators", 5, "spectators per game")
	flag.IntVar(&c.plies, "plies", 60, "moves per game, at most")
	flag.DurationVar(&c.ramp, "ramp", 2*time.Millisecond, "delay between game starts")
	flag.DurationVar(&c.think, "think", 0, "average thinking time before each move")
	flag.DurationVar(&c.stall, "stall", 30*time.Second, "give up on a game that makes no progress for this long")
	flag.Parse()

	r := load(c)
	streams := c.games * (c.spectators + 2)
	fmt.Printf("games %d, spectators/game %d, streams %d, plies/game %d\n", c.games, c.spectators, streams, c.plies)
	fmt.Printf("wall %.2fs  moves %d (%.0f/s)  state events %d (%.0f/s, %.1f MB)  errors %d\n",
		r.wall.Seconds(), r.moves, float64(r.moves)/r.wall.Seconds(),
		r.events, float64(r.events)/r.wall.Seconds(), float64(r.eventBytes)/1e6, r.errors)
	fmt.Printf("move POST     p50 %v  p99 %v  max %v\n", pct(r.moveLatency, .5), pct(r.moveLatency, .99), pct(r.moveLatency, 1))
	fmt.Printf("move → viewer p50 %v  p99 %v  max %v\n", pct(r.viewLatency, .5), pct(r.viewLatency, .99), pct(r.viewLatency, 1))
	fmt.Printf("peak open streams %d, dial retries %d\n", r.peakStreams, r.dialRetries)
	if r.errors > 0 {
		os.Exit(1)
	}
}

// load plays cfg.games games and returns what it measured.
func load(cfg config) result {
	r := &run{cfg: cfg, transport: &http.Transport{MaxIdleConns: 1 << 16, MaxIdleConnsPerHost: 1 << 16}}
	defer r.transport.CloseIdleConnections()
	var wg sync.WaitGroup
	start := time.Now()
	for i := range cfg.games {
		wg.Go(func() { r.game(uint64(i)) })
		time.Sleep(cfg.ramp)
	}
	wg.Wait()
	return result{
		wall:        time.Since(start),
		moves:       r.moves.Load(),
		events:      r.events.Load(),
		errors:      r.errors.Load(),
		eventBytes:  r.eventBytes.Load(),
		dialRetries: r.dialRetries.Load(),
		peakStreams: r.peak.Load(),
		moveLatency: r.moveLatency,
		viewLatency: r.viewLatency,
	}
}

func (r *run) fail(what string, err error) {
	r.errors.Add(1)
	if r.errShown.Add(1) <= 5 {
		fmt.Fprintln(os.Stderr, "error:", what+":", err)
	}
}

// client is one browser: its own cookie jar, so its own guest.
func (r *run) client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar, Transport: r.transport}
}

// do sends the request mk builds. It retries only when the connection never
// opened: anything later may have reached the server, and joins and moves
// are not idempotent (a repeated join could seat a fresh guest).
func (r *run) do(c *http.Client, mk func() *http.Request) (*http.Response, error) {
	var err error
	for attempt := range 8 {
		var resp *http.Response
		if resp, err = c.Do(mk()); err == nil {
			return resp, nil
		}
		var op *net.OpError
		if !errors.As(err, &op) || op.Op != "dial" {
			return nil, err
		}
		r.dialRetries.Add(1)
		time.Sleep(time.Duration(10<<attempt) * time.Millisecond)
	}
	return nil, err
}

func (r *run) post(c *http.Client, path string, body []byte) (*http.Response, error) {
	return r.do(c, func() *http.Request {
		req, _ := http.NewRequest(http.MethodPost, r.cfg.url+path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return req
	})
}

// stream opens code's SSE stream for c, closes opened once the server has
// answered, and calls on with every state event until the game is over or
// the stream ends.
func (r *run) stream(c *http.Client, code string, opened chan<- struct{}, on func(view, time.Time)) {
	resp, err := r.do(c, func() *http.Request {
		req, _ := http.NewRequest(http.MethodGet, r.cfg.url+"/api/games/"+code+"/stream", nil)
		return req
	})
	close(opened)
	if err != nil {
		r.fail("stream", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		r.fail("stream", fmt.Errorf("status %d", resp.StatusCode))
		return
	}
	if n := r.open.Add(1); n > r.peak.Load() {
		r.peak.Store(n) // racy max: close enough for a report
	}
	defer r.open.Add(-1)
	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 64<<10), 4<<20)
	for sc.Scan() {
		data, ok := bytes.CutPrefix(sc.Bytes(), []byte("data:"))
		if !ok {
			continue // event names, blank lines and keep-alive comments
		}
		r.events.Add(1)
		r.eventBytes.Add(int64(len(data)))
		var v view
		if err := json.Unmarshal(bytes.TrimSpace(data), &v); err != nil {
			r.fail("state event", err)
			return
		}
		on(v, time.Now())
		if v.Status == "over" {
			return
		}
	}
}

// game plays one game: create it, seat both players, add spectators, then
// alternate random legal moves until plies or the game ends, and resign so
// every stream closes.
func (r *run) game(seed uint64) {
	rnd := rand.New(rand.NewPCG(seed, 9))
	white, black := r.client(), r.client()
	resp, err := r.post(white, "/api/games", nil)
	if err != nil {
		r.fail("create", err)
		return
	}
	var created struct{ Code string }
	err = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	if err != nil || created.Code == "" {
		r.fail("create", fmt.Errorf("status %d: %v", resp.StatusCode, err))
		return
	}
	code := created.Code

	// When each turn's move was sent, to time its arrival at every viewer.
	var sentMu sync.Mutex
	sent := map[int]time.Time{}
	seen := func(v view, at time.Time) {
		sentMu.Lock()
		t0, ok := sent[v.Seq]
		sentMu.Unlock()
		if ok {
			r.mu.Lock()
			r.viewLatency = append(r.viewLatency, at.Sub(t0))
			r.mu.Unlock()
		}
	}

	var streams sync.WaitGroup
	open := func(c *http.Client, on func(view, time.Time)) {
		opened := make(chan struct{})
		streams.Go(func() { r.stream(c, code, opened, on) })
		<-opened
	}
	// Players join first: the first guest after the creator to open a
	// stream takes Black's seat.
	states := map[string]chan view{"white": make(chan view, 256), "black": make(chan view, 256)}
	clients := map[string]*http.Client{"white": white, "black": black}
	for _, color := range []string{"white", "black"} {
		ch := states[color]
		open(clients[color], func(v view, at time.Time) {
			seen(v, at)
			select {
			case ch <- v:
			default: // nobody is waiting on an old state; drop it
			}
		})
	}
	for range r.cfg.spectators {
		open(r.client(), seen)
	}

	for seq := 0; seq < r.cfg.plies; seq++ {
		mover := "white"
		if seq%2 == 1 {
			mover = "black"
		}
		v, ok := r.waitTurn(states[mover], mover, seq)
		if !ok {
			if v.Status != "over" {
				r.fail("game "+code, fmt.Errorf("no turn for %s at seq %d within %v", mover, seq, r.cfg.stall))
			}
			break
		}
		if r.cfg.think > 0 {
			time.Sleep(time.Duration(rnd.Int64N(int64(2 * r.cfg.think))))
		}
		m := v.Legal[rnd.IntN(len(v.Legal))]
		body, _ := json.Marshal(map[string]any{"from": m.From, "to": m.To, "promo": m.Promo, "seq": seq})
		sentMu.Lock()
		sent[seq+1] = time.Now()
		sentMu.Unlock()
		t0 := time.Now()
		resp, err := r.post(clients[mover], "/api/games/"+code+"/move", body)
		if err != nil {
			r.fail("move", err)
			break
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			r.fail("move", fmt.Errorf("status %d", resp.StatusCode))
			break
		}
		r.mu.Lock()
		r.moveLatency = append(r.moveLatency, time.Since(t0))
		r.mu.Unlock()
		r.moves.Add(1)
	}
	if resp, err := r.post(white, "/api/games/"+code+"/resign", nil); err == nil {
		resp.Body.Close() // 409 if the game had already ended: fine
	} else {
		r.fail("resign", err)
	}
	streams.Wait()
}

// waitTurn waits for mover's state at seq with legal moves. It returns
// false if the game ended or nothing arrived within the stall limit.
func (r *run) waitTurn(states <-chan view, mover string, seq int) (view, bool) {
	stall := time.NewTimer(r.cfg.stall)
	defer stall.Stop()
	for {
		select {
		case v, open := <-states:
			switch {
			case !open:
				return v, false
			case v.Status == "over":
				return v, false
			case v.Seq == seq && v.Status == "playing" && v.Turn == mover && len(v.Legal) > 0:
				return v, true
			}
		case <-stall.C:
			return view{}, false
		}
	}
}

func pct(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	ds = slices.Clone(ds)
	slices.Sort(ds)
	return ds[int(p*float64(len(ds)-1))].Round(time.Microsecond)
}
