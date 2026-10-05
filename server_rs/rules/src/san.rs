//! Standard algebraic notation for the move log.

use std::fmt::Write;

use crate::{Kind, Move, Position};

impl Position {
    /// `m` in standard algebraic notation: "e4", "Nbd2", "exd5", "e8=Q",
    /// "O-O", "Qh5+". A Mamdani move is "M" plus its square: "Mb5". "#" marks
    /// checkmate: a mating move ends the game with no pothole roll, so it is
    /// final. "+" marks check before the pothole roll. `m` must be legal.
    #[must_use]
    pub fn san(&self, m: Move) -> String {
        let mut s = String::with_capacity(8);
        let kind = self.piece_at(m.from).map(crate::Piece::kind);
        let df = i16::from(m.to.file()) - i16::from(m.from.file());
        match kind {
            _ if self.mamdani == Some(m.from) => {
                s.push('M');
                s.push_str(m.to.name());
            }
            Some(Kind::King) if df == 2 => s.push_str("O-O"),
            Some(Kind::King) if df == -2 => s.push_str("O-O-O"),
            _ => {
                let is_pawn = kind == Some(Kind::Pawn);
                let capture = self.piece_at(m.to).is_some() || (is_pawn && df != 0);
                if is_pawn {
                    if capture {
                        s.push(char::from(b'a' + m.from.file()));
                    }
                } else if let Some(k) = kind {
                    s.push(k.letter());
                    s.push_str(&self.disambiguate(m, k));
                }
                if capture {
                    s.push('x');
                }
                s.push_str(m.to.name());
                if let Some(p) = m.promo {
                    let _ = write!(s, "={}", p.kind().letter());
                }
            }
        }
        let mut q = *self;
        q.play(m, &mut ());
        if q.is_mated() {
            s.push('#');
        } else if q.in_check(q.turn) {
            s.push('+');
        }
        s
    }

    /// The file, rank or square needed to tell `m` apart from other legal
    /// moves of the same piece kind to the same square.
    fn disambiguate(&self, m: Move, kind: Kind) -> String {
        let (mut other, mut same_file, mut same_rank) = (false, false, false);
        for o in self.legal_moves() {
            if o.to != m.to
                || o.from == m.from
                || self.mamdani == Some(o.from)
                || self.piece_at(o.from).map(crate::Piece::kind) != Some(kind)
            {
                continue;
            }
            other = true;
            same_file |= o.from.file() == m.from.file();
            same_rank |= o.from.rank() == m.from.rank();
        }
        let name = m.from.name();
        match (other, same_file, same_rank) {
            (false, _, _) => String::new(),
            (true, false, _) => name[..1].to_owned(),
            (true, true, false) => name[1..].to_owned(),
            (true, true, true) => name.to_owned(),
        }
    }
}
