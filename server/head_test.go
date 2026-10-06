package server

import (
	"net/http"
	"testing"
)

// A HEAD request runs a GET route's handler, and a stream's handler joins:
// HEAD on a game's stream took Black's seat (seats are never given back),
// and HEAD on /api/match put the guest in the quick-match line. A HEAD on
// a stream now gets the stream's headers and joins nothing.
func TestHeadOnAStreamJoinsNothing(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()

	head := func(p *player, path string) *http.Response {
		t.Helper()
		req, _ := http.NewRequest(http.MethodHead, ts.URL+path, nil)
		// Close the connection after the reply, as curl -I does, so a handler
		// that joined on HEAD sees the client leave and the test can end.
		req.Close = true
		resp, err := p.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp
	}

	if resp := head(bob, "/api/games/"+code+"/stream"); resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("HEAD stream: %d %q, want 200 text/event-stream", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if v := carol.stream(code).state(); v.You != "black" {
		t.Fatalf("carol is %s after bob's HEAD; the seat should still have been free", v.You)
	}
	if resp := head(bob, "/api/games/NOPE99/stream"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("HEAD on an unknown game's stream: %d, want 404", resp.StatusCode)
	}

	if resp := head(bob, "/api/match"); resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("HEAD match: %d %q, want 200 text/event-stream", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if n := s.match.Looking(); n != 0 {
		t.Errorf("%d looking after a HEAD on /api/match, want 0", n)
	}
	if bob.name() != "" {
		t.Error("a HEAD on /api/match gave the guest a name")
	}
}
