// Package names generates random New York–flavored guest names like
// "pizza-rat-astoria". Names are display-only: they don't need to be unique.
package names

import "math/rand/v2"

// MaxLen matches the guest name limit enforced by PATCH /api/me.
const MaxLen = 20

// Random returns a random name using the global source.
func Random() string { return New(rand.N[int]) }

// New returns a random name, picking indexes with intn(n) in [0, n).
// Tests pass a seeded source; Random uses the global one.
func New(intn func(n int) int) string {
	for {
		name := nouns[intn(len(nouns))] + "-" + places[intn(len(places))]
		if len(name) <= MaxLen && !blocked[name] {
			return name
		}
	}
}
