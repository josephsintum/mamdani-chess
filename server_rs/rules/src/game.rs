//! A game: a position plus the history needed for repetition and replay.

use std::collections::HashMap;

use crate::dice::Recording;
use crate::position::Key;
use crate::{Color, D8, Dice, Event, Kind, Move, Position, ScriptedDice};

/// Why a game ended.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Reason {
    Checkmate,
    Stalemate,
    FiftyMoves,
    Repetition,
    InsufficientMaterial,
    Resignation,
}

impl Reason {
    /// The wire name: `checkmate`, `fifty_moves` and so on.
    #[must_use]
    pub const fn as_str(self) -> &'static str {
        match self {
            Reason::Checkmate => "checkmate",
            Reason::Stalemate => "stalemate",
            Reason::FiftyMoves => "fifty_moves",
            Reason::Repetition => "repetition",
            Reason::InsufficientMaterial => "insufficient_material",
            Reason::Resignation => "resignation",
        }
    }
}

/// How a game ended. No winner means a draw.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Outcome {
    pub winner: Option<Color>,
    pub reason: Reason,
}

impl Outcome {
    const fn win(winner: Color, reason: Reason) -> Outcome {
        Outcome {
            winner: Some(winner),
            reason,
        }
    }

    const fn draw(reason: Reason) -> Outcome {
        Outcome {
            winner: None,
            reason,
        }
    }
}

/// One recorded turn: the move and every d8 rolled, in order. Replaying
/// turns rebuilds a game exactly.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Turn {
    pub mv: Move,
    pub dice: Vec<D8>,
}

/// Why a turn couldn't be played.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
pub enum PlayError {
    #[error("game is over")]
    GameOver,
    #[error("illegal move")]
    Illegal,
}

/// Why a replay failed.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
pub enum ReplayError {
    #[error("turn {turn} ({mv}): {source}")]
    Play {
        turn: usize,
        mv: Move,
        source: PlayError,
    },
    #[error("turn {turn} ({mv}): the recorded dice ran out")]
    MissingDice { turn: usize, mv: Move },
    #[error("turn {turn} ({mv}): {left} recorded dice were not used")]
    UnusedDice { turn: usize, mv: Move, left: usize },
}

/// A game in progress or finished.
#[derive(Clone, Debug)]
pub struct Game {
    pos: Position,
    turns: Vec<Turn>,
    outcome: Option<Outcome>,
    seen: HashMap<Key, u8>,
}

impl Default for Game {
    fn default() -> Game {
        Game::new()
    }
}

impl Game {
    /// A game from [`Position::start`].
    #[must_use]
    pub fn new() -> Game {
        Game::from_position(Position::start())
    }

    /// A game from any position (tests, replays).
    #[must_use]
    pub fn from_position(pos: Position) -> Game {
        let mut g = Game {
            pos,
            turns: Vec::new(),
            outcome: None,
            seen: HashMap::from([(pos.key(), 1)]),
        };
        g.outcome = g.status();
        g
    }

    #[must_use]
    pub fn position(&self) -> &Position {
        &self.pos
    }

    #[must_use]
    pub fn turns(&self) -> &[Turn] {
        &self.turns
    }

    /// `None` while the game is on.
    #[must_use]
    pub fn outcome(&self) -> Option<Outcome> {
        self.outcome
    }

    /// Plays one turn and updates the outcome. Nothing changes on error.
    ///
    /// # Errors
    ///
    /// [`PlayError::GameOver`] once the game has ended, [`PlayError::Illegal`]
    /// for a move that isn't legal.
    pub fn play<D: Dice + ?Sized>(
        &mut self,
        m: Move,
        dice: &mut D,
    ) -> Result<Vec<Event>, PlayError> {
        if self.outcome.is_some() {
            return Err(PlayError::GameOver);
        }
        let mut rec = Recording::new(dice);
        let (next, ev) = self
            .pos
            .apply(m, &mut rec)
            .map_err(|_| PlayError::Illegal)?;
        self.pos = next;
        self.turns.push(Turn {
            mv: m,
            dice: rec.rolls,
        });
        *self.seen.entry(next.key()).or_insert(0) += 1;
        self.outcome = self.status();
        Ok(ev)
    }

    /// `loser` resigns; the other side wins.
    ///
    /// # Errors
    ///
    /// [`PlayError::GameOver`] once the game has ended.
    pub fn resign(&mut self, loser: Color) -> Result<(), PlayError> {
        if self.outcome.is_some() {
            return Err(PlayError::GameOver);
        }
        self.outcome = Some(Outcome::win(loser.other(), Reason::Resignation));
        Ok(())
    }

    /// Rebuilds a game from `start` by playing `turns` with their recorded
    /// dice.
    ///
    /// # Errors
    ///
    /// [`ReplayError`] if a move is illegal or the dice don't match exactly.
    pub fn replay(start: Position, turns: &[Turn]) -> Result<Game, ReplayError> {
        let mut g = Game::from_position(start);
        for (i, t) in turns.iter().enumerate() {
            let (turn, mv) = (i + 1, t.mv);
            let mut script = ScriptedDice::new(t.dice.iter().copied());
            g.play(mv, &mut script)
                .map_err(|source| ReplayError::Play { turn, mv, source })?;
            if script.ran_out() {
                return Err(ReplayError::MissingDice { turn, mv });
            }
            if script.left() > 0 {
                return Err(ReplayError::UnusedDice {
                    turn,
                    mv,
                    left: script.left(),
                });
            }
        }
        Ok(g)
    }

    fn status(&self) -> Option<Outcome> {
        let p = &self.pos;
        if p.legal_moves().is_empty() {
            return Some(if p.threatened() {
                Outcome::win(p.turn.other(), Reason::Checkmate)
            } else {
                Outcome::draw(Reason::Stalemate)
            });
        }
        if p.halfmove >= 100 {
            return Some(Outcome::draw(Reason::FiftyMoves));
        }
        if self.seen.get(&p.key()).is_some_and(|&n| n >= 3) {
            return Some(Outcome::draw(Reason::Repetition));
        }
        if p.cannot_mate(Color::White) && p.cannot_mate(Color::Black) {
            return Some(Outcome::draw(Reason::InsufficientMaterial));
        }
        None
    }
}

impl Position {
    /// Whether `c` can never deliver checkmate: a bare king, or king and one
    /// knight or bishop once the Mamdani has fallen. (While the Mamdani is on
    /// the board it can block escape squares, so a minor piece might mate.)
    #[must_use]
    pub fn cannot_mate(&self, c: Color) -> bool {
        let heavy = self.kind(Kind::Pawn) | self.kind(Kind::Rook) | self.kind(Kind::Queen);
        if !(self.colored(c) & heavy).is_empty() {
            return false;
        }
        let minors =
            (self.colored(c) & (self.kind(Kind::Knight) | self.kind(Kind::Bishop))).count();
        minors == 0 || (minors == 1 && self.mamdani.is_none())
    }
}
