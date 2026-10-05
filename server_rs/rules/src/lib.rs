//! The Pothole Chess: Mamdani Edition rules engine.
//!
//! It is pure: no I/O, no clock and no randomness of its own (dice come in
//! through [`Dice`]). `RULES.md` at the repository root is the
//! specification; this crate matches the Go engine in `rules/` turn for turn.
//!
//! ```
//! use rules::{Game, Move, ScriptedDice, Square};
//!
//! let mut g = Game::new();
//! // e4, then an even roll opens a pothole on d4 (file 4, rank 4).
//! let mut dice = ScriptedDice::from_values(&[2, 4, 4]).unwrap();
//! g.play(Move::new(Square::E2, Square::E4), &mut dice).unwrap();
//! assert!(g.position().is_pothole(Square::D4));
//! ```

mod bitboard;
mod dice;
mod event;
mod game;
mod movegen;
mod position;
mod san;
mod square;
mod turn;

pub use bitboard::{Bitboard, Squares};
pub use dice::{BadDie, D8, Dice, ScriptedDice};
pub use event::{Event, Occupant, RerollReason};
pub use game::{Game, Outcome, PlayError, Reason, ReplayError, Turn};
pub use movegen::{Move, MoveList, ParseMoveError};
pub use position::{Castling, FenError, Key, Position, START_FEN};
pub use square::{Color, Kind, ParseSquareError, Piece, Promo, Square};
pub use turn::{IllegalMove, MAX_REROLLS};
