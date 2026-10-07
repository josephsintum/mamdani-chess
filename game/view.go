package game

import (
	"cmp"
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"mamdani-chess/rules"
)

// Status is where a game is in its life.
type Status string

const (
	Waiting Status = "waiting" // Black's seat is still empty
	Playing Status = "playing"
	Over    Status = "over"
)

// View is one subscriber's picture of the game, sent as the SSE "state"
// event after every change. It is the whole game, so a reconnecting browser
// needs nothing else. Views are shared between streams: read them, never
// change them.
type View struct {
	Code     string      `json:"code"`
	Status   Status      `json:"status"`
	You      string      `json:"you" ts:"Color | 'spectator'"` // "white", "black" or "spectator"
	Board    [64]string  `json:"board"`                        // index 0 = a1; "" or color+kind: "wP", "bQ"
	Mamdani  string      `json:"mamdani"`                      // its square, "" once it has fallen
	Potholes []Pothole   `json:"potholes"`
	Turn     string      `json:"turn" ts:"Color"` // "white" or "black"
	Check    bool        `json:"check"`
	Legal    []MoveJSON  `json:"legal"` // only for the player to move
	Last     []EventJSON `json:"last"`  // what happened on the latest turn, in order
	Log      []LogEntry  `json:"log"`   // one entry per turn
	Lost     LostJSON    `json:"lost"`  // pieces each side has lost to potholes
	Stats    StatsJSON   `json:"stats"`
	Result   *ResultJSON `json:"result"`
	Seq      int         `json:"seq"` // turns played; a move must quote it
	Clock    ClockJSON   `json:"clock"`
	Online   OnlineJSON  `json:"online"`  // which players have the game open
	Players  PlayersJSON `json:"players"` // names; "" for a seat that is empty or was saved without one
	Rematch  RematchJSON `json:"rematch"` // only once the game is over

	data []byte // the encoded view, set once before it is shared (see JSON)
}

// Pothole is an open pothole, the color that rolled it and its rounds left:
// how many more of By's moves it stays open for (1 to rules.HoleRounds).
type Pothole struct {
	Sq   string `json:"sq"`
	By   string `json:"by" ts:"Color"`
	Left int    `json:"left"`
}

// potholesJSON lists p's open potholes, oldest first, so the next one the
// cap would close comes first. It is never nil: no holes encodes as [].
func potholesJSON(p *rules.Position) []Pothole {
	holes := make([]rules.Hole, 0, rules.HoleCap)
	for _, h := range p.Potholes {
		if h.Sq != rules.NoSquare {
			holes = append(holes, h)
		}
	}
	slices.SortFunc(holes, func(a, b rules.Hole) int { return cmp.Compare(a.Seq, b.Seq) })
	out := make([]Pothole, len(holes))
	for i, h := range holes {
		out[i] = Pothole{Sq: h.Sq.String(), By: colorName(h.By), Left: int(h.Left)}
	}
	return out
}

// MoveJSON is a move on the wire. Promo is "", "q", "r", "b" or "n".
type MoveJSON struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Promo string `json:"promo,omitempty"`
}

// EventJSON is one step of a turn. Only the fields that apply to Kind are set.
type EventJSON struct {
	Kind   rules.EventKind    `json:"kind"`
	Sq     string             `json:"sq,omitempty"`
	From   string             `json:"from,omitempty"`
	To     string             `json:"to,omitempty"`
	Promo  string             `json:"promo,omitempty"`
	Piece  string             `json:"piece,omitempty"`
	Color  string             `json:"color,omitempty" ts:"Color"`
	Roll   int                `json:"roll,omitempty"`
	Saved  *bool              `json:"saved,omitempty"`
	Reason rules.RerollReason `json:"reason,omitempty"`
}

// LogEntry is one turn in the move log: the move in algebraic notation and
// what the dice did, e.g. {"e4", "white", "d8 4 → c3"}.
type LogEntry struct {
	SAN   string `json:"san"`
	Color string `json:"color" ts:"Color"`
	Dice  string `json:"dice"`
}

// LostJSON lists the pieces each side has lost to potholes, as piece codes.
type LostJSON struct {
	White []string `json:"white"`
	Black []string `json:"black"`
}

// StatsJSON counts what the dice and the Mamdani have done this game.
type StatsJSON struct {
	SavingRolls int  `json:"savingRolls"`
	Saved       int  `json:"saved"`
	Repaired    int  `json:"repaired"` // potholes the Mamdani fixed
	MamdaniFell bool `json:"mamdaniFell"`
}

// PlayersJSON is each seat's name, as it was when its player sat down.
type PlayersJSON struct {
	White string `json:"white"`
	Black string `json:"black"`
}

// OnlineJSON says which players have a stream open.
type OnlineJSON struct {
	White bool `json:"white"`
	Black bool `json:"black"`
}

// ResultJSON is how the game ended. Winner is "" for a draw, and for an
// aborted or expired game, which nobody wins.
type ResultJSON struct {
	Winner string       `json:"winner,omitempty" ts:"Color"` // "" for a draw
	Draw   bool         `json:"draw"`
	Reason rules.Reason `json:"reason"`
}

