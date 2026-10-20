package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

func TestPageOf(t *testing.T) {
	for in, want := range map[string]string{
		"/": "/", "/game/K7F3QZ?instant": "/game", "/rules": "/rules", "/dev/board": "other", "": "other",
	} {
		if got := pageOf(in); got != want {
			t.Errorf("pageOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsRobot(t *testing.T) {
	const iPhone = "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 Version/17.5 Mobile/15E148 Safari/604.1"
	r := httptest.NewRequest("POST", "/api/error", nil)
	for _, ua := range []string{
		iPhone,
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36",
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
		"Mozilla/5.0 (Macintosh) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/125.0.0.0 Safari/537.36",
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
	r.Header.Set("User-Agent", iPhone)
	r.Header.Set("X-Playtest", "1")
	if !isRobot(r) {
		t.Error("the playtest header not a robot")
	}
}

// errorsSoFar returns the counted browser errors and the latest stored
// message, through the same reads the page uses.
func errorsSoFar(t *testing.T, s *Server) (int, string) {
	t.Helper()
	ctx := t.Context()
	end := time.Now().Add(48 * time.Hour)
	days, err := s.store.Counts(ctx, "", end.UTC().Format("2006-01-02"))
	if err != nil {
		t.Fatal(err)
	}
	latest, err := s.store.LatestError(ctx, time.Time{}, end)
	if err != nil {
		t.Fatal(err)
	}
	return store.Summarize(days, "", end.UTC().Format("2006-01-02")).Errors, latest
}

// A long message is stored cut to 300 characters. (A bad body's 400 is in
// contract's TestErrorReport.)
func TestErrorMessageIsCut(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
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
	newPlaytester(t, ts).post("/api/error", `{"path":"/","message":"TypeError: y"}`)
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
