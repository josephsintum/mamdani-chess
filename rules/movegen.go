package rules

import (
	"fmt"
	"slices"
)

// Move is one move. A Mamdani move has From set to the Mamdani's square.
// Castling is the king's two-square move (e1g1). Promo is NoKind unless a
// pawn promotes.
type Move struct {
	From, To Square
	Promo    Kind
}

var promoLetters = map[Kind]byte{Knight: 'n', Bishop: 'b', Rook: 'r', Queen: 'q'}

// String returns the move in UCI form: "e2e4", "e7e8q".
func (m Move) String() string {
	s := m.From.String() + m.To.String()
	if c, ok := promoLetters[m.Promo]; ok {
		s += string(c)
	}
	return s
}

// ParseMove reads UCI form.
func ParseMove(s string) (Move, error) {
	if len(s) != 4 && len(s) != 5 {
		return Move{}, fmt.Errorf("bad move %q", s)
	}
	from, err := ParseSquare(s[0:2])
	if err != nil {
		return Move{}, err
	}
	to, err := ParseSquare(s[2:4])
	if err != nil {
		return Move{}, err
	}
	m := Move{From: from, To: to}
	if len(s) == 5 {
		for k, c := range promoLetters {
			if c == s[4] {
				m.Promo = k
			}
		}
		if m.Promo == NoKind {
			return Move{}, fmt.Errorf("bad promotion in %q", s)
		}
	}
	return m, nil
}

// LegalMoves returns every legal move for the side to move: its own pieces
// and the Mamdani. A move is legal only if the mover's king is safe after
// the move, the close step and the repair step.
func (p *Position) LegalMoves() []Move {
	var buf [maxMoves]Move
	pseudo := p.pseudoMoves(buf[:0])
	legal := make([]Move, 0, len(pseudo))
	for _, m := range pseudo {
		if p.safe(m) {
			legal = append(legal, m)
		}
	}
	return legal
}

// maxMoves is room for every pseudo-legal move in any reachable position:
// chess tops out at 218 legal moves and the Mamdani adds at most 27. If a
// position ever had more, append would just move to the heap.
const maxMoves = 320

// safe reports whether pseudo-legal m leaves the mover's king unattacked
// once the close and repair steps have run.
func (p *Position) safe(m Move) bool {
	q := *p
	q.play(m, nil)
	return !q.InCheck(p.Turn)
}

// hasLegalMove reports whether the side to move has any legal move. It
// stops at the first one, which is all mate and stalemate checks need.
func (p *Position) hasLegalMove() bool {
	var buf [maxMoves]Move
	for _, m := range p.pseudoMoves(buf[:0]) {
		if p.safe(m) {
			return true
		}
	}
	return false
}

// isLegal reports whether m is one of LegalMoves, without building them all.
func (p *Position) isLegal(m Move) bool {
	var buf [maxMoves]Move
	return slices.Contains(p.pseudoMoves(buf[:0]), m) && p.safe(m)
}

// canLand reports whether a piece of color c may finish a move on s.
func (p *Position) canLand(s Square, c Color) bool {
	if p.IsPothole(s) || s == p.Mamdani {
		return false
	}
	pc := p.Board[s]
	return pc == NoPiece || pc.Color() != c
}

// pseudoMoves appends every pseudo-legal move to moves and returns it.
func (p *Position) pseudoMoves(moves []Move) []Move {
	c := p.Turn
	if p.Mamdani != NoSquare {
		for _, d := range queenDirs {
			for t := p.Mamdani.Offset(d[0], d[1]); t != NoSquare && !p.Blocked(t); t = t.Offset(d[0], d[1]) {
				moves = append(moves, Move{From: p.Mamdani, To: t})
			}
		}
	}
	for s := range Square(64) {
		pc := p.Board[s]
		if pc == NoPiece || pc.Color() != c {
			continue
		}
		switch pc.Kind() {
		case Pawn:
			moves = p.pawnMoves(moves, s)
		case Knight:
			moves = p.stepMoves(moves, s, knightJumps)
		case Bishop:
			moves = p.slideMoves(moves, s, bishopDirs)
		case Rook:
			moves = p.slideMoves(moves, s, rookDirs)
		case Queen:
			moves = p.slideMoves(moves, s, queenDirs)
		case King:
			moves = p.stepMoves(moves, s, queenDirs)
			moves = p.castlingMoves(moves, s)
		}
	}
	return moves
}

