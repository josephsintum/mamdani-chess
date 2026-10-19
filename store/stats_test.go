package store

import (
	"reflect"
	"testing"
	"time"
)

var newYork = must1(time.LoadLocation("America/New_York"))

func must1[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// day returns the instant at hour o'clock in New York on day d of October 2026.
func day(d, hour int) time.Time { return time.Date(2026, 10, d, hour, 0, 0, 0, newYork) }

func TestDayRange(t *testing.T) {
	for _, c := range []struct {
		name               string
		from, to, earliest time.Time
		want               []string
	}{
		{"a range", day(1, 0), day(4, 0), time.Time{}, []string{"2026-10-01", "2026-10-02", "2026-10-03"}},
		// Across the autumn clock change each day is still one row.
		{"across DST", time.Date(2026, 10, 31, 0, 0, 0, 0, newYork), time.Date(2026, 11, 3, 0, 0, 0, 0, newYork), time.Time{},
			[]string{"2026-10-31", "2026-11-01", "2026-11-02"}},
		// All time starts at the earliest row's day, in the stats zone:
		// 03:00 UTC on Oct 3 is still Oct 2 in New York.
		{"all time", time.Time{}, day(4, 0), time.Date(2026, 10, 3, 3, 0, 0, 0, time.UTC), []string{"2026-10-02", "2026-10-03"}},
		{"all time, nothing yet", time.Time{}, day(4, 0), time.Time{}, nil},
	} {
		if got := dayRange(c.from, c.to, c.earliest, newYork); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
}
