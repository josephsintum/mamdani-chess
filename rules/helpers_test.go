package rules

import (
	"slices"
	"testing"
)

// setup parses fen and places the Mamdani (NoSquare for none) and the open
// potholes, oldest first.
func setup(t *testing.T, fen string, mamdani Square, holes ...Hole) Position {
	t.Helper()
	p, err := ParseFEN(fen)
	if err != nil {
		t.Fatal(err)
	}
	p.Mamdani = mamdani
	for i, h := range holes {
		h.Seq = uint32(i + 1)
		p.Potholes[i] = h
	}
	return p
}

// hole is an open pothole on s rolled by c with left rounds to go.
func hole(s Square, c Color, left int8) Hole { return Hole{Sq: s, By: c, Left: left} }

// open returns the open potholes, oldest first.
func open(p Position) []Hole {
	var hs []Hole
	for _, h := range p.Potholes {
		if h.Sq != NoSquare {
			hs = append(hs, h)
		}
	}
	slices.SortFunc(hs, func(a, b Hole) int { return int(a.Seq) - int(b.Seq) })
	return hs
}

// openSquares returns the open potholes' squares, oldest first.
func openSquares(p Position) []Square {
	var ss []Square
	for _, h := range open(p) {
		ss = append(ss, h.Sq)
	}
	return ss
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
