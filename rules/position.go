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
	Board [64]Piece
	// Mamdani is the Mamdani's square, or NoSquare once it has fallen
	// (and in plain-chess positions such as perft).
	Mamdani Square
	// Potholes holds the open pothole each color rolled, or NoSquare.
	// Each player has at most one: theirs closes on their next move,
	// before they can roll again.
	Potholes [2]Square
	Turn     Color
	Castling Castling
	EP       Square // en passant target square, or NoSquare
	Halfmove int    // plies since the last pawn move, capture or fall
	Fullmove int
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
	return s != NoSquare && (p.Potholes[White] == s || p.Potholes[Black] == s)
}

// Blocked reports whether s stops a slider: a piece, the Mamdani or a pothole.
func (p *Position) Blocked(s Square) bool {
	return p.Board[s] != NoPiece || s == p.Mamdani || p.IsPothole(s)
}

// King returns c's king square, or NoSquare if it has none (only in
// hand-built test positions).
func (p *Position) King(c Color) Square {
	k := NewPiece(c, King)
	for s := Square(0); s < 64; s++ {
		if p.Board[s] == k {
			return s
		}
	}
	return NoSquare
}

// Key identifies a position for repetition: pieces, Mamdani, open potholes,
// side to move, castling rights and en passant square.
type Key struct {
	Board    [64]Piece
	Mamdani  Square
	Potholes [2]Square
	Turn     Color
	Castling Castling
	EP       Square
}

func (p *Position) Key() Key {
	return Key{p.Board, p.Mamdani, p.Potholes, p.Turn, p.Castling, p.EP}
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
	p := Position{Mamdani: NoSquare, Potholes: [2]Square{NoSquare, NoSquare}, EP: NoSquare}
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
		for j := 0; j < len(row); j++ {
			c := row[j]
			if c >= '1' && c <= '8' {
				f += int(c - '0')
				continue
			}
			pc, ok := fenPieces[c]
			if !ok || f > 7 {
				return p, fmt.Errorf("fen %q: bad rank %q", fen, row)
			}
			p.Board[r*8+f] = pc
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
