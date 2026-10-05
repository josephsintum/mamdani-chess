//! Plain-chess perft (no Mamdani, no potholes) proves the move generator
//! against published counts: <https://www.chessprogramming.org/Perft_Results>.
//! Port of `rules/perft_test.go`. The deepest counts take a while unoptimised
//! and run with `cargo test --release -- --ignored`.

#![allow(clippy::unwrap_used)]

use rules::{Position, START_FEN};

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

fn check(fen: &str, counts: &[u64]) {
    let p = Position::from_fen(fen).unwrap();
    for (depth, &want) in (1..).zip(counts) {
        assert_eq!(perft(&p, depth), want, "{fen} depth {depth}");
    }
}

const KIWIPETE: &str = "r3k2r/p1ppqpb1/bn2pnp1/3PN3/1p2P3/2N2Q1p/PPPBBPPP/R3K2R w KQkq - 0 1";
const POSITION_3: &str = "8/2p5/3p4/KP5r/1R3p1k/8/4P1P1/8 w - - 0 1";
const POSITION_4: &str = "r3k2r/Pppp1ppp/1b3nbN/nP6/BBP1P3/q4N2/Pp1P2PP/R2Q1RK1 w kq - 0 1";
const POSITION_5: &str = "rnbq1k1r/pp1Pbppp/2p5/8/2B5/8/PPP1NnPP/RNBQK2R w KQ - 1 8";

#[test]
fn perft_start_position() {
    check(START_FEN, &[20, 400, 8902, 197_281]);
}

#[test]
fn perft_kiwipete() {
    check(KIWIPETE, &[48, 2039, 97_862]);
}

#[test]
fn perft_position_3() {
    check(POSITION_3, &[14, 191, 2812, 43_238]);
}

#[test]
fn perft_position_4() {
    check(POSITION_4, &[6, 264, 9467]);
}

#[test]
fn perft_position_5() {
    check(POSITION_5, &[44, 1486, 62_379]);
}

#[test]
#[ignore = "slow: run with --release -- --ignored"]
fn perft_deep() {
    check(START_FEN, &[20, 400, 8902, 197_281, 4_865_609]);
    check(KIWIPETE, &[48, 2039, 97_862, 4_085_603]);
    check(POSITION_3, &[14, 191, 2812, 43_238, 674_624]);
    check(POSITION_4, &[6, 264, 9467, 422_333]);
    check(POSITION_5, &[44, 1486, 62_379, 2_103_487]);
}
