package server

import (
	"crypto/subtle"
	_ "embed"
	"fmt"
	"html/template"
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
	"share": func(n, of int) int {
		if of == 0 {
			return 0
		}
		return int(100*float64(n)/float64(of) + 0.5)
	},
	"sub1": func(n int) int { return n - 1 },
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
	last := to.AddDate(0, 0, -1)
	if from.Year() == last.Year() {
		label = from.Format("Jan 2") + " – " + last.Format("Jan 2, 2006")
	} else {
		label = from.Format("Jan 2, 2006") + " – " + last.Format("Jan 2, 2006")
	}
	return from, to, label
}

// statsPage is what the template reads.
type statsPage struct {
	Range, Days string
	Ranges      []statsRangeOption
	Visits      store.VisitStats
	Funnel      store.Funnel
	Steps       []funnelStep
	DayMax      int // the busiest day's visitors, for the column heights
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
	page := statsPage{Range: label, Days: days, Visits: visits, Funnel: funnel}
	for _, o := range []statsRangeOption{{"7", "7 days", false}, {"30", "30 days", false}, {"90", "90 days", false}, {"all", "All", false}} {
		o.On = o.Days == days || (days != "7" && days != "90" && days != "all" && o.Days == "30")
		page.Ranges = append(page.Ranges, o)
	}
	for _, d := range visits.Days {
		page.DayMax = max(page.DayMax, d.New+d.Returning)
	}
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
		step := funnelStep{Label: st.label, Note: st.note, N: st.n, Shade: i + 1}
		if funnel.Visited > 0 {
			step.Pct = int(100*float64(st.n)/float64(funnel.Visited) + 0.5)
		}
		if i > 0 && prev > 0 {
			step.Drop = int(100*float64(prev-st.n)/float64(prev) + 0.5)
		}
		prev = st.n
		page.Steps = append(page.Steps, step)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := statsTemplate.Execute(w, page); err != nil {
		s.log.Error("render stats", "err", err)
	}
}
