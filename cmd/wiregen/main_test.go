package main

import (
	"os"
	"strings"
	"testing"
)

// The browser imports web/src/lib/wire.gen.ts for the server's JSON types
// and shared numbers. It is written by this command, so it can't drift from
// Go: after changing a wire type, run `go run ./cmd/wiregen`.
func TestGeneratedFileIsCurrent(t *testing.T) {
	want, err := generate("../..")
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile("../../" + outFile)
	if err != nil {
		t.Fatalf("%v (run go run ./cmd/wiregen)", err)
	}
	if string(got) != want {
		t.Fatalf("%s is out of date: run go run ./cmd/wiregen", outFile)
	}
}

func TestFieldTypes(t *testing.T) {
	out, err := generate("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"export type Color = 'white' | 'black';",
		"export type Status = 'waiting' | 'playing' | 'over';",
		"\tyou: Color | 'spectator';",               // a ts tag
		"\tstatus: Status;",                         // a named Go type
		"\tboard: string[];",                        // an array
		"\tresult: ResultJSON | null;",              // a pointer: null when empty
		"\twinner?: Color;",                         // omitempty: left out when empty
		"\tsaved?: boolean;",                        // a pointer with omitempty: left out, never null
		"\tname: string | null;",                    // MeJSON's name before the guest has played
		"\tlast: MoveJSON | null;",                  // Live's latest move
		"export const HOLE_ROUNDS = 3;",             // rules.HoleRounds
		"export const HOLE_CAP = 5;",                // rules.HoleCap
		"/** turns played; a move must quote it */", // the Go comment comes along
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated file has no %q", want)
		}
	}
}
