//! Port of `rules/game_test.go` and `rules/san_test.go`: results,
//! repetition, replay and notation.

#![allow(clippy::unwrap_used)]

mod common;

use common::{dice, mv, setup};
use rules::{
    Color, D8, Game, Outcome, PlayError, Position, Reason, ReplayError, START_FEN, Square, Turn,
};

fn play(g: &mut Game, ucis: &[&str]) {
    for uci in ucis {
        g.play(mv(uci), &mut dice(&[1]))
            .unwrap_or_else(|e| panic!("{uci}: {e}"));
    }
}

fn win(winner: Color, reason: Reason) -> Outcome {
    Outcome {
        winner: Some(winner),
        reason,
    }
}

fn reason(g: &Game) -> Option<Reason> {
    g.outcome().map(|o| o.reason)
}

#[test]
fn fools_mate_ends_the_game() {
    let mut g = Game::new();
    play(&mut g, &["f2f3", "e7e5", "g2g4", "d8h4"]);
    assert_eq!(g.outcome(), Some(win(Color::Black, Reason::Checkmate)));
}

#[test]
fn no_moves_after_the_game_ends() {
    let mut g = Game::new();
    play(&mut g, &["f2f3", "e7e5", "g2g4", "d8h4"]);
    assert_eq!(
        g.play(mv("a2a3"), &mut dice(&[1])),
        Err(PlayError::GameOver)
    );
}

const BACK_RANK: &str = "7k/8/8/8/8/8/6PP/r6K w - - 0 1"; // check from a1

#[test]
fn mamdani_can_block_mate() {
    let g = Game::from_position(setup(BACK_RANK, Some(Square::D4), None, None));
    assert_eq!(g.outcome(), None, "Md1 blocks the check");
}

#[test]
fn without_mamdani_back_rank_check_is_mate() {
    let g = Game::from_position(setup(BACK_RANK, None, None, None));
    assert_eq!(g.outcome(), Some(win(Color::Black, Reason::Checkmate)));
}

const BOXED_KING: &str = "k7/8/1Q6/8/8/8/8/7K b - - 0 1";

#[test]
fn mamdani_moves_prevent_stalemate() {
    let g = Game::from_position(setup(BOXED_KING, Some(Square::H4), None, None));
    assert_eq!(g.outcome(), None);
}

#[test]
fn boxed_king_without_mamdani_is_stalemate() {
    let g = Game::from_position(setup(BOXED_KING, None, None, None));
    assert_eq!(
        g.outcome(),
        Some(Outcome {
            winner: None,
            reason: Reason::Stalemate
        })
    );
}

#[test]
fn third_repetition_draws() {
    let mut g = Game::new();
    play(
        &mut g,
        &["g1f3", "g8f6", "f3g1", "f6g8", "g1f3", "g8f6", "f3g1"],
    );
    assert_eq!(g.outcome(), None, "only two repetitions so far");
    play(&mut g, &["f6g8"]);
    assert_eq!(reason(&g), Some(Reason::Repetition));
}

#[test]
fn mamdani_move_completes_fifty_move_rule() {
    let mut g = Game::from_position(Position::start().with_halfmove(99));
    play(&mut g, &["a5b5"]);
    assert_eq!(reason(&g), Some(Reason::FiftyMoves));
}

#[test]
fn insufficient_material_depends_on_the_mamdani() {
    for (fen, mamdani, insufficient) in [
        ("4k3/8/8/8/8/8/8/4K3 w - - 0 1", Some(Square::A5), true),
        ("4k3/8/8/8/8/8/8/3NK3 w - - 0 1", None, true),
        // The Mamdani can help a knight mate.
        ("4k3/8/8/8/8/8/8/3NK3 w - - 0 1", Some(Square::A5), false),
        ("4k3/8/8/8/8/8/8/3RK3 w - - 0 1", None, false),
    ] {
        let g = Game::from_position(setup(fen, mamdani, None, None));
        assert_eq!(
            reason(&g) == Some(Reason::InsufficientMaterial),
            insufficient,
            "{fen} mamdani {mamdani:?}"
        );
    }
}

