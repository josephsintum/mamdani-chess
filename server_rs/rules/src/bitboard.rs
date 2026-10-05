//! Bitboards and the attack tables built from them.
//!
//! The variant has three kinds of obstacle: pieces, the Mamdani and open
//! potholes. All three stop sliders the same way, so move generation works
//! on one `blocked` mask and the tables here never need to know which is
//! which. Sliders use classical ray attacks (a ray table plus a first-blocker
//! bitscan); magic bitboards would be faster still, but nothing has shown we
//! need them.

use std::ops::{BitAnd, BitAndAssign, BitOr, BitOrAssign, BitXor, Not};

use crate::{Color, Square};

/// A set of squares, one bit per square (bit 0 = a1).
#[derive(Clone, Copy, PartialEq, Eq, Hash, Default)]
pub struct Bitboard(pub u64);

impl Bitboard {
    pub const EMPTY: Bitboard = Bitboard(0);

    #[must_use]
    pub const fn from_square(s: Square) -> Bitboard {
        Bitboard(1 << s.index())
    }

    /// The set with just `s`, or the empty set for `None`.
    #[must_use]
    pub const fn from_option(s: Option<Square>) -> Bitboard {
        match s {
            Some(s) => Bitboard::from_square(s),
            None => Bitboard::EMPTY,
        }
    }

    #[must_use]
    pub const fn contains(self, s: Square) -> bool {
        self.0 & (1 << s.index()) != 0
    }

    #[must_use]
    pub const fn is_empty(self) -> bool {
        self.0 == 0
    }

    #[must_use]
    pub const fn count(self) -> u32 {
        self.0.count_ones()
    }

    /// The lowest square in the set.
    #[must_use]
    pub const fn first(self) -> Option<Square> {
        if self.0 == 0 {
            None
        } else {
            Square::new(self.0.trailing_zeros() as u8)
        }
    }

    /// The highest square in the set.
    #[must_use]
    pub const fn last(self) -> Option<Square> {
        if self.0 == 0 {
            None
        } else {
            Square::new(63 - self.0.leading_zeros() as u8)
        }
    }

    const fn with(self, s: Square) -> Bitboard {
        Bitboard(self.0 | 1 << s.index())
    }
}

impl std::fmt::Debug for Bitboard {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.debug_set().entries(*self).finish()
    }
}

impl From<Square> for Bitboard {
    fn from(s: Square) -> Bitboard {
        Bitboard::from_square(s)
    }
}

impl BitOr for Bitboard {
    type Output = Bitboard;
    fn bitor(self, rhs: Bitboard) -> Bitboard {
        Bitboard(self.0 | rhs.0)
    }
}

impl BitOrAssign for Bitboard {
    fn bitor_assign(&mut self, rhs: Bitboard) {
        self.0 |= rhs.0;
    }
}

impl BitAnd for Bitboard {
    type Output = Bitboard;
    fn bitand(self, rhs: Bitboard) -> Bitboard {
        Bitboard(self.0 & rhs.0)
    }
}

impl BitAndAssign for Bitboard {
    fn bitand_assign(&mut self, rhs: Bitboard) {
        self.0 &= rhs.0;
    }
}

impl BitXor for Bitboard {
    type Output = Bitboard;
    fn bitxor(self, rhs: Bitboard) -> Bitboard {
        Bitboard(self.0 ^ rhs.0)
    }
}

impl Not for Bitboard {
    type Output = Bitboard;
    fn not(self) -> Bitboard {
        Bitboard(!self.0)
    }
}

impl IntoIterator for Bitboard {
    type Item = Square;
    type IntoIter = Squares;

    fn into_iter(self) -> Squares {
        Squares(self.0)
    }
}

/// The squares of a [`Bitboard`], from a1 upwards.
#[derive(Clone, Debug)]
pub struct Squares(u64);

impl Iterator for Squares {
    type Item = Square;

    fn next(&mut self) -> Option<Square> {
        let s = Bitboard(self.0).first()?;
        self.0 &= self.0 - 1;
        Some(s)
    }

    fn size_hint(&self) -> (usize, Option<usize>) {
        let n = self.0.count_ones() as usize;
        (n, Some(n))
    }
}

impl ExactSizeIterator for Squares {}

/// One of the eight queen directions. The first four run towards higher
/// square indices, so their nearest blocker is the lowest set bit; the other
/// four run the other way.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum Dir {
    N,
    E,
    NE,
    NW,
    S,
    W,
    SW,
    SE,
}

