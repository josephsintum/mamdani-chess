//! Squares, colours and pieces.

use std::fmt;
use std::num::NonZeroU8;
use std::str::FromStr;

/// A side. The Mamdani has no colour.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Color {
    White = 0,
    Black = 1,
}

impl Color {
    pub const ALL: [Color; 2] = [Color::White, Color::Black];

    /// The opposing colour.
    #[must_use]
    pub const fn other(self) -> Color {
        match self {
            Color::White => Color::Black,
            Color::Black => Color::White,
        }
    }

    #[must_use]
    pub const fn index(self) -> usize {
        self as usize
    }

    /// "white" or "black", as on the wire.
    #[must_use]
    pub const fn name(self) -> &'static str {
        match self {
            Color::White => "white",
            Color::Black => "black",
        }
    }

    /// Which way this colour's pawns move along the ranks.
    pub(crate) const fn pawn_dir(self) -> i8 {
        match self {
            Color::White => 1,
            Color::Black => -1,
        }
    }
}

impl fmt::Display for Color {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.name())
    }
}

/// A piece type. The Mamdani is not a `Kind`: it is never on the board as a
/// piece (see [`crate::Occupant`]).
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Kind {
    Pawn = 0,
    Knight = 1,
    Bishop = 2,
    Rook = 3,
    Queen = 4,
    King = 5,
}

impl Kind {
    pub const ALL: [Kind; 6] = [
        Kind::Pawn,
        Kind::Knight,
        Kind::Bishop,
        Kind::Rook,
        Kind::Queen,
        Kind::King,
    ];

    #[must_use]
    pub const fn index(self) -> usize {
        self as usize
    }

    /// The upper-case letter used in piece codes and SAN: P N B R Q K.
    #[must_use]
    pub const fn letter(self) -> char {
        match self {
            Kind::Pawn => 'P',
            Kind::Knight => 'N',
            Kind::Bishop => 'B',
            Kind::Rook => 'R',
            Kind::Queen => 'Q',
            Kind::King => 'K',
        }
    }
}

/// What a pawn may promote to. Being its own type means a promotion to a
/// king or pawn can't be written down.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub enum Promo {
    Queen,
    Rook,
    Bishop,
    Knight,
}

impl Promo {
    /// In the order moves are generated.
    pub const ALL: [Promo; 4] = [Promo::Queen, Promo::Rook, Promo::Bishop, Promo::Knight];

    #[must_use]
    pub const fn kind(self) -> Kind {
        match self {
            Promo::Queen => Kind::Queen,
            Promo::Rook => Kind::Rook,
            Promo::Bishop => Kind::Bishop,
            Promo::Knight => Kind::Knight,
        }
    }

    /// The UCI suffix: q r b n.
    #[must_use]
    pub const fn uci(self) -> &'static str {
        match self {
            Promo::Queen => "q",
            Promo::Rook => "r",
            Promo::Bishop => "b",
            Promo::Knight => "n",
        }
    }

    /// Parses a UCI suffix.
    #[must_use]
    pub fn from_uci(s: &str) -> Option<Promo> {
        Promo::ALL.into_iter().find(|p| p.uci() == s)
    }
}

/// A coloured piece, packed into one non-zero byte so `Option<Piece>` is a
/// byte too.
#[derive(Clone, Copy, PartialEq, Eq, Hash)]
pub struct Piece(NonZeroU8);

const PIECE_CODES: [&str; 12] = [
    "wP", "wN", "wB", "wR", "wQ", "wK", "bP", "bN", "bB", "bR", "bQ", "bK",
];

impl Piece {
    #[must_use]
    pub const fn new(color: Color, kind: Kind) -> Piece {
        match NonZeroU8::new(1 + color as u8 * 6 + kind as u8) {
            Some(n) => Piece(n),
            None => unreachable!(),
        }
    }

    const fn slot(self) -> usize {
        (self.0.get() - 1) as usize
    }

    #[must_use]
    pub const fn color(self) -> Color {
        if self.slot() < 6 {
            Color::White
        } else {
            Color::Black
        }
    }

    #[must_use]
    pub const fn kind(self) -> Kind {
        Kind::ALL[self.slot() % 6]
    }

    /// "wP", "bK" and so on, as on the wire.
    #[must_use]
    pub const fn code(self) -> &'static str {
        PIECE_CODES[self.slot()]
    }
}

impl fmt::Debug for Piece {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.code())
    }
}

