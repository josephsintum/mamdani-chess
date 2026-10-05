//! Dice. The engine has no randomness of its own: every roll comes through
//! [`Dice`], and a roll is a [`D8`], so it is 1..=8 by construction.

use std::fmt;

/// One roll of an eight-sided die: always 1..=8.
#[derive(Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct D8(u8);

impl D8 {
    pub const ONE: D8 = D8(1);

    /// The roll `v`, if it is 1..=8.
    #[must_use]
    pub const fn new(v: u8) -> Option<D8> {
        if v >= 1 && v <= 8 { Some(D8(v)) } else { None }
    }

    #[must_use]
    pub const fn get(self) -> u8 {
        self.0
    }

    /// Odd rolls are the lucky ones: no pothole, or a piece saved.
    #[must_use]
    pub const fn is_odd(self) -> bool {
        self.0 % 2 == 1
    }
}

impl fmt::Debug for D8 {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

impl fmt::Display for D8 {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "{}", self.0)
    }
}

/// A number that isn't a d8 roll.
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
#[error("die roll {0} is outside 1..=8")]
pub struct BadDie(pub u8);

impl TryFrom<u8> for D8 {
    type Error = BadDie;

    fn try_from(v: u8) -> Result<D8, BadDie> {
        D8::new(v).ok_or(BadDie(v))
    }
}

/// A source of d8 rolls.
pub trait Dice {
    fn d8(&mut self) -> D8;
}

impl<D: Dice + ?Sized> Dice for &mut D {
    fn d8(&mut self) -> D8 {
        (**self).d8()
    }
}

impl<D: Dice + ?Sized> Dice for Box<D> {
    fn d8(&mut self) -> D8 {
        (**self).d8()
    }
}

/// Preset rolls, in order. Past the end it rolls 1 and remembers that it ran
/// out, so a short script ends the turn quietly and the caller can tell.
#[derive(Clone, Debug, Default)]
pub struct ScriptedDice {
    rolls: Vec<D8>,
    next: usize,
    ran_out: bool,
}

impl ScriptedDice {
    #[must_use]
    pub fn new(rolls: impl IntoIterator<Item = D8>) -> ScriptedDice {
        ScriptedDice {
            rolls: rolls.into_iter().collect(),
            next: 0,
            ran_out: false,
        }
    }

    /// Rolls from plain numbers, checking each is 1..=8.
    ///
    /// # Errors
    ///
    /// [`BadDie`] for the first number outside 1..=8.
    pub fn from_values(values: &[u8]) -> Result<ScriptedDice, BadDie> {
        let rolls = values
            .iter()
            .map(|&v| D8::try_from(v))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(ScriptedDice::new(rolls))
    }

    /// How many rolls have not been used.
    #[must_use]
    pub fn left(&self) -> usize {
        self.rolls.len() - self.next
    }

    /// Whether anything asked for a roll after the script ended.
    #[must_use]
    pub fn ran_out(&self) -> bool {
        self.ran_out
    }
}

impl Dice for ScriptedDice {
    fn d8(&mut self) -> D8 {
        if let Some(&r) = self.rolls.get(self.next) {
            self.next += 1;
            r
        } else {
            self.ran_out = true;
            D8::ONE
        }
    }
}

/// Wraps dice and keeps every roll, so a turn can be replayed.
pub(crate) struct Recording<'a, D: ?Sized> {
    inner: &'a mut D,
    pub(crate) rolls: Vec<D8>,
}

impl<'a, D: Dice + ?Sized> Recording<'a, D> {
    pub(crate) fn new(inner: &'a mut D) -> Self {
        Recording {
            inner,
            rolls: Vec::new(),
        }
    }
}

impl<D: Dice + ?Sized> Dice for Recording<'_, D> {
    fn d8(&mut self) -> D8 {
        let r = self.inner.d8();
        self.rolls.push(r);
        r
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn d8_rejects_zero_and_nine() {
        assert_eq!(D8::new(0), None);
        assert_eq!(D8::new(9), None);
        assert_eq!(D8::new(8).map(D8::get), Some(8));
    }

    #[test]
    fn scripted_dice_reject_bad_values() {
        assert_eq!(
            ScriptedDice::from_values(&[2, 9, 1]).unwrap_err(),
            BadDie(9)
        );
    }

    #[test]
    fn scripted_dice_roll_one_and_flag_when_out() {
        let mut d = ScriptedDice::from_values(&[4]).unwrap();
        assert_eq!(d.d8().get(), 4);
        assert!(!d.ran_out());
        assert_eq!(d.d8(), D8::ONE);
        assert!(d.ran_out());
    }
}
