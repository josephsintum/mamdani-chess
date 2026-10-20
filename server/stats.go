package server

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"mamdani-chess/store"
)

//go:embed stats.html
var statsHTML string

var statsTemplate = template.Must(template.New("stats").Funcs(template.FuncMap{
	"pct": func(n, of int) string {
		if of == 0 {
			return "0"
		}
		return fmt.Sprintf("%.1f", 100*float64(n)/float64(of))
	},
	"share": share,
	"top":   top,
	"sub1":  func(n int) int { return n - 1 },
	"dict": func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	},
}).Parse(statsHTML))

// statsRange reads ?days= (7, 30, 90 or all; anything else is 30) as the
// range's key and its days from <= day < to. To is the start of tomorrow
// in loc, so today counts in full, and before starts the range of the same
// length that ends at from. All time has neither from nor before.
func statsRange(q string, now time.Time, loc *time.Location) (key string, from, to, before time.Time) {
	y, m, d := now.In(loc).Date()
	to = time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	days := 30
	switch q {
	case "7":
		days = 7
	case "90":
		days = 90
	case "all":
		return q, time.Time{}, to, time.Time{}
	}
	from = to.AddDate(0, 0, -days)
	return strconv.Itoa(days), from, to, from.AddDate(0, 0, -days)
}

// rangeLabel names the days from <= day < to, e.g. "Sep 9 – Oct 8, 2026".
func rangeLabel(from, to time.Time) string {
	last := to.AddDate(0, 0, -1)
	if from.Year() == last.Year() {
		return from.Format("Jan 2") + " – " + last.Format("Jan 2, 2006")
	}
	return from.Format("Jan 2, 2006") + " – " + last.Format("Jan 2, 2006")
}

// change is a KPI against the range before: Text like "+18%", "−4%",
// "+3 pts" or "new", and Dir up, down or flat. A zero change shows no chip.
type change struct {
	Text, Dir string
}

// dirOf is the direction of a rounded change.
func dirOf(n int) string {
	switch {
	case n > 0:
		return "up"
	case n < 0:
		return "down"
	}
	return "flat"
}

// signed writes n with a plus or a minus sign (U+2212), and none for 0.
func signed(n int) string {
	switch {
	case n > 0:
		return fmt.Sprintf("+%d", n)
	case n < 0:
		return fmt.Sprintf("−%d", -n)
	}
	return "0"
}

// pctChange is now against before, in percent; "new" when before was 0.
func pctChange(now, before int) change {
	if before == 0 {
		if now == 0 {
			return change{"0%", "flat"}
		}
		return change{"new", "up"}
	}
	p := int(math.Round(100 * float64(now-before) / float64(before)))
	return change{signed(p) + "%", dirOf(p)}
}

// ptsChange is the share n/of against bn/bof, in percentage points of the
// shares the cards show; "new" when the range before had no one.
func ptsChange(n, of, bn, bof int) change {
	if bof == 0 {
		if of == 0 {
			return change{"0 pts", "flat"}
		}
		return change{"new", "up"}
	}
	d := share(n, of) - share(bn, bof)
	return change{signed(d) + " pts", dirOf(d)}
}

// top is the largest count, which a list's longest bar stands for (0 for
// none).
func top(rows []store.Count) int {
	n := 0
	for _, r := range rows {
		n = max(n, r.N)
	}
	return n
}

// total is the sum of the counts.
func total(rows []store.Count) int {
	n := 0
	for _, r := range rows {
		n += r.N
	}
	return n
}

// share is n as a whole percentage of of, 0 when of is 0.
func share(n, of int) int {
	if of == 0 {
		return 0
	}
	return int(100*float64(n)/float64(of) + 0.5)
}

// statsPage is what the template reads.
type statsPage struct {
	Range       string
	Zone        string // the zone the days and hours are in, e.g. "Pacific time (PDT)"
	Ranges      []statsRangeOption
	LatestError string
	// Stale says the counters are from an older version: a rebuild failed.
	Stale bool
	// Each KPI against the range of the same length before; empty for
	// all time. Before names that range.
	Before                           string
	Games, GotGoing, Practice, Found change

	Section                                store.Summary
	QuickTotal, FriendTotal, PracticeTotal int
	DayKindMax, OpenMax                    int
	OpenPct                                [6]int // each open-holes count as a percentage of turns
	LeftKnown, JoinKnown                   int    // games with a winner's clock; joins with a known time
	Decisive                               int
	FriendRows                             []countRow
	DecidedRows                            []countRow
	HeatRows                               []heatRow
}

// countRow is a labelled count, with the colour of its bar when it has one.
type countRow struct {
	Label string
	Color template.CSS // a fixed colour token, never user data
	N     int
}

// PerGame is n over the games with a stats row, to one decimal.
func (p statsPage) PerGame(n int) string {
	if p.Section.Road.Games == 0 {
		return "0"
	}
	return fmt.Sprintf("%.1f", float64(n)/float64(p.Section.Road.Games))
}

type heatRow struct {
	Day   string
	Cells []heatCell
}

type heatCell struct {
	Level int // 0 (none) to 4 (the busiest)
	Title string
}

var heatBlocks = [12]string{"12–2 am", "2–4 am", "4–6 am", "6–8 am", "8–10 am", "10 am–12 pm", "12–2 pm", "2–4 pm", "4–6 pm", "6–8 pm", "8–10 pm", "10 pm–12 am"}

