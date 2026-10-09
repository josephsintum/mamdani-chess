package store

import (
	"context"
	"database/sql"
	"sort"
	"strings"
	"time"
)

// Count is one row of a top list.
type Count struct {
	Label string
	N     int
}

// DayCount is one day's visitors: New made their first visit ever that
// day, Returning had visited before. Day is 2006-01-02 in the stats zone.
type DayCount struct {
	Day            string
	New, Returning int
}

// VisitStats is what the Visitors and Health sections show for a range.
type VisitStats struct {
	Visitors  int // distinct visitors
	Returning int // visitors seen on two or more days in the range
	Views     int
	Days      []DayCount
	// Top lists of distinct visitors (Pages: of views), at most six entries
	// plus "Other". Labels for unknowns are "Unknown".
	Countries, Cities, Sources, Devices, Systems, Browsers, Pages []Count
	Errors                                                        int
	LatestError                                                   string
}

// topN is how many entries a top list keeps before "Other".
const topN = 6

// VisitStats aggregates the visits with from <= at < to. Days are grouped in loc.
func (s *Store) VisitStats(ctx context.Context, from, to time.Time, loc *time.Location) (VisitStats, error) {
	var st VisitStats
	// Each visitor's first visit ever, so "new" means new to the site.
	first := map[string]int64{}
	rows, err := s.db.QueryContext(ctx, `SELECT visitor, MIN(at) FROM visits GROUP BY visitor`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var who string
		var at int64
		if err := rows.Scan(&who, &at); err != nil {
			rows.Close()
			return st, err
		}
		first[who] = at
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}

	rows, err = s.db.QueryContext(ctx,
		`SELECT at, visitor, path, referrer, country, city, device, os, browser FROM visits WHERE at >= ? AND at < ? ORDER BY at`,
		from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return st, err
	}
	defer rows.Close()
	type visitorDays struct {
		days                                                    map[string]bool
		firstPath, firstRef, country, city, device, os, browser string
	}
	visitors := map[string]*visitorDays{}
	dayVisitors := map[string]map[string]bool{} // day → visitors seen
	pages := map[string]int{}
	var earliest *time.Time // the first visit in the range
	for rows.Next() {
		var at int64
		var v Visit
		if err := rows.Scan(&at, &v.Visitor, &v.Path, &v.Referrer, &v.Country, &v.City, &v.Device, &v.OS, &v.Browser); err != nil {
			return st, err
		}
		if earliest == nil { // rows come in time order
			t := time.UnixMilli(at)
			earliest = &t
		}
		st.Views++
		pages[v.Path]++
		day := time.UnixMilli(at).In(loc).Format("2006-01-02")
		vd := visitors[v.Visitor]
		if vd == nil {
			vd = &visitorDays{days: map[string]bool{}, firstPath: v.Path, firstRef: v.Referrer}
			visitors[v.Visitor] = vd
		}
		vd.days[day] = true
		// The first non-empty value wins: a later view rarely knows more.
		if vd.country == "" {
			vd.country, vd.city = v.Country, v.City
		}
		if vd.device == "" {
			vd.device = v.Device
		}
		if vd.os == "" {
			vd.os = v.OS
		}
		if vd.browser == "" {
			vd.browser = v.Browser
		}
		if dayVisitors[day] == nil {
			dayVisitors[day] = map[string]bool{}
		}
		dayVisitors[day][v.Visitor] = true
	}
	if err := rows.Err(); err != nil {
		return st, err
	}

	st.Visitors = len(visitors)
	countries, cities, sources, devices, systems, browsers := map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}, map[string]int{}
	for _, vd := range visitors {
		if len(vd.days) >= 2 {
			st.Returning++
		}
		countries[orUnknown(vd.country)]++
		cities[orUnknown(vd.city)]++
		sources[sourceOf(vd.firstPath, vd.firstRef)]++
		devices[orUnknown(vd.device)]++
		systems[orUnknown(vd.os)]++
		browsers[orUnknown(vd.browser)]++
	}
	// Every day of the range gets a row, quiet ones at zero, so the chart
	// shows the gaps. "All" (a zero from) starts at the first visit's day.
	var earliestAt time.Time
	if earliest != nil {
		earliestAt = *earliest
	}
	for _, day := range dayRange(from, to, earliestAt, loc) {
		dc := DayCount{Day: day}
		t, err := time.ParseInLocation("2006-01-02", day, loc)
		if err != nil {
			return st, err
		}
		dayStart := t.UnixMilli()
		for w := range dayVisitors[day] {
			if first[w] >= dayStart {
				dc.New++
			} else {
				dc.Returning++
			}
		}
		st.Days = append(st.Days, dc)
	}
	st.Countries, st.Cities, st.Sources = top(countries), top(cities), top(sources)
	st.Devices, st.Systems, st.Browsers, st.Pages = top(devices), top(systems), top(browsers), top(pages)

	var latest sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), (SELECT message FROM browser_errors WHERE at >= ? AND at < ? ORDER BY at DESC LIMIT 1) FROM browser_errors WHERE at >= ? AND at < ?`,
		from.UnixMilli(), to.UnixMilli(), from.UnixMilli(), to.UnixMilli()).Scan(&st.Errors, &latest); err != nil {
		return st, err
	}
	st.LatestError = latest.String
	return st, nil
}

// dayRange lists the days (2006-01-02, in loc) from from's up to but not
// including to. A zero from starts at earliest's day instead, and with no
// earliest either there are no days.
func dayRange(from, to, earliest time.Time, loc *time.Location) []string {
	start := from
	if start.IsZero() {
		if earliest.IsZero() {
			return nil
		}
		start = earliest
	}
	var days []string
	y, m, d := start.In(loc).Date()
	for t := time.Date(y, m, d, 0, 0, 0, 0, loc); t.Before(to); t = t.AddDate(0, 0, 1) {
		days = append(days, t.Format("2006-01-02"))
	}
	return days
}

func orUnknown(s string) string {
	if s == "" {
		return "Unknown"
	}
	return s
}

// sourceOf names where a visitor came from, by their first page view in
// the range: a game link, a tagged link, a known site, or nothing (a typed
// URL or a chat app, which pass no referrer).
func sourceOf(path, ref string) string {
	switch {
	case strings.HasPrefix(ref, "ref:"):
		return "Link tagged " + strings.TrimPrefix(ref, "ref:")
	case strings.Contains(ref, "instagram.com"):
		return "Instagram"
	case strings.Contains(ref, "facebook.com"):
		return "Facebook"
	case strings.Contains(ref, "google.") || strings.Contains(ref, "bing.com") || strings.Contains(ref, "duckduckgo.com"):
		return "Search"
	case ref != "":
		return ref
	case path == "/game":
		return "A game invite link"
	}
	return "Direct or a chat app"
}

// top sorts counts by size (ties by label, ignoring case) and folds
// everything past topN into "Other".
func top(m map[string]int) []Count {
	var out []Count
	for label, n := range m {
		out = append(out, Count{label, n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].N != out[j].N {
			return out[i].N > out[j].N
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	if len(out) > topN {
		other := 0
		for _, c := range out[topN:] {
			other += c.N
		}
		out = append(out[:topN], Count{"Other", other})
	}
	return out
}

// Funnel counts visitors who got each step further, for games created in
// the range. Moved, Finished and Again count only guests who also visited
// in the range, so each step is at most the one before. Moved: white made
// ply 0 or black ply 1. Finished: the game ended with a result (not
// aborted, expired or retired). Again: finished two or more games. Games:
// all games created in the range.
type Funnel struct {
	Visited, Opened, Moved, Finished, Again, Games int
}

const finishedReasons = `('checkmate','stalemate','fifty_moves','repetition','insufficient_material','timeout','timeout_vs_insufficient','resignation')`

func (s *Store) FunnelStats(ctx context.Context, from, to time.Time) (Funnel, error) {
	var f Funnel
	a, b := from.UnixMilli(), to.UnixMilli()
	// Later steps count only guests who also visited in the range, so the
	// funnel never rises.
	const seen = `(SELECT DISTINCT visitor FROM visits WHERE at >= ? AND at < ?)`
	for _, q := range []struct {
		dst  *int
		sql  string
		args []any
	}{
		{&f.Visited, `SELECT COUNT(DISTINCT visitor) FROM visits WHERE at >= ? AND at < ?`, []any{a, b}},
		{&f.Opened, `SELECT COUNT(DISTINCT visitor) FROM visits WHERE at >= ? AND at < ? AND path IN ('/game', '/play', '/practice')`, []any{a, b}},
		{&f.Moved, `SELECT COUNT(*) FROM (
			SELECT substr(white, 1, 12) g FROM games WHERE created_at >= ? AND created_at < ? AND EXISTS (SELECT 1 FROM turns WHERE game = code AND ply = 0)
			UNION
			SELECT substr(black, 1, 12) FROM games WHERE created_at >= ? AND created_at < ? AND black IS NOT NULL AND EXISTS (SELECT 1 FROM turns WHERE game = code AND ply = 1)) WHERE g IN ` + seen, []any{a, b, a, b, a, b}},
		{&f.Finished, `SELECT COUNT(*) FROM (
			SELECT substr(white, 1, 12) g FROM games WHERE created_at >= ? AND created_at < ? AND result IN ` + finishedReasons + `
			UNION
			SELECT substr(black, 1, 12) FROM games WHERE created_at >= ? AND created_at < ? AND black IS NOT NULL AND result IN ` + finishedReasons + `) WHERE g IN ` + seen, []any{a, b, a, b, a, b}},
		{&f.Again, `SELECT COUNT(*) FROM (SELECT g FROM (
			SELECT substr(white, 1, 12) g FROM games WHERE created_at >= ? AND created_at < ? AND result IN ` + finishedReasons + `
			UNION ALL
			SELECT substr(black, 1, 12) FROM games WHERE created_at >= ? AND created_at < ? AND black IS NOT NULL AND result IN ` + finishedReasons + `) WHERE g IN ` + seen + ` GROUP BY g HAVING COUNT(*) >= 2)`, []any{a, b, a, b, a, b}},
		{&f.Games, `SELECT COUNT(*) FROM games WHERE created_at >= ? AND created_at < ?`, []any{a, b}},
	} {
		if err := s.db.QueryRowContext(ctx, q.sql, q.args...).Scan(q.dst); err != nil {
			return f, err
		}
	}
	return f, nil
}
