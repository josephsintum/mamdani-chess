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
		for _, l := range strings.Split(text, "\n") {
			if strings.Contains(l, want) {
				return l
			}
		}
	}
	return ""
}

// Every move logging at INFO buried everything else (4,205 move lines for
// 20 games): moves log at DEBUG, the game's own events stay at INFO.
func TestMoveRequestsLogAtDebug(t *testing.T) {
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
	if move := logs.line("path=/api/games/" + code + "/move"); !strings.Contains(move, "level=DEBUG") {
		t.Errorf("move request logged as %q, want DEBUG", move)
	}
	if create := logs.line("method=POST path=/api/games "); !strings.Contains(create, "level=INFO") {
		t.Errorf("create request logged as %q, want INFO", create)
	}
}
