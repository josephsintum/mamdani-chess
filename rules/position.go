package rules

import (
	"fmt"
	"strconv"
	"strings"
)

// Castling is a set of castling rights.
type Castling uint8

const (
	WhiteKingside Castling = 1 << iota
	WhiteQueenside
	BlackKingside
	BlackQueenside
)

// Position is everything needed to generate moves and resolve a turn.
type Position struct {
	// Board is what stands on each square. Read it freely, but change
	// pieces only through put and take, which keep the bitboards in step.
	Board [64]Piece
	// Mamdani is the Mamdani's square, or NoSquare once it has fallen
	// (and in plain-chess positions such as perft).
	Mamdani Square
	// Potholes are the open potholes, in no particular order. A slot
	// with Sq NoSquare is free.
	Potholes [HoleCap]Hole
	Turn     Color
	Castling Castling
	EP       Square // en passant target square, or NoSquare
	Halfmove int    // plies since the last pawn move, capture or fall
	Fullmove int

	// The same pieces as Board, as sets: by color and by kind (indexed by
	// Kind, so byKind[NoKind] stays empty).
	byColor [2]Bitboard
	byKind  [King + 1]Bitboard
}

// put places pc on s, replacing whatever was there.
func (p *Position) put(s Square, pc Piece) {
	p.take(s)
	p.Board[s] = pc
	p.byColor[pc.Color()] |= bit(s)
	p.byKind[pc.Kind()] |= bit(s)
}

// take removes and returns the piece on s, or NoPiece.
func (p *Position) take(s Square) Piece {
	pc := p.Board[s]
	if pc == NoPiece {
		return NoPiece
	}
	p.Board[s] = NoPiece
	p.byColor[pc.Color()] &^= bit(s)
	p.byKind[pc.Kind()] &^= bit(s)
	return pc
}

// pieces returns c's pieces of kind k.
func (p *Position) pieces(c Color, k Kind) Bitboard { return p.byColor[c] & p.byKind[k] }

// HoleRounds is how many of its roller's moves a pothole stays open for:
// it closes when the roller finishes their third move after opening it.
const HoleRounds = 3

// HoleCap is the most potholes open at once. Opening one more closes the
// oldest first.
const HoleCap = 5

// Hole is one open pothole.
type Hole struct {
	Sq Square // NoSquare for a free slot in Position.Potholes
	By Color  // who rolled it; it counts down on their moves
	// Left is the rounds left: the roller's moves until it closes, 1 to
	// HoleRounds. A hole with 1 left closes when its roller next moves.
	Left int8
	// Seq orders the open holes by opening, so the cap knows which is
	// oldest. A new hole gets one more than the newest still open, so it
	// depends only on the holes on the board, not on the game's history.
	Seq uint32
}

// noHoles returns a pothole list with every slot free.
func noHoles() [HoleCap]Hole {
	var h [HoleCap]Hole
	for i := range h {
		h[i].Sq = NoSquare
	}
	return h
}

// potholes returns the open potholes as a set.
func (p *Position) potholes() Bitboard {
	var b Bitboard
	for _, h := range p.Potholes {
		if h.Sq != NoSquare {
			b |= bit(h.Sq)
		}
	}
	return b
}

// oldestHole returns the index in Potholes of the open hole opened first,
// or -1 if none is open, and how many are open.
func (p *Position) oldestHole() (oldest, open int) {
	oldest = -1
	for i, h := range p.Potholes {
		if h.Sq == NoSquare {
			continue
		}
		open++
		if oldest < 0 || h.Seq < p.Potholes[oldest].Seq {
			oldest = i
		}
	}
	return oldest, open
}

// capVictim returns the index of the hole the cap would close if a new one
// opened now, or -1 if there is room.
func (p *Position) capVictim() int {
	if oldest, open := p.oldestHole(); open >= HoleCap {
		return oldest
	}
	return -1
}

// openHole opens a pothole on s rolled by c, with every round to go. With
// HoleCap already open the oldest closes first, so the events read
// pothole_closed, then pothole_opened.
func (p *Position) openHole(s Square, c Color, ev *[]Event) {
	if i := p.capVictim(); i >= 0 {
		p.closeHole(i, ev)
	}
	var seq uint32
	free := -1
	for i, h := range p.Potholes {
		if h.Sq == NoSquare {
			if free < 0 {
				free = i
			}
		} else if h.Seq > seq {
			seq = h.Seq
		}
	}
	p.Potholes[free] = Hole{Sq: s, By: c, Left: HoleRounds, Seq: seq + 1}
	emit(ev, Event{Kind: PotholeOpened, Square: s, Color: c})
}

// closeHole closes the hole in slot i as it reaches the end of its rounds
// or is pushed out by the cap. Color on the event is the hole's roller.
func (p *Position) closeHole(i int, ev *[]Event) {
	h := p.Potholes[i]
	p.Potholes[i].Sq = NoSquare
	emit(ev, Event{Kind: PotholeClosed, Square: h.Sq, Color: h.By})
}

