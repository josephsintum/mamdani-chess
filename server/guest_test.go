package server

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
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

// The app shell sets the guest cookie, so a fresh browser's first requests
// (the visit beacon and a game's stream, which race) all carry the same one.
// A browser that has the cookie keeps it.
func TestTheAppShellSetsTheGuestCookie(t *testing.T) {
	_, ts := newTestServer(t)
	for _, path := range []string{"/", "/game/ABCDEF", "/practice"} {
		alice := newPlayer(t, ts)
		resp, err := alice.c.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		set := resp.Cookies()
		if len(set) != 1 || set[0].Name != guestCookie || len(set[0].Value) != 32 || !set[0].HttpOnly {
			t.Fatalf("GET %s on a fresh browser set %v, want one guest cookie", path, set)
		}
		// A shared cache must never keep it: it would hand one guest to many.
		if cc := resp.Header.Get("Cache-Control"); cc != "private, no-cache" {
			t.Errorf("GET %s: Cache-Control %q, want private, no-cache", path, cc)
		}
		resp, err = alice.c.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if again := resp.Header.Values("Set-Cookie"); again != nil {
			t.Errorf("GET %s with the cookie set another: %q", path, again)
		}
		// The API sees the guest the shell made.
		if status, _ := alice.post("/api/visit", `{"path":"`+path+`"}`); status != http.StatusNoContent {
			t.Errorf("POST /api/visit: %d", status)
		}
		u, _ := url.Parse(ts.URL)
		if got := alice.c.Jar.Cookies(u); len(got) != 1 || got[0].Value != set[0].Value {
			t.Errorf("after the beacon the browser holds %v, want the shell's %s", got, set[0].Value)
		}
	}
}
