package server

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Each stream gets its own retry (see startSSE).
func TestStreamsStartWithARandomRetry(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	code := alice.create()
	seen := map[int]bool{}
	for range 5 {
		resp, err := alice.c.Get(ts.URL + "/api/games/" + code + "/stream")
		if err != nil {
			t.Fatal(err)
		}
		line, _ := bufio.NewReader(resp.Body).ReadString('\n')
		resp.Body.Close()
		ms, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "retry: ")))
		if err != nil || ms < 1000 || ms > 3000 {
			t.Fatalf("first line %q, want retry: 1000 to 3000", line)
		}
		seen[ms] = true
	}
	if len(seen) < 2 {
		t.Error("every stream got the same retry")
	}
}

// The home page, a game, an unknown path, the app's files and the API's
// 404 are in contract's TestAppAndPreviews, TestUnknownAPIPath and
// TestHealthz.
func TestStaticAndFallback(t *testing.T) {
	_, ts := newTestServer(t)
	cases := []struct {
		path, wantBody, wantCache string
		wantStatus                int
	}{
		{"/rules", "rules page", "private, no-cache", 200},
		{"/index", "app shell", "private, no-cache", 404},
		{"/google1234.html", "verification", "no-cache", 200},
		{"/play", "app shell", "private, no-cache", 200},
		{"/game/K7F3QZ/nope", "app shell", "private, no-cache", 404},
		{"/rules.html", "app shell", "private, no-cache", 404},
		{"/200", "app shell", "private, no-cache", 404},
		{"/favicon.svg", "<svg/>", "no-cache", 200},
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
		if resp.Header.Get("Cache-Control") != c.wantCache {
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

	r.waitEnd(time.Second, "Close")
	alice.create() // other requests still work
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
	a.waitEnd(time.Second, "the game crashed")
	if status, body := alice.post(move, `{"from":"e2","to":"e4","seq":0}`); status != http.StatusNotFound {
		t.Errorf("after the crash: %d %s, want 404", status, body)
	}
}

// Accept-Encoding is read case-blind, and a q=0 refuses an encoding. (Each
// encoding by itself is in contract's TestAppAndPreviews.)
func TestPrecompressedFiles(t *testing.T) {
	_, ts := newTestServer(t)
	for _, c := range []struct{ accept, wantEncoding, wantBody string }{
		{"GZIP", "gzip", string(svgGzip)},
		{"br;q=0, gzip", "gzip", string(svgGzip)},
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

// Files outside _app/immutable are revalidated on every visit, so they carry
// an ETag and a match costs a 304 instead of the file. Each encoding is its
// own representation with its own tag.
func TestStaticFilesRevalidate(t *testing.T) {
	_, ts := newTestServer(t)
	tags := map[string]bool{}
	for _, accept := range []string{"br", "gzip", ""} {
		resp := get(t, ts.URL+"/favicon.svg", accept)
		tag := resp.Header.Get("ETag")
		if !strings.HasPrefix(tag, `"`) || tags[tag] {
			t.Fatalf("Accept-Encoding %q: ETag %q, want a new quoted tag", accept, tag)
		}
		tags[tag] = true
		again := get(t, ts.URL+"/favicon.svg", accept, "If-None-Match", tag)
		body, _ := io.ReadAll(again.Body)
		if again.StatusCode != http.StatusNotModified || len(body) != 0 {
			t.Errorf("Accept-Encoding %q, If-None-Match %s: %d %q, want 304", accept, tag, again.StatusCode, body)
		}
	}
	if tag := get(t, ts.URL+"/_app/immutable/app.js", "").Header.Get("ETag"); tag != "" {
		t.Errorf("an immutable file has ETag %q; it never revalidates", tag)
	}
}

// Every stream in a role writes the same encoded view, so writing one
// must not copy or allocate.
func TestWriteEvent(t *testing.T) {
	rec := httptest.NewRecorder()
	rec.Body = bytes.NewBuffer(make([]byte, 0, 64<<10))
	w := &statusRecorder{ResponseWriter: rec, status: http.StatusOK}
	if err := writeEvent(w, w, "state", []byte(`{"seq":1}`)); err != nil {
		t.Fatal(err)
	}
	if got, want := rec.Body.String(), "event: state\ndata: {\"seq\":1}\n\n"; got != want {
		t.Errorf("wrote %q, want %q", got, want)
	}
	data := []byte(`{"seq":2}`)
	if n := testing.AllocsPerRun(100, func() { writeEvent(w, w, "state", data) }); n != 0 {
		t.Errorf("writing an event: %v allocations, want 0", n)
	}
}
