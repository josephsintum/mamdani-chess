package server

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
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
	// A deploy ends every stream at once. A random retry per stream spreads
	// the browsers' own reconnects instead of sending them all back
	// together.
	fmt.Fprintf(w, "retry: %d\n\n", 1000+rand.N(2001))
	fl.Flush()
	return fl, true
}

// writeEvent writes one SSE event whose data is already-encoded JSON, and
// flushes it. The data goes straight to w: every stream in a role sends
// the same view, so copying it through fmt cost each stream a copy.
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
	if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
		return err
	}
	fl.Flush()
	return nil
}
