package names

import (
	"math/rand/v2"
	"regexp"
	"strings"
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
			if ok(n + "-" + p) {
				fit++
			}
		}
	}
	if fit < 2000 {
		t.Errorf("only %d names fit in %d chars and %d words; want at least 2000", fit, MaxLen, MaxWords)
	}
	t.Logf("%d possible names", fit)
}

func TestNew(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	for range 10000 {
		name := New(r.IntN)
		if len(name) > MaxLen || strings.Count(name, "-") >= MaxWords || blocked[name] || !slug.MatchString(name) {
			t.Fatalf("bad name %q", name)
		}
	}
}

func TestRandom(t *testing.T) {
	if Random() == "" {
		t.Fatal("empty name")
	}
}

// A blocked pair that no noun and place can make is a typo, and blocks nothing.
func TestBlockedPairsExist(t *testing.T) {
	pairs := map[string]bool{}
	for _, n := range nouns {
		for _, p := range places {
			pairs[n+"-"+p] = true
		}
	}
	for name := range blocked {
		if !pairs[name] {
			t.Errorf("blocked %q is not a noun-place pair", name)
		}
	}
}
