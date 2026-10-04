# Walking Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prove the whole stack end to end before building the game: a Go server that persists to SQLite and pushes updates over SSE, a SvelteKit static frontend embedded in the binary, CI, and a live Railway deploy where two browser tabs stay in sync.

**Architecture:** One Go binary (`cmd/server`) serves `/api/*` and the built SvelteKit SPA. The demo feature is a throwaway "honk" counter: `POST /api/honk` increments a row in SQLite and broadcasts the new count to every open `GET /api/honk/stream` SSE connection. The frontend is embedded only with the `embedweb` build tag, so `go test ./...` never needs a frontend build. The game-server plan deletes the honk code; everything else (store + migrations, SSE helpers, static fallback, theme tokens, Docker, CI, Railway) is kept.

**Tech Stack:** Go 1.27.1, `modernc.org/sqlite` v1.60.1 (pure Go, no cgo), SvelteKit 3.0 + Svelte 5.57 + TypeScript 6 + Vite 8, `@sveltejs/adapter-static` 4, pnpm 10.20, Node 22 LTS, Docker (distroless), Railway, GitHub Actions.

**Spec:** `docs/superpowers/specs/2026-10-01-mamdani-chess-design.md` (§2 Server, §4 Frontend, §6 Visual design). Rules are not touched by this plan.

## Global Constraints

- Go module is `mamdani-chess`, `go 1.27.1`. Change `go.mod` only through `go get` / `go mod tidy`. Do not touch `names/`.
- Package layout: `cmd/server` (main), `server` (HTTP), `store` (SQLite), `web/` (SvelteKit app + Go `web` package exposing `web.Assets() fs.FS`). Later plans add `rules`, `game`, `match`.
- Frontend: SvelteKit with `adapter-static`, `fallback: 'index.html'`, `ssr = false`. Custom components only; no chessground.
- **SvelteKit 3 differences** (verified against 3.0.0; older tutorials are wrong): config lives in `vite.config.ts` as `sveltekit({ adapter: … })` — there is no `svelte.config.js`; `$lib` is removed, use `#lib/…` backed by `"imports": { "#lib/*": "./src/lib/*" }` in `package.json`; `tsconfig.json` must `"extends": "$app/tsconfig"`. TypeScript must be 6.x (`svelte-check` 4.7 does not accept 7).
- Node `>=22.17` (SvelteKit 3 engine floor). CI and Docker use Node 22; pnpm is pinned by `"packageManager": "pnpm@10.20.0"`.
- Theme: dark only. Colors are CSS custom properties on `:root` in `web/src/lib/theme/tokens.css`; no color literals in components. Fonts: Barlow Condensed (headings), IBM Plex Sans (body), IBM Plex Mono (numbers).
- SSE responses set `Content-Type: text/event-stream`, `Cache-Control: no-cache`, `X-Accel-Buffering: no`, flush after every event, and send a `: ping` comment every 15 s.
- Env: `PORT` (default `8080`), `DB_PATH` (default `data/mamdani.db`; parent dir is created). On Railway `DB_PATH=/data/mamdani.db` on a volume mounted at `/data`.
- Migrations are append-only: never edit or reorder a shipped entry in `store.migrations`.
- Git: stage explicit paths (`git add <paths>`), never `git add -A`. No Claude attribution in commit messages or PRs.

## Review Focus

1. **SSE through Railway's proxy** — a buffering proxy makes the counter look frozen until the connection closes. Expect each honk to appear in the other tab within a second on the deployed URL. Pinned by the `X-Accel-Buffering` header assertion in Task 3 and the deployed two-tab check in Task 7.
2. **Redeploys wiping data** — without a volume, SQLite lives in the container and resets on every deploy. Expect the count to survive a redeploy. Pinned by `TestHonksPersistAcrossReopen` (Task 1) and the redeploy check in Task 7.
3. **Deep links and refresh** — opening or refreshing `/game/K7F3QZ` must load the app, not a 404, while unknown `/api/*` paths must stay JSON 404s. Pinned by `TestStaticAndFallback` (Task 3).
4. **Shutdown with open streams** — `http.Server.Shutdown` waits for active handlers, and SSE handlers never finish on their own, so a deploy could hang for the full timeout. Expect SIGTERM to exit within a second with a stream open. Pinned by the `BaseContext` cancel in Task 5 and its smoke step.
5. **A stalled browser tab** — one tab that stops reading must not block honks for everyone else. Pinned by `TestBroadcasterNeverBlocksOnSlowSubscriber` (Task 2).

---

### Task 1: SQLite store with migrations

