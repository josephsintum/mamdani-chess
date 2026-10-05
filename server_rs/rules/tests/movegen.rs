//! Port of `rules/movegen_test.go`: potholes and the Mamdani in move
//! generation.

mod common;

use common::{apply, legal, setup};
use rules::{Position, Square};

#[test]
fn sliders_stop_at_potholes() {
    let p = setup(
        "4k3/8/8/8/8/8/8/R3K3 w - - 0 1",
        None,
        Some(Square::D1),
        None,
    );
    for uci in ["a1b1", "a1c1", "a1a8"] {
        assert!(legal(&p, uci), "{uci} should be legal");
    }
    assert!(!legal(&p, "a1d1"), "rook landed on a pothole");
}

#[test]
fn knights_jump_potholes_but_cannot_land_on_them() {
    let p = setup(
        "4k3/8/8/8/8/8/8/1N2K3 w - - 0 1",
        None,
        Some(Square::C3),
        Some(Square::C2),
    );
    assert!(legal(&p, "b1d2") && legal(&p, "b1a3"));
    assert!(!legal(&p, "b1c3"), "knight landed on a pothole");
}

#[test]
fn pothole_in_front_of_pawn_blocks_both_steps() {
    let p = setup(
        "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1",
        None,
        None,
        Some(Square::E3),
    );
    assert!(!legal(&p, "e2e3") && !legal(&p, "e2e4"));
}

#[test]
fn pothole_two_ahead_blocks_only_double_step() {
    let p = setup(
        "4k3/8/8/8/8/8/4P3/4K3 w - - 0 1",
        None,
        None,
        Some(Square::E4),
    );
    assert!(legal(&p, "e2e3") && !legal(&p, "e2e4"));
}

#[test]
fn pothole_blocks_check() {
    let p = setup(
        "4k3/8/8/8/8/8/8/4K2r w - - 0 1",
        None,
        None,
        Some(Square::F1),
    );
    assert!(!p.in_check(rules::Color::White));
}

const CASTLE_FEN: &str = "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1";

#[test]
fn both_castles_legal_on_open_board() {
    let p = setup(CASTLE_FEN, None, None, None);
    assert!(legal(&p, "e1g1") && legal(&p, "e1c1"));
}

#[test]
fn pothole_on_b1_blocks_only_queenside_castling() {
    let p = setup(CASTLE_FEN, None, None, Some(Square::B1));
    assert!(!legal(&p, "e1c1") && legal(&p, "e1g1"));
}

#[test]
fn pothole_on_f1_blocks_kingside_castling() {
    let p = setup(CASTLE_FEN, None, None, Some(Square::F1));
    assert!(!legal(&p, "e1g1"));
}

#[test]
fn mamdani_on_b1_blocks_queenside_castling() {
    let p = setup(CASTLE_FEN, Some(Square::B1), None, None);
    assert!(!legal(&p, "e1c1"));
}

#[test]
fn pothole_cancels_en_passant() {
    const FEN: &str = "4k3/8/8/3Pp3/8/8/8/4K3 w - e6 0 1";
    assert!(legal(&setup(FEN, None, None, None), "d5e6"));
    assert!(!legal(&setup(FEN, None, None, Some(Square::E6)), "d5e6"));
}

#[test]
fn start_position_has_33_moves_including_13_for_the_mamdani() {
    // 20 normal moves + 13 Mamdani moves from a5: a6, a4, a3, b5-h5, b6, b4, c3.
    assert_eq!(Position::start().legal_moves().len(), 33);
}

#[test]
fn mamdani_never_captures() {
    let p = Position::start();
    for uci in ["a5a7", "a5a2", "a5c7", "a5d2"] {
        assert!(!legal(&p, uci), "Mamdani captured with {uci}");
    }
}

#[test]
fn mamdani_can_go_straight_back() {
    let (p, _) = apply(&Position::start(), "a5b5", &[1]);
    assert!(legal(&p, "b5a5"));
}

#[test]
fn mamdani_cannot_unpin_own_king() {
    let p = setup(
        "4k3/8/8/8/8/8/8/r3K3 w - - 0 1",
        Some(Square::C1),
        None,
        None,
    );
    assert!(
        legal(&p, "c1b1") && legal(&p, "c1d1"),
        "may slide along the pin line"
    );
    assert!(
        !legal(&p, "c1c2"),
        "moving off the line leaves White in check"
    );
}

#[test]
fn move_is_illegal_if_own_pothole_closing_exposes_king() {
    let p = setup(
        "k3r3/8/8/8/8/8/8/4K3 w - - 0 1",
        None,
        Some(Square::E4),
        None,
    );
    assert_eq!(p.legal_moves().len(), 4);
    assert!(!legal(&p, "e1e2"));
}

#[test]
fn opponents_pothole_stays_open_and_shields_king() {
    let p = setup(
        "k3r3/8/8/8/8/8/8/4K3 w - - 0 1",
        None,
        None,
        Some(Square::E4),
    );
    assert!(legal(&p, "e1e2"));
}

#[test]
fn move_is_illegal_if_repair_exposes_king() {
    let p = setup(
        "k3r3/8/8/8/8/8/8/4K3 w - - 0 1",
        Some(Square::H3),
        None,
        Some(Square::E4),
    );
    assert!(!legal(&p, "h3f3") && !legal(&p, "h3f5"));
    assert!(legal(&p, "h3g3"), "h3g3 leaves the pothole alone");
}
