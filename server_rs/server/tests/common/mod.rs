//! Dice and helpers shared by the server tests.

#![allow(dead_code, clippy::unwrap_used, clippy::expect_used)]

use std::collections::VecDeque;
use std::sync::{Arc, Mutex};
use std::time::Duration;

use rules::{D8, Dice};
use server::game::{DiceFactory, GuestId, Hub};

/// Always rolls 1: no potholes, so moves are plain chess.
pub struct Odd;

impl Dice for Odd {
    fn d8(&mut self) -> D8 {
        D8::ONE
    }
}

pub fn odd() -> DiceFactory {
    Arc::new(|| Box::new(Odd))
}

/// Rolls the given values in order, then 1 forever. Every game the factory
/// makes draws from the same script.
pub fn script(rolls: &[u8]) -> DiceFactory {
    let rolls: Arc<Mutex<VecDeque<D8>>> = Arc::new(Mutex::new(
        rolls.iter().map(|&r| D8::new(r).unwrap()).collect(),
    ));
    Arc::new(move || Box::new(Script(Arc::clone(&rolls))))
}

struct Script(Arc<Mutex<VecDeque<D8>>>);

impl Dice for Script {
    fn d8(&mut self) -> D8 {
        self.0.lock().unwrap().pop_front().unwrap_or(D8::ONE)
    }
}

/// Panics on every roll: a stand-in for a bug inside a game.
pub fn exploding() -> DiceFactory {
    Arc::new(|| Box::new(Boom))
}

struct Boom;

impl Dice for Boom {
    fn d8(&mut self) -> D8 {
        panic!("dice exploded");
    }
}

pub const DAY: Duration = Duration::from_secs(24 * 60 * 60);

pub fn hub(dice: DiceFactory) -> Hub {
    Hub::new(dice, DAY)
}

/// A fixed guest ID per test name, so failures are readable.
pub fn guest(n: u8) -> GuestId {
    format!("{n:032x}").parse().unwrap()
}

pub fn mv(uci: &str) -> rules::Move {
    uci.parse().unwrap()
}
