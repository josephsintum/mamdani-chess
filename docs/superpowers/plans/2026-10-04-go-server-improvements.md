# Go Server Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring the Go server up to what the Rust experiment showed is possible:
- stopped games (crashed or idle) leave memory cleanly;
- a 409 carries the caller's state;
- broadcast cost stays flat as spectators grow;
- static files are stricter and smaller;
- every request is logged.

**Architecture:**
- A game's goroutine gains a stop path. `stop()` closes `done`, closes every stream's channel and tells the hub to forget the game. Later calls get `ErrGone`, which the HTTP layer maps to 404.
- Two things trigger a stop:
  - a recovered panic, because the game may be half-updated;
  - a quiet day, once the game is over or nobody is watching.
- Broadcast builds one `*View` per role, not per stream.
- The rest are small HTTP-layer changes in `server/`, plus one option in `web/vite.config.ts`.

**Tech Stack:** Go 1.27 standard library only (`testing/synctest` for timers), SvelteKit `adapter-static` `precompress`.

**Spec:** `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md` §2. It says:
- "Finished games stay viewable for 24 h, then leave memory".
- "Illegal, out-of-turn, or stale-`seq` moves → `409` with the current `state`".
- "Unknown game code → `404`".

The Rust version these changes copy from is `docs/superpowers/specs/2026-10-04-rust-server-design.md`, with code in `server_rs/server/src/game/actor.rs` and `server_rs/server/src/http/`.

**Prototyped:** every code block here was run in a throwaway worktree before the plan was written. `go vet ./...` and `go test -race ./...` passed. The Rust parity test (`server_rs/server/tests/parity.rs`) still matched 100 games recorded from the changed Go server.

## Global Constraints

- Go version stays as `go.mod` says (`go 1.27.1`). No new Go dependencies: standard library only.
- Do not move or rewrite `go.mod` or `names/` (CLAUDE.md).
- The frontend contract does not change:
  - same routes and status codes for existing cases;
  - same error strings (`waiting for an opponent`, `not your turn`, `the game has moved on; reload the position`, `game is over`, `illegal move`, `you are not playing in this game`);
  - same View JSON.

  New: a 409 body adds a `state` key, and stopped games answer 404 `{"error":"game not found"}`.
- The one-goroutine-per-game model stays (roadmap: "The concurrency model stays as spec §2 says").
- Commits: stage only the files you changed (`git add <paths>`), never `git add -A`. No `Co-Authored-By: Claude` trailer, no "Generated with Claude Code" line.
- After every task: `go vet ./...` and `go test -race ./...` pass.

## Review Focus

- **A request racing a stop.** A move or stream request can arrive after `Hub.Get` returned the game, while the game is stopping. Expected: a quick 404, never a hang. This is pinned in Task 1 (`do` selects on `done`) and tested in Task 2 (join after eviction returns `ErrGone`).
- **A stream open when its game stops.** Expected: the SSE response ends cleanly, and the browser's reconnect then gets 404. Pinned in Task 1 (`TestCrashedGameClosesStreamsAndIsGone`) and Task 2 (`waitClosed` after eviction).
- **Shared views must stay read-only.** Two spectators get the same `*View`. Anything that modifies a received View would corrupt everyone's. Expected: no code writes to a received View. Task 3 adds a test that spectators share a view and that it has no legal moves.
- **`Accept-Encoding` spelled unusually.** For example uppercase `GZIP`, `br;q=0`, or no header at all. Expected: the right file, or the plain one. Pinned in Task 7's table test.
- **SSE behind the logging wrapper.** The status recorder must still pass through `Flush`, or streams stall until the buffer fills. Expected: streams still deliver at once. Covered by the existing stream tests, which go through `Server.ServeHTTP` once Task 8 lands. Task 8 reruns them.

## Files

| File | Change |
| --- | --- |
| `game/game.go` | Call/reply loop, `run`, `stop`, `ErrGone`, `Join` returns an error (Task 1). Idle timer (Task 2). Roles, shared views, `View` (Task 3). |
| `game/hub.go` | Forget a game when it stops (Task 1). `Idle` and `DefaultIdle` (Task 2). |
| `game/game_test.go` | `join` and `waitClosed` helpers; the panic test is rewritten (Task 1). `synctest` eviction tests (Task 2). Shared-view tests (Task 3). |
| `server/games.go` | Join before the SSE headers, end the stream when the game stops, `ErrGone` → 404 (Task 1). 409 with state (Task 4). Required `seq` (Task 5). |
| `server/static.go` | `_app` 404 (Task 6). Precompressed files (Task 7). |
| `server/server.go` | Request logging (Task 8). |
| `server/server_test.go` | `newTestServerWith` and the crash test (Task 1). Tests for each later task. |
| `server_rs/parity/main.go` | The new `Join` signature (Task 1). |
| `web/vite.config.ts` | `precompress: true` (Task 7). |
| `docs/superpowers/plans/2026-10-03-00-roadmap.md` | A note under "Carried forward" (Task 9). |

