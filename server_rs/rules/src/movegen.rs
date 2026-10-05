//! Moves: generation, legality and playing one on the board.

use std::fmt;
use std::str::FromStr;

use arrayvec::ArrayVec;

use crate::bitboard::{
    Bitboard, adjacent, bishop_attacks, king_attacks, knight_attacks, pawn_attacks, queen_attacks,
    rook_attacks,
};
use crate::event::Sink;
use crate::position::Castling;
use crate::{Color, Event, Kind, Occupant, Piece, Position, Promo, Square};

/// One move. A Mamdani move has `from` set to the Mamdani's square.
/// Castling is the king's two-square move (e1g1).
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub struct Move {
    pub from: Square,
    pub to: Square,
    pub promo: Option<Promo>,
}

impl Move {
    #[must_use]
    pub const fn new(from: Square, to: Square) -> Move {
        Move {
            from,
            to,
            promo: None,
        }
    }
}

/// UCI form: "e2e4", "e7e8q".
impl fmt::Display for Move {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}{}", self.from, self.to)?;
        if let Some(p) = self.promo {
            f.write_str(p.uci())?;
        }
        Ok(())
    }
}

/// A string that isn't a UCI move.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("bad move {0:?}")]
pub struct ParseMoveError(pub String);

impl FromStr for Move {
    type Err = ParseMoveError;

    fn from_str(s: &str) -> Result<Move, ParseMoveError> {
        let fail = || ParseMoveError(s.to_owned());
        if !s.is_ascii() || !(4..=5).contains(&s.len()) {
            return Err(fail());
        }
        let from = s[0..2].parse().map_err(|_| fail())?;
        let to = s[2..4].parse().map_err(|_| fail())?;
        let promo = match &s[4..] {
            "" => None,
            p => Some(Promo::from_uci(p).ok_or_else(fail)?),
        };
        Ok(Move { from, to, promo })
    }
}

fn push_targets(moves: &mut MoveList, from: Square, targets: Bitboard) {
    moves.extend(targets.into_iter().map(|to| Move::new(from, to)));
}

/// Room for every move in any reachable position: chess tops out at 218
/// legal moves and the Mamdani adds at most 27, with headroom for
/// pseudo-legal moves that the legality filter drops.
pub type MoveList = ArrayVec<Move, 320>;

impl Position {
    /// Whether any piece of colour `by` attacks `s`. Potholes and the
    /// Mamdani block sliders exactly like pieces do.
    #[must_use]
    pub fn attacked(&self, s: Square, by: Color) -> bool {
        let them = self.colored(by);
        let blocked = self.blocked();
        let straight = self.kind(Kind::Rook) | self.kind(Kind::Queen);
        let diagonal = self.kind(Kind::Bishop) | self.kind(Kind::Queen);
        let hits = (pawn_attacks(by.other(), s) & self.kind(Kind::Pawn))
            | (knight_attacks(s) & self.kind(Kind::Knight))
            | (king_attacks(s) & self.kind(Kind::King))
            | (rook_attacks(s, blocked) & straight)
            | (bishop_attacks(s, blocked) & diagonal);
        !(hits & them).is_empty()
    }

    /// Whether `c`'s king is attacked.
    #[must_use]
    pub fn in_check(&self, c: Color) -> bool {
        self.king(c).is_some_and(|k| self.attacked(k, c.other()))
    }

    /// Every legal move for the side to move: its own pieces and the
    /// Mamdani. A move is legal only if the mover's king is safe after the
    /// move, the close step and the repair step. Those last two change which
    /// squares block, so legality is tested by playing the move on a copy
    /// rather than with pin masks.
    #[must_use]
    pub fn legal_moves(&self) -> MoveList {
        let mut moves = self.pseudo_moves();
        moves.retain(|m| {
            let mut q = *self;
            q.play(*m, &mut ());
            !q.in_check(self.turn)
        });
        moves
    }

    /// The position after `m`'s move, close and repair steps, before any
    /// pothole roll. `m` must be legal; perft and look-ahead use this.
    #[must_use]
    pub fn after_move(&self, m: Move) -> Position {
        let mut q = *self;
        q.play(m, &mut ());
        q
    }

    /// Whether `m` is one of [`Position::legal_moves`].
    #[must_use]
    pub fn is_legal(&self, m: Move) -> bool {
        self.legal_moves().contains(&m)
    }

    fn pseudo_moves(&self) -> MoveList {
        let us = self.turn;
        let blocked = self.blocked();
        // A piece may land on an empty square or an enemy piece, never on a
        // pothole or the Mamdani.
        let landable =
            !(self.colored(us) | self.potholes_bb() | Bitboard::from_option(self.mamdani));
        let mut moves = MoveList::new();

        if let Some(m) = self.mamdani {
            // The Mamdani moves like a queen but never captures.
            push_targets(&mut moves, m, queen_attacks(m, blocked) & !blocked);
        }
        for from in self.pieces(us, Kind::Knight) {
            push_targets(&mut moves, from, knight_attacks(from) & landable);
        }
        for from in self.pieces(us, Kind::Bishop) {
            push_targets(&mut moves, from, bishop_attacks(from, blocked) & landable);
        }
        for from in self.pieces(us, Kind::Rook) {
            push_targets(&mut moves, from, rook_attacks(from, blocked) & landable);
        }
        for from in self.pieces(us, Kind::Queen) {
            push_targets(&mut moves, from, queen_attacks(from, blocked) & landable);
        }
        for from in self.pieces(us, Kind::King) {
            push_targets(&mut moves, from, king_attacks(from) & landable);
            self.castling_moves(&mut moves, from);
        }
        for from in self.pieces(us, Kind::Pawn) {
            self.pawn_moves(&mut moves, from, blocked);
        }
        moves
    }