func (p *Position) stepMoves(moves []Move, from Square, steps [][2]int) []Move {
	c := p.Board[from].Color()
	for _, d := range steps {
		if t := from.Offset(d[0], d[1]); t != NoSquare && p.canLand(t, c) {
			moves = append(moves, Move{From: from, To: t})
		}
	}
	return moves
}

func (p *Position) slideMoves(moves []Move, from Square, dirs [][2]int) []Move {
	c := p.Board[from].Color()
	for _, d := range dirs {
		for t := from.Offset(d[0], d[1]); t != NoSquare; t = t.Offset(d[0], d[1]) {
			if p.canLand(t, c) {
				moves = append(moves, Move{From: from, To: t})
			}
			if p.Blocked(t) {
				break
			}
		}
	}
	return moves
}

func pawnDir(c Color) int {
	if c == White {
		return 1
	}
	return -1
}

func (p *Position) pawnMoves(moves []Move, from Square) []Move {
	c := p.Board[from].Color()
	dir := pawnDir(c)
	add := func(to Square) {
		if to.Rank() == 0 || to.Rank() == 7 {
			for _, k := range []Kind{Queen, Rook, Bishop, Knight} {
				moves = append(moves, Move{From: from, To: to, Promo: k})
			}
			return
		}
		moves = append(moves, Move{From: from, To: to})
	}
	if one := from.Offset(0, dir); one != NoSquare && !p.Blocked(one) {
		add(one)
		startRank := 1
		if c == Black {
			startRank = 6
		}
		if two := one.Offset(0, dir); from.Rank() == startRank && !p.Blocked(two) {
			add(two)
		}
	}
	for _, df := range []int{-1, 1} {
		t := from.Offset(df, dir)
		if t == NoSquare {
			continue
		}
		if pc := p.Board[t]; pc != NoPiece && pc.Color() != c {
			add(t)
		} else if t == p.EP && !p.Blocked(t) && p.Board[t.Offset(0, -dir)] == NewPiece(c.Other(), Pawn) {
			// En passant. A pothole on the target square cancels it.
			add(t)
		}
	}
	return moves
}

func (p *Position) castlingMoves(moves []Move, k Square) []Move {
	c := p.Board[k].Color()
	home, ks, qs := E1, WhiteKingside, WhiteQueenside
	if c == Black {
		home, ks, qs = E8, BlackKingside, BlackQueenside
	}
	if k != home || p.Attacked(k, c.Other()) {
		return moves
	}
	rook := NewPiece(c, Rook)
	// Every square between king and rook must be clear of pieces, the
	// Mamdani and potholes; the king may not pass through or land on an
	// attacked square.
	if p.Castling&ks != 0 && p.Board[k.Offset(3, 0)] == rook &&
		!p.Blocked(k.Offset(1, 0)) && !p.Blocked(k.Offset(2, 0)) &&
		!p.Attacked(k.Offset(1, 0), c.Other()) && !p.Attacked(k.Offset(2, 0), c.Other()) {
		moves = append(moves, Move{From: k, To: k.Offset(2, 0)})
	}
	if p.Castling&qs != 0 && p.Board[k.Offset(-4, 0)] == rook &&
		!p.Blocked(k.Offset(-1, 0)) && !p.Blocked(k.Offset(-2, 0)) && !p.Blocked(k.Offset(-3, 0)) &&
		!p.Attacked(k.Offset(-1, 0), c.Other()) && !p.Attacked(k.Offset(-2, 0), c.Other()) {
		moves = append(moves, Move{From: k, To: k.Offset(-2, 0)})
	}
	return moves
}