---

### Task 1: A game can stop, and a panic stops it

At the moment a recovered panic leaves the game running. But `move()` saves the position in `g.g.Play` and only then updates `last`, `log`, `lost` and `stats`. A panic in between leaves a half-updated game that keeps serving. After this task, the panicking call gets `ErrInternal`, and the game then stops. Its streams close, the hub forgets it, and later calls get `ErrGone`.

**Files:**
- Modify: `game/game.go`, `game/hub.go`, `server/games.go`, `server_rs/parity/main.go`
- Test: `game/game_test.go`, `server/server_test.go`

**Interfaces:**
- Produces:
  - `var ErrGone = errors.New("game not found")`.
  - `func (g *Game) Join(guest string) (*Sub, error)`: `Sub.C` is closed when the game stops.
  - `newGame(code, creator string, dice rules.Dice, onExit func()) *Game`. Task 2 adds an `idle` parameter.
  - In tests: `join(t, g, guest) *Sub`, `waitClosed(t, sub)`, and in `server`, `newTestServerWith(t, dice rules.Dice)`.

- [ ] **Step 1: Write the failing game test**

In `game/game_test.go`, add this helper above `func mv(`, and change every `g.Join(` in the file to `join(t, g, ` (18 places):

```go
// join opens a stream for guest and fails the test if the game has stopped.
func join(t *testing.T, g *Game, guest string) *Sub {
	t.Helper()
	sub, err := g.Join(guest)
	if err != nil {
		t.Fatalf("join %s: %v", guest, err)
	}
	return sub
}
```

Replace `TestPanicInGameIsContained` (keep the `panicky` type above it) with:

```go
func TestPanicRetiresTheGame(t *testing.T) {
	h := NewHub(&panicky{})
	g := h.Create("alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrInternal) {
		t.Fatalf("got %v, want ErrInternal", err)
	}
	if _, ok := h.Get(g.Code()); ok {
		t.Error("the hub still lists a game a panic retired")
	}
	if err := g.Move("alice", mv(t, "e2e4"), 0); !errors.Is(err, ErrGone) {
		t.Errorf("move after the panic: got %v, want ErrGone", err)
	}
	if _, err := g.Join("carol"); !errors.Is(err, ErrGone) {
		t.Errorf("join after the panic: got %v, want ErrGone", err)
	}
	waitClosed(t, a)
}

// waitClosed drains sub and fails the test unless its channel closes.
func waitClosed(t *testing.T, sub *Sub) {
	t.Helper()
	for {
		select {
		case _, ok := <-sub.C:
			if !ok {
				return
			}
		case <-time.After(time.Second):
			t.Fatal("stream still open 1s after the game stopped")
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./game/`
Expected: compile errors (`assignment mismatch: 2 variables but g.Join returns 1 value`, `undefined: ErrGone`).

- [ ] **Step 3: Implement the stop path in `game/game.go`**

Add to the error `var` block:

```go
	// ErrGone is returned once a game has stopped: it sat idle and was
	// evicted, or a panic retired it.
	ErrGone = errors.New("game not found")
```

Replace the `calls chan func()` field in `Game` with:

```go
	calls  chan call
	done   chan struct{} // closed when the game stops
	onExit func()        // tells the hub to forget the game
```

Replace `newGame`, `loop` and `do` with:

```go
// call is one function for the loop to run, and where to send the outcome.
type call struct {
	f     func()
	reply chan error
}

// newGame starts a game with White's seat taken by creator. onExit runs
// when the game stops.
func newGame(code, creator string, dice rules.Dice, onExit func()) *Game {
	g := &Game{
		code:   code,
		dice:   dice,
		calls:  make(chan call),
		done:   make(chan struct{}),
		onExit: onExit,
		g:      rules.NewGame(),
		seats:  [2]string{creator, ""},
		subs:   map[*Sub]struct{}{},
	}
	go g.loop()
	return g
}

func (g *Game) loop() {
	for {
		c := <-g.calls
		if err := g.run(c.f); err != nil {
			// A panic may have left the game half-updated (the position
			// moved but the log didn't, say), so the game can't go on.
			// Stop before replying, so the caller never sees it listed.
			g.stop()
			c.reply <- err
			return
		}
		c.reply <- nil
	}
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
	g.subs = nil
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
```

