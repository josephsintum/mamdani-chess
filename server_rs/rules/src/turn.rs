//! One full turn: move, close, repair, pothole roll, placement, resolution.

use crate::bitboard::between;
use crate::event::Sink;
use crate::{Color, Dice, Event, Move, Occupant, Position, RerollReason, Square};

/// Placement re-rolls allowed after the first try; past them no pothole
/// opens this turn.
pub const MAX_REROLLS: usize = 64;

/// A move that isn't in [`Position::legal_moves`].
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[error("illegal move")]
pub struct IllegalMove;

impl Position {
    /// Plays one full turn and returns the new position and what happened,
    /// in order. A move that checkmates ends the game at once: no pothole
    /// roll follows, so the dice can't undo a mate made on the board.
    ///
    /// # Errors
    ///
    /// [`IllegalMove`] if `m` isn't legal here.
    pub fn apply<D: Dice + ?Sized>(
        &self,
        m: Move,
        dice: &mut D,
    ) -> Result<(Position, Vec<Event>), IllegalMove> {
        if !self.is_legal(m) {
            return Err(IllegalMove);
        }
        let mover = self.turn;
        let mut next = *self;
        let mut ev = Vec::new();
        next.play(m, &mut ev);
        if !next.is_mated() {
            next.roll_pothole(mover, dice, &mut ev);
        }
        Ok((next, ev))
    }

    /// Whether the side to move is checkmated: no legal move, and its king
    /// attacked once its own open pothole is counted as closed. Every move
    /// closes that pothole, so a check it is only holding off can't be
    /// escaped either.
    #[must_use]
    pub fn is_mated(&self) -> bool {
        self.threatened() && self.legal_moves().is_empty()
    }

    /// Whether the side to move's king is attacked, ignoring its own open
    /// pothole.
    pub(crate) fn threatened(&self) -> bool {
        let mut q = *self;
        q.potholes[q.turn.index()] = None;
        q.in_check(q.turn)
    }

    fn roll_pothole<D: Dice + ?Sized>(&mut self, mover: Color, dice: &mut D, ev: &mut Vec<Event>) {
        let roll = dice.d8();
        ev.push(Event::RolledPothole { roll, color: mover });
        if roll.is_odd() {
            return;
        }
        for _ in 0..=MAX_REROLLS {
            let (file, rank) = (dice.d8().get() - 1, dice.d8().get() - 1);
            let Some(sq) = Square::from_coords(file, rank) else {
                unreachable!("two d8 rolls always name a square");
            };
            ev.push(Event::Target { sq });
            if let Some(reason) = self.reroll_reason(sq, mover) {
                ev.push(Event::Reroll { sq, reason });
                continue;
            }
            self.resolve(sq, mover, dice, ev);
            return;
        }
        ev.push(Event::NoPothole);
    }

    /// Why the placement dice must be rolled again for `s`, or `None` if `s`
    /// is a valid target.
    fn reroll_reason(&self, s: Square, mover: Color) -> Option<RerollReason> {
        if self
            .piece_at(s)
            .is_some_and(|p| p.kind() == crate::Kind::King)
        {
            return Some(RerollReason::King);
        }
        if self.is_pothole(s) {
            return Some(RerollReason::Pothole);
        }
        if self.next_to_mamdani(s) {
            return None; // repaired the moment it opens; nothing changes
        }
        // Would the fall leave the roller in check once the hole is gone?
        // While open, the hole blocks the same lines the piece did, so the
        // test is made without it: the roller must not inherit an
        // unanswerable check when their own pothole closes after their next
        // move.
        let mut gone = *self;
        gone.remove(s);
        if gone.in_check(mover) {
            return Some(RerollReason::Exposes);
        }
        // Would the outcome checkmate the next player? A roll never wins.
        let mut opened = gone;
        opened.potholes[mover.index()] = Some(s);
        if opened.is_mated() {
            return Some(RerollReason::Checkmate);
        }
        None
    }

    fn resolve<D: Dice + ?Sized>(
        &mut self,
        s: Square,
        mover: Color,
        dice: &mut D,
        ev: &mut Vec<Event>,
    ) {
        if self.next_to_mamdani(s) {
            ev.push(Event::Repaired { sq: s });
            return;
        }
        if self.mamdani == Some(s) {
            // The Mamdani always gets a saving roll, made by the roller.
            if saving_roll(s, Occupant::Mamdani, mover, dice, ev) {
                return;
            }
            ev.push(Event::Fell {
                sq: s,
                occupant: Occupant::Mamdani,
            });
            self.mamdani = None;
        } else if let Some(piece) = self.piece_at(s) {
            let occupant = Occupant::Piece(piece);
            if self.mamdani_reaches(s) && saving_roll(s, occupant, piece.color(), dice, ev) {
                return;
            }
            ev.push(Event::Fell { sq: s, occupant });
            self.take(s);
            self.halfmove = 0;
            self.castling.remove(crate::position::Castling::lost_at(s));
        }
        self.potholes[mover.index()] = Some(s);
        ev.push(Event::PotholeOpened {
            sq: s,
            color: mover,
        });
    }

    /// Whether the Mamdani has a clear queen line to `s`: same rank, file or
    /// diagonal, nothing in between. That is all a saving roll needs.
    fn mamdani_reaches(&self, s: Square) -> bool {
        self.mamdani
            .filter(|&m| m != s)
            .and_then(|m| between(m, s))
            .is_some_and(|path| (path & self.blocked()).is_empty())
    }
}

/// Rolls to save `occupant` on `sq`; `color` rolls. Odd saves.
fn saving_roll<D: Dice + ?Sized>(
    sq: Square,
    occupant: Occupant,
    color: Color,
    dice: &mut D,
    ev: &mut impl Sink,
) -> bool {
    let roll = dice.d8();
    let saved = roll.is_odd();
    ev.push(Event::SavingRoll {
        sq,
        occupant,
        roll,
        saved,
        color,
    });
    saved
}
