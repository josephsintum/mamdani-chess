//! A position: pieces, the Mamdani, open potholes and the usual chess state.

use crate::bitboard::{Bitboard, adjacent};
use crate::{Color, Kind, Piece, Square};

/// A set of castling rights.
#[derive(Clone, Copy, Debug, Default, PartialEq, Eq, Hash)]
pub struct Castling(u8);

impl Castling {
    pub const NONE: Castling = Castling(0);
    pub const WHITE_KINGSIDE: Castling = Castling(1);
    pub const WHITE_QUEENSIDE: Castling = Castling(2);
    pub const BLACK_KINGSIDE: Castling = Castling(4);
    pub const BLACK_QUEENSIDE: Castling = Castling(8);
    pub const ALL: Castling = Castling(15);

    #[must_use]
    pub const fn contains(self, other: Castling) -> bool {
        self.0 & other.0 == other.0
    }

    #[must_use]
    pub const fn union(self, other: Castling) -> Castling {
        Castling(self.0 | other.0)
    }

    pub(crate) fn remove(&mut self, other: Castling) {
        self.0 &= !other.0;
    }

    pub(crate) const fn kingside(c: Color) -> Castling {
        match c {
            Color::White => Castling::WHITE_KINGSIDE,
            Color::Black => Castling::BLACK_KINGSIDE,
        }
    }

    pub(crate) const fn queenside(c: Color) -> Castling {
        match c {
            Color::White => Castling::WHITE_QUEENSIDE,
            Color::Black => Castling::BLACK_QUEENSIDE,
        }
    }

    /// The rights that disappear when a piece leaves, is captured on or
    /// falls from `s`.
    pub(crate) fn lost_at(s: Square) -> Castling {
        match s {
            Square::E1 => Castling::WHITE_KINGSIDE.union(Castling::WHITE_QUEENSIDE),
            Square::H1 => Castling::WHITE_KINGSIDE,
            Square::A1 => Castling::WHITE_QUEENSIDE,
            Square::E8 => Castling::BLACK_KINGSIDE.union(Castling::BLACK_QUEENSIDE),
            Square::H8 => Castling::BLACK_KINGSIDE,
            Square::A8 => Castling::BLACK_QUEENSIDE,
            _ => Castling::NONE,
        }
    }
}

/// Everything needed to generate moves and resolve a turn. It is `Copy` and
/// small, so trying a move is a copy and never allocates.
///
/// The pieces live twice: as bitboards for move generation and as a mailbox
/// for "what is on this square". `put` and `take` keep the two in step.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Position {
    board: [Option<Piece>; 64],
    by_color: [Bitboard; 2],
    by_kind: [Bitboard; 6],
    pub(crate) mamdani: Option<Square>,
    /// The open pothole each colour rolled. Each has at most one: theirs
    /// closes on their next move, before they can roll again.
    pub(crate) potholes: [Option<Square>; 2],
    pub(crate) turn: Color,
    pub(crate) castling: Castling,
    pub(crate) ep: Option<Square>,
    pub(crate) halfmove: u16,
    pub(crate) fullmove: u16,
}

/// Identifies a position for threefold repetition: pieces, Mamdani, open
/// potholes, side to move, castling rights and en passant square.
#[derive(Clone, Copy, Debug, PartialEq, Eq, Hash)]
pub struct Key {
    by_color: [Bitboard; 2],
    by_kind: [Bitboard; 6],
    mamdani: Option<Square>,
    potholes: [Option<Square>; 2],
    turn: Color,
    castling: Castling,
    ep: Option<Square>,
}

/// FEN that doesn't parse.
#[derive(Debug, Clone, PartialEq, Eq, thiserror::Error)]
#[error("fen {fen:?}: {problem}")]
pub struct FenError {
    fen: String,
    problem: &'static str,
}

/// The standard chess start, without the Mamdani.
pub const START_FEN: &str = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1";

