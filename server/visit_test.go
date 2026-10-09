package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

func TestVisitorOf(t *testing.T) {
	id := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	for in, want := range map[string]string{id: "0123456789ab", "0123456789ab": "0123456789ab", "abc": "abc", "": ""} {
		if got := visitorOf(in); got != want {
			t.Errorf("visitorOf(%q) = %q, want %q", in, got, want)
		}
	}
}

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

func TestDeviceOf(t *testing.T) {
	for _, c := range []struct {
		w, h  int
		touch bool
		want  string
	}{{390, 844, true, "phone"}, {844, 390, true, "phone"}, {639, 1200, true, "phone"}, {768, 1024, true, "tablet"}, {1024, 1366, true, "tablet"},
		{1440, 900, false, "desktop"}, {1440, 900, true, "tablet"}, {2560, 1440, false, "desktop"}, {0, 0, false, ""}} {
		if got := deviceOf(c.w, c.h, c.touch); got != c.want {
			t.Errorf("deviceOf(%d, %d, %v) = %q, want %q", c.w, c.h, c.touch, got, c.want)
		}
	}
}

func TestParseUA(t *testing.T) {
	for ua, want := range map[string][2]string{
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1":    {"iOS", "Safari"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 Instagram 334.0.4.32.98":      {"iOS", "Instagram (in app)"},
		"Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Mobile Safari/537.36":                      {"Android", "Chrome"},
		"Mozilla/5.0 (Linux; Android 14; SM-S928B) AppleWebKit/537.36 (KHTML, like Gecko) SamsungBrowser/25.0 Chrome/121.0.0.0 Mobile Safari/537.36": {"Android", "Samsung"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15":                      {"macOS", "Safari"},
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36":                      {"macOS", "Chrome"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36 Edg/125.0.0.0":              {"Windows", "Edge"},
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:126.0) Gecko/20100101 Firefox/126.0":                                                           {"Windows", "Firefox"},
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) FxiOS/126.0 Mobile/15E148 Safari/605.1.15":  {"iOS", "Firefox"},
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0.0.0 Safari/537.36":                                      {"Linux", "Chrome"},
		"curl/8.6.0": {"", ""},
	} {
		os, browser := parseUA(ua)
		if os != want[0] || browser != want[1] {
			t.Errorf("parseUA(%q) = %q, %q; want %q, %q", ua, os, browser, want[0], want[1])
		}
	}
}

