package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"testing"
	"time"
)

// The guest cookie works as a login, so only its hash may leave the HTTP
// layer: the database (and the seats and logs) hold SHA-256(cookie).
func TestOnlyTheCookiesHashIsStored(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	code := alice.create()
	u, _ := url.Parse(ts.URL)
	var cookie string
	for _, c := range alice.c.Jar.Cookies(u) {
		if c.Name == "guest" {
			cookie = c.Value
		}
	}
	if len(cookie) != 32 {
		t.Fatalf("guest cookie %q", cookie)
	}
	sum := sha256.Sum256([]byte(cookie))
	want := hex.EncodeToString(sum[:])
	saved, err := s.store.LoadForRestore(t.Context(), time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved) != 1 || saved[0].Code != code || saved[0].White != want {
		t.Fatalf("saved %+v, want white = SHA-256 of the cookie %s", saved, want)
	}
	// The same cookie is still the same guest.
	if v := alice.stream(code).state(); v.You != "white" {
		t.Fatalf("alice is %s in her own game, want white", v.You)
	}
}
