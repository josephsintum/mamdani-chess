//! Port of `rules/turn_test.go`: pothole rolls, placement, re-rolls, saving
//! rolls and the repair step.

#![allow(clippy::unwrap_used)]

mod common;

use common::{apply, dice, find, kinds, mv, setup};
use rules::{
    Color, Event, IllegalMove, Kind, MAX_REROLLS, Occupant, Piece, Position, RerollReason, Square,
};

#[test]
fn odd_roll_opens_no_pothole() {
    let (p, ev) = apply(&Position::start(), "e2e4", &[3]);
    assert_eq!(kinds(&ev), ["moved", "rolled_pothole"]);
    assert_eq!(
        (p.pothole(Color::White), p.pothole(Color::Black), p.turn()),
        (None, None, Color::Black)
    );
}

#[test]
fn illegal_moves_are_rejected() {
    // What a buggy or hostile client might send.
    for uci in ["e2e5", "e7e5", "e2e4q", "a5a7", "e3e4"] {
        assert_eq!(
            Position::start()
                .apply(mv(uci), &mut dice(&[]))
                .unwrap_err(),
            IllegalMove,
            "{uci}"
        );
    }
}

#[test]
fn promotion_without_a_piece_is_illegal() {
    let p = setup("4k3/P7/8/8/8/8/8/4K3 w - - 0 1", None, None, None);
    assert!(p.apply(mv("a7a8"), &mut dice(&[])).is_err());
}

#[test]
fn even_roll_opens_pothole_on_rolled_square() {
    // Even roll, then file 4 rank 4: d4.
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 4, 4]);
    assert_eq!(
        kinds(&ev),
        ["moved", "rolled_pothole", "target", "pothole_opened"]
    );
    assert_eq!(p.pothole(Color::White), Some(Square::D4));
}

#[test]
fn pothole_closes_after_rollers_next_move() {
    let (p, _) = apply(&Position::start(), "e2e4", &[2, 4, 4]);
    let (p, _) = apply(&p, "g8f6", &[1]);
    assert_eq!(
        p.pothole(Color::White),
        Some(Square::D4),
        "closed too early"
    );
    let (p, ev) = apply(&p, "g1f3", &[1]);
    assert_eq!(
        find(&ev, "pothole_closed"),
        Some(Event::PotholeClosed { sq: Square::D4 })
    );
    assert_eq!(p.pothole(Color::White), None);
}

#[test]
fn both_sides_can_have_a_pothole_open() {
    let (p, _) = apply(&Position::start(), "e2e4", &[2, 4, 4]); // White: d4
    let (p, _) = apply(&p, "e7e5", &[4, 8, 3]); // Black: h3
    assert_eq!(
        (p.pothole(Color::White), p.pothole(Color::Black)),
        (Some(Square::D4), Some(Square::H3))
    );
}

#[test]
fn king_square_is_rerolled() {
    let (_, ev) = apply(&Position::start(), "e2e4", &[2, 5, 1, 4, 4]); // e1, then d4
    assert_eq!(
        find(&ev, "reroll"),
        Some(Event::Reroll {
            sq: Square::E1,
            reason: RerollReason::King
        })
    );
    assert_eq!(
        find(&ev, "pothole_opened"),
        Some(Event::PotholeOpened {
            sq: Square::D4,
            color: Color::White
        })
    );
}

#[test]
fn existing_pothole_is_rerolled() {
    let p = Position::start().with_potholes(None, Some(Square::D4));
    let (_, ev) = apply(&p, "e2e4", &[6, 4, 4, 8, 3]); // d4, then h3
    assert!(matches!(
        find(&ev, "reroll"),
        Some(Event::Reroll {
            reason: RerollReason::Pothole,
            ..
        })
    ));
}

#[test]
fn reroll_cap_ends_in_no_pothole() {
    let mut rolls = vec![2];
    for _ in 0..=MAX_REROLLS {
        rolls.extend([5, 1]); // e1, the king, every time
    }
    let (_, ev) = apply(&Position::start(), "e2e4", &rolls);
    assert_eq!(ev.last(), Some(&Event::NoPothole));
}

#[test]
fn pothole_next_to_mamdani_is_repaired_as_it_opens() {
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 2, 4]); // b4, next to a5
    assert_eq!(
        find(&ev, "repaired"),
        Some(Event::Repaired { sq: Square::B4 })
    );
    assert_eq!(
        (p.pothole(Color::White), p.pothole(Color::Black)),
        (None, None)
    );
}

