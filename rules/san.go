package rules

import "strings"

var sanLetters = map[Kind]string{Knight: "N", Bishop: "B", Rook: "R", Queen: "Q", King: "K"}

// SAN returns m in standard algebraic notation for the log: "e4", "Nbd2",
// "exd5", "e8=Q", "O-O", "Qh5+". A Mamdani move is "M" plus its square:
// "Mb5". "#" marks checkmate: a mating move ends the game with no pothole
// roll, so it is final. "+" marks check before the pothole roll. m must be
// legal in p.
func (p *Position) SAN(m Move) string {
	var b strings.Builder
	pc := p.Board[m.From]
	switch {
	case m.From == p.Mamdani:
		b.WriteByte('M')
		b.WriteString(m.To.String())
	case pc.Kind() == King && m.To.File()-m.From.File() == 2:
		b.WriteString("O-O")
	case pc.Kind() == King && m.From.File()-m.To.File() == 2:
		b.WriteString("O-O-O")
	default:
		capture := p.Board[m.To] != NoPiece || (pc.Kind() == Pawn && m.From.File() != m.To.File())
		if pc.Kind() == Pawn {
			if capture {
				b.WriteByte(byte('a' + m.From.File()))
			}
		} else {
			b.WriteString(sanLetters[pc.Kind()])
			b.WriteString(p.disambiguate(m))
		}
		if capture {
			b.WriteByte('x')
		}
		b.WriteString(m.To.String())
		if m.Promo != NoKind {
			b.WriteByte('=')
			b.WriteString(sanLetters[m.Promo])
		}
	}
	q := *p
	q.play(m, nil)
	switch {
	case q.Mated():
		b.WriteByte('#')
	case q.InCheck(q.Turn):
		b.WriteByte('+')
	}
	return b.String()
}

// disambiguate returns the file, rank or square needed to tell m apart from
// other legal moves of the same piece kind to the same square.
func (p *Position) disambiguate(m Move) string {
	kind := p.Board[m.From].Kind()
	var sameFile, sameRank, other bool
	for _, o := range p.LegalMoves() {
		if o.To != m.To || o.From == m.From || p.Board[o.From].Kind() != kind {
			continue
		}
		other = true
		sameFile = sameFile || o.From.File() == m.From.File()
		sameRank = sameRank || o.From.Rank() == m.From.Rank()
	}
	switch {
	case !other:
		return ""
	case !sameFile:
		return m.From.String()[:1]
	case !sameRank:
		return m.From.String()[1:]
	}
	return m.From.String()
}
