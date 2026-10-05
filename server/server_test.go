package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// odd always rolls 1, so no potholes open and moves are plain chess.
type odd struct{}

func (odd) D8() int { return 1 }

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	return newTestServerWith(t, odd{})
}

func newTestServerWith(t *testing.T, dice rules.Dice) (*Server, *httptest.Server) {
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
		"favicon.svg.br":        {Data: []byte("brotli bytes")},
		"favicon.svg.gz":        {Data: svgGzip},
	}
	s := New(st, game.NewHub(dice, st), assets)
	s.log = slog.New(slog.DiscardHandler) // tests that check logging swap in their own
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return s, ts
}

// player is one browser: its own cookie jar, so its own guest ID.
type player struct {
	t   *testing.T
	c   *http.Client
	url string
}

func newPlayer(t *testing.T, ts *httptest.Server) *player {
	jar, _ := cookiejar.New(nil)
	return &player{t: t, c: &http.Client{Jar: jar}, url: ts.URL}
}

func (p *player) post(path, body string) (int, string) {
	p.t.Helper()
	resp, err := p.c.Post(p.url+path, "application/json", strings.NewReader(body))
	if err != nil {
		p.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, strings.TrimSpace(string(b))
}

func (p *player) create() string {
	p.t.Helper()
	status, body := p.post("/api/games", "")
	var out struct{ Code string }
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &out) != nil || len(out.Code) != 6 {
		p.t.Fatalf("create: %d %s", status, body)
	}
	return out.Code
}

// sseReader reads "event:"/"data:" pairs and comments from a stream.
type sseReader struct {
	t  *testing.T
	sc *bufio.Scanner
}

// next returns the next event name and data, or ("comment", text) for a heartbeat.
func (r sseReader) next() (string, string) {
	r.t.Helper()
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
	r.t.Fatalf("stream ended: %v", r.sc.Err())
	return "", ""
}

// state reads the next "state" event as a View.
func (r sseReader) state() game.View {
	r.t.Helper()
	ev, data := r.next()
	var v game.View
	if ev != "state" || json.Unmarshal([]byte(data), &v) != nil {
		r.t.Fatalf("got %q %q, want a state event", ev, data)
	}
	return v
}

func (p *player) stream(code string) sseReader {
	p.t.Helper()
	resp, err := p.c.Get(p.url + "/api/games/" + code + "/stream")
	if err != nil {
		p.t.Fatal(err)
	}
	p.t.Cleanup(func() { resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		p.t.Fatalf("Content-Type = %q", ct)
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" {
		p.t.Fatal("missing X-Accel-Buffering: no")
	}
	return sseReader{t: p.t, sc: bufio.NewScanner(resp.Body)}
}

func TestFriendGameOverHTTP(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)

	code := alice.create()
	a := alice.stream(code)
	if v := a.state(); v.You != "white" || v.Status != game.Waiting {
		t.Fatalf("alice: you=%s status=%s", v.You, v.Status)
	}
	b := bob.stream(code)
	if v := b.state(); v.You != "black" || v.Status != game.Playing {
		t.Fatalf("bob: you=%s status=%s", v.You, v.Status)
	}
	if v := a.state(); v.Status != game.Playing || len(v.Legal) != 33 {
		t.Fatalf("alice after bob joins: status=%s legal=%d", v.Status, len(v.Legal))
	}
	if v := carol.stream(code).state(); v.You != "spectator" {
		t.Fatalf("carol: you=%s", v.You)
	}

	if status, body := alice.post("/api/games/"+code+"/move", `{"from":"e2","to":"e4","seq":0}`); status != http.StatusNoContent {
		t.Fatalf("alice e2e4: %d %s", status, body)
	}
	if v := b.state(); v.Seq != 1 || v.Board[28] != "wP" || len(v.Legal) != 32 || v.Log[0] != (game.LogEntry{SAN: "e4", Color: "white", Dice: "d8 1"}) {
		t.Fatalf("bob after e4: seq=%d e4=%q legal=%d log=%q", v.Seq, v.Board[28], len(v.Legal), v.Log)
	}
	if v := a.state(); v.Seq != 1 || len(v.Legal) != 0 {
		t.Fatalf("alice after e4: seq=%d legal=%d", v.Seq, len(v.Legal))
	}
}

