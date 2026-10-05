package game

import (
	"encoding/json"
	"testing"

	"mamdani-chess/rules"
)

func TestEventJSONOmitsFieldsThatDontApply(t *testing.T) {
	cases := []struct {
		e    rules.Event
		want string
	}{
		{rules.Event{Kind: rules.RolledPothole, Roll: 3, Color: rules.Black},
			`{"kind":"rolled_pothole","color":"black","roll":3}`},
		{rules.Event{Kind: rules.Target, Square: rules.A1},
			`{"kind":"target","sq":"a1"}`},
		{rules.Event{Kind: rules.SavingRoll, Square: rules.D2, Piece: rules.NewPiece(rules.White, rules.Pawn), Roll: 6, Saved: false, Color: rules.White},
			`{"kind":"saving_roll","sq":"d2","piece":"wP","color":"white","roll":6,"saved":false}`},
		{rules.Event{Kind: rules.Fell, Square: rules.A5, Piece: rules.MamdaniPiece},
			`{"kind":"fell","sq":"a5","piece":"M"}`},
		{rules.Event{Kind: rules.Moved, Move: rules.Move{From: rules.E7, To: rules.E8, Promo: rules.Queen}, Piece: rules.NewPiece(rules.White, rules.Pawn), Color: rules.White},
			`{"kind":"moved","from":"e7","to":"e8","promo":"q","piece":"wP","color":"white"}`},
		{rules.Event{Kind: rules.NoPothole}, `{"kind":"no_pothole"}`},
		{rules.Event{Kind: rules.PotholeClosed, Square: rules.C4, Color: rules.Black},
			`{"kind":"pothole_closed","sq":"c4","color":"black"}`},
		{rules.Event{Kind: rules.Repaired, Square: rules.C4}, `{"kind":"repaired","sq":"c4"}`},
	}
	for _, c := range cases {
		b, err := json.Marshal(eventJSON(c.e))
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != c.want {
			t.Errorf("got  %s\nwant %s", b, c.want)
		}
	}
}

func TestParseMove(t *testing.T) {
	m, ok := ParseMove(MoveJSON{From: "e7", To: "e8", Promo: "n"})
	if !ok || m != (rules.Move{From: rules.E7, To: rules.E8, Promo: rules.Knight}) {
		t.Errorf("got %v %v", m, ok)
	}
	for _, bad := range []MoveJSON{{From: "e9", To: "e8"}, {From: "e7", To: ""}, {From: "e7", To: "e8", Promo: "k"}} {
		if _, ok := ParseMove(bad); ok {
			t.Errorf("%+v should not parse", bad)
		}
	}
}

func TestDescribe(t *testing.T) {
	ev := []rules.Event{
		{Kind: rules.Moved},
		{Kind: rules.RolledPothole, Roll: 4},
		{Kind: rules.Target, Square: rules.E1},
		{Kind: rules.Reroll, Square: rules.E1, Reason: rules.ReasonKing},
		{Kind: rules.Target, Square: rules.D2},
		{Kind: rules.SavingRoll, Roll: 5, Saved: true},
	}
	if got, want := describe(ev), "d8 4 → e1 (re-roll: king) → d2 · save 5 ✓"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	ev = []rules.Event{
		{Kind: rules.Moved},
		{Kind: rules.Repaired, Square: rules.D4},
		{Kind: rules.RolledPothole, Roll: 2},
		{Kind: rules.Target, Square: rules.G8},
		{Kind: rules.Fell, Piece: rules.NewPiece(rules.Black, rules.Knight)},
	}
	if got, want := describe(ev), "repairs d4 · d8 2 → g8 · bN falls"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestDescribeShowsOnlyTheCapClosing(t *testing.T) {
	// f5 runs out of rounds before the roll: every turn has those, so the
	// log leaves it out. c4 is the cap making room for e4.
	ev := []rules.Event{
		{Kind: rules.Moved},
		{Kind: rules.PotholeClosed, Square: rules.F5, Color: rules.White},
		{Kind: rules.RolledPothole, Roll: 2},
		{Kind: rules.Target, Square: rules.E4},
		{Kind: rules.PotholeClosed, Square: rules.C4, Color: rules.Black},
		{Kind: rules.PotholeOpened, Square: rules.E4, Color: rules.White},
	}
	if got, want := describe(ev), "d8 2 → e4 · c4 closes"; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