/// A square, 0..64: a1 = 0, b1 = 1, ..., h8 = 63.
#[derive(Clone, Copy, PartialEq, Eq, Hash, PartialOrd, Ord)]
pub struct Square(u8);

#[rustfmt::skip]
const SQUARE_NAMES: [&str; 64] = [
    "a1", "b1", "c1", "d1", "e1", "f1", "g1", "h1",
    "a2", "b2", "c2", "d2", "e2", "f2", "g2", "h2",
    "a3", "b3", "c3", "d3", "e3", "f3", "g3", "h3",
    "a4", "b4", "c4", "d4", "e4", "f4", "g4", "h4",
    "a5", "b5", "c5", "d5", "e5", "f5", "g5", "h5",
    "a6", "b6", "c6", "d6", "e6", "f6", "g6", "h6",
    "a7", "b7", "c7", "d7", "e7", "f7", "g7", "h7",
    "a8", "b8", "c8", "d8", "e8", "f8", "g8", "h8",
];

macro_rules! named_squares {
    ($($name:ident = $index:expr),* $(,)?) => {
        impl Square {
            $(pub const $name: Square = Square($index);)*
        }
    };
}

#[rustfmt::skip]
named_squares! {
    A1 = 0, B1 = 1, C1 = 2, D1 = 3, E1 = 4, F1 = 5, G1 = 6, H1 = 7,
    A2 = 8, B2 = 9, C2 = 10, D2 = 11, E2 = 12, F2 = 13, G2 = 14, H2 = 15,
    A3 = 16, B3 = 17, C3 = 18, D3 = 19, E3 = 20, F3 = 21, G3 = 22, H3 = 23,
    A4 = 24, B4 = 25, C4 = 26, D4 = 27, E4 = 28, F4 = 29, G4 = 30, H4 = 31,
    A5 = 32, B5 = 33, C5 = 34, D5 = 35, E5 = 36, F5 = 37, G5 = 38, H5 = 39,
    A6 = 40, B6 = 41, C6 = 42, D6 = 43, E6 = 44, F6 = 45, G6 = 46, H6 = 47,
    A7 = 48, B7 = 49, C7 = 50, D7 = 51, E7 = 52, F7 = 53, G7 = 54, H7 = 55,
    A8 = 56, B8 = 57, C8 = 58, D8 = 59, E8 = 60, F8 = 61, G8 = 62, H8 = 63,
}

impl Square {
    /// The square with this index, if it is on the board.
    #[must_use]
    pub const fn new(index: u8) -> Option<Square> {
        if index < 64 {
            Some(Square(index))
        } else {
            None
        }
    }

    /// The square on file 0..8 (a..h) and rank 0..8 (1..8).
    #[must_use]
    pub const fn from_coords(file: u8, rank: u8) -> Option<Square> {
        if file < 8 && rank < 8 {
            Some(Square(rank * 8 + file))
        } else {
            None
        }
    }

    /// Every square, a1 to h8.
    pub fn all() -> impl Iterator<Item = Square> {
        (0..64).map(Square)
    }

    #[must_use]
    pub const fn index(self) -> usize {
        self.0 as usize
    }

    /// 0..8 for files a..h.
    #[must_use]
    pub const fn file(self) -> u8 {
        self.0 % 8
    }

    /// 0..8 for ranks 1..8.
    #[must_use]
    pub const fn rank(self) -> u8 {
        self.0 / 8
    }

    /// The square `df` files and `dr` ranks away, if that is on the board.
    #[must_use]
    pub const fn offset(self, df: i8, dr: i8) -> Option<Square> {
        let f = self.file() as i8 + df;
        let r = self.rank() as i8 + dr;
        if f < 0 || f > 7 || r < 0 || r > 7 {
            None
        } else {
            Some(Square((r * 8 + f) as u8))
        }
    }

    /// "a1".."h8".
    #[must_use]
    pub const fn name(self) -> &'static str {
        SQUARE_NAMES[self.0 as usize]
    }
}

impl fmt::Display for Square {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.name())
    }
}

impl fmt::Debug for Square {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        f.write_str(self.name())
    }
}

/// A square name that isn't "a1".."h8".
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("bad square {0:?}")]
pub struct ParseSquareError(pub String);

impl FromStr for Square {
    type Err = ParseSquareError;

    fn from_str(s: &str) -> Result<Square, ParseSquareError> {
        match s.as_bytes() {
            &[f @ b'a'..=b'h', r @ b'1'..=b'8'] => Ok(Square((r - b'1') * 8 + (f - b'a'))),
            _ => Err(ParseSquareError(s.to_owned())),
        }
    }
}