Change `Join` so it reports a stopped game:

```go
// Join opens a stream for guest. The creator plays White, the first other
// guest takes Black, and everyone after that watches. A guest who reconnects
// keeps their seat. The current view is already waiting on the Sub. C is
// closed when the game stops.
func (g *Game) Join(guest string) (*Sub, error) {
	ch := make(chan *View, 1)
	sub := &Sub{C: ch, ch: ch, guest: guest}
	err := g.do(func() {
		if g.seats[rules.Black] == "" && guest != g.seats[rules.White] {
			g.seats[rules.Black] = guest
			g.subs[sub] = struct{}{}
			g.broadcast() // White's view changes from waiting to playing
			return
		}
		g.subs[sub] = struct{}{}
		send(sub, g.view(guest))
	})
	if err != nil {
		return nil, err
	}
	return sub, nil
}
```

`Leave` stays as it is. After a stop it gets `ErrGone`, which it ignores.

- [ ] **Step 4: Let the hub forget stopped games (`game/hub.go`)**

In `Create`, replace `g := newGame(code, creator, h.dice)` with:

```go
	g := newGame(code, creator, h.dice, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		delete(h.games, code)
	})
```

Also change the `Hub` comment to: `// Hub maps game codes to live games. A game leaves the hub when it stops.`

- [ ] **Step 5: Run the game tests**

Run: `go test -race ./game/`
Expected: PASS. A `panic in game` error log line is expected.

- [ ] **Step 6: Write the failing server test**

In `server/server_test.go`, split the test server so the dice can be chosen. Replace the first lines of `newTestServer` with:

```go
func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	return newTestServerWith(t, odd{})
}

func newTestServerWith(t *testing.T, dice rules.Dice) (*Server, *httptest.Server) {
	t.Helper()
```

In its body, change `New(st, game.NewHub(odd{}), assets)` to `New(st, game.NewHub(dice), assets)`. Then add `"mamdani-chess/rules"` to the imports, and add:

```go
// boom panics on every roll: a stand-in for a bug inside a game.
type boom struct{}

func (boom) D8() int { panic("dice exploded") }

func TestCrashedGameClosesStreamsAndIsGone(t *testing.T) {
	_, ts := newTestServerWith(t, boom{})
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	a := alice.stream(code)
	a.state()
	bob.stream(code).state()
	a.state() // playing now
	move := "/api/games/" + code + "/move"
	if status, body := alice.post(move, `{"from":"e2","to":"e4","seq":0}`); status != http.StatusInternalServerError {
		t.Fatalf("the crashing move: %d %s, want 500", status, body)
	}
	ended := make(chan struct{})
	go func() {
		for a.sc.Scan() {
		}
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("stream still open 1s after the game crashed")
	}
	if status, body := alice.post(move, `{"from":"e2","to":"e4","seq":0}`); status != http.StatusNotFound {
		t.Errorf("after the crash: %d %s, want 404", status, body)
	}
}
```

Run: `go test ./server/`
Expected: compile error in `server/games.go` (`assignment mismatch` on `g.Join`).

- [ ] **Step 7: Update the HTTP layer (`server/games.go`)**

In `gameStream`, join before writing the SSE headers, so a stopped game can still get a 404. Replace everything from `fl, ok := startSSE(w)` down to `defer g.Leave(sub)` with:

```go
	sub, err := g.Join(guest)
	if err != nil { // the game stopped since Get
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
		return
	}
	defer g.Leave(sub)
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
```

In the `select`, end the stream when the game stops:

```go
		case v, open := <-sub.C:
			if !open { // the game stopped
				return
			}
			if writeEvent(w, fl, "state", v) != nil {
				return
			}
```

In `writeGameResult`, add this case before the `ErrBadDie` case:

```go
	case errors.Is(err, game.ErrGone):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
```

- [ ] **Step 8: Update the parity recorder (`server_rs/parity/main.go`)**

Replace the loop that joins the three guests with:

```go
	for _, guest := range []string{"white", "black", "spectator"} {
		sub, err := g.Join(guest)
		if err != nil {
			return nil, err
		}
		subs[guest] = sub
	}
```

- [ ] **Step 9: Run everything**

Run: `go vet ./... && go test -race ./...`
Expected: PASS for every package.

- [ ] **Step 10: Commit**

