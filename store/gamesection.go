package store

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// KindDay is one day's games created by kind, plus practice games started.
type KindDay struct {
	Day                     string
	Quick, Friend, Practice int
}

// Heat is games started by weekday (Monday first) and two-hour block.
type Heat [7][12]int

// GameSection is what the Games, Quick match and The road sections and the
// Health tiles show for a range. Zero-valued where there is nothing.
type GameSection struct {
	Games, GotGoing int // games created in the range; of those, ended with a result
	Days            []KindDay
	// Ends: Checkmate, Mate by a roll, Resignation, Out of time, Draw,
	// Aborted, Nobody came, in that order, zeros dropped.
	Ends []Count
	// Lengths: moves by both sides in games that got going, every bucket kept.
	Lengths                    []Count
	MedianMoves, MedianMinutes int
	OnClock                    int    // ended on time
	WinnerClockLeft            string // median of the winner's clock at the end, "3:48"
	Heat                       Heat
	Friend                     struct {
		Links, Joined, Finished, Rematches int
		MedianJoin                         string
	}
	Quick struct {
		Searches, Matched, GaveUp, Alone int
		Waits                            []Count
		MedianWait, MedianGaveUp         string
	}
	Road struct {
		Games, Opened, Fell, SavingRolls, Saved, Repaired, Reset, MateByRoll, ClosedRounds, ClosedCap, NoFall int
		OpenHist                                                                                              [6]int
		// Decisive games: the winner lost fewer, more or as many pieces to the road as the loser.
		LessWon, MoreWon, Same int
	}
	Reconnects, Restarts int
}

// gameRow is one game in the range with what the section needs.
type gameRow struct {
	kind, result, winner string
	created, joined      time.Time
	hasBlack, rematch    bool
	moves                int       // moves by both sides
	first, ended         time.Time // the first turn and the end; zero when none
	winnerMS             int64     // the winner's clock after the last turn
	hasClock             bool
	stats                *GameStats
}

func isFinished(reason string) bool {
	switch reason {
	case "checkmate", "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout", "timeout_vs_insufficient", "resignation":
		return true
	}
	return false
}

func msTime(n sql.NullInt64) time.Time {
	if !n.Valid {
		return time.Time{}
	}
	return time.UnixMilli(n.Int64)
}

