package server

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"reflect"
	"regexp"
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
	if before := rangeBefore(from, to); before != time.Date(2026, 8, 10, 0, 0, 0, 0, ny) || rangeLabel(before, from) != "Aug 10 – Sep 8, 2026" {
		t.Errorf("the 30 days before: %v %q", before, rangeLabel(before, from))
	}
	// Across the clock change, still 7 calendar days.
	if before := rangeBefore(time.Date(2026, 11, 3, 0, 0, 0, 0, ny), time.Date(2026, 11, 10, 0, 0, 0, 0, ny)); before != time.Date(2026, 10, 27, 0, 0, 0, 0, ny) {
		t.Errorf("the week before Nov 3: %v", before)
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

func TestChanges(t *testing.T) {
	for _, c := range []struct {
		now, before int
		want        change
	}{
		{5, 4, change{"+25%", "up"}}, {1, 2, change{"−50%", "down"}}, {3, 3, change{"0%", "flat"}},
		{3, 0, change{"new", "up"}}, {0, 0, change{"0%", "flat"}}, {0, 7, change{"−100%", "down"}}, {1000, 999, change{"0%", "flat"}},
	} {
		if got := pctChange(c.now, c.before); got != c.want {
			t.Errorf("pctChange(%d, %d) = %v, want %v", c.now, c.before, got, c.want)
		}
	}
	for _, c := range []struct {
		n, of, bn, bof int
		want           change
	}{
		{1, 5, 0, 4, change{"+20 pts", "up"}}, {1, 4, 1, 2, change{"−25 pts", "down"}}, {1, 3, 2, 6, change{"0 pts", "flat"}},
		{1, 3, 0, 0, change{"new", "up"}}, {0, 0, 0, 0, change{"0 pts", "flat"}},
	} {
		if got := ptsChange(c.n, c.of, c.bn, c.bof); got != c.want {
			t.Errorf("ptsChange(%d/%d, %d/%d) = %v, want %v", c.n, c.of, c.bn, c.bof, got, c.want)
		}
	}
}

// A later step can pass the one before (Black resigns without moving: two
// finished, one moved); the drop is then 0, never negative.
func TestFunnelStepsDropNeverNegative(t *testing.T) {
	steps := funnelSteps(store.Funnel{Visited: 4, Opened: 3, Moved: 1, Finished: 2, Again: 0})
	var drops []int
	for _, s := range steps {
		drops = append(drops, s.Drop)
	}
	if want := []int{0, 25, 67, 0, 100}; !reflect.DeepEqual(drops, want) {
		t.Errorf("drops %v, want %v", drops, want)
	}
}

// Each KPI shows its change against the range of the same length before:
// this week has 5 visitors (one on two days) and 1 game, last week 4
// visitors and 2 games.
func TestStatsComparesWithTheRangeBefore(t *testing.T) {
	s, ts := newTestServer(t)
	s.StatsPassword = "hunter2"
	ctx := t.Context()
	now := time.Now()
	add := func(at time.Time, who string) {
		t.Helper()
		if err := s.store.AddVisit(ctx, store.Visit{At: at, Visitor: who, Path: "/"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, who := range []string{"p1", "p2", "p3", "p4"} {
		add(now.AddDate(0, 0, -9), who+"0000000000")
	}
	for _, who := range []string{"c1", "c2", "c3", "c4", "c5"} {
		add(now.Add(-time.Minute), who+"0000000000")
	}
	add(now.AddDate(0, 0, -2), "c10000000000")
	for i, at := range []time.Time{now.AddDate(0, 0, -9), now.AddDate(0, 0, -10), now.Add(-time.Minute)} {
		if err := s.store.CreateGame(ctx, store.Game{Code: fmt.Sprintf("G0000%d", i), White: "w", WhiteName: "w", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	get := func(days string) string {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats?days="+days, nil)
		req.SetBasicAuth("", "hunter2")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	chips := regexp.MustCompile(`<span class="chip (\w+)"[^>]*>(?:<svg.*?</svg>)?([^<]*)</span>`)
	var got []string
	for _, m := range chips.FindAllStringSubmatch(get("7"), -1) {
		got = append(got, m[1]+" "+html.UnescapeString(m[2])) // the template writes + as &#43;
	}
	// Visitors 5 vs 4, Played a game 0 vs 0, Games 1 vs 2, Came back 20% vs 0%.
	if want := []string{"up +25%", "flat 0%", "down −50%", "up +20 pts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("chips %q, want %q", got, want)
	}
	if body := get("all"); strings.Contains(body, `class="chip`) {
		t.Error("all time shows a change")
	}
}