```bash
git add game/game.go game/hub.go game/game_test.go server/games.go server/server_test.go server_rs/parity/main.go
git commit -m "game: a panic stops the game instead of leaving it half-updated"
```

---

### Task 2: Evict quiet games

Spec §2: "Finished games stay viewable for 24 h, then leave memory." Rule: after `Idle` with no calls, a game stops if it is over or nobody is watching. A game with someone connected is never dropped.

**Files:**
- Modify: `game/game.go`, `game/hub.go`
- Test: `game/game_test.go`

**Interfaces:**
- Consumes: `stop()`, `ErrGone`, `join`, `waitClosed` (Task 1).
- Produces: `const DefaultIdle = 24 * time.Hour`. `Hub.Idle time.Duration`: the eviction delay for games created afterwards, set by `NewHub` to `DefaultIdle`. `newGame(code, creator string, dice rules.Dice, idle time.Duration, onExit func()) *Game`.

- [ ] **Step 1: Write the failing tests**

Add `"testing/synctest"` to the imports in `game/game_test.go`, then add the tests below. `synctest` gives the bubble a fake clock, so `time.Sleep(DefaultIdle)` returns at once. It also requires every goroutine started in the bubble to have exited by the end, which is why the last test waits for its game to go.

```go
func TestFinishedGameIsEvictedAfterAQuietDay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{})
		g := h.Create("alice")
		b := join(t, g, "bob") // still watching: a finished game goes anyway
		if err := g.Resign("alice"); err != nil {
			t.Fatal(err)
		}
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Fatal("finished game still listed after a quiet day")
		}
		waitClosed(t, b)
	})
}

func TestUnwatchedGameIsEvictedAfterAQuietDay(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{})
		g := h.Create("alice") // nobody ever opens the link
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Fatal("unwatched game still listed after a quiet day")
		}
		if _, err := g.Join("alice"); !errors.Is(err, ErrGone) {
			t.Errorf("join after eviction: got %v, want ErrGone", err)
		}
	})
}

func TestWatchedGameInProgressIsKept(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := NewHub(odd{})
		g := h.Create("alice")
		a, b := join(t, g, "alice"), join(t, g, "bob")
		time.Sleep(3 * DefaultIdle)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); !ok {
			t.Fatal("a game with both players watching was evicted")
		}
		// Once both leave it is unwatched, and goes after another quiet
		// day. (synctest also needs the game's goroutine gone by the end.)
		g.Leave(a)
		g.Leave(b)
		time.Sleep(DefaultIdle + time.Minute)
		synctest.Wait()
		if _, ok := h.Get(g.Code()); ok {
			t.Fatal("game still listed a quiet day after everyone left")
		}
	})
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./game/ -run Quiet`
Expected: compile error, `undefined: DefaultIdle`.

- [ ] **Step 3: Add the idle timer (`game/game.go`)**

Add `"time"` to the imports and an `idle time.Duration` field to `Game`, next to `dice`. Replace `newGame`'s signature, doc comment and field list:

```go
// newGame starts a game with White's seat taken by creator. After idle with
// no calls, the game stops if it is over or nobody is watching; onExit runs
// when it stops.
func newGame(code, creator string, dice rules.Dice, idle time.Duration, onExit func()) *Game {
	g := &Game{
		code:   code,
		dice:   dice,
		idle:   idle,
		calls:  make(chan call),
		done:   make(chan struct{}),
		onExit: onExit,
		g:      rules.NewGame(),
		seats:  [2]string{creator, ""},
		subs:   map[*Sub]struct{}{},
	}
	go g.loop()
	return g
}
```

Replace `loop` with:

```go
func (g *Game) loop() {
	idle := time.NewTimer(g.idle)
	defer idle.Stop()
	for {
		select {
		case c := <-g.calls:
			if err := g.run(c.f); err != nil {
				// A panic may have left the game half-updated (the position
				// moved but the log didn't, say), so the game can't go on.
				// Stop before replying, so the caller never sees it listed.
				g.stop()
				c.reply <- err
				return
			}
			c.reply <- nil
			idle.Reset(g.idle)
		case <-idle.C:
			if g.g.Result.Over || len(g.subs) == 0 {
				slog.Info("evicting idle game", "code", g.code)
				g.stop()
				return
			}
			idle.Reset(g.idle)
		}
	}
}
```

- [ ] **Step 4: Give the hub an idle setting (`game/hub.go`)**

Add `"time"` to the imports. Replace the `Hub` type and `NewHub` with:

```go
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
```

