//! Plays seeded random games and checks the invariants RULES.md promises
//! after every turn. Port of `rules/random_test.go`: 500 games by default,
//! 10,000 with `cargo test --release -- --ignored`.
//!
//! The seeds don't reproduce the Go games (different generators); the
//! server's parity test is what compares the two engines.

#![allow(clippy::unwrap_used)]

use rand::{RngExt, SeedableRng};
use rand_pcg::Pcg64;
use rules::{D8, Dice, Game, Move, Position, Reason};

struct RandDice<'a>(&'a mut Pcg64);

impl Dice for RandDice<'_> {
    fn d8(&mut self) -> D8 {
        D8::new(self.0.random_range(1..=8)).unwrap()
    }
}

fn play_random_game(seed: u64) {
    let mut r = Pcg64::seed_from_u64(seed);
    let mut g = Game::new();
    for ply in 0..300 {
        if g.outcome().is_some() {
            break;
        }
        let moves = g.position().legal_moves();
        let m = moves[r.random_range(0..moves.len())];
        let before = *g.position();
        g.play(m, &mut RandDice(&mut r))
            .unwrap_or_else(|e| panic!("seed {seed} ply {ply} {m}: {e}"));
        check_invariants(seed, ply, &before, m, &g);
    }
    let replayed = Game::replay(Position::start(), g.turns())
        .unwrap_or_else(|e| panic!("seed {seed}: replay: {e}"));
    assert_eq!(
        replayed.position(),
        g.position(),
        "seed {seed}: replay differs"
    );
    assert_eq!(
        replayed.outcome(),
        g.outcome(),
        "seed {seed}: replay differs"
    );
}

fn check_invariants(seed: u64, ply: usize, before: &Position, m: Move, g: &Game) {
    let p = g.position();
    let at = format!("seed {seed} ply {ply} after {m}");
    assert!(
        p.king(rules::Color::White).is_some() && p.king(rules::Color::Black).is_some(),
        "{at}: a king fell"
    );
    for c in rules::Color::ALL {
        if let Some(s) = p.pothole(c) {
            assert!(
                p.piece_at(s).is_none() && p.mamdani() != Some(s),
                "{at}: something stands on the pothole at {s}"
            );
        }
    }
    if let Some(s) = p.mamdani() {
        assert!(p.piece_at(s).is_none(), "{at}: the Mamdani shares {s}");
    }
    assert!(
        before.mamdani().is_some() || p.mamdani().is_none(),
        "{at}: the Mamdani came back"
    );
    assert!(
        !p.in_check(p.turn().other()),
        "{at}: the player who just moved is in check"
    );
    let mated_on_board = before.after_move(m).is_mated();
    let checkmate = g.outcome().map(|o| o.reason) == Some(Reason::Checkmate);
    // The dice never undo a mate made on the board, and never deliver one.
    assert_eq!(mated_on_board, checkmate, "{at}: dice changed a checkmate");
}

fn play_many(games: u64) {
    std::thread::scope(|s| {
        let workers = std::thread::available_parallelism().map_or(4, |n| n.get() as u64);
        for w in 0..workers {
            s.spawn(move || {
                for seed in (w..games).step_by(workers as usize) {
                    play_random_game(seed);
                }
            });
        }
    });
}

#[test]
fn random_games_keep_the_invariants() {
    play_many(500);
}

#[test]
#[ignore = "slow: run with --release -- --ignored"]
fn ten_thousand_random_games_keep_the_invariants() {
    play_many(10_000);
}