// heatRows shades each weekday × two-hour block by its share of the
// busiest block: 0 for none, then up to a quarter, half, 85%, and the rest.
func heatRows(h store.Heat) []heatRow {
	busiest := 0
	for _, row := range h {
		for _, n := range row {
			busiest = max(busiest, n)
		}
	}
	days := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var rows []heatRow
	for d, row := range h {
		hr := heatRow{Day: days[d]}
		for b, n := range row {
			level := 0
			switch t := 100 * n / max(busiest, 1); {
			case n == 0:
			case t <= 25:
				level = 1
			case t <= 50:
				level = 2
			case t <= 85:
				level = 3
			default:
				level = 4
			}
			hr.Cells = append(hr.Cells, heatCell{Level: level, Title: fmt.Sprintf("%s %s: %d games", days[d], heatBlocks[b], n)})
		}
		rows = append(rows, hr)
	}
	return rows
}

type statsRangeOption struct {
	Days, Label string
	On          bool
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	if s.StatsPassword == "" {
		s.static(w, r)
		return
	}
	if _, pass, ok := r.BasicAuth(); !ok || subtle.ConstantTimeCompare([]byte(pass), []byte(s.StatsPassword)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="stats"`)
		http.Error(w, "stats need the password", http.StatusUnauthorized)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), statsTimeout)
	defer cancel()
	loc := s.store.Zone()
	key, from, to, prevFrom := statsRange(r.URL.Query().Get("days"), time.Now(), loc)
	fromDay, toDay := "", store.DayOf(to, loc)
	readFrom := ""
	if !from.IsZero() {
		fromDay = store.DayOf(from, loc)
		readFrom = store.DayOf(prevFrom, loc)
	}
	// One read for the range and the range before it.
	counted, err := s.store.Counts(ctx, readFrom, toDay)
	if err != nil {
		s.internalError(w, "stats counters", err)
		return
	}
	split := slices.IndexFunc(counted, func(d store.DayCounts) bool { return d.Day >= fromDay })
	if split < 0 {
		split = len(counted)
	}
	section := store.Summarize(counted[split:], fromDay, toDay)
	latest, err := s.store.LatestError(ctx, from, to)
	if err != nil {
		s.internalError(w, "latest error", err)
		return
	}
	page := statsPage{Range: "All time", Zone: zoneLabel(loc, time.Now()), LatestError: latest, Stale: s.store.CountersStale()}
	page.PracticeTotal = practiceGames(section)
	if !from.IsZero() {
		prev := store.Summarize(counted[:split], readFrom, fromDay)
		page.Range = rangeLabel(from, to)
		page.Before = "vs " + rangeLabel(prevFrom, from)
		page.Games = pctChange(section.Games, prev.Games)
		page.GotGoing = pctChange(section.GotGoing, prev.GotGoing)
		page.Practice = pctChange(page.PracticeTotal, practiceGames(prev))
		page.Found = ptsChange(section.Quick.Matched, section.Quick.Searches, prev.Quick.Matched, prev.Quick.Searches)
	}
	for _, o := range []statsRangeOption{{"7", "7 days", false}, {"30", "30 days", false}, {"90", "90 days", false}, {"all", "All", false}} {
		o.On = o.Days == key
		page.Ranges = append(page.Ranges, o)
	}
	page.Section = section
	for _, d := range section.Days {
		page.QuickTotal += d.Quick
		page.FriendTotal += d.Friend
		page.DayKindMax = max(page.DayKindMax, d.Quick+d.Friend+d.Practice)
	}
	page.LeftKnown = total(section.WinnerLeft)
	page.JoinKnown = total(section.Friend.Join)
	turns := 0
	for _, n := range section.Road.OpenHist {
		turns += n
	}
	for i, n := range section.Road.OpenHist {
		page.OpenPct[i] = share(n, turns)
		page.OpenMax = max(page.OpenMax, page.OpenPct[i])
	}
	page.HeatRows = heatRows(section.Heat)
	page.FriendRows = []countRow{{Label: "Someone joined", N: section.Friend.Joined}, {Label: "Game finished", N: section.Friend.Finished}}
	road := section.Road
	page.Decisive = road.LessWon + road.MoreWon + road.Same
	page.DecidedRows = []countRow{
		{"Lost less to the road, and won", template.CSS("var(--c1)"), road.LessWon},
		{"Lost more to the road, and still won", template.CSS("var(--c3)"), road.MoreWon},
		{"Lost the same", template.CSS("var(--text-3)"), road.Same},
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := statsTemplate.Execute(w, page); err != nil {
		s.log.Error("render stats", "err", err)
	}
}

// statsTimeout bounds the page's reads, so it can never hold the one
// connection long enough to keep a game's save waiting.
const statsTimeout = 2 * time.Second

// usZones name the US zones by region, as people say them.
var usZones = map[string]string{
	"America/Los_Angeles": "Pacific", "America/Denver": "Mountain", "America/Phoenix": "Mountain",
	"America/Chicago": "Central", "America/New_York": "Eastern",
}

// zoneLabel names loc for the page: "Pacific time (PDT)" for a US zone,
// "London time (BST)" for another, with the abbreviation in use at now.
func zoneLabel(loc *time.Location, now time.Time) string {
	name, abbr := loc.String(), now.In(loc).Format("MST")
	if name == "UTC" {
		return name
	}
	if region, ok := usZones[name]; ok {
		return region + " time (" + abbr + ")"
	}
	city := strings.ReplaceAll(name[strings.LastIndex(name, "/")+1:], "_", " ")
	return city + " time (" + abbr + ")"
}

// practiceGames is how many practice games a summary's days started.
func practiceGames(g store.Summary) int {
	n := 0
	for _, d := range g.Days {
		n += d.Practice
	}
	return n
}
