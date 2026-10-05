//! What a viewer sees: the `state` event's JSON. The field names and
//! meanings are the frontend's contract (`web/src/lib/game.ts`).

use std::fmt::Write;

use serde::Serialize;

use super::Code;
use rules::{Event, Move, Occupant};

/// One role's picture of the game, sent after every change. It is the whole
/// game, so a reconnecting browser needs nothing else.
#[derive(Clone, Debug, Serialize)]
pub struct View {
    pub code: Code,
    pub status: Status,
    /// "white", "black" or "spectator".
    pub you: &'static str,
    /// Index 0 = a1; "" or a piece code such as "wP".
    pub board: Vec<&'static str>,
    /// The Mamdani's square, "" once it has fallen.
    pub mamdani: &'static str,
    pub potholes: Vec<PotholeView>,
    /// "white" or "black".
    pub turn: &'static str,
    pub check: bool,
    /// Filled only for the player to move.
    pub legal: Vec<MoveView>,
    /// What happened on the latest turn, in order.
    pub last: Vec<EventView>,
    pub log: Vec<LogEntry>,
    /// Piece codes each side has lost to potholes.
    pub lost: Lost,
    pub stats: Stats,
    pub result: Option<ResultView>,
    /// Turns played; a move must quote it.
    pub seq: usize,
}

#[derive(Clone, Copy, Debug, PartialEq, Eq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum Status {
    /// Black's seat is still empty.
    Waiting,
    Playing,
    Over,
}

/// An open pothole and the colour that rolled it.
#[derive(Clone, Debug, Serialize)]
pub struct PotholeView {
    pub sq: &'static str,
    pub by: &'static str,
}

/// A move on the wire. `promo` is "q", "r", "b" or "n", or absent.
#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
pub struct MoveView {
    pub from: &'static str,
    pub to: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub promo: Option<&'static str>,
}

impl From<Move> for MoveView {
    fn from(m: Move) -> MoveView {
        MoveView {
            from: m.from.name(),
            to: m.to.name(),
            promo: m.promo.map(rules::Promo::uci),
        }
    }
}

/// One step of a turn. Only the fields that apply to `kind` are present.
#[derive(Clone, Debug, Default, PartialEq, Eq, Serialize)]
pub struct EventView {
    pub kind: &'static str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sq: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub from: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub to: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub promo: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub piece: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub color: Option<&'static str>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub roll: Option<u8>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub saved: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<&'static str>,
}

impl From<&Event> for EventView {
    fn from(e: &Event) -> EventView {
        let base = EventView {
            kind: e.kind(),
            ..EventView::default()
        };
        match *e {
            Event::Moved {
                mv,
                occupant,
                color,
            } => {
                let m = MoveView::from(mv);
                EventView {
                    from: Some(m.from),
                    to: Some(m.to),
                    promo: m.promo,
                    piece: Some(occupant.code()),
                    color: Some(color.name()),
                    ..base
                }
            }
            Event::Captured { sq, piece } => EventView {
                sq: Some(sq.name()),
                piece: Some(piece.code()),
                ..base
            },
            Event::Fell { sq, occupant } => EventView {
                sq: Some(sq.name()),
                piece: Some(occupant.code()),
                ..base
            },
            Event::PotholeClosed { sq } | Event::Repaired { sq } | Event::Target { sq } => {
                EventView {
                    sq: Some(sq.name()),
                    ..base
                }
            }
            Event::RolledPothole { roll, color } => EventView {
                roll: Some(roll.get()),
                color: Some(color.name()),
                ..base
            },
            Event::Reroll { sq, reason } => EventView {
                sq: Some(sq.name()),
                reason: Some(reason.as_str()),
                ..base
            },
            Event::SavingRoll {
                sq,
                occupant,
                roll,
                saved,
                color,
            } => EventView {
                sq: Some(sq.name()),
                piece: Some(occupant.code()),
                roll: Some(roll.get()),
                saved: Some(saved),
                color: Some(color.name()),
                ..base
            },
            Event::PotholeOpened { sq, color } => EventView {
                sq: Some(sq.name()),
                color: Some(color.name()),
                ..base
            },
            Event::NoPothole => base,
        }
    }
}

/// One turn in the move log: the move in algebraic notation and what the
/// dice did, e.g. `{"e4", "white", "d8 4 → c3"}`.
#[derive(Clone, Debug, PartialEq, Eq, Serialize)]
pub struct LogEntry {
    pub san: String,
    pub color: &'static str,
    pub dice: String,
}

#[derive(Clone, Debug, Default, Serialize)]
pub struct Lost {
    pub white: Vec<&'static str>,
    pub black: Vec<&'static str>,
}

/// What the dice and the Mamdani have done this game.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Stats {
    pub saving_rolls: u32,
    pub saved: u32,
    /// Potholes the Mamdani fixed.
    pub repaired: u32,
    pub mamdani_fell: bool,
}

/// How the game ended. No winner means a draw.
#[derive(Clone, Debug, Serialize)]
pub struct ResultView {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub winner: Option<&'static str>,
    pub draw: bool,
    pub reason: &'static str,
}

