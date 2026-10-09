package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"mamdani-chess/store"
)

func TestStatsRange(t *testing.T) {
	ny, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 10, 8, 23, 30, 0, 0, ny)
	from, to, label := statsRange("30", now, ny)
	if to != time.Date(2026, 10, 9, 0, 0, 0, 0, ny) || from != time.Date(2026, 9, 9, 0, 0, 0, 0, ny) || label != "Sep 9 – Oct 8, 2026" {
		t.Errorf("30: %v %v %q", from, to, label)
	}
	if from, _, label := statsRange("all", now, ny); !from.IsZero() || label != "All time" {
		t.Errorf("all: %v %q", from, label)
	}
	if from, _, _ := statsRange("nope", now, ny); from != time.Date(2026, 9, 9, 0, 0, 0, 0, ny) {
		t.Errorf("an unknown range isn't 30 days: %v", from)
	}
}

func TestStatsNeedsPassword(t *testing.T) {
	s, ts := newTestServer(t)
	get := func(user, pass string) (int, http.Header, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats?days=7", nil)
		if user != "" || pass != "" {
			req.SetBasicAuth(user, pass)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, string(b)
	}
	// No password configured: the app shell, as for any unknown path.
	if status, _, body := get("", ""); status != 200 || !strings.Contains(body, "app shell") {
		t.Errorf("without STATS_PASSWORD: %d %q", status, body)
	}
	s.StatsPassword = "hunter2"
	if status, h, _ := get("", ""); status != 401 || h.Get("WWW-Authenticate") != `Basic realm="stats"` {
		t.Errorf("no credentials: %d %q", status, h.Get("WWW-Authenticate"))
	}
	if status, _, _ := get("me", "wrong"); status != 401 {
		t.Errorf("wrong password: %d", status)
	}
	status, h, body := get("anyone", "hunter2")
	if status != 200 || !strings.HasPrefix(h.Get("Content-Type"), "text/html") || !strings.Contains(body, "<h1>Stats</h1>") {
		t.Errorf("right password: %d %q", status, h.Get("Content-Type"))
	}
}

func TestStatsShowsTheNumbers(t *testing.T) {
	s, ts := newTestServer(t)
	s.StatsPassword = "hunter2"
	ctx := t.Context()
	now := time.Now()
	for _, v := range []store.Visit{
		{At: now.Add(-time.Hour), Visitor: "ann000000000", Path: "/", Country: "Canada", City: "Toronto", Device: "phone", OS: "iOS", Browser: "Safari"},
		{At: now.Add(-time.Minute), Visitor: "ann000000000", Path: "/game", Device: "phone", OS: "iOS", Browser: "Safari"},
	} {
		if err := s.store.AddVisit(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.AddBrowserError(ctx, store.BrowserError{At: now, Visitor: "ann000000000", Path: "/game", Message: "TypeError: <x>", Browser: "Safari"}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats", nil)
	req.SetBasicAuth("", "hunter2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	for _, want := range []string{"Toronto", "Canada", "phone", "Safari", "TypeError: &lt;x&gt;", `class="num">1<`, "Direct or a chat app"} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if strings.Contains(body, "<x>") {
		t.Error("an error message was not escaped")
	}
}