// GameSection aggregates the games created with from <= at < to, the
// quick-match searches that started and the events that happened in it.
// Days and the heat map are in loc.
func (s *Store) GameSection(ctx context.Context, from, to time.Time, loc *time.Location) (GameSection, error) {
	var out GameSection
	a, b := from.UnixMilli(), to.UnixMilli()
	rows, err := s.db.QueryContext(ctx, `
		SELECT g.kind, g.result, g.winner, g.created_at, g.joined_at, g.black IS NOT NULL, g.rematch_of IS NOT NULL, g.ended_at,
		       (SELECT COUNT(*) FROM turns t WHERE t.game = g.code),
		       (SELECT MIN(at) FROM turns t WHERE t.game = g.code),
		       (SELECT white_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       (SELECT black_ms FROM turns t WHERE t.game = g.code ORDER BY ply DESC LIMIT 1),
		       s.moves, s.opened, s.reset, s.closed_rounds, s.closed_cap, s.repaired, s.fell, s.saving_rolls, s.saved, s.white_lost, s.black_lost, s.mate_by_roll, s.open_hist
		FROM games g LEFT JOIN game_stats s ON s.game = g.code
		WHERE g.created_at >= ? AND g.created_at < ? AND g.result IS NOT 'retired' ORDER BY g.created_at`, a, b)
	if err != nil {
		return out, err
	}
	var games []gameRow
	for rows.Next() {
		var r gameRow
		var result, winner, hist sql.NullString
		var created int64
		var joined, ended, first, whiteMS, blackMS sql.NullInt64
		var st [11]sql.NullInt64 // moves, opened, reset, closed_rounds, closed_cap, repaired, fell, saving_rolls, saved, white_lost, black_lost
		var mate sql.NullBool
		if err := rows.Scan(&r.kind, &result, &winner, &created, &joined, &r.hasBlack, &r.rematch, &ended, &r.moves, &first, &whiteMS, &blackMS,
			&st[0], &st[1], &st[2], &st[3], &st[4], &st[5], &st[6], &st[7], &st[8], &st[9], &st[10], &mate, &hist); err != nil {
			rows.Close()
			return out, err
		}
		r.result, r.winner = result.String, winner.String
		r.created, r.joined, r.ended, r.first = time.UnixMilli(created), msTime(joined), msTime(ended), msTime(first)
		switch {
		case r.winner == "white" && whiteMS.Valid:
			r.winnerMS, r.hasClock = whiteMS.Int64, true
		case r.winner == "black" && blackMS.Valid:
			r.winnerMS, r.hasClock = blackMS.Int64, true
		}
		if st[0].Valid {
			h, err := parseHist(hist.String)
			if err != nil {
				rows.Close()
				return out, err
			}
			n := func(i int) int { return int(st[i].Int64) }
			r.stats = &GameStats{Moves: n(0), Opened: n(1), Reset: n(2), ClosedRounds: n(3), ClosedCap: n(4), Repaired: n(5),
				Fell: n(6), SavingRolls: n(7), Saved: n(8), WhiteLost: n(9), BlackLost: n(10), MateByRoll: mate.Bool, OpenHist: h}
			r.moves = r.stats.Moves
		}
		games = append(games, r)
	}
	rows.Close() // one connection: close before the next query
	if err := rows.Err(); err != nil {
		return out, err
	}

	searches, err := s.Searches(ctx, from, to)
	if err != nil {
		return out, err
	}
	type event struct {
		at   time.Time
		kind string
	}
	var events []event
	erows, err := s.db.QueryContext(ctx, `SELECT at, kind FROM events WHERE at >= ? AND at < ? ORDER BY at`, a, b)
	if err != nil {
		return out, err
	}
	for erows.Next() {
		var at int64
		var e event
		if err := erows.Scan(&at, &e.kind); err != nil {
			erows.Close()
			return out, err
		}
		e.at = time.UnixMilli(at)
		events = append(events, e)
	}
	erows.Close()
	if err := erows.Err(); err != nil {
		return out, err
	}

	// Days: every day of the range, quiet ones at zero.
	var earliest time.Time // rows come in time order
	if len(games) > 0 {
		earliest = games[0].created
	}
	for _, e := range events {
		if e.kind == "practice" && (earliest.IsZero() || e.at.Before(earliest)) {
			earliest = e.at
		}
	}
	index := map[string]int{}
	for _, d := range dayRange(from, to, earliest, loc) {
		index[d] = len(out.Days)
		out.Days = append(out.Days, KindDay{Day: d})
	}
	dayOf := func(t time.Time) *KindDay {
		if i, ok := index[t.In(loc).Format("2006-01-02")]; ok {
			return &out.Days[i]
		}
		return nil
	}

	var ends struct{ checkmate, roll, resign, timeout, draw, aborted, expired int }
	var moves, minutes, clocks, joins []int
	lengths := make([]Count, len(lengthBuckets))
	for i, l := range lengthBuckets {
		lengths[i].Label = l.label
	}
	for _, g := range games {
		out.Games++
		if d := dayOf(g.created); d != nil {
			if g.kind == "quick" {
				d.Quick++
			} else {
				d.Friend++
			}
		}
		t := g.created.In(loc)
		out.Heat[(int(t.Weekday())+6)%7][t.Hour()/2]++
		if g.kind == "friend" {
			out.Friend.Links++
			if g.hasBlack {
				out.Friend.Joined++
			}
			if !g.joined.IsZero() {
				joins = append(joins, int(g.joined.Sub(g.created).Milliseconds()))
			}
		}
		if g.rematch {
			out.Friend.Rematches++
		}
		got := isFinished(g.result)
		switch g.result {
		case "checkmate":
			if g.stats != nil && g.stats.MateByRoll {
				ends.roll++
			} else {
				ends.checkmate++
			}
		case "resignation":
			ends.resign++
		case "timeout":
			ends.timeout++
		case "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout_vs_insufficient":
			ends.draw++
		case "aborted":
			ends.aborted++
		case "expired":
			ends.expired++
		}
		if g.kind == "friend" && got {
			out.Friend.Finished++
		}
		if g.result == "timeout" || g.result == "timeout_vs_insufficient" {
			out.OnClock++
		}
		if got {
			out.GotGoing++
			moves = append(moves, g.moves)
			for i, l := range lengthBuckets {
				if g.moves <= l.max {
					lengths[i].N++
					break
				}
			}
			if !g.first.IsZero() && !g.ended.IsZero() {
				minutes = append(minutes, int(g.ended.Sub(g.first)/time.Minute))
			}
			if g.winner != "" && g.hasClock {
				clocks = append(clocks, int(g.winnerMS))
			}
		}
		if g.stats == nil {
			continue
		}
		st, r := g.stats, &out.Road
		r.Games++
		r.Opened += st.Opened
		r.Fell += st.Fell
		r.SavingRolls += st.SavingRolls
		r.Saved += st.Saved
		r.Repaired += st.Repaired
		r.Reset += st.Reset
		r.ClosedRounds += st.ClosedRounds
		r.ClosedCap += st.ClosedCap
		if st.MateByRoll {
			r.MateByRoll++
		}
		if st.Fell == 0 {
			r.NoFall++
		}
		for i, n := range st.OpenHist {
			r.OpenHist[i] += n
		}
		if got && g.winner != "" {
			won, lost := st.WhiteLost, st.BlackLost
			if g.winner == "black" {
				won, lost = lost, won
			}
			switch {
			case won < lost:
				r.LessWon++
			case won > lost:
				r.MoreWon++
			default:
				r.Same++
			}
		}
	}
	for _, e := range []struct {
		label string
		n     int
	}{{"Checkmate", ends.checkmate}, {"Mate by a roll", ends.roll}, {"Resignation", ends.resign}, {"Out of time", ends.timeout}, {"Draw", ends.draw}, {"Aborted", ends.aborted}, {"Nobody came", ends.expired}} {
		if e.n > 0 {
			out.Ends = append(out.Ends, Count{e.label, e.n})
		}
	}
	out.Lengths = lengths
	out.MedianMoves = median(moves)
	out.MedianMinutes = median(minutes)
	out.WinnerClockLeft = clockOrEmpty(median(clocks), len(clocks))
	out.Friend.MedianJoin = clockOrEmpty(median(joins), len(joins))

	// Quick match.
	out.Quick.Waits = []Count{{"under 10s", 0}, {"10–30s", 0}, {"30–60s", 0}, {"1–2 min", 0}, {"2 min+", 0}}
	var waits, gaveUp []int
	for _, sr := range searches {
		out.Quick.Searches++
		if sr.Others == 0 {
			out.Quick.Alone++
		}
		ms := int(sr.EndedAt.Sub(sr.StartedAt).Milliseconds())
		if !sr.Matched {
			out.Quick.GaveUp++
			gaveUp = append(gaveUp, ms)
			continue
		}
		out.Quick.Matched++
		waits = append(waits, ms)
		switch {
		case ms < 10_000:
			out.Quick.Waits[0].N++
		case ms < 30_000:
			out.Quick.Waits[1].N++
		case ms < 60_000:
			out.Quick.Waits[2].N++
		case ms < 120_000:
			out.Quick.Waits[3].N++
		default:
			out.Quick.Waits[4].N++
		}
	}
	out.Quick.MedianWait = clockOrEmpty(median(waits), len(waits))
	out.Quick.MedianGaveUp = clockOrEmpty(median(gaveUp), len(gaveUp))

	for _, e := range events {
		switch e.kind {
		case "practice":
			if d := dayOf(e.at); d != nil {
				d.Practice++
			}
		case "reconnect":
			out.Reconnects++
		case "restart":
			out.Restarts++
		}
	}
	return out, nil
}

// lengthBuckets are the game-length bars: label and the most moves in it.
var lengthBuckets = []struct {
	label string
	max   int
}{{"1–10", 10}, {"11–20", 20}, {"21–30", 30}, {"31–40", 40}, {"41–50", 50}, {"51–60", 60}, {"61–80", 80}, {"81+", int(^uint(0) >> 1)}}

// median is the middle value, or for an even count the mean of the middle
// two rounded half up; 0 for an empty set.
func median(ns []int) int {
	if len(ns) == 0 {
		return 0
	}
	sorted := append([]int(nil), ns...)
	sort.Ints(sorted)
	m := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[m]
	}
	return (sorted[m-1] + sorted[m] + 1) / 2
}

// clock writes milliseconds as m:ss, minutes unbounded.
func clock(ms int64) string {
	s := (ms + 500) / 1000
	if ms < 0 {
		s = 0
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// clockOrEmpty is clock(ms), or "" when the median is of no values.
func clockOrEmpty(ms, count int) string {
	if count == 0 {
		return ""
	}
	return clock(int64(ms))
}