// resultJSON is r on the wire, or nil while the game is on.
func resultJSON(r rules.Result) *ResultJSON {
	if !r.Over {
		return nil
	}
	return &ResultJSON{Winner: winnerName(r), Draw: r.Draw, Reason: r.Reason}
}

// boardJSON is p's board as piece codes, index 0 = a1.
func boardJSON(p *rules.Position) (b [64]string) {
	for s, pc := range p.Board {
		b[s] = pieceCode(pc)
	}
	return b
}

var kindLetters = map[rules.Kind]string{
	rules.Pawn: "P", rules.Knight: "N", rules.Bishop: "B",
	rules.Rook: "R", rules.Queen: "Q", rules.King: "K",
}

// pieceCode returns "wP", "bK" and so on, "M" for the Mamdani, "" for none.
func pieceCode(p rules.Piece) string { return pieceCodes[p] }

// pieceCodes holds every piece's code, so filling a board never allocates.
// NoPiece's slot stays "".
var pieceCodes = func() (codes [16]string) {
	for k, letter := range kindLetters {
		for _, c := range []rules.Color{rules.White, rules.Black} {
			codes[rules.NewPiece(c, k)] = colorName(c)[:1] + letter
		}
	}
	codes[rules.MamdaniPiece] = "M"
	return codes
}()

func colorName(c rules.Color) string { return c.String() }

func squareName(s rules.Square) string {
	if s == rules.NoSquare {
		return ""
	}
	return s.String()
}

// promoLetters is indexed by kind, not a map: moveJSON runs for every
// legal move in a view.
var promoLetters = [rules.Mamdani + 1]string{
	rules.Queen: "q", rules.Rook: "r", rules.Bishop: "b", rules.Knight: "n",
}

func moveJSON(m rules.Move) MoveJSON {
	return MoveJSON{From: m.From.String(), To: m.To.String(), Promo: promoLetters[m.Promo]}
}

// ParseMove turns a wire move back into a rules.Move. Each field is checked
// for length first, so a square split across From and To isn't accepted.
func ParseMove(m MoveJSON) (rules.Move, bool) {
	if len(m.From) != 2 || len(m.To) != 2 || len(m.Promo) > 1 {
		return rules.Move{}, false
	}
	mv, err := rules.ParseMove(m.From + m.To + m.Promo)
	return mv, err == nil
}

func eventJSON(e rules.Event) EventJSON {
	j := EventJSON{Kind: e.Kind}
	switch e.Kind {
	case rules.Moved:
		mv := moveJSON(e.Move)
		j.From, j.To, j.Promo = mv.From, mv.To, mv.Promo
		j.Piece, j.Color = pieceCode(e.Piece), colorName(e.Color)
	case rules.Captured, rules.Fell:
		j.Sq, j.Piece = squareName(e.Square), pieceCode(e.Piece)
	case rules.Repaired, rules.Target:
		j.Sq = squareName(e.Square)
	case rules.PotholeClosed, rules.PotholeOpened:
		j.Sq, j.Color = squareName(e.Square), colorName(e.Color)
	case rules.RolledPothole:
		j.Roll, j.Color = e.Roll, colorName(e.Color)
	case rules.Reroll:
		j.Sq, j.Reason = squareName(e.Square), e.Reason
	case rules.SavingRoll:
		j.Sq, j.Piece, j.Roll, j.Saved, j.Color = squareName(e.Square), pieceCode(e.Piece), e.Roll, new(e.Saved), colorName(e.Color)
	}
	return j
}

// describe sums up what the dice did on a turn, e.g. "d8 4 → d3 · save 5 ✓".
// A hole closing after the roll is the cap making room ("c4 closes"); holes
// that count down to zero close before the roll and are left out, as every
// turn has them.
func describe(events []rules.Event) string {
	var parts []string
	rolled := false
	for _, e := range events {
		last := len(parts) - 1
		switch e.Kind {
		case rules.Repaired:
			if rolled {
				parts[last] += " repaired"
			} else {
				parts = append(parts, "repairs "+e.Square.String())
			}
		case rules.PotholeClosed:
			if rolled {
				parts = append(parts, e.Square.String()+" closes")
			}
		case rules.RolledPothole:
			rolled = true
			parts = append(parts, "d8 "+strconv.Itoa(e.Roll))
		case rules.Target:
			parts[last] += " → " + e.Square.String()
		case rules.Reroll:
			parts[last] += " (re-roll: " + string(e.Reason) + ")"
		case rules.SavingRoll:
			mark := "✗"
			if e.Saved {
				mark = "✓"
			}
			parts = append(parts, "save "+strconv.Itoa(e.Roll)+" "+mark)
		case rules.Fell:
			parts = append(parts, pieceCode(e.Piece)+" falls")
		case rules.NoPothole:
			parts = append(parts, "no pothole")
		}
	}
	return strings.Join(parts, " · ")
}

// JSON returns v encoded as JSON. A view sent on streams was encoded once,
// by the game, before it was shared, so every stream in a role writes the
// same bytes instead of encoding the view again.
func (v *View) JSON() []byte {
	if v.data != nil {
		return v.data
	}
	b, _ := json.Marshal(v) // a View is plain data: it always encodes
	return b
}
