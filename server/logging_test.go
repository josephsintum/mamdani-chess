package server

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// lockedBuffer is a log sink safe to read while the server writes to it
// (a request's line is written just after its response is sent).
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

// line waits up to a second for a log line containing want.
func (b *lockedBuffer) line(want string) string {
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		b.mu.Lock()
		text := b.buf.String()
		b.mu.Unlock()
		for l := range strings.SplitSeq(text, "\n") {
			if strings.Contains(l, want) {
				return l
			}
		}
	}
	return ""
}

// At INFO, every request would use Railway's 500 log lines/s with about 17
// new visitors a second (a first visit loads about 30 files). Successful
// requests log at DEBUG; refusals and errors stay at INFO.
func TestSuccessfulRequestsLogAtDebug(t *testing.T) {
	s, ts := newTestServer(t)
	logs := &lockedBuffer{}
	s.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	bob.stream(code).state()
	if status, body := alice.post("/api/games/"+code+"/move", `{"from":"e2","to":"e4","seq":0}`); status != http.StatusNoContent {
		t.Fatalf("move: %d %s", status, body)
	}
	if move := logs.line("path=/api/games/" + code + "/move status=204"); !strings.Contains(move, "level=DEBUG") {
		t.Errorf("move request logged as %q, want DEBUG", move)
	}
	if create := logs.line("method=POST path=/api/games "); !strings.Contains(create, "level=DEBUG") {
		t.Errorf("create request logged as %q, want DEBUG", create)
	}
	if status, _ := alice.post("/api/games/"+code+"/move", `{"from":"e2","to":"e4","seq":0}`); status != http.StatusConflict {
		t.Fatalf("a stale move: %d, want 409", status)
	}
	if refused := logs.line("status=409"); !strings.Contains(refused, "level=INFO") {
		t.Errorf("refused move logged as %q, want INFO", refused)
	}
}

// A slow request logs at WARN. A stream stays open by design, so it never
// counts as slow.
func TestSlowRequestsLogAtWarn(t *testing.T) {
	s, ts := newTestServer(t)
	logs := &lockedBuffer{}
	s.log = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s.slow = 20 * time.Millisecond
	s.mux.HandleFunc("GET /api/test-slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
	})
	s.mux.HandleFunc("GET /api/test-slow-fail", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(30 * time.Millisecond)
		w.WriteHeader(http.StatusInternalServerError)
	})
	get(t, ts.URL+"/api/test-slow", "")
	get(t, ts.URL+"/api/test-slow-fail", "")
	if line := logs.line("path=/api/test-slow "); !strings.Contains(line, "level=WARN") {
		t.Errorf("slow request logged as %q, want WARN", line)
	}
	if line := logs.line("path=/api/test-slow-fail"); !strings.Contains(line, "level=WARN") {
		t.Errorf("slow failing request logged as %q, want WARN", line)
	}
	alice := newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	time.Sleep(30 * time.Millisecond)
	s.Close() // ends the stream
	if line := logs.line("path=/api/games/" + code + "/stream"); !strings.Contains(line, "level=DEBUG") {
		t.Errorf("a stream logged as %q, want DEBUG", line)
	}
}
