package rules

import (
	"fmt"
	"slices"
	"strings"
)

// Move is one move. A Mamdani move has From set to the Mamdani's square.
// Castling is the king's two-square move (e1g1). Promo is NoKind unless a
// pawn promotes.
type Move struct {
	From, To Square
	Promo    Kind
}

// promoLetters are the UCI promotion letters, one per Kind from Knight to
// Queen.
const promoLetters = "nbrq"

// String returns the move in UCI form: "e2e4", "e7e8q".
func (m Move) String() string {
	s := m.From.String() + m.To.String()
	if Knight <= m.Promo && m.Promo <= Queen {
		i := m.Promo - Knight
		s += promoLetters[i : i+1]
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
		i := strings.IndexByte(promoLetters, s[4])
		if i < 0 {
			return Move{}, fmt.Errorf("bad promotion in %q", s)
		}
		m.Promo = Knight + Kind(i)
	}
	return m, nil
}

// LegalMoves returns every legal move for the side to move: its own pieces
// and the Mamdani. A move is legal only if the mover's king is safe after
// the move, the countdown and the repair step.
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
// once the countdown and repair steps have run.
func (p *Position) safe(m Move) bool {
	q := *p
	q.play(m, nil)
	return !q.InCheck(p.Turn)
}

// hasLegalMove reports whether the side to move has any legal move. It
// stops at the first one, which is all mate and stalemate checks need.
func (p *Position) hasLegalMove() bool {
	var buf [maxMoves]Move
	return slices.ContainsFunc(p.pseudoMoves(buf[:0]), p.safe)
}

// isLegal reports whether m is one of LegalMoves, without building them all.
func (p *Position) isLegal(m Move) bool {
	var buf [maxMoves]Move
	return slices.Contains(p.pseudoMoves(buf[:0]), m) && p.safe(m)
}

// pseudoMoves appends every pseudo-legal move to moves and returns it.
func (p *Position) pseudoMoves(moves []Move) []Move {
	us := p.Turn
	blocked := p.blocked()
	// A piece may land on an empty square or an enemy piece, never on a
	// pothole or the Mamdani.
	landable := ^(p.byColor[us] | p.potholes())
	if p.Mamdani != NoSquare {
		landable &^= bit(p.Mamdani)
		// The Mamdani moves like a queen but never captures.
		moves = appendTargets(moves, p.Mamdani, queenAttacks(p.Mamdani, blocked)&^blocked)
	}
	for b := p.pieces(us, Knight); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, knightAttacks[from]&landable)
	}
	for b := p.pieces(us, Bishop); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, bishopAttacks(from, blocked)&landable)
	}
	for b := p.pieces(us, Rook); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, rookAttacks(from, blocked)&landable)
	}
	for b := p.pieces(us, Queen); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, queenAttacks(from, blocked)&landable)
	}
	for b := p.pieces(us, King); b != 0; {
		from := b.pop()
		moves = appendTargets(moves, from, kingAttacks[from]&landable)
		moves = p.castlingMoves(moves, from, blocked)
	}
	for b := p.pieces(us, Pawn); b != 0; {
		moves = p.pawnMoves(moves, b.pop(), blocked)
	}
	return moves
}

// appendTargets appends a move from the square from to each square in
// targets.
func appendTargets(moves []Move, from Square, targets Bitboard) []Move {
	for targets != 0 {
		moves = append(moves, Move{From: from, To: targets.pop()})
	}
	return moves
}

func pawnDir(c Color) int {
	if c == White {
		return 1
	}
	return -1
}

func (p *Position) pawnMoves(moves []Move, from Square, blocked Bitboard) []Move {
	c := p.Turn
	dir := pawnDir(c)
	if one := from.Offset(0, dir); one != NoSquare && !blocked.Has(one) {
		moves = appendPawnMove(moves, from, one)
		startRank := 1
		if c == Black {
			startRank = 6
		}
		if two := one.Offset(0, dir); from.Rank() == startRank && !blocked.Has(two) {
			moves = appendPawnMove(moves, from, two)
		}
	}
	enemyPawn := NewPiece(c.Other(), Pawn)
	for t := pawnAttacks[c][from]; t != 0; {
		to := t.pop()
		capture := p.byColor[c.Other()].Has(to)
		// En passant. A pothole on the target square cancels it.
		enPassant := to == p.EP && !blocked.Has(to) && p.Board[to.Offset(0, -dir)] == enemyPawn
		if capture || enPassant {
			moves = appendPawnMove(moves, from, to)
		}
	}
	return moves
}

var promoKinds = [...]Kind{Queen, Rook, Bishop, Knight}

// appendPawnMove appends a pawn move, as the four promotions on the last rank.
func appendPawnMove(moves []Move, from, to Square) []Move {
	if to.Rank() == 0 || to.Rank() == 7 {
		for _, k := range promoKinds {
			moves = append(moves, Move{From: from, To: to, Promo: k})
		}
		return moves
	}
	return append(moves, Move{From: from, To: to})
}

func (p *Position) castlingMoves(moves []Move, k Square, blocked Bitboard) []Move {
	c, them := p.Turn, p.Turn.Other()
	home, ks, qs := E1, WhiteKingside, WhiteQueenside
	if c == Black {
		home, ks, qs = E8, BlackKingside, BlackQueenside
	}
	if k != home || p.Castling&(ks|qs) == 0 || p.attackedWith(k, them, blocked) {
		return moves
	}
	rook := NewPiece(c, Rook)
	// Every square between king and rook must be clear of pieces, the
	// Mamdani and potholes; the king may not pass through or land on an
	// attacked square.
	if p.Castling&ks != 0 && p.Board[k+3] == rook && blocked&(bit(k+1)|bit(k+2)) == 0 &&
		!p.attackedWith(k+1, them, blocked) && !p.attackedWith(k+2, them, blocked) {
		moves = append(moves, Move{From: k, To: k + 2})
	}
	if p.Castling&qs != 0 && p.Board[k-4] == rook && blocked&(bit(k-1)|bit(k-2)|bit(k-3)) == 0 &&
		!p.attackedWith(k-1, them, blocked) && !p.attackedWith(k-2, them, blocked) {
		moves = append(moves, Move{From: k, To: k - 2})
	}
	return moves
}
