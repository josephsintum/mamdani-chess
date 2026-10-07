package contract

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"mamdani-chess/game"
)

// check is one contract check: its deadline, and the streams it opened,
// which close when it ends.
type check struct {
	t   *testing.T
	ctx context.Context
}

// newCheck starts a check that must finish within timeout (bun test's
// limit for the same check), or skips it when there is no server to test.
func newCheck(t *testing.T, timeout time.Duration) *check {
	t.Helper()
	if base == "" {
		t.Skip(skipReason)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel) // also ends every stream the check opened
	return &check{t: t, ctx: ctx}
}

// players returns n browsers, each with its own cookie, so its own guest.
func (c *check) players(n int) []*player {
	ps := make([]*player, n)
	for i := range ps {
		ps[i] = &player{c: c}
	}
	return ps
}

// do sends a request with no cookie and returns the response with its body
// read: a plain fetch, as a link preview bot or curl makes.
func (c *check) do(method, path string, header http.Header, body string) (*http.Response, string) {
	c.t.Helper()
	resp, err := send(c.ctx, method, path, header, body)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp, string(b)
}

// send sends one request to the server under test. A body is sent as JSON.
func send(ctx context.Context, method, path string, header http.Header, body string) (*http.Response, error) {
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, r)
	if err != nil {
		return nil, err
	}
	for k, vs := range header {
		req.Header[k] = vs
	}
	return http.DefaultClient.Do(req)
}

// player is one browser: its own cookie, so its own guest. It keeps the
// cookie by hand, as a browser would send it back, rather than through a
// jar, so the checks see exactly what the server set.
type player struct {
	c      *check
	cookie string
}

// send sends a request with the player's cookie and keeps any new cookie
// the server sets.
func (p *player) send(ctx context.Context, method, path string, header http.Header, body string) *http.Response {
	p.c.t.Helper()
	if header == nil {
		header = http.Header{}
	}
	if p.cookie != "" {
		header.Set("Cookie", p.cookie)
	}
	resp, err := send(ctx, method, path, header, body)
	if err != nil {
		p.c.t.Fatal(err)
	}
	if set := resp.Header.Get("Set-Cookie"); set != "" {
		p.cookie, _, _ = strings.Cut(set, ";")
	}
	return resp
}

// fetch makes a request and returns the response with its body read.
func (p *player) fetch(method, path string) (*http.Response, string) {
	p.c.t.Helper()
	resp := p.send(p.c.ctx, method, path, nil, "")
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		p.c.t.Fatalf("%s %s: %v", method, path, err)
	}
	return resp, string(b)
}

// get returns the status and the body as sent.
func (p *player) get(path string) (int, string) {
	p.c.t.Helper()
	resp, body := p.fetch(http.MethodGet, path)
	return resp.StatusCode, body
}

// post sends body as JSON and returns the status and the body without its
// trailing newline.
func (p *player) post(path, body string) (int, string) {
	p.c.t.Helper()
	resp := p.send(p.c.ctx, http.MethodPost, path, http.Header{"Content-Type": {"application/json"}}, body)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		p.c.t.Fatalf("POST %s: %v", path, err)
	}
	return resp.StatusCode, strings.TrimSpace(string(b))
}

// create starts a friend game with the player as White.
func (p *player) create() string {
	p.c.t.Helper()
	status, body := p.post("/api/games", "")
	var out struct{ Code string }
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &out) != nil || len(out.Code) != 6 {
		p.c.t.Fatalf("create: %d %s", status, body)
	}
	return out.Code
}

// open opens a stream. It closes when the check ends, or on close.
func (p *player) open(path string) *sseReader {
	p.c.t.Helper()
	ctx, cancel := context.WithCancel(p.c.ctx)
	resp := p.send(ctx, http.MethodGet, path, nil, "")
	p.c.t.Cleanup(func() { resp.Body.Close() })
	return &sseReader{t: p.c.t, resp: resp, br: bufio.NewReader(resp.Body), cancel: cancel}
}

// stream opens a game's stream, which takes Black's seat if it is free.
func (p *player) stream(code string) *sseReader {
	p.c.t.Helper()
	r := p.open("/api/games/" + code + "/stream")
	if ct := r.resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		p.c.t.Fatalf("stream Content-Type = %q", ct)
	}
	if xab := r.resp.Header.Get("X-Accel-Buffering"); xab != "no" {
		p.c.t.Fatalf("stream X-Accel-Buffering = %q, want no", xab)
	}
	return r
}

// queue opens the quick-match stream and reads its "queued" event: how
// many others are looking.
func (p *player) queue() (*sseReader, int) {
	p.c.t.Helper()
	r := p.open("/api/match")
	if ct := r.resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		p.c.t.Fatalf("match Content-Type = %q", ct)
	}
	event, data := r.next()
	var out struct{ Looking int }
	if event != "queued" || json.Unmarshal([]byte(data), &out) != nil {
		p.c.t.Fatalf("got %q %q, want a queued event", event, data)
	}
	return r, out.Looking
}

