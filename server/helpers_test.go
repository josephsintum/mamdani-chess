package server

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
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
		"200.html":              {Data: []byte("<!doctype html><html><head><meta charset=\"utf-8\"></head><body>app shell</body></html>")},
		"index.html":            {Data: []byte("<!doctype html><html><head><meta charset=\"utf-8\"><title>Mamdani Chess</title></head><body>home page</body></html>")},
		"rules.html":            {Data: []byte("<!doctype html><html><head><meta charset=\"utf-8\"><title>Rules · Mamdani Chess</title></head><body>rules page</body></html>")},
		"_app/immutable/app.js": {Data: []byte("console.log(1)")},
		"favicon.svg":           {Data: []byte("<svg/>")},
		"google1234.html":       {Data: []byte("google-site-verification: verification")},
		"favicon.svg.br":        {Data: []byte("brotli bytes")},
		"favicon.svg.gz":        {Data: svgGzip},
	}
	s := New(st, game.NewHub(dice, st), assets)
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)
	return s, ts
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

// get fetches url with the given Accept-Encoding ("" for none) and any
// further header name, value pairs.
func get(t *testing.T, url, acceptEncoding string, header ...string) *http.Response {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	// A bare Transport would add gzip itself and hide the header we sent.
	resp, err := (&http.Transport{DisableCompression: true}).RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// player is one browser: its own cookie jar, so its own guest ID.
type player struct {
	t   *testing.T
	c   *http.Client
	url string
}

func newPlayer(t *testing.T, ts *httptest.Server) *player {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	return &player{t: t, c: &http.Client{Jar: jar}, url: ts.URL}
}

// playtestHeader adds the header the playtest script sends to every request.
type playtestHeader struct{}

func (playtestHeader) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-Playtest", "1")
	return http.DefaultTransport.RoundTrip(r)
}

// newPlaytester is a browser the playtest script drives.
func newPlaytester(t *testing.T, ts *httptest.Server) *player {
	t.Helper()
	p := newPlayer(t, ts)
	p.c.Transport = playtestHeader{}
	return p
}

func (p *player) get(path string) (int, string) {
	p.t.Helper()
	resp, err := p.c.Get(p.url + path)
	if err != nil {
		p.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
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

// name is the caller's name from GET /api/me; "" for null.
func (p *player) name() string {
	p.t.Helper()
	status, body := p.get("/api/me")
	var out struct{ Name *string }
	if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil {
		p.t.Fatalf("GET /api/me: %d %s", status, body)
	}
	if out.Name == nil {
		return ""
	}
	return *out.Name
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

// open opens the stream at path, closed when the test ends.
func (p *player) open(path string) sseReader {
	p.t.Helper()
	resp, err := p.c.Get(p.url + path)
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

func (p *player) stream(code string) sseReader {
	p.t.Helper()
	return p.open("/api/games/" + code + "/stream")
}

// queue opens the quick-match stream and reads its "queued" event.
func (p *player) queue() (sseReader, int) {
	p.t.Helper()
	r := p.open("/api/match")
	ev, data := r.next()
	var q struct{ Looking int }
	if ev != "queued" || json.Unmarshal([]byte(data), &q) != nil {
		p.t.Fatalf("got %q %q, want queued", ev, data)
	}
	return r, q.Looking
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

// waitEnd reads to the end of the stream, failing if the server hasn't
// ended it within d (after why, e.g. "Close").
func (r sseReader) waitEnd(d time.Duration, why string) {
	r.t.Helper()
	ended := make(chan struct{})
	go func() {
		for r.sc.Scan() {
		}
		close(ended)
	}()
	select {
	case <-ended:
	case <-time.After(d):
		r.t.Fatalf("stream still open %v after %s", d, why)
	}
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

func matchedCode(r sseReader) string {
	r.t.Helper()
	ev, data := r.next()
	var m struct{ Code string }
	if ev != "matched" || json.Unmarshal([]byte(data), &m) != nil || len(m.Code) != 6 {
		r.t.Fatalf("got %q %q, want matched", ev, data)
	}
	return m.Code
}

// matchPair puts a and b through quick match and returns their game's code
// once both streams have ended (each search is logged as its stream ends).
func matchPair(t *testing.T, a, b *player) string {
	t.Helper()
	as, _ := a.queue()
	bs, _ := b.queue()
	code := matchedCode(as)
	if got := matchedCode(bs); got != code {
		t.Fatalf("matched to %s and %s", code, got)
	}
	as.waitEnd(2*time.Second, "matched")
	bs.waitEnd(2*time.Second, "matched")
	return code
}

// rematchOf has both players ask for a rematch of a finished game and
// returns the new game's code, read from white's stream.
func rematchOf(t *testing.T, code string, white, black *player) string {
	t.Helper()
	w := white.stream(code)
	w.state()
	rematch := "/api/games/" + code + "/rematch"
	for _, p := range []*player{white, black} {
		if status, body := p.post(rematch, `{}`); status != http.StatusNoContent {
			t.Fatalf("rematch: %d %s", status, body)
		}
	}
	for {
		if next := w.state().Rematch.Code; next != "" {
			return next
		}
	}
}

// counted sums every counter so far, by key.
func counted(t *testing.T, s *Server) map[string]int {
	t.Helper()
	days, err := s.store.Counts(t.Context(), "", time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	sum := map[string]int{}
	for _, d := range days {
		for k, n := range d.N {
			sum[k] += n
		}
	}
	return sum
}

// kindOf reads a saved game's kind and playtest mark.
func kindOf(t *testing.T, s *Server, code string) (string, bool) {
	t.Helper()
	kind, playtest, err := s.store.GameKind(t.Context(), code)
	if err != nil {
		t.Fatalf("kind of %s: %v", code, err)
	}
	return kind, playtest
}

func hasStats(t *testing.T, s *Server, code string) bool {
	t.Helper()
	has, err := s.store.HasGameStats(t.Context(), code)
	if err != nil {
		t.Fatal(err)
	}
	return has
}
