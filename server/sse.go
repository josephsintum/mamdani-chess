package server

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// heartbeat is how often a quiet stream writes a comment, so idle
// connections stay open.
const heartbeat = 15 * time.Second

// startSSE sets the event-stream headers and returns the flusher. Every
// handler runs behind statusRecorder, which flushes.
func startSSE(w http.ResponseWriter) http.Flusher {
	fl := w.(http.Flusher)
	w.Header().Set("Connection", "keep-alive")
	headSSE(w)
	// A deploy ends every stream at once. A random retry per stream spreads
	// the browsers' own reconnects instead of sending them all back
	// together.
	fmt.Fprintf(w, "retry: %d\n\n", 1000+rand.N(2001))
	fl.Flush()
	return fl
}

// headSSE writes a stream's status and headers. On its own it answers a
// HEAD request on a stream: a route for GET also serves HEAD, and opening
// a stream joins (a game's seat, the quick-match line), so a HEAD must
// return before that: a link checker or curl -I would otherwise take
// Black's seat for good.
func headSSE(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Accel-Buffering", "no") // stop proxies buffering the stream
	w.WriteHeader(http.StatusOK)
}

// pump hands each value from ch to send, with a heartbeat while ch is
// quiet. The stream ends when the client leaves, the server closes, ch
// closes, a write fails or send reports false.
func pump[T any](s *Server, r *http.Request, w http.ResponseWriter, fl http.Flusher, ch <-chan T, send func(T) bool) {
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case v, open := <-ch:
			if !open || !send(v) {
				return
			}
		case <-ticker.C:
			if writeHeartbeat(w, fl) != nil {
				return
			}
		}
	}
}

// writeEvent writes one SSE event whose data is already-encoded JSON, and
// flushes it. It writes data straight to w, uncopied: every stream in a
// role sends the same bytes.
func writeEvent(w http.ResponseWriter, fl http.Flusher, event string, data []byte) error {
	for _, s := range [...]string{"event: ", event, "\ndata: "} {
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if _, err := io.WriteString(w, "\n\n"); err != nil {
		return err
	}
	fl.Flush()
	return nil
}

// writeLastEvent writes one SSE event that also tells the browser to wait
// retry before reconnecting, for a stream the server is about to end on
// purpose.
func writeLastEvent(w http.ResponseWriter, fl http.Flusher, event string, data []byte, retry time.Duration) error {
	if _, err := fmt.Fprintf(w, "retry: %d\n", retry.Milliseconds()); err != nil {
		return err
	}
	return writeEvent(w, fl, event, data)
}

// writeHeartbeat writes an SSE comment so idle connections stay open.
func writeHeartbeat(w http.ResponseWriter, fl http.Flusher) error {
	if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
		return err
	}
	fl.Flush()
	return nil
}
