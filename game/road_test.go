package game

import (
	"testing"

	"mamdani-chess/rules"
	"mamdani-chess/store"
)

// A countdown close comes before the roll; a cap close comes after it.
func TestRoadCountsClosesBySource(t *testing.T) {
	var r road
	pos := rules.NewGame().Pos
	r.add([]rules.Event{
		{Kind: rules.Moved}, {Kind: rules.PotholeClosed, Square: 1}, {Kind: rules.Repaired, Square: 2},
		{Kind: rules.RolledPothole, Roll: 4}, {Kind: rules.Target, Square: 3}, {Kind: rules.Reroll, Square: 3},
		{Kind: rules.Target, Square: 4}, {Kind: rules.SavingRoll, Square: 4, Piece: rules.NewPiece(rules.Black, rules.Pawn), Roll: 2},
		{Kind: rules.Fell, Square: 4, Piece: rules.NewPiece(rules.Black, rules.Pawn)},
		{Kind: rules.PotholeClosed, Square: 5}, {Kind: rules.PotholeOpened, Square: 4},
	}, &pos)
	r.add([]rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 2}, {Kind: rules.Target, Square: 7}, {Kind: rules.PotholeReset, Square: 7}}, &pos)
	r.add([]rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 2}, {Kind: rules.Target, Square: 8}, {Kind: rules.Repaired, Square: 8}}, &pos)
	r.add([]rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole, Roll: 6}, {Kind: rules.Target, Square: 9}, {Kind: rules.Fell, Square: 9, Piece: rules.MamdaniPiece}, {Kind: rules.PotholeOpened, Square: 9}}, &pos)
	got := r.stats(Resignation)
	want := store.GameStats{Moves: 4, Opened: 2, Reset: 1, ClosedRounds: 1, ClosedCap: 1, Repaired: 2, Fell: 2, SavingRolls: 1, BlackLost: 1, OpenHist: [6]int{4}}
	if got != want {
		t.Fatalf("stats\n got %+v\nwant %+v", got, want)
	}
	// The Mamdani fell by its event, whatever the position says.
	if v, want := r.view(), (StatsJSON{SavingRolls: 1, Repaired: 2, MamdaniFell: true}); v != want {
		t.Errorf("view %+v, want %+v", v, want)
	}
}

// A mate is by a roll when the final turn rolled the dice; an earlier
// turn's roll doesn't count.
func TestRoadMateByRoll(t *testing.T) {
	pos := rules.NewGame().Pos
	moved := []rules.Event{{Kind: rules.Moved}}
	rolled := []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole}, {Kind: rules.Target}, {Kind: rules.PotholeOpened}}
	var byMove, byRoll road
	byMove.add(rolled, &pos)
	byMove.add(moved, &pos)
	byRoll.add(rolled, &pos)
	if byMove.stats(rules.Checkmate).MateByRoll || !byRoll.stats(rules.Checkmate).MateByRoll || byRoll.stats(Resignation).MateByRoll {
		t.Fatal("only a checkmate whose final turn rolled is a mate by a roll")
	}
}

// StatsOf replays a saved game's dice, and a restored game carries the
// same stats on.
func TestRoadStatsSurviveRestore(t *testing.T) {
	turns := []store.Turn{
		{Ply: 0, Move: "e2e4", Dice: []int{2, 4, 4}},    // even: a pothole opens on d4
		{Ply: 1, Move: "e7e5", Dice: []int{1}},          // odd: nothing
		{Ply: 2, Move: "g1f3", Dice: []int{2, 5, 5, 2}}, // even: e5, where the Mamdani has a clear line: the saving roll (2) fails, so Black's pawn falls
	}
	// d4 stays open throughout, and e5 opens on the third turn.
	want := store.GameStats{Moves: 3, Opened: 2, Fell: 1, SavingRolls: 1, BlackLost: 1, OpenHist: [6]int{0, 2, 1}}
	got, err := StatsOf(turns, store.Result{Reason: "resignation", Winner: "white"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StatsOf\n got %+v\nwant %+v", got, want)
	}
	// Restore builds the same accumulator.
	hub := NewHub(odd{}, nil)
	sg := store.SavedGame{Game: store.Game{Code: "ABCDEF", White: "w", Black: "b"}, Turns: turns}
	if n := hub.Restore([]store.SavedGame{sg}); n != 1 {
		t.Fatalf("restored %d", n)
	}
	rg, _ := hub.Get("ABCDEF")
	var restored store.GameStats
	rg.do(func() {
		restored = rg.road.stats(Resignation)
	})
	if restored != want {
		t.Fatalf("after restore\n got %+v\nwant %+v", restored, want)
	}
}
