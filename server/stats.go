package server

import (
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
	"math"
	"net/http"
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

// statsRange reads ?days= (7, 30, 90 or all; anything else is 30): to is
// the start of tomorrow in loc, so today counts in full.
func statsRange(q string, now time.Time, loc *time.Location) (from, to time.Time, label string) {
	y, m, d := now.In(loc).Date()
	to = time.Date(y, m, d+1, 0, 0, 0, 0, loc)
	days := 30
	switch q {
	case "7":
		days = 7
	case "90":
		days = 90
	case "all":
		return time.Time{}, to, "All time"
	}
	from = to.AddDate(0, 0, -days)
	return from, to, rangeLabel(from, to)
}

// rangeLabel names the days from <= day < to, e.g. "Sep 9 – Oct 8, 2026".
func rangeLabel(from, to time.Time) string {
	last := to.AddDate(0, 0, -1)
	if from.Year() == last.Year() {
		return from.Format("Jan 2") + " – " + last.Format("Jan 2, 2006")
	}
	return from.Format("Jan 2, 2006") + " – " + last.Format("Jan 2, 2006")
}

// rangeBefore is the range of the same number of days that ends at from.
func rangeBefore(from, to time.Time) time.Time {
	days := 0
	for t := from; t.Before(to); t = t.AddDate(0, 0, 1) {
		days++
	}
	return from.AddDate(0, 0, -days)
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

// share is n as a whole percentage of of, 0 when of is 0.
func share(n, of int) int {
	if of == 0 {
		return 0
	}
	return int(100*float64(n)/float64(of) + 0.5)
}

// statsPage is what the template reads.
type statsPage struct {
	Range, Days string
	Ranges      []statsRangeOption
	Visits      store.VisitStats
	Funnel      store.Funnel
	Steps       []funnelStep
	DayMax      int // the busiest day's visitors, for the column heights
	// Each KPI against the range of the same length before; empty for
	// all time. Before names that range.
	Before                            string
	Visitors, Played, Games, CameBack change

	Section                                 store.GameSection
	QuickTotal, FriendTotal, PracticeTotal  int
	DayKindMax, LengthMax, WaitMax, OpenMax int
	OpenPct                                 [6]int // each open-holes count as a percentage of turns
	Decisive                                int
	FriendRows                              []countRow
	DecidedRows                             []countRow
	HeatRows                                []heatRow
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
	top := 0
	for _, row := range h {
		for _, n := range row {
			top = max(top, n)
		}
	}
	days := [7]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	var rows []heatRow
	for d, row := range h {
		hr := heatRow{Day: days[d]}
		for b, n := range row {
			level := 0
			switch t := 100 * n / max(top, 1); {
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

type funnelStep struct {
	Label, Note string
	N           int
	Pct         int // of visited
	Drop        int // percent lost since the step before; 0 on the first
	Shade       int // 1 (darkest) to 5
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
	loc := s.StatsZone
	if loc == nil {
		loc = time.UTC
	}
	days := r.URL.Query().Get("days")
	from, to, label := statsRange(days, time.Now(), loc)
	visits, err := s.store.VisitStats(r.Context(), from, to, loc)
	if err != nil {
		s.internalError(w, "visit stats", err)
		return
	}
	funnel, err := s.store.FunnelStats(r.Context(), from, to)
	if err != nil {
		s.internalError(w, "funnel stats", err)
		return
	}
	page := statsPage{Range: label, Days: days, Visits: visits, Funnel: funnel, Steps: funnelSteps(funnel)}
	if !from.IsZero() {
		prevFrom := rangeBefore(from, to)
		prevVisits, err := s.store.VisitStats(r.Context(), prevFrom, from, loc)
		if err != nil {
			s.internalError(w, "visit stats before", err)
			return
		}
		prevFunnel, err := s.store.FunnelStats(r.Context(), prevFrom, from)
		if err != nil {
			s.internalError(w, "funnel stats before", err)
			return
		}
		page.Before = "vs " + rangeLabel(prevFrom, from)
		page.Visitors = pctChange(visits.Visitors, prevVisits.Visitors)
		page.Played = pctChange(funnel.Moved, prevFunnel.Moved)
		page.Games = pctChange(funnel.Games, prevFunnel.Games)
		page.CameBack = ptsChange(visits.Returning, visits.Visitors, prevVisits.Returning, prevVisits.Visitors)
	}
	for _, o := range []statsRangeOption{{"7", "7 days", false}, {"30", "30 days", false}, {"90", "90 days", false}, {"all", "All", false}} {
		o.On = o.Days == days || (days != "7" && days != "90" && days != "all" && o.Days == "30")
		page.Ranges = append(page.Ranges, o)
	}
	for _, d := range visits.Days {
		page.DayMax = max(page.DayMax, d.New+d.Returning)
	}
	section, err := s.store.GameSection(r.Context(), from, to, loc)
	if err != nil {
		s.internalError(w, "game section", err)
		return
	}
	page.Section = section
	for _, d := range section.Days {
		page.QuickTotal += d.Quick
		page.FriendTotal += d.Friend
		page.PracticeTotal += d.Practice
		page.DayKindMax = max(page.DayKindMax, d.Quick+d.Friend+d.Practice)
	}
	for _, c := range section.Lengths {
		page.LengthMax = max(page.LengthMax, c.N)
	}
	for _, c := range section.Quick.Waits {
		page.WaitMax = max(page.WaitMax, c.N)
	}
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

// funnelSteps lays out the funnel's rows: each step's share of the
// visitors and the share lost since the step before (never below 0: Black
// can finish a game by resigning without moving).
func funnelSteps(funnel store.Funnel) []funnelStep {
	var steps []funnelStep
	prev := 0
	for i, st := range []struct {
		label, note string
		n           int
	}{
		{"Visited", "", funnel.Visited},
		{"Opened a game", "quick match, a friend link, practice or watching", funnel.Opened},
		{"Made a move", "", funnel.Moved},
		{"Finished a game", "", funnel.Finished},
		{"Played again", "finished a second game, any day in the range", funnel.Again},
	} {
		step := funnelStep{Label: st.label, Note: st.note, N: st.n, Pct: share(st.n, funnel.Visited), Shade: i + 1}
		if i > 0 && prev > 0 {
			step.Drop = max(0, int(100*float64(prev-st.n)/float64(prev)+0.5))
		}
		prev = st.n
		steps = append(steps, step)
	}
	return steps
}
