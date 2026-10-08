package game

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"mamdani-chess/rules"
)

// The browser plays the dice from web/src/lib/dice-timing.json; the server
// must pause the next clock by the same numbers, or a clock starts mid-roll.
func TestDiceTimingMatchesTheBrowser(t *testing.T) {
	raw, err := os.ReadFile("../web/src/lib/dice-timing.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		MoveMs     int            `json:"moveMs"`
		MarginMs   int            `json:"marginMs"`
		MinPauseMs int            `json:"minPauseMs"`
		PlayMs     map[string]int `json:"playMs"`
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	ms := func(n int) time.Duration { return time.Duration(n) * time.Millisecond }
	if MoveTime != ms(table.MoveMs) || PauseMargin != ms(table.MarginMs) || ResolveDelay != ms(table.MinPauseMs) {
		t.Errorf("move %v margin %v min %v, the browser has %d, %d, %d ms", MoveTime, PauseMargin, ResolveDelay, table.MoveMs, table.MarginMs, table.MinPauseMs)
	}
	samples := map[string]rules.Event{
		"rolled_pothole_odd":  {Kind: rules.RolledPothole, Roll: 3},
		"rolled_pothole_even": {Kind: rules.RolledPothole, Roll: 6},
		"target":              {Kind: rules.Target},
		"reroll":              {Kind: rules.Reroll},
		"saving_roll":         {Kind: rules.SavingRoll},
		"fell":                {Kind: rules.Fell},
		"pothole_opened":      {Kind: rules.PotholeOpened},
		"pothole_reset":       {Kind: rules.PotholeReset},
		"repaired":            {Kind: rules.Repaired},
		"pothole_closed":      {Kind: rules.PotholeClosed},
		"no_pothole":          {Kind: rules.NoPothole},
		"other":               {Kind: rules.Captured},
	}
	if len(table.PlayMs) != len(samples) {
		t.Errorf("the browser times %d kinds of step, the server %d", len(table.PlayMs), len(samples))
	}
	for name, e := range samples {
		want, ok := table.PlayMs[name]
		if !ok {
			t.Errorf("the browser has no time for %s", name)
			continue
		}
		if got := playTime(e); got != ms(want) {
			t.Errorf("%s: the server plays %v, the browser %d ms", name, got, want)
		}
	}
}
