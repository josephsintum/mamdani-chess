package server

import (
	"bufio"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"mamdani-chess/game"
)

// queue opens the quick-match stream and reads its "queued" event.
func (p *player) queue() (sseReader, int) {
	p.t.Helper()
	resp, err := p.c.Get(p.url + "/api/match")
	if err != nil {
		p.t.Fatal(err)
	}
	p.t.Cleanup(func() { resp.Body.Close() })
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		p.t.Fatalf("Content-Type = %q", ct)
	}
	r := sseReader{t: p.t, sc: bufio.NewScanner(resp.Body)}
	ev, data := r.next()
	var q struct{ Looking int }
	if ev != "queued" || json.Unmarshal([]byte(data), &q) != nil {
		p.t.Fatalf("got %q %q, want queued", ev, data)
	}
	return r, q.Looking
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

func TestQuickMatchOverHTTP(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)

	a, looking := alice.queue()
	if looking != 1 {
		t.Fatalf("alice queued: looking %d, want 1", looking)
	}
	var list struct{ Looking int }
	if _, body := carol.get("/api/games"); json.Unmarshal([]byte(body), &list) != nil || list.Looking != 1 {
		t.Fatalf("GET /api/games while alice waits: %s", body)
	}
	b, _ := bob.queue()
	code := matchedCode(a)
	if got := matchedCode(b); got != code {
		t.Fatalf("bob matched to %s, alice to %s", got, code)
	}

	va := alice.stream(code).state()
	vb := bob.stream(code).state()
	if va.Status != game.Playing || va.You == vb.You || va.You == "spectator" || vb.You == "spectator" {
		t.Fatalf("alice %s, bob %s, status %s; want one each side, playing", va.You, vb.You, va.Status)
	}
	if va.Players.White == "" || va.Players.Black == "" {
		t.Fatalf("players %+v, want both named", va.Players)
	}
	var live struct {
		Games   []game.Live
		Looking int
	}
	if _, body := carol.get("/api/games"); json.Unmarshal([]byte(body), &live) != nil || live.Looking != 0 || len(live.Games) != 1 || live.Games[0].Code != code {
		t.Fatalf("GET /api/games after the match: %s", body)
	}
}

func TestQuickMatchGivesTheGuestAName(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	alice.queue()
	if alice.name() == "" {
		t.Fatal("joining quick match gave no name")
	}
}

func TestLiveGamesIsEmptyJSONList(t *testing.T) {
	_, ts := newTestServer(t)
	status, body := newPlayer(t, ts).get("/api/games")
	if status != http.StatusOK || body != "{\"games\":[],\"looking\":0}\n" {
		t.Fatalf("GET /api/games: %d %q", status, body)
	}
}

func TestClosingTheStreamLeavesTheQueue(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	resp, err := alice.c.Get(ts.URL + "/api/match")
	if err != nil {
		t.Fatal(err)
	}
	r := sseReader{t: t, sc: bufio.NewScanner(resp.Body)}
	if ev, _ := r.next(); ev != "queued" {
		t.Fatalf("got %q", ev)
	}
	resp.Body.Close() // Cancel, or a closed tab
	deadline := time.Now().Add(2 * time.Second)
	for s.match.Looking() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("still looking 2s after the stream closed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// After "matched" the server ends the stream, and tells EventSource to wait
// an hour before reconnecting, so the guest isn't queued again.
func TestMatchedEndsTheStreamWithALongRetry(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	resp, err := alice.c.Get(ts.URL + "/api/match")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	bob.queue()
	body, _ := io.ReadAll(resp.Body) // returns once the server ends the stream
	if !strings.Contains(string(body), "retry: 3600000\nevent: matched\n") {
		t.Fatalf("stream was %q", body)
	}
}

func TestASeatedGuestCantQueue(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	bob.stream(code).state() // the game is on: alice is seated
	if me := alice.me(); me.Game == nil || *me.Game != code {
		t.Fatalf("GET /api/me game = %v, want %s", me.Game, code)
	}
	status, body := alice.get("/api/match")
	var out struct{ Code string }
	if status != http.StatusConflict || json.Unmarshal([]byte(body), &out) != nil || out.Code != code {
		t.Fatalf("queueing while seated: %d %s, want 409 with the game's code", status, body)
	}
}

func TestLookingLeavesYouOut(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	alice.queue()
	var a, b struct{ Looking int }
	_, body := alice.get("/api/games")
	json.Unmarshal([]byte(body), &a)
	_, body = bob.get("/api/games")
	json.Unmarshal([]byte(body), &b)
	if a.Looking != 0 || b.Looking != 1 {
		t.Fatalf("alice sees %d looking, bob sees %d; want 0 and 1", a.Looking, b.Looking)
	}
}

func TestLowercaseCodeFindsTheGame(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	code := alice.create()
	if status, body := alice.get("/api/games/" + strings.ToLower(code)); status != http.StatusOK {
		t.Fatalf("GET with a lowercase code: %d %s", status, body)
	}
}
