package store

import "strings"

// Count is one labelled count, a row of a list or a histogram's column.
type Count struct {
	Label string
	N     int
}

// finished are the results of a game that was played to the end (not
// aborted, expired or retired); finishedReasons is the same list in SQL.
var (
	finished        = []string{"checkmate", "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout", "timeout_vs_insufficient", "resignation"}
	finishedReasons = "('" + strings.Join(finished, "','") + "')"
)