func TestMoveErrors(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	bob.stream(code).state()
	move := "/api/games/" + code + "/move"
	cases := []struct {
		who        *player
		path, body string
		want       int
	}{
		{carol, move, `{"from":"e2","to":"e4","seq":0}`, http.StatusForbidden},
		{bob, move, `{"from":"e7","to":"e5","seq":0}`, http.StatusConflict},   // not your turn
		{alice, move, `{"from":"e2","to":"e4","seq":5}`, http.StatusConflict}, // stale
		{alice, move, `{"from":"e2","to":"e5","seq":0}`, http.StatusConflict}, // illegal
		{alice, move, `{"from":"e9","to":"e4","seq":0}`, http.StatusBadRequest},
		{alice, move, `{"from":"e2","to":"e4"}`, http.StatusBadRequest}, // no seq
		{alice, move, `not json`, http.StatusBadRequest},
		{alice, move, `{"from":"` + strings.Repeat("x", 5000) + `"}`, http.StatusBadRequest},
		{alice, "/api/games/NOPE99/move", `{"from":"e2","to":"e4","seq":0}`, http.StatusNotFound},
	}
	for _, c := range cases {
		if status, body := c.who.post(c.path, c.body); status != c.want {
			t.Errorf("%s %.40s: %d %s, want %d", c.path, c.body, status, body, c.want)
		}
	}
	resp, err := http.Get(ts.URL + "/api/games/NOPE99/stream")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown game stream: %d, want 404", resp.StatusCode)
	}
}

func TestGuestCookie(t *testing.T) {
	_, ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/games", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	var c *http.Cookie
	for _, k := range resp.Cookies() {
		if k.Name == "guest" {
			c = k
		}
	}
	if c == nil || len(c.Value) != 32 || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" {
		t.Fatalf("guest cookie %+v", c)
	}
}

func TestGameStreamHeartbeat(t *testing.T) {
	s, ts := newTestServer(t)
	s.heartbeat = 10 * time.Millisecond
	alice := newPlayer(t, ts)
	r := alice.stream(alice.create())
	r.state()
	if ev, text := r.next(); ev != "comment" || text != "ping" {
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
		{"/_app/immutable/gone.js", "404 page not found", "no-cache", 404},
		{"/api/nope", `{"error":"not found"}`, "", 404},
		{"/healthz", `"status":"ok"`, "", 200},
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

func TestCloseEndsStreamsButNotRequests(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	r := alice.stream(alice.create())
	r.state()

	s.Close()
	s.Close() // idempotent

	ended := make(chan struct{})
	go func() {
		for r.sc.Scan() {
		}
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(time.Second):
		t.Fatal("stream still open 1s after Close")
	}
	alice.create() // other requests still work
}

func TestResignOverHTTP(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	b := bob.stream(code)
	b.state()
	resign := "/api/games/" + code + "/resign"
	if status, body := carol.post(resign, ""); status != http.StatusForbidden {
		t.Errorf("spectator resigns: %d %s", status, body)
	}
	if status, body := alice.post(resign, ""); status != http.StatusNoContent {
		t.Fatalf("alice resigns: %d %s", status, body)
	}
	if v := b.state(); v.Result == nil || v.Result.Winner != "black" || v.Result.Reason != game.Resignation {
		t.Fatalf("bob sees result %+v", v.Result)
	}
	if status, _ := bob.post(resign, ""); status != http.StatusConflict {
		t.Errorf("resign after the game ended: %d, want 409", status)
	}
	if status, _ := alice.post("/api/games/NOPE99/resign", ""); status != http.StatusNotFound {
		t.Errorf("unknown game: %d, want 404", status)
	}
}

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

func TestRequestsAreLogged(t *testing.T) {
	s, ts := newTestServer(t)
	var buf bytes.Buffer
	s.log = slog.New(slog.NewTextHandler(&buf, nil))
	get(t, ts.URL+"/api/nope", "")
	if line := buf.String(); !strings.Contains(line, "method=GET path=/api/nope status=404") {
		t.Errorf("log %q", line)
	}
}