In `Create`, pass `h.Idle`: `newGame(code, creator, h.dice, h.Idle, func() { ... })`.

- [ ] **Step 5: Run the tests**

Run: `go test -race ./game/`
Expected: PASS. The three `Quiet`/`Kept` tests take milliseconds, because time is fake.

- [ ] **Step 6: Commit**

```bash
git add game/game.go game/hub.go game/game_test.go
git commit -m "game: evict games after a quiet day once over or unwatched"
```

---

### Task 3: One view per role

`broadcast` builds and sends a fresh `*View` per stream. But a view differs only by role: White, Black or spectator (`you`, and `legal` for the player to move). Build at most three per change and share them, so cost no longer grows with spectators. This copies `broadcast` in `server_rs/server/src/game/actor.rs`.

**Files:**
- Modify: `game/game.go`
- Test: `game/game_test.go`

**Interfaces:**
- Produces: `func (g *Game) View(guest string) (*View, error)`, guest's current view, used by Task 4. The unexported `role`, `roleOf` and `viewFor(r role)` replace `view(guest)`.

- [ ] **Step 1: Write the failing tests**

```go
func TestStreamsInOneRoleShareAView(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	a := join(t, g, "alice")
	join(t, g, "bob")
	c1, c2 := join(t, g, "carol"), join(t, g, "dave")
	recv(t, a)
	recv(t, c1)
	recv(t, c2)
	if err := g.Move("alice", mv(t, "e2e4"), 0); err != nil {
		t.Fatal(err)
	}
	v1, v2 := recv(t, c1), recv(t, c2)
	if v1 != v2 {
		t.Error("two spectators got separately built views")
	}
	if v1.You != "spectator" || len(v1.Legal) != 0 {
		t.Errorf("shared spectator view: you=%s legal=%d", v1.You, len(v1.Legal))
	}
	if va := recv(t, a); va == v1 || va.You != "white" {
		t.Errorf("white shares the spectators' view: you=%s", va.You)
	}
}

func TestViewIsTheCallersRole(t *testing.T) {
	g := NewHub(odd{}).Create("alice")
	join(t, g, "bob")
	for guest, want := range map[string]string{"alice": "white", "bob": "black", "carol": "spectator"} {
		v, err := g.View(guest)
		if err != nil || v.You != want {
			t.Errorf("%s: view %+v err %v, want you=%s", guest, v, err, want)
		}
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./game/ -run 'Share|ViewIs'`
Expected: compile error, `g.View undefined`.

- [ ] **Step 3: Implement roles (`game/game.go`)**

Replace `broadcast` with:

```go
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
	var views [3]*View
	for sub := range g.subs {
		r := g.roleOf(sub.guest)
		if views[r] == nil {
			views[r] = g.viewFor(r)
		}
		send(sub, views[r])
	}
}

// View returns guest's current view of the game.
func (g *Game) View(guest string) (*View, error) {
	var v *View
	err := g.do(func() { v = g.viewFor(g.roleOf(guest)) })
	return v, err
}
```

Rename `func (g *Game) view(guest string) *View` to `func (g *Game) viewFor(r role) *View`, and inside it replace `color, seated := g.seatOf(guest)` with:

```go
	color, seated := rules.Color(r), r != roleSpectator
```

In `Join`, change `send(sub, g.view(guest))` to `send(sub, g.viewFor(g.roleOf(guest)))`. Add `// Views are shared between streams: read them, never change them.` to the `View` type's comment in `game/view.go`.

- [ ] **Step 4: Run the tests**

Run: `go test -race ./game/ ./server/`
Expected: PASS.

- [ ] **Step 5: Check the Rust parity test still matches**

Run:

```bash
go run ./server_rs/parity -n 100 -seed 7 > /tmp/parity.jsonl
(cd server_rs && PARITY_FILE=/tmp/parity.jsonl cargo test --release -p server --test parity)
```

Expected: `parity: 100 games match`. This proves the views didn't change. Skip it if Rust isn't installed; CI runs it.

- [ ] **Step 6: Commit**

```bash
git add game/game.go game/view.go game/game_test.go
git commit -m "game: build one view per role and share it between streams"
```

---

### Task 4: A 409 carries the caller's state

Spec §2: "Illegal, out-of-turn, or stale-`seq` moves → `409` with the current `state`; the browser resyncs." Add a `state` key next to `error`. The frontend only reads `.error` today (`web/src/lib/game.ts`), so this is additive.

**Files:**
- Modify: `server/games.go`
- Test: `server/server_test.go`

