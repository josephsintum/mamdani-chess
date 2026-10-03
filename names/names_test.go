package names

import (
	"math/rand/v2"
	"regexp"
	"testing"
)

var slug = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

func TestWordLists(t *testing.T) {
	for label, list := range map[string][]string{"nouns": nouns, "places": places} {
		seen := map[string]bool{}
		for _, w := range list {
			if !slug.MatchString(w) {
				t.Errorf("%s: %q is not a lowercase slug", label, w)
			}
			if seen[w] {
				t.Errorf("%s: %q is listed twice", label, w)
			}
			seen[w] = true
		}
	}
}

func TestEnoughNamesFit(t *testing.T) {
	fit := 0
	for _, n := range nouns {
		for _, p := range places {
			if name := n + "-" + p; len(name) <= MaxLen && !blocked[name] {
				fit++
			}
		}
	}
	if fit < 2000 {
		t.Errorf("only %d names fit in %d chars; want at least 2000", fit, MaxLen)
	}
	t.Logf("%d possible names", fit)
}

func TestNew(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 10000 {
		name := New(r.IntN)
		if len(name) > MaxLen || blocked[name] || !slug.MatchString(name) {
			t.Fatalf("bad name %q", name)
		}
	}
}

func TestRandom(t *testing.T) {
	if Random() == "" {
		t.Fatal("empty name")
	}
}
