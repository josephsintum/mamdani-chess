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
			if ok(p + "-" + n) {
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
			pairs[p+"-"+n] = true
		}
	}
	for name := range blocked {
		if !pairs[name] {
			t.Errorf("blocked %q is not a place-noun pair", name)
		}
	}
}

// The place comes first, the way New Yorkers say it: "harlem-cheesecake".
func TestPlaceComesFirst(t *testing.T) {
	first := func(int) int { return 0 }
	if got, want := New(first), places[0]+"-"+nouns[0]; got != want {
		t.Fatalf("New = %q, want %q", got, want)
	}
}

// With the place first, a person word reads as a label for the people who
// live there, so these never appear.
func TestStereotypePairsAreBlocked(t *testing.T) {
	for _, name := range []string{
		"chinatown-yapper", "harlem-hustler", "flushing-pigeon",
		"williamsburg-landlord", "crown-heights-kvetch", "jackson-heights-cabbie",
		"bensonhurst-vercetti", "elmhurst-npc", "harlem-raccoon",
		"chinatown-rat-czar", "chinatown-hot-dog", "flushing-papaya-dog",
		"sunset-park-dog-walker",
	} {
		if !blocked[name] {
			t.Errorf("%q is not blocked", name)
		}
	}
	// Harmless pairs with the same places stay possible.
	for _, name := range []string{"chinatown-dumpling", "harlem-cheesecake", "flatbush-bike-lane"} {
		if blocked[name] {
			t.Errorf("%q is blocked", name)
		}
	}
}

// Plain "rat" is gone: it also means snitch, so "bronx-rat" read as an
// insult. pizza-rat and rat-czar keep the joke.
func TestNoPlainRat(t *testing.T) {
	for _, n := range nouns {
		if n == "rat" {
			t.Fatal(`nouns still has "rat"`)
		}
	}
}