impl Dir {
    const ALL: [Dir; 8] = [
        Dir::N,
        Dir::E,
        Dir::NE,
        Dir::NW,
        Dir::S,
        Dir::W,
        Dir::SW,
        Dir::SE,
    ];
    pub(crate) const ROOK: [Dir; 4] = [Dir::N, Dir::E, Dir::S, Dir::W];
    pub(crate) const BISHOP: [Dir; 4] = [Dir::NE, Dir::NW, Dir::SW, Dir::SE];

    const fn step(self) -> (i8, i8) {
        match self {
            Dir::N => (0, 1),
            Dir::E => (1, 0),
            Dir::NE => (1, 1),
            Dir::NW => (-1, 1),
            Dir::S => (0, -1),
            Dir::W => (-1, 0),
            Dir::SW => (-1, -1),
            Dir::SE => (1, -1),
        }
    }

    const fn ascending(self) -> bool {
        (self as usize) < 4
    }

    /// The direction that leads from `from` to `to`, if they share a rank,
    /// file or diagonal.
    const fn between(from: Square, to: Square) -> Option<Dir> {
        let df = to.file() as i8 - from.file() as i8;
        let dr = to.rank() as i8 - from.rank() as i8;
        if (df == 0 && dr == 0) || (df != 0 && dr != 0 && df.abs() != dr.abs()) {
            return None;
        }
        let step = (df.signum(), dr.signum());
        let mut i = 0;
        while i < 8 {
            let d = Dir::ALL[i];
            let s = d.step();
            if s.0 == step.0 && s.1 == step.1 {
                return Some(d);
            }
            i += 1;
        }
        None
    }
}

const fn ray_table() -> [[Bitboard; 64]; 8] {
    let mut table = [[Bitboard::EMPTY; 64]; 8];
    let mut d = 0;
    while d < 8 {
        let (df, dr) = Dir::ALL[d].step();
        let mut i = 0;
        while i < 64 {
            let mut bb = Bitboard::EMPTY;
            let mut at = Square::new(i as u8);
            while let Some(s) = at {
                at = s.offset(df, dr);
                if let Some(t) = at {
                    bb = bb.with(t);
                }
            }
            table[d][i] = bb;
            i += 1;
        }
        d += 1;
    }
    table
}

const fn leaper_table(steps: &[(i8, i8)]) -> [Bitboard; 64] {
    let mut table = [Bitboard::EMPTY; 64];
    let mut i = 0;
    while i < 64 {
        let mut bb = Bitboard::EMPTY;
        let mut j = 0;
        while j < steps.len() {
            if let Some(s) = Square::new(i as u8)
                && let Some(t) = s.offset(steps[j].0, steps[j].1)
            {
                bb = bb.with(t);
            }
            j += 1;
        }
        table[i] = bb;
        i += 1;
    }
    table
}

/// Squares beyond each square in each direction, not including it.
static RAYS: [[Bitboard; 64]; 8] = ray_table();

static KNIGHT: [Bitboard; 64] = leaper_table(&[
    (1, 2),
    (2, 1),
    (2, -1),
    (1, -2),
    (-1, -2),
    (-2, -1),
    (-2, 1),
    (-1, 2),
]);

static KING: [Bitboard; 64] = leaper_table(&[
    (1, 0),
    (-1, 0),
    (0, 1),
    (0, -1),
    (1, 1),
    (1, -1),
    (-1, 1),
    (-1, -1),
]);

/// `PAWN[c][s]`: the squares a pawn of colour `c` on `s` attacks.
static PAWN: [[Bitboard; 64]; 2] = [
    leaper_table(&[(-1, 1), (1, 1)]),
    leaper_table(&[(-1, -1), (1, -1)]),
];

pub(crate) fn knight_attacks(s: Square) -> Bitboard {
    KNIGHT[s.index()]
}

pub(crate) fn king_attacks(s: Square) -> Bitboard {
    KING[s.index()]
}

pub(crate) fn pawn_attacks(c: Color, s: Square) -> Bitboard {
    PAWN[c.index()][s.index()]
}

/// The squares a slider on `s` reaches in direction `d`: every empty square
/// up to and including the first blocked one.
fn ray_attacks(d: Dir, s: Square, blocked: Bitboard) -> Bitboard {
    let ray = RAYS[d as usize][s.index()];
    let hits = ray & blocked;
    let first = if d.ascending() {
        hits.first()
    } else {
        hits.last()
    };
    match first {
        Some(b) => ray ^ RAYS[d as usize][b.index()],
        None => ray,
    }
}