impl Position {
    fn empty() -> Position {
        Position {
            board: [None; 64],
            by_color: [Bitboard::EMPTY; 2],
            by_kind: [Bitboard::EMPTY; 6],
            mamdani: None,
            potholes: [None; 2],
            turn: Color::White,
            castling: Castling::NONE,
            ep: None,
            halfmove: 0,
            fullmove: 1,
        }
    }

    /// The standard chess setup with the Mamdani on a5.
    #[must_use]
    pub fn start() -> Position {
        let mut p = Position::empty();
        for (file, kind) in [
            Kind::Rook,
            Kind::Knight,
            Kind::Bishop,
            Kind::Queen,
            Kind::King,
            Kind::Bishop,
            Kind::Knight,
            Kind::Rook,
        ]
        .into_iter()
        .enumerate()
        {
            let file = file as u8;
            for (color, back, front) in [(Color::White, 0, 1), (Color::Black, 7, 6)] {
                if let (Some(b), Some(f)) = (
                    Square::from_coords(file, back),
                    Square::from_coords(file, front),
                ) {
                    p.put(b, Piece::new(color, kind));
                    p.put(f, Piece::new(color, Kind::Pawn));
                }
            }
        }
        p.castling = Castling::ALL;
        p.mamdani = Some(Square::A5);
        p
    }

    /// Reads standard FEN. The result has no Mamdani and no potholes; add
    /// them with [`Position::with_mamdani`] and [`Position::with_potholes`].
    ///
    /// # Errors
    ///
    /// [`FenError`] saying which field is wrong.
    pub fn from_fen(fen: &str) -> Result<Position, FenError> {
        let fail = |problem| FenError {
            fen: fen.to_owned(),
            problem,
        };
        let fields: Vec<&str> = fen.split_whitespace().collect();
        let &[placement, side, castling, ep, halfmove, fullmove] = fields.as_slice() else {
            return Err(fail("want 6 fields"));
        };
        let mut p = Position::empty();
        let ranks: Vec<&str> = placement.split('/').collect();
        if ranks.len() != 8 {
            return Err(fail("want 8 ranks"));
        }
        for (i, row) in ranks.iter().enumerate() {
            let rank = 7 - i as u8;
            let mut file = 0u8;
            for c in row.chars() {
                if let Some(n) = c.to_digit(10).filter(|n| (1..=8).contains(n)) {
                    file = file.saturating_add(n as u8);
                    continue;
                }
                let piece = fen_piece(c).ok_or_else(|| fail("bad piece"))?;
                let s = Square::from_coords(file, rank).ok_or_else(|| fail("rank too long"))?;
                p.put(s, piece);
                file += 1;
            }
            if file != 8 {
                return Err(fail("rank is not 8 squares"));
            }
        }
        p.turn = match side {
            "w" => Color::White,
            "b" => Color::Black,
            _ => return Err(fail("bad side to move")),
        };
        for c in castling.chars() {
            p.castling = p.castling.union(match c {
                'K' => Castling::WHITE_KINGSIDE,
                'Q' => Castling::WHITE_QUEENSIDE,
                'k' => Castling::BLACK_KINGSIDE,
                'q' => Castling::BLACK_QUEENSIDE,
                '-' => Castling::NONE,
                _ => return Err(fail("bad castling")),
            });
        }
        if ep != "-" {
            p.ep = Some(ep.parse().map_err(|_| fail("bad en passant square"))?);
        }
        p.halfmove = halfmove.parse().map_err(|_| fail("bad halfmove clock"))?;
        p.fullmove = fullmove.parse().map_err(|_| fail("bad fullmove number"))?;
        Ok(p)
    }

    /// This position with the Mamdani on `s` (or none).
    #[must_use]
    pub fn with_mamdani(mut self, s: Option<Square>) -> Position {
        self.mamdani = s;
        self
    }

    /// This position with the open potholes White and Black rolled.
    #[must_use]
    pub fn with_potholes(mut self, white: Option<Square>, black: Option<Square>) -> Position {
        self.potholes = [white, black];
        self
    }

