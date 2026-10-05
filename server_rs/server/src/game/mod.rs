//! Live games. Each game is an actor: one task owns its state and takes
//! commands over a channel, so nothing inside a game needs a lock. The
//! [`Hub`] finds games by code.

mod actor;
mod hub;
mod view;

use std::fmt;
use std::str::FromStr;
use std::sync::Arc;

pub use hub::{DiceFactory, GameHandle, Hub};
pub use view::{EventView, LogEntry, Lost, MoveView, PotholeView, ResultView, Stats, Status, View};

/// Fair dice for real games: each game gets its own generator, seeded from
/// the operating system.
#[must_use]
pub fn fair_dice() -> DiceFactory {
    Arc::new(|| Box::new(RngDice(rand::make_rng::<rand::rngs::StdRng>())))
}

struct RngDice<R>(R);

impl<R: rand::Rng> rules::Dice for RngDice<R> {
    fn d8(&mut self) -> rules::D8 {
        use rand::RngExt;
        // invariant: 1..=8 is always a valid roll.
        rules::D8::new(self.0.random_range(1..=8)).unwrap_or(rules::D8::ONE)
    }
}

/// A guest's identity: 16 random bytes, carried as 32 hex characters in the
/// `guest` cookie.
#[derive(Clone, Copy, PartialEq, Eq, Hash)]
pub struct GuestId([u8; 16]);

impl GuestId {
    /// A fresh random ID.
    #[must_use]
    pub fn random() -> GuestId {
        let mut b = [0; 16];
        rand::fill(&mut b);
        GuestId(b)
    }
}

impl fmt::Display for GuestId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.iter().try_for_each(|b| write!(f, "{b:02x}"))
    }
}

impl fmt::Debug for GuestId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "GuestId({self})")
    }
}

/// A string that isn't 32 hex characters.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[error("guest id must be 32 hex characters")]
pub struct BadGuestId;

impl FromStr for GuestId {
    type Err = BadGuestId;

    fn from_str(s: &str) -> Result<GuestId, BadGuestId> {
        if s.len() != 32 {
            return Err(BadGuestId);
        }
        let mut b = [0; 16];
        for (i, byte) in b.iter_mut().enumerate() {
            *byte = s
                .get(2 * i..2 * i + 2)
                .and_then(|h| u8::from_str_radix(h, 16).ok())
                .ok_or(BadGuestId)?;
        }
        Ok(GuestId(b))
    }
}

/// The characters a game code is made of: no lookalikes (0, O, 1, I, L).
const CODE_ALPHABET: &[u8; 31] = b"ABCDEFGHJKMNPQRSTUVWXYZ23456789";

/// A six-character share code such as `K7F3QZ`.
#[derive(Clone, Copy, PartialEq, Eq, Hash)]
pub struct Code([u8; 6]);

impl Code {
    #[must_use]
    pub fn random() -> Code {
        Code(std::array::from_fn(|_| {
            CODE_ALPHABET[rand::random_range(0..CODE_ALPHABET.len())]
        }))
    }

    #[must_use]
    pub fn as_str(&self) -> &str {
        // invariant: every byte comes from CODE_ALPHABET, which is ASCII.
        std::str::from_utf8(&self.0).unwrap_or_default()
    }
}

impl fmt::Display for Code {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.as_str())
    }
}

impl fmt::Debug for Code {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "Code({self})")
    }
}

/// A string that isn't a game code.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[error("not a game code")]
pub struct BadCode;

impl FromStr for Code {
    type Err = BadCode;

    fn from_str(s: &str) -> Result<Code, BadCode> {
        let bytes: [u8; 6] = s.as_bytes().try_into().map_err(|_| BadCode)?;
        if bytes.iter().all(|b| CODE_ALPHABET.contains(b)) {
            Ok(Code(bytes))
        } else {
            Err(BadCode)
        }
    }
}

impl serde::Serialize for Code {
    fn serialize<S: serde::Serializer>(&self, s: S) -> Result<S::Ok, S::Error> {
        s.serialize_str(self.as_str())
    }
}

/// Who is looking at a game. Every viewer in the same role sees the same
/// view, so the game builds one view per role, not per viewer.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Role {
    White,
    Black,
    Spectator,
}

impl Role {
    const ALL: [Role; 3] = [Role::White, Role::Black, Role::Spectator];

    const fn index(self) -> usize {
        self as usize
    }

    #[must_use]
    pub const fn name(self) -> &'static str {
        match self {
            Role::White => "white",
            Role::Black => "black",
            Role::Spectator => "spectator",
        }
    }

    const fn seat(color: rules::Color) -> Role {
        match color {
            rules::Color::White => Role::White,
            rules::Color::Black => Role::Black,
        }
    }

    const fn color(self) -> Option<rules::Color> {
        match self {
            Role::White => Some(rules::Color::White),
            Role::Black => Some(rules::Color::Black),
            Role::Spectator => None,
        }
    }
}

/// Why a move or resignation was refused by the game's rules or turn order.
/// The messages are shown to players.
#[derive(Clone, Copy, Debug, PartialEq, Eq, thiserror::Error)]
pub enum Conflict {
    #[error("waiting for an opponent")]
    Waiting,
    #[error("not your turn")]
    NotYourTurn,
    #[error("the game has moved on; reload the position")]
    Stale,
    #[error("game is over")]
    GameOver,
    #[error("illegal move")]
    Illegal,
}

/// Why a game command failed.
#[derive(Clone, Debug, thiserror::Error)]
pub enum GameError {
    #[error("you are not playing in this game")]
    NotPlayer,
    /// The caller's current view comes back with the conflict, so the
    /// client can resync without waiting for the stream.
    #[error("{conflict}")]
    Conflict {
        conflict: Conflict,
        state: Arc<View>,
    },
    /// The game's task has stopped: it was evicted or it crashed.
    #[error("the game is no longer running")]
    Gone,
}
