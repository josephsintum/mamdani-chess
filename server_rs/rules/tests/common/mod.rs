//! Helpers shared by the rules tests, mirroring `rules/helpers_test.go`.

#![allow(dead_code, clippy::unwrap_used, clippy::expect_used)]

use rules::{Event, Move, Position, ScriptedDice, Square};

/// Parses `fen` and places the Mamdani and the open potholes rolled by
/// White and Black.
pub fn setup(
    fen: &str,
    mamdani: Option<Square>,
    white_hole: Option<Square>,
    black_hole: Option<Square>,
) -> Position {
    Position::from_fen(fen)
        .unwrap()
        .with_mamdani(mamdani)
        .with_potholes(white_hole, black_hole)
}

pub fn dice(rolls: &[u8]) -> ScriptedDice {
    ScriptedDice::from_values(rolls).unwrap()
}

pub fn mv(uci: &str) -> Move {
    uci.parse().unwrap()
}

pub fn legal(p: &Position, uci: &str) -> bool {
    p.is_legal(mv(uci))
}

/// Plays one turn and panics if the move is illegal or the dice script is
/// not used up exactly.
pub fn apply(p: &Position, uci: &str, rolls: &[u8]) -> (Position, Vec<Event>) {
    let mut d = dice(rolls);
    let out = p
        .apply(mv(uci), &mut d)
        .unwrap_or_else(|e| panic!("{uci}: {e}"));
    assert!(!d.ran_out(), "{uci}: dice script too short");
    assert_eq!(d.left(), 0, "{uci}: dice left over");
    out
}

pub fn kinds(ev: &[Event]) -> Vec<&'static str> {
    ev.iter().map(Event::kind).collect()
}

/// The first event of the given kind.
pub fn find(ev: &[Event], kind: &str) -> Option<Event> {
    ev.iter().copied().find(|e| e.kind() == kind)
}