    /// This position with the fifty-move count set.
    #[must_use]
    pub fn with_halfmove(mut self, halfmove: u16) -> Position {
        self.halfmove = halfmove;
        self
    }

    #[must_use]
    pub fn piece_at(&self, s: Square) -> Option<Piece> {
        self.board[s.index()]
    }

    /// The Mamdani's square, or `None` once it has fallen.
    #[must_use]
    pub fn mamdani(&self) -> Option<Square> {
        self.mamdani
    }

    /// The open pothole `c` rolled, if any.
    #[must_use]
    pub fn pothole(&self, c: Color) -> Option<Square> {
        self.potholes[c.index()]
    }

    #[must_use]
    pub fn turn(&self) -> Color {
        self.turn
    }

    #[must_use]
    pub fn castling(&self) -> Castling {
        self.castling
    }

    #[must_use]
    pub fn en_passant(&self) -> Option<Square> {
        self.ep
    }

    /// Plies since the last pawn move, capture or fall.
    #[must_use]
    pub fn halfmove(&self) -> u16 {
        self.halfmove
    }

    #[must_use]
    pub fn fullmove(&self) -> u16 {
        self.fullmove
    }

    #[must_use]
    pub fn key(&self) -> Key {
        Key {
            by_color: self.by_color,
            by_kind: self.by_kind,
            mamdani: self.mamdani,
            potholes: self.potholes,
            turn: self.turn,
            castling: self.castling,
            ep: self.ep,
        }
    }

    /// `c`'s king, if it has one (only hand-built test positions don't).
    #[must_use]
    pub fn king(&self, c: Color) -> Option<Square> {
        self.pieces(c, Kind::King).first()
    }

    #[must_use]
    pub fn is_pothole(&self, s: Square) -> bool {
        self.potholes.contains(&Some(s))
    }

    pub(crate) fn pieces(&self, c: Color, k: Kind) -> Bitboard {
        self.by_color[c.index()] & self.by_kind[k.index()]
    }

    pub(crate) fn colored(&self, c: Color) -> Bitboard {
        self.by_color[c.index()]
    }

    pub(crate) fn kind(&self, k: Kind) -> Bitboard {
        self.by_kind[k.index()]
    }

    pub(crate) fn occupied(&self) -> Bitboard {
        self.by_color[0] | self.by_color[1]
    }

    pub(crate) fn potholes_bb(&self) -> Bitboard {
        Bitboard::from_option(self.potholes[0]) | Bitboard::from_option(self.potholes[1])
    }

    /// Every square that stops a slider: pieces, the Mamdani and potholes.
    pub(crate) fn blocked(&self) -> Bitboard {
        self.occupied() | Bitboard::from_option(self.mamdani) | self.potholes_bb()
    }

    pub(crate) fn is_blocked(&self, s: Square) -> bool {
        self.blocked().contains(s)
    }

    pub(crate) fn next_to_mamdani(&self, s: Square) -> bool {
        self.mamdani.is_some_and(|m| adjacent(s, m))
    }

    pub(crate) fn put(&mut self, s: Square, p: Piece) {
        self.take(s);
        self.board[s.index()] = Some(p);
        let bit = Bitboard::from_square(s);
        self.by_color[p.color().index()] |= bit;
        self.by_kind[p.kind().index()] |= bit;
    }

    pub(crate) fn take(&mut self, s: Square) -> Option<Piece> {
        let p = self.board[s.index()].take()?;
        let keep = !Bitboard::from_square(s);
        self.by_color[p.color().index()] &= keep;
        self.by_kind[p.kind().index()] &= keep;
        Some(p)
    }

    /// Takes whatever stands on `s` off the board: a piece or the Mamdani.
    pub(crate) fn remove(&mut self, s: Square) {
        if self.mamdani == Some(s) {
            self.mamdani = None;
        } else {
            self.take(s);
        }
    }
}