/// Sums up what the dice did on a turn, e.g. "d8 4 → d3 · save 5 ✓".
pub(super) fn describe(events: &[Event]) -> String {
    let mut parts: Vec<String> = Vec::new();
    let mut rolled = false;
    for e in events {
        match *e {
            Event::Repaired { sq } => match parts.last_mut() {
                Some(last) if rolled => last.push_str(" repaired"),
                _ => parts.push(format!("repairs {sq}")),
            },
            Event::RolledPothole { roll, .. } => {
                rolled = true;
                parts.push(format!("d8 {roll}"));
            }
            Event::Target { sq } => {
                if let Some(last) = parts.last_mut() {
                    let _ = write!(last, " → {sq}");
                }
            }
            Event::Reroll { reason, .. } => {
                if let Some(last) = parts.last_mut() {
                    let _ = write!(last, " (re-roll: {})", reason.as_str());
                }
            }
            Event::SavingRoll { roll, saved, .. } => {
                let mark = if saved { "✓" } else { "✗" };
                parts.push(format!("save {roll} {mark}"));
            }
            Event::Fell { occupant, .. } => parts.push(format!("{} falls", occupant.code())),
            Event::NoPothole => parts.push("no pothole".to_owned()),
            _ => {}
        }
    }
    parts.join(" · ")
}

/// The piece that fell, if `e` is a piece falling: for the `lost` lists.
pub(super) fn fallen_piece(e: Event) -> Option<rules::Piece> {
    match e {
        Event::Fell {
            occupant: Occupant::Piece(p),
            ..
        } => Some(p),
        _ => None,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use rules::{Color, D8, Kind, Piece, Promo, RerollReason, Square};

    fn json(e: Event) -> String {
        serde_json::to_string(&EventView::from(&e)).unwrap()
    }

    fn roll(v: u8) -> D8 {
        D8::new(v).unwrap()
    }

    #[test]
    fn rolled_pothole_carries_roll_and_color_only() {
        let e = Event::RolledPothole {
            roll: roll(3),
            color: Color::Black,
        };
        assert_eq!(
            json(e),
            r#"{"kind":"rolled_pothole","color":"black","roll":3}"#
        );
    }

    #[test]
    fn target_carries_square_only() {
        let e = Event::Target { sq: Square::A1 };
        assert_eq!(json(e), r#"{"kind":"target","sq":"a1"}"#);
    }

    #[test]
    fn failed_saving_roll_still_sends_saved_false() {
        let e = Event::SavingRoll {
            sq: Square::D2,
            occupant: Occupant::Piece(Piece::new(Color::White, Kind::Pawn)),
            roll: roll(6),
            saved: false,
            color: Color::White,
        };
        assert_eq!(
            json(e),
            r#"{"kind":"saving_roll","sq":"d2","piece":"wP","color":"white","roll":6,"saved":false}"#
        );
    }

    #[test]
    fn fallen_mamdani_is_coded_m() {
        let e = Event::Fell {
            sq: Square::A5,
            occupant: Occupant::Mamdani,
        };
        assert_eq!(json(e), r#"{"kind":"fell","sq":"a5","piece":"M"}"#);
    }

    #[test]
    fn promotion_move_carries_promo_letter() {
        let e = Event::Moved {
            mv: Move {
                from: Square::E7,
                to: Square::E8,
                promo: Some(Promo::Queen),
            },
            occupant: Occupant::Piece(Piece::new(Color::White, Kind::Pawn)),
            color: Color::White,
        };
        assert_eq!(
            json(e),
            r#"{"kind":"moved","from":"e7","to":"e8","promo":"q","piece":"wP","color":"white"}"#
        );
    }

    #[test]
    fn no_pothole_is_kind_only() {
        assert_eq!(json(Event::NoPothole), r#"{"kind":"no_pothole"}"#);
    }

    fn moved() -> Event {
        Event::Moved {
            mv: Move::new(Square::E2, Square::E4),
            occupant: Occupant::Piece(Piece::new(Color::White, Kind::Pawn)),
            color: Color::White,
        }
    }

    #[test]
    fn describe_reroll_then_save() {
        let ev = [
            moved(),
            Event::RolledPothole {
                roll: roll(4),
                color: Color::White,
            },
            Event::Target { sq: Square::E1 },
            Event::Reroll {
                sq: Square::E1,
                reason: RerollReason::King,
            },
            Event::Target { sq: Square::D2 },
            Event::SavingRoll {
                sq: Square::D2,
                occupant: Occupant::Mamdani,
                roll: roll(5),
                saved: true,
                color: Color::White,
            },
        ];
        assert_eq!(describe(&ev), "d8 4 → e1 (re-roll: king) → d2 · save 5 ✓");
    }

    #[test]
    fn describe_repair_step_then_fall() {
        let ev = [
            moved(),
            Event::Repaired { sq: Square::D4 },
            Event::RolledPothole {
                roll: roll(2),
                color: Color::White,
            },
            Event::Target { sq: Square::G8 },
            Event::Fell {
                sq: Square::G8,
                occupant: Occupant::Piece(Piece::new(Color::Black, Kind::Knight)),
            },
        ];
        assert_eq!(describe(&ev), "repairs d4 · d8 2 → g8 · bN falls");
    }
}
