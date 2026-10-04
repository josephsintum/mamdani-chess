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
