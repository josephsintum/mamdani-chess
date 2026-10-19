package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPageOf(t *testing.T) {
	for in, want := range map[string]string{
		"/": "/", "/game/K7F3QZ": "/game", "/game/K7F3QZ?instant": "/game", "/play": "/play", "/practice": "/practice",
		"/how-to-play": "/how-to-play", "/rules": "/rules", "/about": "/about", "/dev/board": "other", "": "other", "/stats": "other",
	} {
		if got := pageOf(in); got != want {
			t.Errorf("pageOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsRobot(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/error", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/125.0.0.0 Safari/537.36")
	if !isRobot(r) {
		t.Error("headless Chrome not a robot")
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 Version/17.5 Mobile/15E148 Safari/604.1")
	if isRobot(r) {
		t.Error("a phone is a robot")
	}
	for _, ua := range []string{
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 334.0.4.32.98",
		// Cubot phones carry "bot" in their model name.
		"Mozilla/5.0 (Linux; Android 11; CUBOT KINGKONG 7 Build/RP1A.200720.011) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/96.0.4664.104 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 10; CUBOT_X30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	} {
		r.Header.Set("User-Agent", ua)
		if isRobot(r) {
			t.Errorf("a person's browser is a robot: %q", ua)
		}
	}
	for _, ua := range []string{
		"Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		"facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php) SomeCrawler",
		"Mozilla/5.0 (compatible; Baiduspider/2.0)",
		"Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)",
		"Twitterbot/1.0",
		"DuckDuckBot/1.1; (+http://duckduckgo.com/duckduckbot.html)",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_5) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/13.1.1 Safari/605.1.15 (Applebot/0.1; +http://www.apple.com/go/applebot)",
	} {
		r.Header.Set("User-Agent", ua)
		if !isRobot(r) {
			t.Errorf("not a robot: %q", ua)
		}
	}
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 Version/17.5 Mobile/15E148 Safari/604.1")
	r.Header.Set("X-Playtest", "1")
	if !isRobot(r) {
		t.Error("the playtest header not a robot")
	}
}

// errorsSoFar returns the stored browser errors through the same path the
// page uses.
func errorsSoFar(t *testing.T, s *Server) (int, string) {
	t.Helper()
	st, err := s.store.BrowserErrors(t.Context(), time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	return st.N, st.Latest
}

func TestErrorBodies(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, body := alice.post("/api/error", `nope`); status != http.StatusBadRequest || body != `{"error":"bad request body"}` {
		t.Errorf("bad error report: %d %s", status, body)
	}
	long := strings.Repeat("x", 500)
	if status, _ := alice.post("/api/error", `{"path":"/game/K7F3QZ","message":"`+long+`"}`); status != http.StatusNoContent {
		t.Errorf("error report: %d", status)
	}
	if n, latest := errorsSoFar(t, s); n != 1 || len(latest) != 300 {
		t.Errorf("errors %d, latest %d chars; want 1 and 300", n, len(latest))
	}
}

// An error report knows nothing about who sent it: no guest cookie is
// minted for it, and a robot's report isn't stored.
func TestErrorKeepsNoOne(t *testing.T) {
	s, ts := newTestServer(t)
	resp, err := http.Post(ts.URL+"/api/error", "application/json", strings.NewReader(`{"path":"/","message":"TypeError: x"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent || len(resp.Cookies()) != 0 {
		t.Errorf("POST /api/error: %d, cookies %v; want 204 and none", resp.StatusCode, resp.Cookies())
	}
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/error", strings.NewReader(`{"path":"/","message":"TypeError: y"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Playtest", "1")
	if resp, err = http.DefaultClient.Do(req); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if n, latest := errorsSoFar(t, s); n != 1 || latest != "TypeError: x" {
		t.Errorf("stored %d errors, latest %q; want only the person's", n, latest)
	}
}

// A game code in an error message (a URL in a stack, say) is never stored.
func TestErrorDropsGameCodes(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, _ := alice.post("/api/error", `{"path":"/game/K7F3QZ","message":"failed: https://mamdanichess.com/game/K7F3QZ?instant and /api/games/AB12CD/stream"}`); status != http.StatusNoContent {
		t.Errorf("error report: %d", status)
	}
	if _, latest := errorsSoFar(t, s); latest != "failed: https://mamdanichess.com/game?instant and /api/games/stream" {
		t.Errorf("stored %q", latest)
	}
}