// me is GET /api/me, decoded, with the raw JSON for checks on its shape.
type me struct {
	Name           *string `json:"name"`
	ChangesLeft    int     `json:"changesLeft"`
	ChangesResetAt *int64  `json:"changesResetAt"`
	Game           *string `json:"game"`
	raw            []byte
}

func (p *player) me() me {
	p.c.t.Helper()
	status, body := p.get("/api/me")
	var out me
	if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil {
		p.c.t.Fatalf("/api/me: %d %s", status, body)
	}
	out.raw = []byte(body)
	return out
}

// name is the player's guest name, or "" before they have one.
func (p *player) name() string {
	p.c.t.Helper()
	if n := p.me().Name; n != nil {
		return *n
	}
	return ""
}

// view is a game as one guest sees it, from a "state" event or a GET: the
// typed View for reading, and the raw JSON for checks on its exact shape.
type view struct {
	game.View
	raw []byte
}

func parseView(t *testing.T, data string) view {
	t.Helper()
	var v game.View
	if err := json.Unmarshal([]byte(data), &v); err != nil {
		t.Fatalf("view %.200s: %v", data, err)
	}
	return view{View: v, raw: []byte(data)}
}

// get returns the JSON at path in the view (see lookup), or nil.
func (v view) get(path ...string) json.RawMessage {
	raw, _ := lookup(v.raw, path...)
	return raw
}

// sseReader reads "event:"/"data:" pairs and comments from a stream.
type sseReader struct {
	t      *testing.T
	resp   *http.Response
	br     *bufio.Reader
	cancel context.CancelFunc
}

// next returns the next event's name and data, or ("comment", text) for a
// heartbeat. The check fails if the stream ends first.
func (r *sseReader) next() (string, string) {
	r.t.Helper()
	var event, data string
	for {
		line, err := r.br.ReadString('\n')
		if err != nil {
			r.t.Fatalf("stream ended: %v", err)
		}
		line = strings.TrimSuffix(line, "\n")
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
}

// state reads the next "state" event.
func (r *sseReader) state() view {
	r.t.Helper()
	event, data := r.next()
	if event != "state" {
		r.t.Fatalf("got %q %q, want a state event", event, data)
	}
	return parseView(r.t, data)
}

// until reads states until one matches: every view is delivered, so a
// check waits for the one it wants.
func (r *sseReader) until(ok func(view) bool) view {
	r.t.Helper()
	for {
		if v := r.state(); ok(v) {
			return v
		}
	}
}

// rest reads to the end of the stream and returns what was left.
func (r *sseReader) rest() string {
	r.t.Helper()
	b, err := io.ReadAll(r.br)
	if err != nil {
		r.t.Fatalf("reading the rest of the stream: %v", err)
	}
	return string(b)
}

// close closes the connection, as a closed tab does.
func (r *sseReader) close() {
	r.cancel()
	r.resp.Body.Close()
}

// keys returns the keys of the JSON object raw, in the order they were sent.
func keys(t *testing.T, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("not a JSON object: %.200s", raw)
	}
	var out []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("reading keys of %.200s: %v", raw, err)
		}
		out = append(out, tok.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("reading keys of %.200s: %v", raw, err)
		}
	}
	return out
}

// lookup returns the JSON at path in raw, where each step is an object key
// or an array index, and whether it is there.
func lookup(raw []byte, path ...string) (json.RawMessage, bool) {
	for _, step := range path {
		var obj map[string]json.RawMessage
		if json.Unmarshal(raw, &obj) == nil && obj != nil {
			next, ok := obj[step]
			if !ok {
				return nil, false
			}
			raw = next
			continue
		}
		var arr []json.RawMessage
		i, err := strconv.Atoi(step)
		if json.Unmarshal(raw, &arr) != nil || err != nil || i < 0 || i >= len(arr) {
			return nil, false
		}
		raw = arr[i]
	}
	return raw, true
}

// header is a response header as fetch's headers.get reads it: nil when
// absent, else its values joined by ", ".
func header(resp *http.Response, name string) any {
	vs := resp.Header.Values(name)
	if len(vs) == 0 {
		return nil
	}
	return strings.Join(vs, ", ")
}

// expectJSON fails unless got, encoded as JSON, equals the JSON want, as
// bun's toEqual compares: objects by their keys and values, in any order.
// got may hold json.RawMessage values straight from a response.
func expectJSON(t *testing.T, got any, want string) {
	t.Helper()
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad want %s: %v", want, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("got  %s\nwant %s", b, want)
	}
}

// expectKeys fails unless the JSON object raw has exactly these keys, in
// this order.
func expectKeys(t *testing.T, what string, raw []byte, want ...string) {
	t.Helper()
	if got := keys(t, raw); !reflect.DeepEqual(got, want) {
		t.Errorf("%s keys = %q, want %q", what, got, want)
	}
}

// js quotes s as a JSON string, for building a want.
func js(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