pub(crate) fn rook_attacks(s: Square, blocked: Bitboard) -> Bitboard {
    Dir::ROOK
        .into_iter()
        .fold(Bitboard::EMPTY, |acc, d| acc | ray_attacks(d, s, blocked))
}

pub(crate) fn bishop_attacks(s: Square, blocked: Bitboard) -> Bitboard {
    Dir::BISHOP
        .into_iter()
        .fold(Bitboard::EMPTY, |acc, d| acc | ray_attacks(d, s, blocked))
}

pub(crate) fn queen_attacks(s: Square, blocked: Bitboard) -> Bitboard {
    rook_attacks(s, blocked) | bishop_attacks(s, blocked)
}

/// The squares strictly between `a` and `b` if they share a rank, file or
/// diagonal.
pub(crate) fn between(a: Square, b: Square) -> Option<Bitboard> {
    let d = Dir::between(a, b)? as usize;
    Some((RAYS[d][a.index()] ^ RAYS[d][b.index()]) & !Bitboard::from_square(b))
}

/// Whether `a` and `b` are different squares that touch, diagonals included.
pub(crate) fn adjacent(a: Square, b: Square) -> bool {
    king_attacks(a).contains(b)
}

#[cfg(test)]
mod tests {
    use super::*;

    /// The squares a slider on `s` reaches walking one step at a time: the
    /// mailbox way, as the Go engine does it.
    fn naive_slide(s: Square, dirs: &[Dir], blocked: Bitboard) -> Bitboard {
        let mut out = Bitboard::EMPTY;
        for d in dirs {
            let (df, dr) = d.step();
            let mut at = s.offset(df, dr);
            while let Some(t) = at {
                out |= Bitboard::from_square(t);
                if blocked.contains(t) {
                    break;
                }
                at = t.offset(df, dr);
            }
        }
        out
    }

    /// A deterministic spread of blocker patterns, sparse to dense.
    fn patterns() -> impl Iterator<Item = Bitboard> {
        let mut x: u64 = 0x9E37_79B9_7F4A_7C15;
        (0..400).map(move |i| {
            x ^= x << 13;
            x ^= x >> 7;
            x ^= x << 17;
            let y = x.rotate_left(i % 64);
            Bitboard(match i % 4 {
                0 => x & y,
                1 => x,
                2 => x | y,
                _ => x & y & x.rotate_left(17),
            })
        })
    }

    #[test]
    fn rook_attacks_match_naive_walk_for_every_square_and_pattern() {
        for blocked in patterns() {
            for s in Square::all() {
                assert_eq!(
                    rook_attacks(s, blocked),
                    naive_slide(s, &Dir::ROOK, blocked),
                    "{s} {blocked:?}"
                );
            }
        }
    }

    #[test]
    fn bishop_attacks_match_naive_walk_for_every_square_and_pattern() {
        for blocked in patterns() {
            for s in Square::all() {
                assert_eq!(
                    bishop_attacks(s, blocked),
                    naive_slide(s, &Dir::BISHOP, blocked),
                    "{s} {blocked:?}"
                );
            }
        }
    }

    #[test]
    fn knight_on_corner_attacks_two_squares() {
        assert_eq!(
            knight_attacks(Square::A1).into_iter().collect::<Vec<_>>(),
            [Square::C2, Square::B3]
        );
    }

    #[test]
    fn white_pawn_attacks_diagonally_forward() {
        assert_eq!(
            pawn_attacks(Color::White, Square::E4)
                .into_iter()
                .collect::<Vec<_>>(),
            [Square::D5, Square::F5]
        );
    }

    #[test]
    fn between_is_strict_and_only_on_queen_lines() {
        assert_eq!(
            between(Square::A5, Square::D2).map(|b| b.into_iter().collect::<Vec<_>>()),
            Some(vec![Square::C3, Square::B4])
        );
        assert_eq!(between(Square::A1, Square::B3), None);
        assert_eq!(between(Square::A1, Square::A1), None);
        assert_eq!(between(Square::A1, Square::A2), Some(Bitboard::EMPTY));
    }

    #[test]
    fn adjacent_excludes_the_square_itself() {
        assert!(adjacent(Square::E4, Square::D5));
        assert!(!adjacent(Square::E4, Square::E4));
        assert!(!adjacent(Square::A1, Square::H1));
    }
}
