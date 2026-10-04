package rules

import (
	"slices"
	"testing"
)

// setup parses fen and places the Mamdani (NoSquare for none) and the open
// potholes rolled by White and Black (NoSquare for none).
func setup(t *testing.T, fen string, mamdani, whiteHole, blackHole Square) Position {
	t.Helper()
	p, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	p.Mamdani = mamdani
	p.Potholes = [2]Square{whiteHole, blackHole}
	return p
}

func dice(rolls ...int) *ScriptedDice { return &ScriptedDice{Rolls: rolls} }

func mv(t *testing.T, uci string) Move {
	t.Helper()
	m, err := ParseMove(uci)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func legal(t *testing.T, p Position, uci string) bool {
	t.Helper()
	return slices.Contains(p.LegalMoves(), mv(t, uci))
}

// apply plays one turn and fails the test if the move is illegal or the
// dice script is not used up exactly.
func apply(t *testing.T, p Position, uci string, d *ScriptedDice) (Position, []Event) {
	t.Helper()
	next, ev, err := Apply(p, mv(t, uci), d)
	if err != nil {
		t.Fatalf("%s: %v", uci, err)
	}
	if d.Left() != 0 {
		t.Fatalf("%s: %d dice left over", uci, d.Left())
	}
	return next, ev
}

func kinds(ev []Event) []EventKind {
	var ks []EventKind
	for _, e := range ev {
		ks = append(ks, e.Kind)
	}
	return ks
}

func find(ev []Event, k EventKind) (Event, bool) {
	for _, e := range ev {
		if e.Kind == k {
			return e, true
		}
	}
	return Event{}, false
}