fn fen_piece(c: char) -> Option<Piece> {
    let color = if c.is_ascii_uppercase() {
        Color::White
    } else {
        Color::Black
    };
    let kind = match c.to_ascii_lowercase() {
        'p' => Kind::Pawn,
        'n' => Kind::Knight,
        'b' => Kind::Bishop,
        'r' => Kind::Rook,
        'q' => Kind::Queen,
        'k' => Kind::King,
        _ => return None,
    };
    Some(Piece::new(color, kind))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn start_position_matches_start_fen_plus_mamdani() {
        let fen = Position::from_fen(START_FEN)
            .unwrap()
            .with_mamdani(Some(Square::A5));
        assert_eq!(Position::start(), fen);
    }

    #[test]
    fn start_position_has_pieces_mamdani_and_rights() {
        let p = Position::start();
        assert_eq!(
            p.piece_at(Square::E1),
            Some(Piece::new(Color::White, Kind::King))
        );
        assert_eq!(
            p.piece_at(Square::D8),
            Some(Piece::new(Color::Black, Kind::Queen))
        );
        assert_eq!(p.piece_at(Square::E4), None);
        assert_eq!(p.mamdani(), Some(Square::A5));
        assert_eq!(p.potholes, [None, None]);
        assert_eq!(
            (p.turn(), p.castling(), p.en_passant(), p.fullmove()),
            (Color::White, Castling::ALL, None, 1)
        );
    }

    #[test]
    fn from_fen_reads_side_ep_and_clocks() {
        let p = Position::from_fen("4k3/8/8/3Pp3/8/8/8/4K3 b - e6 3 40").unwrap();
        assert_eq!(
            (p.turn(), p.en_passant(), p.halfmove(), p.fullmove()),
            (Color::Black, Some(Square::E6), 3, 40)
        );
        assert_eq!((p.castling(), p.mamdani()), (Castling::NONE, None));
    }

    #[test]
    fn from_fen_rejects_malformed_input() {
        for bad in [
            "",
            "8/8/8/8/8/8/8 w - - 0 1",
            "9/8/8/8/8/8/8/8 w - - 0 1",
            "8/8/8/8/8/8/8/8 x - - 0 1",
            "8/8/8/8/8/8/8/8 w X - 0 1",
            "8/8/8/8/8/8/8/8 w - z9 0 1",
            "8/8/8/8/8/8/8/8 w - - zero 1",
            "rnbqkbnr/ppppxppp/8/8/8/8/8/8 w - - 0 1",
        ] {
            assert!(Position::from_fen(bad).is_err(), "{bad:?} should fail");
        }
    }

    #[test]
    fn piece_packing_round_trips() {
        let p = Piece::new(Color::Black, Kind::Knight);
        assert_eq!(
            (p.color(), p.kind(), p.code()),
            (Color::Black, Kind::Knight, "bN")
        );
        assert_eq!(Piece::new(Color::White, Kind::Pawn).color(), Color::White);
        assert_eq!(std::mem::size_of::<Option<Piece>>(), 1);
    }

    #[test]
    fn squares_parse_and_print() {
        for (name, sq, file, rank) in [
            ("a1", Square::A1, 0, 0),
            ("h1", Square::H1, 7, 0),
            ("e4", Square::E4, 4, 3),
            ("h8", Square::H8, 7, 7),
        ] {
            let s: Square = name.parse().unwrap();
            assert_eq!((s, s.file(), s.rank(), s.name()), (sq, file, rank, name));
        }
        for bad in ["", "i1", "a9", "a10", "A1"] {
            assert!(bad.parse::<Square>().is_err(), "{bad:?} should fail");
        }
        assert_eq!(Square::H1.offset(1, 0), None);
        assert_eq!(Square::A1.offset(0, -1), None);
        assert_eq!(Square::E4.offset(-1, 1), Some(Square::D5));
    }
}