**Interfaces:**
- Consumes: `(*game.Game).View(guest)` (Task 3), `ErrGone` (Task 1).
- Produces: `writeGameResult(w http.ResponseWriter, g *game.Game, guest string, err error)`.

- [ ] **Step 1: Write the failing test**

```go
func TestConflictCarriesTheCallersState(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	bob.stream(code).state()
	status, body := bob.post("/api/games/"+code+"/move", `{"from":"e7","to":"e5","seq":0}`)
	var out struct {
		Error string
		State *game.View
	}
	if status != http.StatusConflict || json.Unmarshal([]byte(body), &out) != nil {
		t.Fatalf("got %d %s, want a 409", status, body)
	}
	if out.Error != "not your turn" || out.State == nil || out.State.You != "black" || out.State.Seq != 0 {
		t.Errorf("error %q state %+v", out.Error, out.State)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./server/ -run Conflict`
Expected: FAIL, `state <nil>`.

- [ ] **Step 3: Implement**

In `server/games.go`, replace `writeGameResult` with the version below, and change its two callers to `writeGameResult(w, g, guest, g.Move(guest, m, req.Seq))` and `writeGameResult(w, g, guest, g.Resign(guest))`.

```go
// writeGameResult maps the outcome of a move or resignation to a response.
// Success is 204: the new state arrives on the stream. A conflict carries
// the caller's current view, so the client can resync at once.
func writeGameResult(w http.ResponseWriter, g *game.Game, guest string, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, game.ErrNotPlayer):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
	case errors.Is(err, game.ErrGone):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "game not found"})
	case errors.Is(err, rules.ErrBadDie), errors.Is(err, game.ErrInternal):
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
	default:
		body := map[string]any{"error": err.Error()}
		if v, verr := g.View(guest); verr == nil {
			body["state"] = v
		}
		writeJSON(w, http.StatusConflict, body)
	}
}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./server/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/games.go server/server_test.go
git commit -m "server: a 409 carries the caller's current state"
```

---

### Task 5: Require `seq` in a move

A missing `seq` currently decodes as 0. On the first turn that lets a move made from an unknown position through. The frontend always sends `seq`.

**Files:**
- Modify: `server/games.go`
- Test: `server/server_test.go`

- [ ] **Step 1: Write the failing test case**

In `TestMoveErrors`, add after the `e9` case:

```go
		{alice, move, `{"from":"e2","to":"e4"}`, http.StatusBadRequest}, // no seq
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./server/ -run TestMoveErrors`
Expected: FAIL, `... 204 , want 400`.

- [ ] **Step 3: Implement**

```go
type moveRequest struct {
	game.MoveJSON
	Seq *int `json:"seq"` // required: a move must say which position it was made from
}
```

In `gameMove`, change the decode check to `if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Seq == nil {`. Pass `*req.Seq` to `g.Move`.

- [ ] **Step 4: Run the tests**

Run: `go test -race ./server/`
Expected: PASS. The friend-game test still sends `"seq":0`, so that still works.

- [ ] **Step 5: Commit**

```bash
git add server/games.go server/server_test.go
git commit -m "server: a move must include seq"
```

---

### Task 6: 404 for a missing build asset

A request for a missing `/_app/...` file currently gets `index.html` with status 200. That happens to a stale tab after a deploy asks for an old chunk. A browser can't run HTML as a script, and SvelteKit's chunk-load recovery needs a real error.

**Files:**
- Modify: `server/static.go`
- Test: `server/server_test.go`

- [ ] **Step 1: Write the failing test case**

In `TestStaticAndFallback`, add after the `app.js` case:

```go
		{"/_app/immutable/gone.js", "404 page not found", "no-cache", 404},
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./server/ -run TestStaticAndFallback`
Expected: FAIL, `status 200, want 404`.

- [ ] **Step 3: Implement**

In `static`, replace the fallback `if` with:

```go
	if info, err := fs.Stat(s.assets, name); err != nil || info.IsDir() {
		if strings.HasPrefix(name, "_app/") {
			// A missing build asset (an old chunk after a deploy, say) is a
			// real 404: the app shell in its place can't run as a script.
			w.Header().Set("Cache-Control", "no-cache")
			http.NotFound(w, r)
			return
		}
		name = "index.html"
	}
```

- [ ] **Step 4: Run the tests**

Run: `go test -race ./server/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add server/static.go server/server_test.go
git commit -m "server: 404 for a missing build asset instead of the app shell"
```

---

### Task 7: Serve precompressed files