#[test]
fn piece_without_mamdani_line_falls_without_saving_roll() {
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 7, 8]); // g8 knight; no line from a5
    assert_eq!(
        find(&ev, "fell"),
        Some(Event::Fell {
            sq: Square::G8,
            occupant: Occupant::Piece(Piece::new(Color::Black, Kind::Knight))
        })
    );
    assert_eq!(find(&ev, "saving_roll"), None);
    assert_eq!(
        (
            p.piece_at(Square::G8),
            p.pothole(Color::White),
            p.halfmove()
        ),
        (None, Some(Square::G8), 0)
    );
}

#[test]
fn odd_saving_roll_by_owner_saves_piece_on_mamdani_line() {
    // a5-b4-c3-d2 is a clear diagonal, so the d2 pawn gets a saving roll,
    // made by its owner.
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 4, 2, 5]);
    assert!(matches!(
        find(&ev, "saving_roll"),
        Some(Event::SavingRoll {
            saved: true,
            color: Color::White,
            roll,
            ..
        }) if roll.get() == 5
    ));
    assert_eq!(
        (p.piece_at(Square::D2), p.pothole(Color::White)),
        (Some(Piece::new(Color::White, Kind::Pawn)), None)
    );
}

#[test]
fn even_saving_roll_fails_and_piece_falls() {
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 4, 2, 6]);
    assert!(matches!(
        find(&ev, "saving_roll"),
        Some(Event::SavingRoll { saved: false, .. })
    ));
    assert_eq!(
        (p.piece_at(Square::D2), p.pothole(Color::White)),
        (None, Some(Square::D2))
    );
}

#[test]
fn roller_saves_the_mamdani_on_odd() {
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 1, 5, 3]); // a5, saved
    assert!(matches!(
        find(&ev, "saving_roll"),
        Some(Event::SavingRoll {
            saved: true,
            occupant: Occupant::Mamdani,
            color: Color::White,
            ..
        })
    ));
    assert_eq!(p.mamdani(), Some(Square::A5));
}

#[test]
fn mamdani_falls_on_even_and_saving_rolls_end() {
    let (p, ev) = apply(&Position::start(), "e2e4", &[2, 1, 5, 4]); // a5, falls
    assert!(matches!(
        find(&ev, "fell"),
        Some(Event::Fell {
            occupant: Occupant::Mamdani,
            ..
        })
    ));
    assert_eq!(
        (p.mamdani(), p.pothole(Color::White)),
        (None, Some(Square::A5))
    );
    // With the Mamdani gone there are no more saving rolls: a7 falls at once.
    let (_, ev) = apply(&p, "e7e5", &[2, 1, 7]);
    assert_eq!(find(&ev, "saving_roll"), None);
}

#[test]
fn fall_that_would_expose_roller_is_rerolled() {
    // The e2 bishop shields the white king from the e8 rook.
    let p = setup("k3r3/8/8/8/8/8/4B2P/4K3 w - - 0 1", None, None, None);
    let (_, ev) = apply(&p, "h2h3", &[2, 5, 2, 8, 8]); // e2, then h8
    assert_eq!(
        find(&ev, "reroll"),
        Some(Event::Reroll {
            sq: Square::E2,
            reason: RerollReason::Exposes
        })
    );
}

#[test]
fn fall_that_would_checkmate_is_rerolled() {
    // Ra8+ is answered only by the c7 knight (Nxa8 or Ne8). If it fell,
    // Black would be mated by the roll, so the dice go again.
    let p = setup("7k/2n3pp/8/8/8/8/8/R5K1 w - - 0 1", None, None, None);
    let (p, ev) = apply(&p, "a1a8", &[2, 3, 7, 1, 1]); // c7, then a1
    assert_eq!(
        find(&ev, "reroll"),
        Some(Event::Reroll {
            sq: Square::C7,
            reason: RerollReason::Checkmate
        })
    );
    assert_eq!(
        p.piece_at(Square::C7),
        Some(Piece::new(Color::Black, Kind::Knight))
    );
}

#[test]
fn moving_mamdani_next_to_pothole_repairs_it() {
    let p = Position::start().with_potholes(None, Some(Square::D4));
    let (p, ev) = apply(&p, "a5c5", &[1]); // c5 touches d4
    assert_eq!(
        find(&ev, "repaired"),
        Some(Event::Repaired { sq: Square::D4 })
    );
    assert_eq!(p.pothole(Color::Black), None);
}

#[test]
fn mamdani_move_does_not_reset_fifty_move_count() {
    let (p, _) = apply(&Position::start().with_halfmove(10), "a5b5", &[1]);
    assert_eq!(p.halfmove(), 11);
}

#[test]
fn failed_apply_leaves_position_unchanged() {
    let p = Position::start();
    let before = p;
    let _ = p.apply(mv("e2e5"), &mut dice(&[]));
    assert_eq!(p, before);
}
