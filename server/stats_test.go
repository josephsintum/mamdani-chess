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

// The page names the zone its days and hours are in: a US zone by its
// region, any other by its city, each with today's abbreviation.
func TestZoneLabel(t *testing.T) {
	for _, c := range []struct {
		zone string
		at   time.Time
		want string
	}{
		{"America/Los_Angeles", time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC), "Pacific time (PDT)"},
		{"America/Los_Angeles", time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC), "Pacific time (PST)"},
		{"America/New_York", time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC), "Eastern time (EDT)"},
		{"Europe/London", time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), "London time (BST)"},
		{"UTC", time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), "UTC"},
	} {
		loc, err := time.LoadLocation(c.zone)
		if err != nil {
			t.Fatal(err)
		}
		if got := zoneLabel(loc, c.at); got != c.want {
			t.Errorf("%s on %s: %q, want %q", c.zone, c.at.Format("Jan 2"), got, c.want)
		}
	}
	var b strings.Builder
	if err := statsTemplate.Execute(&b, statsPage{Range: "Oct 13 – Oct 19, 2026", Zone: "Pacific time (PDT)"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "Oct 13 – Oct 19, 2026 · Pacific time (PDT)") {
		t.Error("the page doesn't name its zone under the range")
	}
}

// The page says when its counters are from an older version.
func TestStatsSaysWhenCountersAreStale(t *testing.T) {
	for _, stale := range []bool{false, true} {
		var b strings.Builder
		if err := statsTemplate.Execute(&b, statsPage{Stale: stale}); err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(b.String(), "a rebuild failed, see the logs"); got != stale {
			t.Errorf("stale %v: the line shows %v", stale, got)
		}
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

func TestChanges(t *testing.T) {
	for _, c := range []struct {
		now, before int
		want        change
	}{
		// Up, down and flat are on the page in TestStatsComparesWithTheRangeBefore.
		{3, 0, change{"new", "up"}}, {0, 0, change{"0%", "flat"}}, {1000, 999, change{"0%", "flat"}},
	} {
		if got := pctChange(c.now, c.before); got != c.want {
			t.Errorf("pctChange(%d, %d) = %v, want %v", c.now, c.before, got, c.want)
		}
	}
	for _, c := range []struct {
		n, of, bn, bof int
		want           change
	}{
		{1, 4, 1, 2, change{"−25 pts", "down"}}, {1, 3, 2, 6, change{"0 pts", "flat"}},
		{1, 3, 0, 0, change{"new", "up"}}, {0, 0, 0, 0, change{"0 pts", "flat"}},
	} {
		if got := ptsChange(c.n, c.of, c.bn, c.bof); got != c.want {
			t.Errorf("ptsChange(%d/%d, %d/%d) = %v, want %v", c.n, c.of, c.bn, c.bof, got, c.want)
		}
	}
}

// Each KPI shows its change against the range of the same length before:
// this week has 1 game, 2 practice games and 1 of 2 searches matched; the
// week before 2 games, 1 practice game and none of 1 search matched.
func TestStatsComparesWithTheRangeBefore(t *testing.T) {
	s, ts := newTestServer(t)
	s.StatsPassword = "hunter2"
	ctx := t.Context()
	now := time.Now()
	lastWeek := now.AddDate(0, 0, -9)
	for i, at := range []time.Time{lastWeek, now.AddDate(0, 0, -10), now.Add(-time.Minute)} {
		if err := s.store.CreateGame(ctx, store.Game{Code: fmt.Sprintf("G0000%d", i), White: "w", WhiteName: "w", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	for _, at := range []time.Time{lastWeek, now.Add(-time.Hour), now.Add(-time.Minute)} {
		if err := s.store.AddEvent(ctx, "practice", at); err != nil {
			t.Fatal(err)
		}
	}
	for _, sr := range []store.Search{
		{StartedAt: lastWeek, EndedAt: lastWeek.Add(time.Minute)},
		{StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-time.Hour).Add(time.Minute)},
		{StartedAt: now.Add(-time.Minute), EndedAt: now.Add(-time.Minute).Add(10 * time.Second), Matched: true},
	} {
		if err := s.store.AddSearch(ctx, sr); err != nil {
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
	// Games 1 vs 2, got going 0 vs 0, practice 2 vs 1, found an opponent 50% vs 0%.
	if want := []string{"down −50%", "flat 0%", "up +100%", "up +50 pts"}; !reflect.DeepEqual(got, want) {
		t.Errorf("chips %q, want %q", got, want)
	}
	if body := get("all"); strings.Contains(body, `class="chip`) {
		t.Error("all time shows a change")
	}
}

// The page shows the games, the road and the browser errors (escaped), and
// nothing about visitors or which games were played.
func TestStatsShowsTheGames(t *testing.T) {
	s, ts := newTestServer(t)
	s.StatsPassword = "hunter2"
	ctx := t.Context()
	now := time.Now()
	if err := s.store.AddBrowserError(ctx, store.BrowserError{At: now, Path: "/game", Message: "TypeError: <x>"}); err != nil {
		t.Fatal(err)
	}
	for _, g := range []struct {
		code, kind, reason, winner string
		st                         *store.GameStats
	}{
		{"G00001", "quick", "checkmate", "white", &store.GameStats{Moves: 30, Opened: 5, Fell: 2, BlackLost: 2, OpenHist: [6]int{5, 10, 10, 5, 0, 0}}},
		{"G00002", "friend", "resignation", "black", &store.GameStats{Moves: 12, Opened: 3, Fell: 1, WhiteLost: 1, MateByRoll: false, OpenHist: [6]int{2, 5, 5, 0, 0, 0}}},
		// Two more resignations: a row after the first is the longest bar.
		{"G00003", "quick", "resignation", "white", nil},
		{"G00004", "quick", "resignation", "white", nil},
	} {
		if err := s.store.CreateGame(ctx, store.Game{Code: g.code, Kind: g.kind, White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: now.Add(-2 * time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if err := s.store.AddTurn(ctx, g.code, store.Turn{Ply: 0, Move: "e2e4", WhiteMS: 500000, BlackMS: 600000, At: now.Add(-time.Hour)}); err != nil {
			t.Fatal(err)
		}
		if err := s.store.EndGame(ctx, g.code, store.Result{EndedAt: now.Add(-30 * time.Minute), Reason: g.reason, Winner: g.winner}, nil, g.st); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.store.AddSearch(ctx, store.Search{StartedAt: now.Add(-time.Hour), EndedAt: now.Add(-time.Hour).Add(19 * time.Second), Matched: true}); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/stats?days=7", nil)
	req.SetBasicAuth("", "hunter2")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	body := string(b)
	for _, want := range []string{
		`<h2>Games</h2>`, `<h2>Quick match</h2>`, `<h2>The road</h2>`,
		"Checkmate", "Resignation", "Quick match 3", "Friend link 1", // games per day legend with totals
		"How full the road gets", "Did the road decide it?", "Lost less to the road, and won",
		"found an opponent", "Gave up within 30 s", "Under 1 min left", "Joined within 5 min", "Streams reopened", "Server restarts",
		"from the 2 games with a stats row", "Rematches of friend games", "pieces, the Mamdani included",
		"Mon 10 am–12 pm: 0 games", "Mon 10 pm–12 am: 0 games", "latest: TypeError: &lt;x&gt;",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	// The bars scale to the largest row, not the first.
	for label, want := range map[string]string{"Checkmate": "33.3", "Resignation": "100.0"} {
		m := regexp.MustCompile(`<span>` + label + `</span>.*?width:([0-9.]+)%`).FindStringSubmatch(body)
		if m == nil || m[1] != want {
			t.Errorf("%s bar: %v, want width %s%%", label, m, want)
		}
	}
	for _, never := range []string{"G00001", "G00002", "soon", "{{", "ZgotmplZ", "<x>", "Visitors", "Countries", "Cities", "Devices", "Browsers", "From visit to game"} {
		if strings.Contains(body, never) {
			t.Errorf("the page shows %q", never)
		}
	}
}
