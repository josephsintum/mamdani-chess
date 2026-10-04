package rules

// EventKind names one step of a turn, in the order RULES.md plays them.
type EventKind string

const (
	Moved         EventKind = "moved"          // Move, Piece (MamdaniPiece for a Mamdani move), Color = mover
	Captured      EventKind = "captured"       // Square, Piece
	PotholeClosed EventKind = "pothole_closed" // Square
	Repaired      EventKind = "repaired"       // Square: by the repair step, or a new pothole next to the Mamdani
	RolledPothole EventKind = "rolled_pothole" // Roll: odd = nothing, even = a pothole opens
	Target        EventKind = "target"         // Square picked by the two placement d8s
	Reroll        EventKind = "reroll"         // Square, Reason
	SavingRoll    EventKind = "saving_roll"    // Square, Piece, Roll, Saved, Color = who rolls
	Fell          EventKind = "fell"           // Square, Piece
	PotholeOpened EventKind = "pothole_opened" // Square, Color = roller
	NoPothole     EventKind = "no_pothole"     // re-roll cap reached
)

// RerollReason says why placement dice were re-rolled.
type RerollReason string

const (
	ReasonKing      RerollReason = "king"      // kings never fall
	ReasonPothole   RerollReason = "pothole"   // already a pothole
	ReasonExposes   RerollReason = "exposes"   // the fall would leave the roller in check
	ReasonCheckmate RerollReason = "checkmate" // the result would checkmate the next player
)

// Event is one thing that happened during a turn. Fields not listed for a
// Kind are zero.
type Event struct {
	Kind   EventKind
	Move   Move
	Square Square
	Piece  Piece
	Color  Color
	Roll   int
	Saved  bool
	Reason RerollReason
}
