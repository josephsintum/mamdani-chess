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
	got := r.stats(rules.Result{}, nil, [2][]string{nil, {"p"}})
	want := store.GameStats{Moves: 4, Opened: 2, Reset: 1, ClosedRounds: 1, ClosedCap: 1, Repaired: 2, Fell: 2, SavingRolls: 1, BlackLost: 1, OpenHist: [6]int{4}}
	if got != want {
		t.Fatalf("stats\n got %+v\nwant %+v", got, want)
	}
}

func TestRoadMateByRoll(t *testing.T) {
	var r road
	byMove := r.stats(rules.Result{Over: true, Reason: rules.Checkmate}, []rules.Event{{Kind: rules.Moved}}, [2][]string{})
	byRoll := r.stats(rules.Result{Over: true, Reason: rules.Checkmate}, []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole}, {Kind: rules.Target}, {Kind: rules.PotholeOpened}}, [2][]string{})
	resigned := r.stats(rules.Result{Over: true, Reason: Resignation}, []rules.Event{{Kind: rules.Moved}, {Kind: rules.RolledPothole}}, [2][]string{})
	if byMove.MateByRoll || !byRoll.MateByRoll || resigned.MateByRoll {
		t.Fatalf("byMove %v byRoll %v resigned %v", byMove.MateByRoll, byRoll.MateByRoll, resigned.MateByRoll)
	}
}

// StatsOf replays a saved game: the same turns give the same stats as a
// game played live, and a restored game carries them on.
func TestRoadStatsSurviveRestore(t *testing.T) {
	// Play two turns through the rules with fixed dice, both live and by replay.
	turns := []store.Turn{
		{Ply: 0, Move: "e2e4", Dice: []int{2, 4, 4}},    // even: a pothole opens on d4
		{Ply: 1, Move: "e7e5", Dice: []int{1}},          // odd: nothing
		{Ply: 2, Move: "g1f3", Dice: []int{2, 5, 5, 2}}, // even: e5, where the Mamdani has a clear line: the saving roll (2) fails, so Black's pawn falls
	}
	live := &road{}
	g := rules.NewGame()
	for _, t2 := range turns {
		m, _ := rules.ParseMove(t2.Move)
		ev, err := g.Play(m, &rules.ScriptedDice{Rolls: t2.Dice})
		if err != nil {
			t.Fatal(err)
		}
		live.add(ev, &g.Pos)
	}
	want := live.stats(rules.Result{Over: true, Reason: Resignation, Winner: rules.White}, nil, [2][]string{nil, {"p"}})
	got, err := StatsOf(turns, store.Result{Reason: "resignation", Winner: "white"})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("StatsOf\n got %+v\nwant %+v", got, want)
	}
	if got.Moves != 3 || got.Opened < 1 || got.OpenHist[0]+got.OpenHist[1]+got.OpenHist[2] != 3 {
		t.Fatalf("stats look wrong: %+v", got)
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
		restored = rg.road.stats(rules.Result{Over: true, Reason: Resignation, Winner: rules.White}, nil, rg.lost)
	})
	if restored != want {
		t.Fatalf("after restore\n got %+v\nwant %+v", restored, want)
	}
}
