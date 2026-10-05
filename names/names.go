// Package names generates random New York–flavored guest names like
// "pizza-rat-astoria". Names are display-only: they don't need to be unique.
package names

import (
	"math/rand/v2"
	"strings"
)

// MaxLen is the longest name New returns. Where even that doesn't fit (a
// phone's live-game card), the page cuts the name with an ellipsis.
const MaxLen = 24

// MaxWords is the most words a name has, so a two-word noun takes a
// one-word place: "chopped-cheese-manhattan", never
// "chopped-cheese-jackson-heights".
const MaxWords = 3

// Random returns a random name using the global source.
func Random() string { return New(rand.N[int]) }

// New returns a random name, picking indexes with intn(n) in [0, n).
// Tests pass a seeded source; Random uses the global one.
func New(intn func(n int) int) string {
	for {
		if name := nouns[intn(len(nouns))] + "-" + places[intn(len(places))]; ok(name) {
			return name
		}
	}
}

// ok reports whether a noun-place pair may be used as a name.
func ok(name string) bool {
	return len(name) <= MaxLen && strings.Count(name, "-") < MaxWords && !blocked[name]
}
