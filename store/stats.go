package store

import "time"

// Count is one labelled count, a row of a list or a histogram's column.
type Count struct {
	Label string
	N     int
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

// finishedReasons are the results of a game that was played to the end
// (not aborted, expired or retired).
const finishedReasons = `('checkmate','stalemate','fifty_moves','repetition','insufficient_material','timeout','timeout_vs_insufficient','resignation')`
