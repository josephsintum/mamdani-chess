// Package rules is the Mamdani Chess rules engine.
// It is pure: no I/O, no clock, and no randomness of its own (dice come in
// through the Dice interface). RULES.md is the specification.
package rules

import "fmt"

// Color is a side. The Mamdani has no color.
type Color uint8

const (
	White Color = iota
	Black
)

// Other returns the opposing color.
func (c Color) Other() Color { return c ^ 1 }

func (c Color) String() string {
	if c == White {
		return "white"
	}
	return "black"
}

// Kind is a piece type. Mamdani never appears on Position.Board; it is only
// used to name the Mamdani in events.
type Kind uint8

const (
	NoKind Kind = iota
	Pawn
	Knight
	Bishop
	Rook
	Queen
	King
	Mamdani
)

// Piece packs a Kind and a Color. The zero value is an empty square.
type Piece uint8

const NoPiece Piece = 0

// MamdaniPiece names the Mamdani in events (Fell, SavingRoll).
const MamdaniPiece = Piece(Mamdani)

func NewPiece(c Color, k Kind) Piece { return Piece(k) | Piece(c)<<3 }
func (p Piece) Kind() Kind           { return Kind(p & 7) }
func (p Piece) Color() Color         { return Color(p >> 3) }

// Square is 0..63: a1=0, b1=1, ..., h8=63.
type Square int8

const NoSquare Square = -1

// Named squares.
const (
	A1 Square = iota
	B1
	C1
	D1
	E1
	F1
	G1
	H1
	A2
	B2
	C2
	D2
	E2
	F2
	G2
	H2
	A3
	B3
	C3
	D3
	E3
	F3
	G3
	H3
	A4
	B4
	C4
	D4
	E4
	F4
	G4
	H4
	A5
	B5
	C5
	D5
	E5
	F5
	G5
	H5
	A6
	B6
	C6
	D6
	E6
	F6
	G6
	H6
	A7
	B7
	C7
	D7
	E7
	F7
	G7
	H7
	A8
	B8
	C8
	D8
	E8
	F8
	G8
	H8
)

func (s Square) File() int { return int(s) % 8 }
func (s Square) Rank() int { return int(s) / 8 }

// Offset returns the square df files and dr ranks away, or NoSquare if that
// is off the board.
func (s Square) Offset(df, dr int) Square {
	f, r := s.File()+df, s.Rank()+dr
	if f < 0 || f > 7 || r < 0 || r > 7 {
		return NoSquare
	}
	return Square(r*8 + f)
}

func (s Square) String() string {
	if s == NoSquare {
		return "-"
	}
	return squareNames[s]
}

// squareNames holds every square's name, so naming one never allocates:
// views name a square for every legal move, event and pothole.
var squareNames = func() (names [64]string) {
	for s := range names {
		names[s] = string([]byte{byte('a' + s%8), byte('1' + s/8)})
	}
	return names
}()

// ParseSquare parses "a1".."h8".
func ParseSquare(name string) (Square, error) {
	if len(name) != 2 || name[0] < 'a' || name[0] > 'h' || name[1] < '1' || name[1] > '8' {
		return NoSquare, fmt.Errorf("bad square %q", name)
	}
	return Square(int(name[1]-'1')*8 + int(name[0]-'a')), nil
}

// Sq is ParseSquare for literals; it panics on a bad name.
func Sq(name string) Square {
	s, err := ParseSquare(name)
	if err != nil {
		panic(err)
	}
	return s
}

// adjacent reports whether a and b are different squares that touch,
// including diagonally.
func adjacent(a, b Square) bool {
	df, dr := abs(a.File()-b.File()), abs(a.Rank()-b.Rank())
	return max(df, dr) == 1
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sign(x int) int {
	switch {
	case x > 0:
		return 1
	case x < 0:
		return -1
	}
	return 0
}