// blocked returns every square that stops a slider: pieces, the Mamdani
// and potholes.
func (p *Position) blocked() Bitboard {
	b := p.byColor[White] | p.byColor[Black] | p.potholes()
	if p.Mamdani != NoSquare {
		b |= bit(p.Mamdani)
	}
	return b
}

const startFEN = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"

// StartPosition is the standard chess setup with the Mamdani on a5.
func StartPosition() Position {
	p, err := ParseFEN(startFEN)
	if err != nil {
		panic(err)
	}
	p.Mamdani = A5
	return p
}

// IsPothole reports whether s holds an open pothole.
func (p *Position) IsPothole(s Square) bool {
	return p.potholes().Has(s)
}

// Blocked reports whether s stops a slider: a piece, the Mamdani or a pothole.
func (p *Position) Blocked(s Square) bool {
	return p.blocked().Has(s)
}

// King returns c's king square, or NoSquare if it has none (only in
// hand-built test positions).
func (p *Position) King(c Color) Square {
	return p.pieces(c, King).First()
}

// Key identifies a position for repetition: pieces, Mamdani, open potholes
// with their rollers and rounds left, side to move, castling rights and en
// passant square.
type Key struct {
	Board    [64]Piece
	Mamdani  Square
	Potholes [HoleCap]Hole
	Turn     Color
	Castling Castling
	EP       Square
}

// Key returns p's repetition key. Holes are listed in square order, free
// slots last, so the same holes match whichever slots they sit in. Seq is
// left out: it only says which hole is oldest, and that already follows
// from each hole's roller and rounds left, since at most one hole opens
// per move.
func (p *Position) Key() Key {
	holes := noHoles()
	n := 0
	for _, h := range p.Potholes {
		if h.Sq == NoSquare {
			continue
		}
		h.Seq = 0
		i := n
		for ; i > 0 && holes[i-1].Sq > h.Sq; i-- {
			holes[i] = holes[i-1]
		}
		holes[i] = h
		n++
	}
	return Key{p.Board, p.Mamdani, holes, p.Turn, p.Castling, p.EP}
}

var fenPieces = map[byte]Piece{
	'P': NewPiece(White, Pawn), 'N': NewPiece(White, Knight), 'B': NewPiece(White, Bishop),
	'R': NewPiece(White, Rook), 'Q': NewPiece(White, Queen), 'K': NewPiece(White, King),
	'p': NewPiece(Black, Pawn), 'n': NewPiece(Black, Knight), 'b': NewPiece(Black, Bishop),
	'r': NewPiece(Black, Rook), 'q': NewPiece(Black, Queen), 'k': NewPiece(Black, King),
}

// ParseFEN reads standard FEN. The result has no Mamdani and no potholes;
// set those fields directly when a position needs them.
func ParseFEN(fen string) (Position, error) {
	p := Position{Mamdani: NoSquare, Potholes: noHoles(), EP: NoSquare}
	fields := strings.Fields(fen)
	if len(fields) != 6 {
		return p, fmt.Errorf("fen %q: want 6 fields", fen)
	}
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != 8 {
		return p, fmt.Errorf("fen %q: want 8 ranks", fen)
	}
	for i, row := range ranks {
		r, f := 7-i, 0
		for _, c := range []byte(row) {
			if c >= '1' && c <= '8' {
				f += int(c - '0')
				continue
			}
			pc, ok := fenPieces[c]
			if !ok || f > 7 {
				return p, fmt.Errorf("fen %q: bad rank %q", fen, row)
			}
			p.put(Square(r*8+f), pc)
			f++
		}
		if f != 8 {
			return p, fmt.Errorf("fen %q: rank %q is not 8 squares", fen, row)
		}
	}
	switch fields[1] {
	case "w":
		p.Turn = White
	case "b":
		p.Turn = Black
	default:
		return p, fmt.Errorf("fen %q: bad side to move", fen)
	}
	for _, c := range fields[2] {
		switch c {
		case 'K':
			p.Castling |= WhiteKingside
		case 'Q':
			p.Castling |= WhiteQueenside
		case 'k':
			p.Castling |= BlackKingside
		case 'q':
			p.Castling |= BlackQueenside
		case '-':
		default:
			return p, fmt.Errorf("fen %q: bad castling", fen)
		}
	}
	if fields[3] != "-" {
		s, err := ParseSquare(fields[3])
		if err != nil {
			return p, fmt.Errorf("fen %q: %w", fen, err)
		}
		p.EP = s
	}
	var err error
	if p.Halfmove, err = strconv.Atoi(fields[4]); err != nil {
		return p, fmt.Errorf("fen %q: bad halfmove clock", fen)
	}
	if p.Fullmove, err = strconv.Atoi(fields[5]); err != nil {
		return p, fmt.Errorf("fen %q: bad fullmove number", fen)
	}
	return p, nil
}