func TestReferrerOf(t *testing.T) {
	for in, want := range map[string]string{
		"":                                 "",
		"https://l.instagram.com/?u=x&e=y": "l.instagram.com",
		"https://www.google.com/search?q=mamdani":      "google.com",
		"https://mamdanichess.com/game/K7F3QZ":         "",
		"ref:ig":                                       "ref:ig",
		"ref:" + "a-very-long-tag-that-goes-on-and-on": "ref:a-very-long-tag-that",
		"not a url": "",
	} {
		if got := referrerOf(in, "mamdanichess.com"); got != want {
			t.Errorf("referrerOf(%q) = %q, want %q", in, got, want)
		}
	}
	if got := referrerOf("http://localhost:8080/rules", "localhost:8080"); got != "" {
		t.Errorf("own host with a port: referrerOf = %q, want empty", got)
	}
	// A host is at most 253 characters, so a longer one is cut there.
	long := strings.Repeat("a", 300) + ".example"
	if got := referrerOf("https://"+long+"/x", "mamdanichess.com"); got != long[:253] {
		t.Errorf("a long host: referrerOf = %d chars, want 253", len(got))
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/visit", nil)
	r.RemoteAddr = "10.0.0.5:1234"
	if ip, ok := clientIP(r); !ok || ip.String() != "10.0.0.5" {
		t.Errorf("from RemoteAddr: %v %v", ip, ok)
	}
	r.Header.Set("X-Real-IP", "203.0.113.9")
	if ip, _ := clientIP(r); ip.String() != "203.0.113.9" {
		t.Errorf("from X-Real-IP: %v", ip)
	}
	r.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.77")
	if ip, _ := clientIP(r); ip.String() != "203.0.113.77" {
		t.Errorf("from X-Forwarded-For: %v", ip)
	}
	r.Header.Set("X-Forwarded-For", "garbage")
	if _, ok := clientIP(r); ok {
		t.Error("garbage parsed as an address")
	}
}

func TestIsRobot(t *testing.T) {
	r := httptest.NewRequest("POST", "/api/visit", nil)
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

// lastVisit returns the stats for everything stored, so a test can check a
// row through the same path the page uses.
func lastVisit(t *testing.T, s *Server) store.VisitStats {
	t.Helper()
	st, err := s.store.VisitStats(t.Context(), time.Time{}, time.Now().Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestVisitStoresNothingIdentifying(t *testing.T) {
	s, ts := newTestServer(t)
	s.Geo = geoFunc(func(ip netip.Addr) (string, string) {
		if ip.String() != "203.0.113.9" {
			t.Errorf("looked up %v", ip)
		}
		return "Canada", "Toronto"
	})
	alice := newPlayer(t, ts)
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/visit",
		strings.NewReader(`{"path":"/game/K7F3QZ?instant","w":390,"h":844,"touch":true,"referrer":"https://l.instagram.com/?u=secret"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1")
	resp, err := alice.c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /api/visit: %d", resp.StatusCode)
	}
	if len(resp.Cookies()) != 1 || resp.Cookies()[0].Name != "guest" {
		t.Fatalf("a first visit set cookies %v, want the guest cookie", resp.Cookies())
	}
	// The visitor key is the first 12 hex characters of the guest ID (the
	// cookie's SHA-256), never the cookie or the whole ID.
	sum := sha256.Sum256([]byte(resp.Cookies()[0].Value))
	id := hex.EncodeToString(sum[:])
	keys, err := s.store.Visitors(t.Context(), time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || !regexp.MustCompile(`^[0-9a-f]{12}$`).MatchString(keys[0]) || !strings.HasPrefix(id, keys[0]) {
		t.Errorf("stored visitor keys %q, want [%q]", keys, id[:12])
	}
	st := lastVisit(t, s)
	got := []any{st.Views, st.Pages, st.Sources, st.Countries, st.Cities, st.Devices, st.Systems, st.Browsers}
	want := []any{1, []store.Count{{Label: "/game", N: 1}}, []store.Count{{Label: "Instagram", N: 1}}, []store.Count{{Label: "Canada", N: 1}}, []store.Count{{Label: "Toronto", N: 1}},
		[]store.Count{{Label: "phone", N: 1}}, []store.Count{{Label: "iOS", N: 1}}, []store.Count{{Label: "Safari", N: 1}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
}

func TestVisitIgnoresRobots(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	for _, h := range []http.Header{
		{"User-Agent": {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/125.0.0.0 Safari/537.36"}},
		{"X-Playtest": {"1"}},
	} {
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/visit", strings.NewReader(`{"path":"/","w":1440,"h":900}`))
		req.Header = h
		req.Header.Set("Content-Type", "application/json")
		resp, err := alice.c.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("%v: %d", h, resp.StatusCode)
		}
	}
	if st := lastVisit(t, s); st.Views != 0 {
		t.Errorf("robots stored %d views", st.Views)
	}
}

func TestVisitAndErrorBodies(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, body := alice.post("/api/visit", `nope`); status != http.StatusBadRequest || body != `{"error":"bad request body"}` {
		t.Errorf("bad visit: %d %s", status, body)
	}
	long := strings.Repeat("x", 500)
	if status, _ := alice.post("/api/error", `{"path":"/game/K7F3QZ","message":"`+long+`"}`); status != http.StatusNoContent {
		t.Errorf("error report: %d", status)
	}
	st := lastVisit(t, s)
	if st.Errors != 1 || len(st.LatestError) != 300 {
		t.Errorf("errors %d, latest %d chars; want 1 and 300", st.Errors, len(st.LatestError))
	}
}

// A game code in an error message (a URL in a stack, say) is never stored.
func TestErrorDropsGameCodes(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, _ := alice.post("/api/error", `{"path":"/game/K7F3QZ","message":"failed: https://mamdanichess.com/game/K7F3QZ?instant and /api/games/AB12CD/stream"}`); status != http.StatusNoContent {
		t.Errorf("error report: %d", status)
	}
	if st := lastVisit(t, s); st.LatestError != "failed: https://mamdanichess.com/game?instant and /api/games/stream" {
		t.Errorf("stored %q", st.LatestError)
	}
}