**Files:**
- Modify: `go.mod`, create `go.sum` (via `go get`)
- Create: `.gitignore`
- Create: `store/store.go`, `store/migrate.go`, `store/honk.go`
- Test: `store/store_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces:
  - `store.Open(path string) (*store.Store, error)` — creates the parent dir, opens SQLite in WAL mode with one connection, applies pending migrations.
  - `(*Store).Close() error`, `(*Store).Version(ctx) (int, error)`
  - `(*Store).Honk(ctx) (int64, error)` — increments and returns the new count. `(*Store).Honks(ctx) (int64, error)` — current count. (Throwaway; removed by the game-server plan.)
  - `store.migrations []string` — the append-only migration list later plans extend.

- [ ] **Step 1: Add the SQLite driver and ignore local data**

```bash
go get modernc.org/sqlite@v1.60.1
```

Create `.gitignore` at the repo root:

```gitignore
data/
bin/
```

- [ ] **Step 2: Write the failing tests**

Create `store/store_test.go`:

```go
package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "nested", "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s, path
}

func TestMigrationsAreIdempotent(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	s.Close()

	s, err := Open(path) // second open must not re-run migrations
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s.Close()
	v, err := s.Version(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v != len(migrations) {
		t.Fatalf("version = %d, want %d", v, len(migrations))
	}
	var rows int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_version`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != len(migrations) {
		t.Fatalf("schema_version has %d rows, want %d", rows, len(migrations))
	}
}

func TestHonkIncrements(t *testing.T) {
	ctx := context.Background()
	s, _ := openTemp(t)
	defer s.Close()
	for want := int64(1); want <= 3; want++ {
		got, err := s.Honk(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Honk = %d, want %d", got, want)
		}
	}
}

func TestHonksPersistAcrossReopen(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	s.Honk(ctx)
	s.Honk(ctx)
	s.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	n, err := s.Honks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("Honks after reopen = %d, want 2", n)
	}
}
```

- [ ] **Step 3: Run the tests to see them fail**

Run: `go test ./store`
Expected: FAIL — build errors such as `undefined: Open` and `undefined: migrations`.

- [ ] **Step 4: Write the store**

Create `store/store.go`:

```go
// Package store persists games in SQLite.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Store wraps the SQLite database.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the database at path and applies migrations.
func Open(path string) (*Store, error) {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create db dir: %w", err)
		}
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	// SQLite allows one writer; a single connection avoids SQLITE_BUSY.
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }
```

Create `store/migrate.go`:

```go
package store

import (
	"context"
	"fmt"
)

// migrations run in order; each index is a schema version. Append only:
// never edit or reorder a migration that has shipped.
var migrations = []string{
	// 1: walking-skeleton honk counter (dropped by the game-server plan).
	`CREATE TABLE honks (id INTEGER PRIMARY KEY CHECK (id = 1), count INTEGER NOT NULL);
	 INSERT INTO honks (id, count) VALUES (1, 0);`,
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_version (version INTEGER NOT NULL)`); err != nil {
		return fmt.Errorf("create schema_version: %w", err)
	}
	var version int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&version); err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	for i := version; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			tx.Rollback()
			return fmt.Errorf("record migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", i+1, err)
		}
	}
	return nil
}

// Version returns the applied schema version.
func (s *Store) Version(ctx context.Context) (int, error) {
	var v int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&v)
	return v, err
}
```

Create `store/honk.go`:

```go
package store

import "context"

// Honk adds one to the skeleton counter and returns the new value.
// Throwaway: removed by the game-server plan.
func (s *Store) Honk(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx,
		`UPDATE honks SET count = count + 1 WHERE id = 1 RETURNING count`).Scan(&n)
	return n, err
}