`adapter-static` can write `.br` and `.gz` copies of every file (`precompress: true`). The server then sends the smallest copy the browser accepts. Measured in the prototype: the build grows from 224 KB to 516 KB on disk, and so does the embedded binary. The bytes sent per page load shrink.

**Files:**
- Modify: `server/static.go`, `web/vite.config.ts`
- Test: `server/server_test.go`

- [ ] **Step 1: Write the failing test**

In `newTestServerWith`, add two entries to the `fstest.MapFS`:

```go
		"favicon.svg.br":        {Data: []byte("brotli bytes")},
		"favicon.svg.gz":        {Data: svgGzip},
```

Add `"bytes"` and `"compress/gzip"` to the imports, then:

```go
// svgGzip is "<svg/>" gzipped, the way `precompress` writes favicon.svg.gz.
// It must be real gzip: Go's default client asks for gzip and unpacks it.
var svgGzip = func() []byte {
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	zw.Write([]byte("<svg/>"))
	zw.Close()
	return b.Bytes()
}()

func get(t *testing.T, url, acceptEncoding string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	// A bare Transport would add gzip itself and hide the header we sent.
	resp, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestPrecompressedFiles(t *testing.T) {
	_, ts := newTestServer(t)
	for _, c := range []struct{ accept, wantEncoding, wantBody string }{
		{"gzip, deflate, br", "br", "brotli bytes"},
		{"gzip", "gzip", string(svgGzip)},
		{"GZIP", "gzip", string(svgGzip)},
		{"br;q=0, gzip", "gzip", string(svgGzip)},
		{"", "", "<svg/>"},
	} {
		resp := get(t, ts.URL+"/favicon.svg", c.accept)
		body, _ := io.ReadAll(resp.Body)
		if got := resp.Header.Get("Content-Encoding"); got != c.wantEncoding || string(body) != c.wantBody {
			t.Errorf("Accept-Encoding %q: encoding %q body %q", c.accept, got, body)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "image/svg+xml" {
			t.Errorf("Accept-Encoding %q: Content-Type %q", c.accept, ct)
		}
		if resp.Header.Get("Vary") != "Accept-Encoding" {
			t.Errorf("Accept-Encoding %q: missing Vary", c.accept)
		}
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./server/ -run TestPrecompressedFiles`
Expected: FAIL, `encoding "" body "<svg/>"`.

- [ ] **Step 3: Implement (`server/static.go`)**

Add `"mime"` and `"strconv"` to the imports. Replace the last line of `static` (`http.ServeFileFS(w, r, s.assets, name)`) with:

```go
	w.Header().Add("Vary", "Accept-Encoding")
	file, encoding := s.compressed(r, name)
	if encoding != "" {
		w.Header().Set("Content-Encoding", encoding)
		if ct := mime.TypeByExtension(path.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	}
	http.ServeFileFS(w, r, s.assets, file)
}

// compressed picks the precompressed copy of name that the build has and
// the browser accepts, brotli first, or name itself with no encoding.
func (s *Server) compressed(r *http.Request, name string) (file, encoding string) {
	accept := r.Header.Get("Accept-Encoding")
	for _, c := range []struct{ encoding, ext string }{{"br", ".br"}, {"gzip", ".gz"}} {
		if !accepts(accept, c.encoding) {
			continue
		}
		if _, err := fs.Stat(s.assets, name+c.ext); err == nil {
			return name + c.ext, c.encoding
		}
	}
	return name, ""
}

// accepts reports whether an Accept-Encoding header allows encoding. A
// token with q=0 is a refusal.
func accepts(header, encoding string) bool {
	for _, part := range strings.Split(header, ",") {
		token, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		if !strings.EqualFold(strings.TrimSpace(token), encoding) {
			continue
		}
		if q, ok := strings.CutPrefix(strings.TrimSpace(params), "q="); ok {
			if v, err := strconv.ParseFloat(q, 64); err == nil && v == 0 {
				return false
			}
		}
		return true
	}
	return false
}
```

Content-Type is set from the original name because `.br` and `.gz` have no type of their own.

- [ ] **Step 4: Run the tests**

Run: `go test -race ./server/`
Expected: PASS. `TestStaticAndFallback` still passes: its default client asks for gzip, gets the real gzip fixture, and unpacks it.

- [ ] **Step 5: Turn on precompression in the build (`web/vite.config.ts`)**

```ts
			adapter: adapter({ fallback: 'index.html', precompress: true })
```

Run: `cd web && pnpm build && ls build | head`
Expected: `favicon.svg.br`, `favicon.svg.gz`, `index.html.br` and `index.html.gz` sit next to the originals. adapter-static compresses after writing the fallback page, so the SPA shell is compressed too.

