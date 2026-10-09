package server

import (
	"net/http/httptest"
	"testing"
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

func TestDeviceOf(t *testing.T) {
	for _, c := range []struct {
		w, h int
		want string
	}{{390, 844, "phone"}, {844, 390, "phone"}, {639, 1200, "phone"}, {768, 1024, "tablet"}, {1024, 1366, "tablet"}, {1440, 900, "desktop"}, {0, 0, ""}} {
		if got := deviceOf(c.w, c.h); got != c.want {
			t.Errorf("deviceOf(%d, %d) = %q, want %q", c.w, c.h, got, c.want)
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
	r.Header.Set("X-Playtest", "1")
	if !isRobot(r) {
		t.Error("the playtest header not a robot")
	}
}