// Honks returns the current counter value.
func (s *Store) Honks(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT count FROM honks WHERE id = 1`).Scan(&n)
	return n, err
}
```

- [ ] **Step 5: Run the tests to see them pass**

Run: `go mod tidy && go vet ./store && go test -race ./store`
Expected: `ok  	mamdani-chess/store`. `go.mod` now lists `modernc.org/sqlite v1.60.1` as a direct requirement.

- [ ] **Step 6: Commit**

```bash
git add .gitignore go.mod go.sum store/
git commit -m "Add SQLite store with append-only migrations"
```

---

### Task 2: SSE helpers and broadcaster

**Files:**
- Create: `server/sse.go`
- Test: `server/sse_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces (package-private, used by Task 3 and the game-server plan):
  - `startSSE(w http.ResponseWriter) (http.Flusher, bool)` — writes stream headers and a 200.
  - `writeEvent(w, fl, event string, v any) error` — `event: <name>\ndata: <json>\n\n`, then flush.
  - `writeHeartbeat(w, fl) error` — `: ping\n\n`, then flush.
  - `newBroadcaster() *broadcaster`; `(*broadcaster).subscribe() (<-chan int64, func())`; `(*broadcaster).publish(int64)`. Each subscriber channel holds only the latest value, so publish never blocks.

- [ ] **Step 1: Write the failing tests**

Create `server/sse_test.go`:

```go
package server

import (
	"testing"
	"time"
)

func TestBroadcasterNeverBlocksOnSlowSubscriber(t *testing.T) {
	b := newBroadcaster()
	stalled, cancelStalled := b.subscribe() // never read until the end
	defer cancelStalled()
	live, cancelLive := b.subscribe()
	defer cancelLive()

	done := make(chan struct{})
	go func() {
		for i := int64(1); i <= 100; i++ {
			b.publish(i)
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked on a subscriber that isn't reading")
	}

	// Both subscribers hold only the latest value.
	if got := <-stalled; got != 100 {
		t.Fatalf("stalled subscriber got %d, want 100", got)
	}
	if got := <-live; got != 100 {
		t.Fatalf("live subscriber got %d, want 100", got)
	}
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	b := newBroadcaster()
	ch, cancel := b.subscribe()
	cancel()
	b.publish(1)
	select {
	case v := <-ch:
		t.Fatalf("got %d after unsubscribe", v)
	default:
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./server`
Expected: FAIL — `undefined: newBroadcaster`.

- [ ] **Step 3: Write the helpers**

Create `server/sse.go`:

```go
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// startSSE sets the event-stream headers and returns the flusher.
func startSSE(w http.ResponseWriter) (http.Flusher, bool) {
	fl, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no") // stop proxies buffering the stream
	w.WriteHeader(http.StatusOK)
	fl.Flush()
	return fl, true
}

// writeEvent writes one SSE event with a JSON payload and flushes it.
func writeEvent(w http.ResponseWriter, fl http.Flusher, event string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// writeHeartbeat writes an SSE comment so idle connections stay open.
func writeHeartbeat(w http.ResponseWriter, fl http.Flusher) error {
	if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// broadcaster fans a value out to subscribers. Each subscriber holds only
// the latest value: a slow reader skips stale counts instead of blocking.
type broadcaster struct {
	mu   sync.Mutex
	subs map[chan int64]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{subs: map[chan int64]struct{}{}}
}

func (b *broadcaster) subscribe() (<-chan int64, func()) {
	ch := make(chan int64, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

func (b *broadcaster) publish(v int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case <-ch: // drop the stale value
		default:
		}
		ch <- v
	}
}
```

- [ ] **Step 4: Run the tests to see them pass**

Run: `go vet ./server && go test -race ./server`
Expected: `ok  	mamdani-chess/server`.

- [ ] **Step 5: Commit**

```bash
git add server/sse.go server/sse_test.go
git commit -m "Add SSE helpers and latest-value broadcaster"
```

---

### Task 3: HTTP routes, honk stream and static fallback

**Files:**
- Create: `server/server.go`, `server/honk.go`, `server/static.go`
- Test: `server/server_test.go`

**Interfaces:**
- Consumes: `store.Store` (`Honk`, `Honks`) from Task 1; `startSSE`, `writeEvent`, `writeHeartbeat`, `broadcaster` from Task 2.
- Produces:
  - `server.New(st *store.Store, assets fs.FS) *server.Server`; `*Server` implements `http.Handler`.
  - Routes: `GET /healthz` → `{"status":"ok"}`; `POST /api/honk` → `{"count":N}`; `GET /api/honk/stream` → SSE `honk` events `{"count":N}`, current value first; any other `/api/*` → 404 `{"error":"not found"}`; everything else → file from `assets`, or `index.html` if no such file.
  - `writeJSON(w, status int, v any)` (package-private, reused by later handlers).
  - Cache headers: `_app/immutable/*` → `public, max-age=31536000, immutable`; all other static responses → `no-cache`.

- [ ] **Step 1: Write the failing tests**

Create `server/server_test.go`:

```go
package server

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"mamdani-chess/store"
)

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	assets := fstest.MapFS{
		"index.html":            {Data: []byte("<!doctype html>app shell")},
		"_app/immutable/app.js": {Data: []byte("console.log(1)")},
		"favicon.svg":           {Data: []byte("<svg/>")},
	}
	s := New(st, assets)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return s, ts
}

// sseReader reads "event:"/"data:" pairs and comments from a stream.
type sseReader struct{ sc *bufio.Scanner }

// next returns the next event name and data, or ("comment", text) for a heartbeat.
func (r sseReader) next(t *testing.T) (string, string) {
	t.Helper()
	var event, data string
	for r.sc.Scan() {
		line := r.sc.Text()
		switch {
		case line == "":
			if event != "" || data != "" {
				return event, data
			}
		case strings.HasPrefix(line, ":"):
			return "comment", strings.TrimSpace(line[1:])
		case strings.HasPrefix(line, "event: "):
			event = line[len("event: "):]
		case strings.HasPrefix(line, "data: "):
			data = line[len("data: "):]
		}
	}
	t.Fatalf("stream ended: %v", r.sc.Err())
	return "", ""
}

func openStream(t *testing.T, url string) sseReader {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatal("missing X-Accel-Buffering: no")
	}
	return sseReader{bufio.NewScanner(resp.Body)}
}

func TestHonkStreamSendsCurrentThenUpdates(t *testing.T) {
	_, ts := newTestServer(t)
	a := openStream(t, ts.URL+"/api/honk/stream")
	b := openStream(t, ts.URL+"/api/honk/stream")
	for _, s := range []sseReader{a, b} {
		if ev, data := s.next(t); ev != "honk" || data != `{"count":0}` {
			t.Fatalf("first event = %q %q", ev, data)
		}
	}

	resp, err := http.Post(ts.URL+"/api/honk", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.TrimSpace(string(body)) != `{"count":1}` {
		t.Fatalf("POST body = %s", body)
	}

	for _, s := range []sseReader{a, b} {
		if ev, data := s.next(t); ev != "honk" || data != `{"count":1}` {
			t.Fatalf("broadcast = %q %q", ev, data)
		}
	}
}

func TestHonkStreamHeartbeat(t *testing.T) {
	s, ts := newTestServer(t)
	s.heartbeat = 10 * time.Millisecond
	r := openStream(t, ts.URL+"/api/honk/stream")
	r.next(t) // current count
	if ev, text := r.next(t); ev != "comment" || text != "ping" {
		t.Fatalf("got %q %q, want heartbeat", ev, text)
	}
}

func TestStaticAndFallback(t *testing.T) {
	_, ts := newTestServer(t)
	cases := []struct {
		path, wantBody, wantCache string
		wantStatus                int
	}{
		{"/", "app shell", "no-cache", 200},
		{"/game/K7F3QZ", "app shell", "no-cache", 200},
		{"/favicon.svg", "<svg/>", "no-cache", 200},
		{"/_app/immutable/app.js", "console.log(1)", "public, max-age=31536000, immutable", 200},
		{"/api/nope", `{"error":"not found"}`, "", 404},
		{"/healthz", `{"status":"ok"}`, "", 200},
	}
	for _, c := range cases {
		resp, err := http.Get(ts.URL + c.path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != c.wantStatus {
			t.Errorf("%s: status %d, want %d", c.path, resp.StatusCode, c.wantStatus)
		}
		if !strings.Contains(string(body), c.wantBody) {
			t.Errorf("%s: body %q, want it to contain %q", c.path, body, c.wantBody)
		}
		if c.wantCache != "" && resp.Header.Get("Cache-Control") != c.wantCache {
			t.Errorf("%s: Cache-Control %q, want %q", c.path, resp.Header.Get("Cache-Control"), c.wantCache)
		}
	}
}
```

- [ ] **Step 2: Run the tests to see them fail**

Run: `go test ./server`
Expected: FAIL — `undefined: New`.

- [ ] **Step 3: Write the server and routes**

Create `server/server.go`:

```go
// Package server is the HTTP layer: routes, SSE and the static frontend.
package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"time"

	"mamdani-chess/store"
)

// Server routes API and frontend requests.
type Server struct {
	store     *store.Store
	assets    fs.FS
	honks     *broadcaster
	heartbeat time.Duration
	mux       *http.ServeMux
}

// New returns a Server backed by st that serves the frontend from assets.
func New(st *store.Store, assets fs.FS) *Server {
	s := &Server{
		store:     st,
		assets:    assets,
		honks:     newBroadcaster(),
		heartbeat: 15 * time.Second,
		mux:       http.NewServeMux(),
	}
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("POST /api/honk", s.honk)
	s.mux.HandleFunc("GET /api/honk/stream", s.honkStream)
	s.mux.HandleFunc("/api/", s.apiNotFound)
	s.mux.HandleFunc("/", s.static)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) apiNotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

Create `server/static.go`:

```go
package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// static serves the built SvelteKit app. Paths that aren't files fall back
// to index.html so client-side routes like /game/K7F3QZ load the app.
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "" {
		name = "index.html"
	}
	if info, err := fs.Stat(s.assets, name); err != nil || info.IsDir() {
		name = "index.html"
	}
	if strings.HasPrefix(name, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFileFS(w, r, s.assets, name)
}
```

Create `server/honk.go`:

```go
package server

import (
	"net/http"
	"time"
)

// The honk counter proves the full loop (POST → SQLite → SSE → every tab).
// Throwaway: removed by the game-server plan.

type honkCount struct {
	Count int64 `json:"count"`
}

func (s *Server) honk(w http.ResponseWriter, r *http.Request) {
	n, err := s.store.Honk(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "honk failed"})
		return
	}
	s.honks.publish(n)
	writeJSON(w, http.StatusOK, honkCount{n})
}

func (s *Server) honkStream(w http.ResponseWriter, r *http.Request) {
	// Subscribe before reading the current value so no honk slips between.
	updates, cancel := s.honks.subscribe()
	defer cancel()
	current, err := s.store.Honks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "read failed"})
		return
	}
	fl, ok := startSSE(w)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if writeEvent(w, fl, "honk", honkCount{current}) != nil {
		return
	}
	last := current
	ticker := time.NewTicker(s.heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case n := <-updates:
			if n <= last { // already sent a newer count
				continue
			}
			last = n
			if writeEvent(w, fl, "honk", honkCount{n}) != nil {
				return
			}
		case <-ticker.C:
			if writeHeartbeat(w, fl) != nil {
				return
			}
		}
	}
}
```

Why `n <= last` in the stream loop: the handler subscribes *before* reading the current count, so a honk landing in between is both read and queued. Skipping counts that aren't newer means a tab never sees the number go backwards.

- [ ] **Step 4: Run the tests to see them pass**

Run: `go vet ./... && go test -race ./...`
Expected: `ok` for `names`, `server` and `store`.

- [ ] **Step 5: Commit**

```bash
git add server/server.go server/honk.go server/static.go server/server_test.go
git commit -m "Add HTTP routes: honk SSE stream, health check, SPA fallback"
```

---

### Task 4: SvelteKit app and the web assets package

**Files:**
- Create: `web/package.json`, `web/pnpm-lock.yaml` (generated), `web/vite.config.ts`, `web/tsconfig.json`, `web/.gitignore`
- Create: `web/src/app.html`, `web/src/app.d.ts`, `web/src/lib/theme/tokens.css`
- Create: `web/src/routes/+layout.ts`, `web/src/routes/+layout.svelte`, `web/src/routes/+page.svelte`, `web/static/favicon.svg`
- Create: `web/assets_dev.go`, `web/assets_embed.go`

**Interfaces:**
- Consumes: `POST /api/honk`, `GET /api/honk/stream` from Task 3.
- Produces:
  - `web.Assets() fs.FS` — with `-tags embedweb`, the build compiled into the binary; without it, `web/build` from disk if built, else a one-page "Frontend not built" notice.
  - `web/src/lib/theme/tokens.css` — the theme tokens every later component uses (import as `#lib/theme/tokens.css`; already imported once in `+layout.svelte`).
  - Vite dev server on `:5173` proxying `/api` to `:8080`.

Write these files by hand; don't run `sv create` (it scaffolds extras we'd delete and may prompt).

- [ ] **Step 1: Write the package and config files**

Create `web/package.json`:

```json
{
	"name": "mamdani-chess-web",
	"private": true,
	"type": "module",
	"imports": {
		"#lib/*": "./src/lib/*"
	},
	"packageManager": "pnpm@10.20.0",
	"engines": {
		"node": ">=22.17"
	},
	"scripts": {
		"dev": "vite dev",
		"build": "vite build",
		"preview": "vite preview",
		"check": "svelte-kit sync && svelte-check --tsconfig ./tsconfig.json"
	},
	"devDependencies": {
		"@sveltejs/adapter-static": "^4.0.0",
		"@sveltejs/kit": "^3.0.0",
		"@sveltejs/vite-plugin-svelte": "^7.3.1",
		"svelte": "^5.57.1",
		"svelte-check": "^4.7.6",
		"typescript": "^6.0.3",
		"vite": "^8.3.2"
	}
}
```

Create `web/vite.config.ts`:

```ts
import adapter from '@sveltejs/adapter-static';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [
		sveltekit({
			// SPA: one index.html; the Go server falls back to it for every route.
			adapter: adapter({ fallback: 'index.html' })
		})
	],
	server: {
		// `pnpm dev` on :5173 talks to `go run ./cmd/server` on :8080.
		proxy: { '/api': 'http://localhost:8080' }
	}
});
```

Create `web/tsconfig.json`:

```json
{
	"extends": "$app/tsconfig",
	"compilerOptions": {
		"strict": true,
		"checkJs": true,
		"skipLibCheck": true,
		"sourceMap": true
	},
	"include": ["src", "test", "*"],
	"exclude": ["src/service-worker"]
}
```

Create `web/.gitignore`:

```gitignore
node_modules/
.svelte-kit/
build/
```

- [ ] **Step 2: Install dependencies**

Run: `pnpm --dir web install`
Expected: installs `@sveltejs/kit 3.0.0`, `svelte 5.57.1`, `typescript 6.0.3`, `vite 8.3.2` and writes `web/pnpm-lock.yaml`. pnpm may note that TypeScript 7 exists; ignore it (svelte-check needs 6).

- [ ] **Step 3: Write the app shell and theme**

Create `web/src/app.html`:

```html
<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		<meta name="theme-color" content="#141518" />
		<link rel="icon" href="%sveltekit.assets%/favicon.svg" />
		<link rel="preconnect" href="https://fonts.googleapis.com" />
		<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin="anonymous" />
		<link
			rel="stylesheet"
			href="https://fonts.googleapis.com/css2?family=Barlow+Condensed:wght@700;800&family=IBM+Plex+Mono:wght@500&family=IBM+Plex+Sans:wght@400;600&display=swap"
		/>
		%sveltekit.head%
	</head>
	<body data-sveltekit-preload-data="hover">
		<div style="display: contents">%sveltekit.body%</div>
	</body>
</html>
```

Create `web/src/app.d.ts`:

```ts
// See https://svelte.dev/docs/kit/types#app.d.ts
declare global {
	namespace App {}
}

export {};
```

Create `web/src/lib/theme/tokens.css` (values from spec §6; `--accent-text` is the dark text the spec puts on yellow fills):

```css
/* Road-works theme tokens (spec §6). Dark only for now; components use
   these variables, never color literals, so a light theme can be added. */
:root {
	--bg: #141518;
	--surface: #1e2024;
	--surface-2: #2a2d32;
	--line: #3a3e44;
	--text: #f1eee6;
	--text-body: #c9c5bb;
	--text-muted: #a8a49b;
	--accent: #f2c230;
	--accent-text: #141518;
	--hazard: #ff7a3d;
	--board-light: #d9d3c4;
	--board-dark: #857e72;
	--board-last-light: #e9d98b;
	--board-last-dark: #b5a24f;
	--target-ring: #f2c230;

	--font-display: 'Barlow Condensed', 'Arial Narrow', sans-serif;
	--font-body: 'IBM Plex Sans', system-ui, sans-serif;
	--font-mono: 'IBM Plex Mono', ui-monospace, monospace;

	color-scheme: dark;
}

*,
*::before,
*::after {
	box-sizing: border-box;
}

body {
	margin: 0;
	background: var(--bg);
	color: var(--text-body);
	font-family: var(--font-body);
	-webkit-font-smoothing: antialiased;
}
```

Create `web/src/routes/+layout.ts`:

```ts
// Static SPA: every page renders in the browser.
export const ssr = false;
export const prerender = false;
```

Create `web/src/routes/+layout.svelte`:

```svelte
<script lang="ts">
	import '#lib/theme/tokens.css';

	let { children } = $props();
</script>

{@render children()}
```

Create `web/static/favicon.svg` (the traffic-cone mark):

```svg
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><path d="M16 3 26 27H6Z" fill="#ff7a3d"/><path d="M10.5 17h11l1.5 4h-14Z" fill="#f1eee6"/><rect x="4" y="27" width="24" height="3" rx="1" fill="#ff7a3d"/></svg>
```

- [ ] **Step 4: Write the honk page**

Create `web/src/routes/+page.svelte`:

```svelte
<script lang="ts">
	// Walking-skeleton demo: one shared counter over SSE.
	// Throwaway: replaced by the home page in a later plan.
	import { onMount } from 'svelte';

	let count = $state<number | null>(null);
	let connected = $state(false);

	onMount(() => {
		const source = new EventSource('/api/honk/stream');
		source.onopen = () => (connected = true);
		source.onerror = () => (connected = false);
		source.addEventListener('honk', (e) => {
			count = JSON.parse((e as MessageEvent<string>).data).count;
		});
		return () => source.close();
	});

	async function honk() {
		await fetch('/api/honk', { method: 'POST' });
	}
</script>

<svelte:head>
	<title>Pothole Chess</title>
</svelte:head>

<main>
	<h1>Pothole Chess</h1>
	<p class="count" aria-live="polite">{count ?? '–'}</p>
	<button onclick={honk} disabled={!connected}>Honk</button>
	<p class="status">{connected ? 'Live' : 'Reconnecting…'}</p>
</main>

<style>
	main {
		min-height: 100dvh;
		display: grid;
		place-content: center;
		justify-items: center;
		gap: 16px;
		padding: 16px;
	}
	h1 {
		margin: 0;
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 48px;
		text-transform: uppercase;
		color: var(--text);
	}
	.count {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 72px;
		color: var(--accent);
	}
	button {
		min-height: 44px;
		padding: 0 32px;
		border: 0;
		border-radius: 4px;
		background: var(--accent);
		color: var(--accent-text);
		font-family: var(--font-display);
		font-weight: 800;
		font-size: 24px;
		text-transform: uppercase;
		cursor: pointer;
	}
	button:disabled {
		opacity: 0.5;
		cursor: default;
	}
	.status {
		margin: 0;
		color: var(--text-muted);
		font-size: 14px;
	}
</style>
```

- [ ] **Step 5: Type-check and build**

Run: `pnpm --dir web check && pnpm --dir web build`
Expected: `svelte-check` reports `0 ERRORS 0 WARNINGS`; the build ends with `Wrote site to "build"` and `web/build/` contains `index.html`, `favicon.svg` and `_app/`.

If the build fails with `module_removed_lib` or `tsconfig_extends_missing`, a file was written from a SvelteKit 2 template; recheck it against the files above.

- [ ] **Step 6: Write the Go assets package**

Create `web/assets_dev.go`:

```go
//go:build !embedweb

package web

import (
	"io/fs"
	"os"
	"testing/fstest"
)

// Assets serves web/build from disk when it exists, so `go run ./cmd/server`
// works after `pnpm build` without the embedweb tag. Without a build it
// serves a page saying so.
func Assets() fs.FS {
	if info, err := os.Stat("web/build/index.html"); err == nil && !info.IsDir() {
		return os.DirFS("web/build")
	}
	return fstest.MapFS{"index.html": {Data: []byte(
		"<!doctype html><title>Not built</title><p>Frontend not built. " +
			"Run <code>pnpm --dir web build</code>, or use the Vite dev server on :5173.</p>")}}
}
```

Create `web/assets_embed.go`:

```go
//go:build embedweb

package web

import (
	"embed"
	"io/fs"
)

//go:embed all:build
var build embed.FS

// Assets returns the SvelteKit build compiled into the binary.
func Assets() fs.FS {
	sub, err := fs.Sub(build, "build")
	if err != nil {
		panic(err)
	}
	return sub
}
```

- [ ] **Step 7: Check both build modes compile**

Run: `go vet ./... && go vet -tags embedweb ./... && go list ./...`
Expected: no output from vet; `go list` shows exactly `mamdani-chess/names`, `mamdani-chess/server`, `mamdani-chess/store` and `mamdani-chess/web` — nothing under `web/node_modules`. (`-tags embedweb` needs `web/build` from Step 5.)

- [ ] **Step 8: Commit**

```bash
git add web/package.json web/pnpm-lock.yaml web/vite.config.ts web/tsconfig.json web/.gitignore \
  web/src web/static web/assets_dev.go web/assets_embed.go
git commit -m "Add SvelteKit app with road-works theme tokens and honk page"
```

---

### Task 5: Server entry point and local end-to-end check

**Files:**
- Create: `cmd/server/main.go`

**Interfaces:**
- Consumes: `store.Open` (Task 1), `server.New` (Task 3), `web.Assets` (Task 4).
- Produces: the `server` binary. Reads `PORT` and `DB_PATH`; exits 0 on SIGINT/SIGTERM after closing open streams.

- [ ] **Step 1: Write main**

Create `cmd/server/main.go`:

```go
// Command server runs Pothole Chess: Mamdani Edition.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mamdani-chess/server"
	"mamdani-chess/store"
	"mamdani-chess/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data/mamdani.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// Cancelling base ends open SSE streams so shutdown doesn't hang on them.
	base, stopStreams := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(st, web.Assets()),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return base },
	}

	sig, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "db", dbPath)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		stopStreams()
		return err
	case <-sig.Done():
	}
	slog.Info("shutting down")
	stopStreams()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

`BaseContext` is what makes shutdown fast: every request context derives from `base`, so `stopStreams()` ends each SSE loop and `Shutdown` has nothing left to wait for.

- [ ] **Step 2: Build with the embedded frontend**

Run: `pnpm --dir web build && go build -tags embedweb -o bin/server ./cmd/server`
Expected: no errors; `bin/server` exists. (`bin/` is git-ignored.)

- [ ] **Step 3: Scripted smoke test, including shutdown with a stream open**

Run:

```bash
PORT=18080 DB_PATH=/tmp/mamdani-smoke/m.db ./bin/server & PID=$!; sleep 1
curl -sN --max-time 2 localhost:18080/api/honk/stream > /tmp/mamdani-stream.txt & sleep 0.3
curl -s -XPOST localhost:18080/api/honk; echo
curl -s -XPOST localhost:18080/api/honk; echo
sleep 2; cat /tmp/mamdani-stream.txt
curl -s localhost:18080/game/K7F3QZ | head -c 15; echo
curl -s -o /dev/null -w "%{http_code} %{content_type}\n" localhost:18080/api/nope
curl -sN localhost:18080/api/honk/stream > /dev/null & sleep 0.3
kill -TERM $PID; time wait $PID; echo "exit=$?"
```

Expected:
- the two POSTs print `{"count":1}` and `{"count":2}`;
- the stream file shows three `event: honk` blocks with counts 0, 1, 2;
- `/game/K7F3QZ` prints `<!doctype html>`;
- `/api/nope` prints `404 application/json`;
- the kill returns in well under a second, `exit=0`, and the log shows `shutting down`.

- [ ] **Step 4: Manual two-tab check in dev mode**

Terminal 1: `go run ./cmd/server`. Terminal 2: `pnpm --dir web dev`.
Open `http://localhost:5173` in two tabs. Expected: both show the same count and "Live"; clicking HONK in one tab updates the other within a second. Stop the Go server: both tabs show "Reconnecting…" and the button disables; start it again: they reconnect and show the persisted count.

- [ ] **Step 5: Commit**

```bash
git add cmd/server/main.go
git commit -m "Add server entry point with graceful shutdown"
```

---

### Task 6: Docker image, Railway config and CI

**Files:**
- Create: `Dockerfile`, `.dockerignore`, `railway.json`, `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: `web/` build (Task 4), `cmd/server` (Task 5).
- Produces: a container that listens on `$PORT` (Railway sets it) and stores data at `/data/mamdani.db`; a CI workflow with `go` and `web` jobs.

- [ ] **Step 1: Write the Dockerfile**

Create `Dockerfile`:

```dockerfile
# syntax=docker/dockerfile:1

# 1. Build the SvelteKit app.
FROM node:22-alpine AS web
RUN corepack enable
WORKDIR /src/web
COPY web/package.json web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

# 2. Build the Go server with the frontend embedded.
FROM golang:1.27-alpine AS server
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=web /src/web/build ./web/build
RUN CGO_ENABLED=0 go build -tags embedweb -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# 3. Run it. Root image: Railway volumes mount as root, so a nonroot
#    user could not write the SQLite file on /data.
FROM gcr.io/distroless/static-debian12
COPY --from=server /out/server /server
ENV PORT=8080 DB_PATH=/data/mamdani.db
EXPOSE 8080
ENTRYPOINT ["/server"]
```

Create `.dockerignore`:

```gitignore
.git
**/node_modules
web/.svelte-kit
web/build
data
docs
```

- [ ] **Step 2: Build and run the image locally**

Requires Docker running (start Docker Desktop first).

Run:

```bash
docker build -t mamdani-chess .
docker run --rm -d --name mamdani -p 18080:8080 -v mamdani-data:/data mamdani-chess
sleep 1; curl -s localhost:18080/healthz; echo; curl -s -XPOST localhost:18080/api/honk; echo
docker restart mamdani && sleep 1 && curl -s -XPOST localhost:18080/api/honk; echo
docker stop mamdani
```

Expected: `{"status":"ok"}`, then `{"count":1}`, then after the restart `{"count":2}` (the named volume kept the database). `docker stop` returns within a second or two.

- [ ] **Step 3: Write the Railway config**

Create `railway.json`:

```json
{
	"$schema": "https://railway.com/railway.schema.json",
	"build": {
		"builder": "DOCKERFILE",
		"dockerfilePath": "Dockerfile"
	},
	"deploy": {
		"healthcheckPath": "/healthz",
		"healthcheckTimeout": 30,
		"restartPolicyType": "ON_FAILURE",
		"restartPolicyMaxRetries": 5
	}
}
```

- [ ] **Step 4: Write the CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:

jobs:
  go:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - uses: actions/setup-go@v7
        with:
          go-version-file: go.mod
      - run: go vet ./...
      - run: go test -race ./...

  web:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v7
      - uses: pnpm/action-setup@v6
        with:
          package_json_file: web/package.json
      - uses: actions/setup-node@v7
        with:
          node-version: 22
          cache: pnpm
          cache-dependency-path: web/pnpm-lock.yaml
      - run: pnpm install --frozen-lockfile
      - run: pnpm check
      - run: pnpm build
```

`pnpm/action-setup` reads the pnpm version from `web/package.json`'s `packageManager` field, so CI and local use the same pnpm.

- [ ] **Step 5: Re-run everything CI runs**

Run: `go vet ./... && go test -race ./... && (cd web && pnpm install --frozen-lockfile && pnpm check && pnpm build)`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add Dockerfile .dockerignore railway.json .github/workflows/ci.yml
git commit -m "Add Dockerfile, Railway config and CI workflow"
```

---

### Task 7: Deploy to Railway

Steps marked **(you)** need the project owner: they log in or create paid resources. Run them in the Claude Code prompt with a leading `!` (e.g. `! railway login`) so the output lands in the session.

**Files:** none.

**Interfaces:**
- Consumes: `railway.json`, `Dockerfile` (Task 6).
- Produces: a public Railway URL serving the app, with SQLite on a volume.

- [ ] **Step 1 (you): Log in and create the project**

```bash
railway login
railway init --name mamdani-chess
```

Expected: `railway status` shows project `mamdani-chess`.

- [ ] **Step 2 (you): First deploy, which creates the service**

```bash
railway up --detach
```

Expected: the build log shows the three Docker stages; the deploy goes healthy on `/healthz`. If the CLI asks which service to link, pick the one this deploy created (`railway service` links it later).

- [ ] **Step 3 (you): Attach a volume and set the database path**

```bash
railway volume add --mount-path /data
railway variable set DB_PATH=/data/mamdani.db
railway domain
```

Expected: `railway volume list` shows a volume mounted at `/data`; `railway domain` prints a `https://….up.railway.app` URL. Setting the variable triggers a redeploy.

- [ ] **Step 4: Deployed checks**

With `URL` set to the printed domain:

```bash
curl -s $URL/healthz; echo
curl -s -XPOST $URL/api/honk; echo
curl -sN --max-time 3 $URL/api/honk/stream & sleep 1; curl -s -XPOST $URL/api/honk > /dev/null; wait
```

Expected: `{"status":"ok"}`; a count; and the stream prints the second `event: honk` **before** the 3 s timeout ends it (if both events arrive only at the end, the proxy is buffering — see Review Focus 1).

Then open `$URL` in two browsers (one on a phone): HONK in one updates the other within a second. Refresh `$URL/game/TEST01`: the honk page loads, not a 404.

- [ ] **Step 5: Redeploy persistence check**

Note the current count, run `railway redeploy --yes` (or `railway up --detach`), wait for healthy, reload `$URL`. Expected: the same count — the volume kept the database.

- [ ] **Step 6: Record the URL**

Add the deployed URL under "Sources of truth" in `CLAUDE.md`:

```markdown
| Live app | https://<your-domain>.up.railway.app (Railway project `mamdani-chess`, SQLite on volume `/data`) |
```

```bash
git add CLAUDE.md
git commit -m "Record Railway deploy URL"
```

---

## Done when

- `go vet ./... && go test -race ./...` and `pnpm check && pnpm build` pass locally and in CI.
- The Railway URL serves the honk page; two tabs sync live; the count survives a redeploy.
- The next plan (rules engine) can start on top of this layout without moving any file.
