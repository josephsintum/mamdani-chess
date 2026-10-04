package game

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	for range 1000 {
		c := NewCode()
		if len(c) != 6 || strings.ContainsAny(c, "0O1IL") {
			t.Fatalf("bad code %q", c)
		}
	}
}

func TestCryptoDice(t *testing.T) {
	seen := map[int]bool{}
	for range 2000 {
		r := CryptoDice{}.D8()
		if r < 1 || r > 8 {
			t.Fatalf("roll %d", r)
		}
		seen[r] = true
	}
	if len(seen) != 8 {
		t.Errorf("only saw %v in 2000 rolls", seen)
	}
}