- [ ] **Step 6: Commit**

```bash
git add server/static.go server/server_test.go web/vite.config.ts
git commit -m "server: serve the build's brotli and gzip copies when accepted"
```

---

### Task 8: Log every request

One line per request: method, path, status and duration. Health checks log at debug level, so Railway's probes don't flood the log.

**Files:**
- Modify: `server/server.go`
- Test: `server/server_test.go`

**Interfaces:**
- Consumes: the `get` test helper and the `bytes` import from Task 7.
- Produces: `Server.log *slog.Logger`, defaulting to `slog.Default()`. Tests can replace it.

- [ ] **Step 1: Write the failing test**

Add `"log/slog"` to the test imports. In `newTestServerWith`, after `s := New(...)`, add:

```go
	s.log = slog.New(slog.DiscardHandler) // tests that check logging swap in their own
```

Then:

```go
func TestRequestsAreLogged(t *testing.T) {
	s, ts := newTestServer(t)
	var buf bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&buf, nil))
	get(t, ts.URL+"/api/nope", "")
	if line := buf.String(); !strings.Contains(line, "method=GET path=/api/nope status=404") {
		t.Errorf("log %q", line)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./server/ -run Logged`
Expected: compile error, `s.log undefined`.

- [ ] **Step 3: Implement (`server/server.go`)**

Add `"log/slog"` to the imports, add `log *slog.Logger` to `Server`, and set `log: slog.Default(),` in `New`. Replace `ServeHTTP` with:

```go
// ServeHTTP routes the request and logs one line for it: method, path,
// status and how long it took. Health checks log at debug level.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	s.mux.ServeHTTP(rec, r)
	level := slog.LevelInfo
	if r.URL.Path == "/healthz" {
		level = slog.LevelDebug
	}
	s.log.Log(r.Context(), level, "request",
		"method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
}

// statusRecorder remembers the status a handler wrote. It passes Flush
// through, so SSE streams still work behind it.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wrote = true
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the real writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
```

A stream logs once, when it ends, with its full duration.

- [ ] **Step 4: Run all server tests**

Run: `go test -race ./server/`
Expected: PASS. The SSE tests (`TestFriendGameOverHTTP`, `TestGameStreamHeartbeat`, `TestCloseEndsStreamsButNotRequests`) now go through the recorder, which proves `Flush` passes through.

- [ ] **Step 5: Commit**

```bash
git add server/server.go server/server_test.go
git commit -m "server: log one line per request"
```

---

### Task 9: Verify end to end and record it

**Files:**
- Modify: `docs/superpowers/plans/2026-10-03-00-roadmap.md`

- [ ] **Step 1: Full checks**

Run:

```bash
go vet ./... && go test -race ./...
(cd web && pnpm check && pnpm test && pnpm build)
go run ./server_rs/parity -n 300 -seed 1000 > /tmp/parity.jsonl
(cd server_rs && PARITY_FILE=/tmp/parity.jsonl cargo test --release -p server --test parity)
docker build -t mamdani-chess .
```

Expected: everything passes, and you see `parity: 300 games match`. The parity and Docker steps need Rust and Docker; CI runs both if they aren't installed locally.

- [ ] **Step 2: Play a game in two browsers**

Run `go run ./cmd/server` and `pnpm dev` in `web/`. Then:
1. Create a game in one browser and join it in a private window.
2. Play a few moves and resign.
3. Watch the server log: one `request` line per call, and a stream's line appears when it closes.
4. In devtools, the JS files load with `Content-Encoding: br`. This needs the embedded build: `go build -tags embedweb ./cmd/server` after `pnpm build`.

- [ ] **Step 3: Note it in the roadmap**

Under "Carried forward to later plans" in `docs/superpowers/plans/2026-10-03-00-roadmap.md`, add:

```markdown
- **Game lifecycle (plan [2026-10-04-go-server-improvements](2026-10-04-go-server-improvements.md)).** A game stops when a panic retires it or after a quiet day once over or unwatched; stopped games answer 404. Milestone 05's timers go in the same `select` in `game.loop`, and its tests can use `testing/synctest` instead of the spec's `Clock` interface. Persistence must reload stopped-but-unfinished games from SQLite on demand.
```

- [ ] **Step 4: Commit**

```bash
git add docs/superpowers/plans/2026-10-03-00-roadmap.md
git commit -m "Docs: roadmap notes the game lifecycle from the server improvements"
```