fn three_turns() -> Game {
    let mut g = Game::new();
    for (uci, rolls) in [
        ("e2e4", &[2, 4, 4][..]),
        ("e7e5", &[2, 4, 2, 6]),
        ("a5b5", &[1]),
    ] {
        g.play(mv(uci), &mut dice(rolls)).unwrap();
    }
    g
}

#[test]
fn replay_rebuilds_the_same_game() {
    let g = three_turns();
    let r = Game::replay(Position::start(), g.turns()).unwrap();
    assert_eq!((r.position(), r.outcome()), (g.position(), g.outcome()));
}

#[test]
fn replay_with_missing_dice_fails() {
    let g = three_turns();
    let short = [Turn {
        mv: g.turns()[0].mv,
        dice: vec![D8::new(2).unwrap()],
    }];
    assert!(matches!(
        Game::replay(Position::start(), &short),
        Err(ReplayError::MissingDice { turn: 1, .. })
    ));
}

#[test]
fn replay_with_extra_dice_fails() {
    let g = three_turns();
    let mut long = g.turns().to_vec();
    long[2].dice.push(D8::ONE);
    assert!(matches!(
        Game::replay(Position::start(), &long),
        Err(ReplayError::UnusedDice {
            turn: 3,
            left: 1,
            ..
        })
    ));
}

#[test]
fn mate_on_the_board_is_not_undone_by_dice() {
    // With an even roll the dice could drop the h4 queen and break the
    // mate. A checkmating move ends the game before any pothole roll.
    let mut g = Game::new();
    play(&mut g, &["f2f3", "e7e5", "g2g4"]);
    let mut d = dice(&[2, 1, 3, 8, 4]);
    g.play(mv("d8h4"), &mut d).unwrap();
    assert_eq!(g.outcome(), Some(win(Color::Black, Reason::Checkmate)));
    assert_eq!(d.left(), 5);
    assert!(g.turns().last().unwrap().dice.is_empty());
}

#[test]
fn check_held_off_only_by_own_pothole_is_mate() {
    // White's own hole on e4 blocks the e8 rook. Every white move closes it,
    // and nothing can block the e-file or step off it.
    let p = setup(
        "k3r3/8/8/8/8/8/3P1P2/3RKR2 w - - 0 1",
        None,
        Some(Square::E4),
        None,
    );
    assert_eq!(
        Game::from_position(p).outcome(),
        Some(win(Color::Black, Reason::Checkmate))
    );
}

#[test]
fn resigning_hands_the_win_to_the_other_side() {
    let mut g = Game::new();
    g.resign(Color::White).unwrap();
    assert_eq!(g.outcome(), Some(win(Color::Black, Reason::Resignation)));
    assert_eq!(g.resign(Color::Black), Err(PlayError::GameOver));
}

#[test]
fn san_matches_standard_notation_plus_mamdani() {
    let castle = "r3k2r/8/8/8/8/8/8/R3K2R w KQkq - 0 1";
    for (fen, mamdani, uci, want) in [
        (START_FEN, Some(Square::A5), "e2e4", "e4"),
        (START_FEN, Some(Square::A5), "g1f3", "Nf3"),
        (START_FEN, Some(Square::A5), "a5b5", "Mb5"),
        (castle, None, "e1g1", "O-O"),
        (castle, None, "e1c1", "O-O-O"),
        ("4k3/P7/8/8/8/8/8/4K3 w - - 0 1", None, "a7a8q", "a8=Q+"),
        ("4k3/8/8/3p4/4P3/8/8/4K3 w - - 0 1", None, "e4d5", "exd5"),
        ("4k3/8/8/8/8/8/8/R4RK1 w - - 0 1", None, "a1d1", "Rad1"),
        ("4k3/8/8/8/8/R7/8/R3K3 w - - 0 1", None, "a1a2", "R1a2"),
        ("4k3/8/8/8/8/8/8/4K2Q w - - 0 1", None, "h1h5", "Qh5+"),
        (
            "rnbqkbnr/pppp1ppp/8/4p3/6P1/5P2/PPPPP2P/RNBQKBNR b KQkq g3 0 2",
            None,
            "d8h4",
            "Qh4#",
        ),
    ] {
        let p = setup(fen, mamdani, None, None);
        assert_eq!(p.san(mv(uci)), want, "{fen} {uci}");
    }
}
