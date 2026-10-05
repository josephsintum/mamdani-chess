// Command parity plays seeded random games through the Go game server and
// writes what every role saw after every turn, one JSON line per game. The
// Rust server's parity test (server_rs/server/tests/parity.rs) replays the
// same moves with the same dice and checks it shows the same thing.
//
//	go run ./server_rs/parity -n 200 > games.jsonl
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"

	"mamdani-chess/game"
	"mamdani-chess/rules"
)

// recDice rolls from a seeded generator and remembers every roll.
type recDice struct {
	r     *rand.Rand
	rolls []int
}

func (d *recDice) D8() int {
	v := d.r.IntN(8) + 1
	d.rolls = append(d.rolls, v)
	return v
}

// views holds each role's view as JSON, with the log cut to its last entry
// (it only ever grows, so checking the newest entry every turn checks it
// all) to keep the file small.
type views struct {
	White     json.RawMessage `json:"white"`
	Black     json.RawMessage `json:"black"`
	Spectator json.RawMessage `json:"spectator"`
}

type turn struct {
	Guest  string         `json:"guest"`  // "white" or "black": who acts
	Action string         `json:"action"` // "move" or "resign"
	Move   *game.MoveJSON `json:"move,omitempty"`
	Seq    int            `json:"seq"`
	Dice   []int          `json:"dice"`
	Error  string         `json:"error,omitempty"` // set when the game refused
	Views  *views         `json:"views,omitempty"` // after a successful turn
}

type record struct {
	Seed  uint64 `json:"seed"`
	Start views  `json:"start"`
	Turns []turn `json:"turns"`
}

func main() {
	n := flag.Int("n", 50, "games to play")
	seed := flag.Uint64("seed", 1, "seed of the first game")
	plies := flag.Int("plies", 300, "longest game")
	flag.Parse()

	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	enc := json.NewEncoder(out)
	for i := range uint64(*n) {
		rec, err := play(*seed+i, *plies)
		if err != nil {
			fmt.Fprintln(os.Stderr, "parity:", err)
			os.Exit(1)
		}
		if err := enc.Encode(rec); err != nil {
			fmt.Fprintln(os.Stderr, "parity:", err)
			os.Exit(1)
		}
	}
}

func play(seed uint64, plies int) (*record, error) {
	r := rand.New(rand.NewPCG(seed, 2))
	dice := &recDice{r: r}
	g := game.NewHub(dice).Create("white")
	subs := map[string]*game.Sub{}
	cur := map[string]*game.View{}
	for _, guest := range []string{"white", "black", "spectator"} {
		subs[guest] = g.Join(guest)
	}
	snapshot := func() (views, error) {
		for guest, sub := range subs {
			select {
			case v := <-sub.C:
				cur[guest] = v
			default:
			}
		}
		var vs views
		var err error
		for guest, dst := range map[string]*json.RawMessage{"white": &vs.White, "black": &vs.Black, "spectator": &vs.Spectator} {
			if *dst, err = trimmed(cur[guest]); err != nil {
				return vs, err
			}
		}
		return vs, nil
	}

	rec := &record{Seed: seed}
	var err error
	if rec.Start, err = snapshot(); err != nil {
		return nil, err
	}
	for ply := 0; ply < plies; ply++ {
		mover := cur["white"].Turn
		if cur["white"].Status != game.Playing {
			break
		}
		t := turn{Guest: mover, Seq: ply}
		dice.rolls = nil
		switch roll := r.IntN(200); {
		case roll == 0:
			t.Action = "resign"
			err = g.Resign(mover)
		case roll < 5:
			// The other player tries to move out of turn.
			t.Guest = other(mover)
			t.Action = "move"
			m := cur[mover].Legal[0]
			t.Move = &m
			err = g.Move(t.Guest, mustParse(m), ply)
		default:
			t.Action = "move"
			legal := cur[mover].Legal
			m := legal[r.IntN(len(legal))]
			t.Move = &m
			err = g.Move(mover, mustParse(m), ply)
		}
		t.Dice = append([]int{}, dice.rolls...)
		if err != nil {
			t.Error = err.Error()
			ply-- // nothing was played
		} else {
			vs, err := snapshot()
			if err != nil {
				return nil, err
			}
			t.Views = &vs
		}
		rec.Turns = append(rec.Turns, t)
		if t.Action == "resign" && t.Error == "" {
			break
		}
	}
	return rec, nil
}

func other(color string) string {
	if color == "white" {
		return "black"
	}
	return "white"
}

func mustParse(m game.MoveJSON) rules.Move {
	mv, ok := game.ParseMove(m)
	if !ok {
		panic(fmt.Sprintf("legal move %+v does not parse", m))
	}
	return mv
}

// trimmed is v as JSON with its log cut to the last entry.
func trimmed(v *game.View) (json.RawMessage, error) {
	c := *v
	if n := len(c.Log); n > 1 {
		c.Log = c.Log[n-1:]
	}
	return json.Marshal(c)
}
