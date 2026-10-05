//! What happens during a turn, step by step, in the order RULES.md plays it.

use crate::{Color, D8, Move, Piece, Square};

/// Something that can stand on a square and fall into a pothole: a piece,
/// or the Mamdani.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Occupant {
    Piece(Piece),
    Mamdani,
}

impl Occupant {
    /// The wire code: "wP", "bK" ..., or "M" for the Mamdani.
    #[must_use]
    pub const fn code(self) -> &'static str {
        match self {
            Occupant::Piece(p) => p.code(),
            Occupant::Mamdani => "M",
        }
    }
}

/// Why the placement dice were rolled again.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum RerollReason {
    /// Kings never fall.
    King,
    /// There is already a pothole there.
    Pothole,
    /// The fall would leave the roller in check.
    Exposes,
    /// The result would checkmate the next player: a roll never wins.
    Checkmate,
}

impl RerollReason {
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            RerollReason::King => "king",
            RerollReason::Pothole => "pothole",
            RerollReason::Exposes => "exposes",
            RerollReason::Checkmate => "checkmate",
        }
    }
}

/// One step of a turn. Each variant carries exactly the facts that apply to
/// it.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Event {
    /// A move by `color`; `occupant` is the piece or the Mamdani.
    Moved {
        mv: Move,
        occupant: Occupant,
        color: Color,
    },
    Captured {
        sq: Square,
        piece: Piece,
    },
    /// The mover's own pothole from their previous turn.
    PotholeClosed {
        sq: Square,
    },
    /// By the repair step, or a new pothole opening next to the Mamdani.
    Repaired {
        sq: Square,
    },
    /// Odd: nothing happens. Even: a pothole opens.
    RolledPothole {
        roll: D8,
        color: Color,
    },
    /// The square the two placement dice picked.
    Target {
        sq: Square,
    },
    Reroll {
        sq: Square,
        reason: RerollReason,
    },
    /// `color` is who rolls: the piece's owner, or the roller for the
    /// Mamdani.
    SavingRoll {
        sq: Square,
        occupant: Occupant,
        roll: D8,
        saved: bool,
        color: Color,
    },
    Fell {
        sq: Square,
        occupant: Occupant,
    },
    /// `color` rolled it.
    PotholeOpened {
        sq: Square,
        color: Color,
    },
    /// The re-roll cap was reached.
    NoPothole,
}

impl Event {
    /// The wire name: `moved`, `pothole_opened` and so on.
    #[must_use]
    pub const fn kind(&self) -> &'static str {
        match self {
            Event::Moved { .. } => "moved",
            Event::Captured { .. } => "captured",
            Event::PotholeClosed { .. } => "pothole_closed",
            Event::Repaired { .. } => "repaired",
            Event::RolledPothole { .. } => "rolled_pothole",
            Event::Target { .. } => "target",
            Event::Reroll { .. } => "reroll",
            Event::SavingRoll { .. } => "saving_roll",
            Event::Fell { .. } => "fell",
            Event::PotholeOpened { .. } => "pothole_opened",
            Event::NoPothole => "no_pothole",
        }
    }
}

/// Where `play` sends events. `()` throws them away, so perft and legality
/// checks pay nothing for them.
pub(crate) trait Sink {
    fn push(&mut self, e: Event);
}

impl Sink for () {
    fn push(&mut self, _: Event) {}
}

impl Sink for Vec<Event> {
    fn push(&mut self, e: Event) {
        Vec::push(self, e);
    }
}
