//! `cargo bench -p rules`: move generation (perft) and whole games.

#![allow(clippy::unwrap_used)]

use std::hint::black_box;

use criterion::{Criterion, criterion_group, criterion_main};
use rand::{RngExt, SeedableRng};
use rand_pcg::Pcg64;
use rules::{D8, Dice, Game, Position};

fn perft(p: &Position, depth: u32) -> u64 {
    let moves = p.legal_moves();
    if depth == 1 {
        return moves.len() as u64;
    }
    moves
        .iter()
        .map(|&m| perft(&p.after_move(m), depth - 1))
        .sum()
}

struct RandDice<'a>(&'a mut Pcg64);

impl Dice for RandDice<'_> {
    fn d8(&mut self) -> D8 {
        D8::new(self.0.random_range(1..=8)).unwrap()
    }
}

/// One seeded random game of up to 300 plies, as the random-game test plays.
fn random_game(seed: u64) -> usize {
    let mut r = Pcg64::seed_from_u64(seed);
    let mut g = Game::new();
    while g.outcome().is_none() && g.turns().len() < 300 {
        let moves = g.position().legal_moves();
        let m = moves[r.random_range(0..moves.len())];
        g.play(m, &mut RandDice(&mut r)).unwrap();
    }
    g.turns().len()
}

fn benches(c: &mut Criterion) {
    let start = Position::start().with_mamdani(None);
    c.bench_function("perft start depth 4", |b| {
        b.iter(|| perft(black_box(&start), 4));
    });
    let mut seed = 0;
    c.bench_function("random game", |b| {
        b.iter(|| {
            seed += 1;
            random_game(black_box(seed))
        });
    });
}

criterion_group!(engine, benches);
criterion_main!(engine);