    fn pawn_moves(&self, moves: &mut MoveList, from: Square, blocked: Bitboard) {
        let us = self.turn;
        let dir = us.pawn_dir();
        let mut add = |to: Square| {
            if to.rank() == 0 || to.rank() == 7 {
                for p in Promo::ALL {
                    moves.push(Move {
                        from,
                        to,
                        promo: Some(p),
                    });
                }
            } else {
                moves.push(Move::new(from, to));
            }
        };
        if let Some(one) = from.offset(0, dir)
            && !blocked.contains(one)
        {
            add(one);
            let start_rank = if us == Color::White { 1 } else { 6 };
            if from.rank() == start_rank
                && let Some(two) = one.offset(0, dir)
                && !blocked.contains(two)
            {
                add(two);
            }
        }
        let enemy_pawn = Piece::new(us.other(), Kind::Pawn);
        for to in pawn_attacks(us, from) {
            let capture = self.colored(us.other()).contains(to);
            // En passant. A pothole on the target square cancels it.
            let en_passant = self.ep == Some(to)
                && !blocked.contains(to)
                && to.offset(0, -dir).and_then(|s| self.piece_at(s)) == Some(enemy_pawn);
            if capture || en_passant {
                add(to);
            }
        }
    }

    fn castling_moves(&self, moves: &mut MoveList, king: Square) {
        let us = self.turn;
        let home = if us == Color::White {
            Square::E1
        } else {
            Square::E8
        };
        if king != home || self.attacked(king, us.other()) {
            return;
        }
        let rook = Some(Piece::new(us, Kind::Rook));
        // Every square between king and rook must be clear of pieces, the
        // Mamdani and potholes; the king may not pass through or land on an
        // attacked square.
        let clear = |df: i8| king.offset(df, 0).is_some_and(|s| !self.is_blocked(s));
        let safe = |df: i8| {
            king.offset(df, 0)
                .is_some_and(|s| !self.attacked(s, us.other()))
        };
        let rook_at = |df: i8| king.offset(df, 0).and_then(|s| self.piece_at(s)) == rook;
        if self.castling.contains(Castling::kingside(us))
            && rook_at(3)
            && clear(1)
            && clear(2)
            && safe(1)
            && safe(2)
            && let Some(to) = king.offset(2, 0)
        {
            moves.push(Move::new(king, to));
        }
        if self.castling.contains(Castling::queenside(us))
            && rook_at(-4)
            && clear(-1)
            && clear(-2)
            && clear(-3)
            && safe(-1)
            && safe(-2)
            && let Some(to) = king.offset(-2, 0)
        {
            moves.push(Move::new(king, to));
        }
    }

    /// Makes pseudo-legal move `m` for the side to move, then runs the close
    /// and repair steps and passes the turn.
    pub(crate) fn play(&mut self, m: Move, ev: &mut impl Sink) {
        let mover = self.turn;
        if self.mamdani == Some(m.from) {
            ev.push(Event::Moved {
                mv: m,
                occupant: Occupant::Mamdani,
                color: mover,
            });
            self.mamdani = Some(m.to);
            self.ep = None;
            self.halfmove += 1; // Mamdani moves never reset the 50-move count
        } else {
            self.move_piece(m, ev);
        }
        if mover == Color::Black {
            self.fullmove += 1;
        }
        // Close: the mover's own pothole from their previous turn.
        if let Some(s) = self.potholes[mover.index()].take() {
            ev.push(Event::PotholeClosed { sq: s });
        }
        self.repair(ev);
        self.turn = mover.other();
    }

    fn move_piece(&mut self, m: Move, ev: &mut impl Sink) {
        let Some(pc) = self.take(m.from) else {
            return; // not pseudo-legal; callers never do this
        };
        let c = pc.color();
        ev.push(Event::Moved {
            mv: m,
            occupant: Occupant::Piece(pc),
            color: c,
        });

        let is_pawn = pc.kind() == Kind::Pawn;
        let cap_sq = if is_pawn && self.ep == Some(m.to) && self.piece_at(m.to).is_none() {
            m.to.offset(0, -c.pawn_dir()).unwrap_or(m.to)
        } else {
            m.to
        };
        let captured = self.take(cap_sq);
        if let Some(piece) = captured {
            ev.push(Event::Captured { sq: cap_sq, piece });
        }
        let landed = m.promo.map_or(pc, |p| Piece::new(c, p.kind()));
        self.put(m.to, landed);

        if pc.kind() == Kind::King && m.to.file().abs_diff(m.from.file()) == 2 {
            let (rook_from, rook_to) = if m.to.file() > m.from.file() {
                (m.from.offset(3, 0), m.from.offset(1, 0))
            } else {
                (m.from.offset(-4, 0), m.from.offset(-1, 0))
            };
            if let (Some(rf), Some(rt)) = (rook_from, rook_to)
                && let Some(rook) = self.take(rf)
            {
                self.put(rt, rook);
            }
        }

        self.castling.remove(Castling::lost_at(m.from));
        self.castling.remove(Castling::lost_at(m.to));

        self.ep = None;
        if is_pawn && m.to.rank().abs_diff(m.from.rank()) == 2 {
            self.ep = m.from.offset(0, c.pawn_dir());
        }

        if is_pawn || captured.is_some() {
            self.halfmove = 0;
        } else {
            self.halfmove += 1;
        }
    }

    /// Removes every open pothole next to the Mamdani.
    fn repair(&mut self, ev: &mut impl Sink) {
        let Some(m) = self.mamdani else {
            return;
        };
        for hole in &mut self.potholes {
            if let Some(s) = *hole
                && adjacent(s, m)
            {
                *hole = None;
                ev.push(Event::Repaired { sq: s });
            }
        }
    }
}
