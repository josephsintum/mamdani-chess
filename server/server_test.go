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

func TestCloseEndsStreamsButNotRequests(t *testing.T) {
	s, ts := newTestServer(t)
	r := openStream(t, ts.URL+"/api/honk/stream")
	r.next(t) // current count

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

	resp, err := http.Post(ts.URL+"/api/honk", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || strings.TrimSpace(string(body)) != `{"count":1}` {
		t.Fatalf("POST after Close = %d %s", resp.StatusCode, body)
	}
}
