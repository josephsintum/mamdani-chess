package store

// Count is one labelled count, a row of a list or a histogram's column.
type Count struct {
	Label string
	N     int
}

// finishedReasons are the results of a game that was played to the end
// (not aborted, expired or retired).
const finishedReasons = `('checkmate','stalemate','fifty_moves','repetition','insufficient_material','timeout','timeout_vs_insufficient','resignation')`
