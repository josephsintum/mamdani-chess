package server

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func (p *player) get(path string) (int, string) {
	p.t.Helper()
	resp, err := p.c.Get(p.url + path)
	if err != nil {
		p.t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

// name is the caller's name from GET /api/me; "" for null.
func (p *player) name() string {
	p.t.Helper()
	status, body := p.get("/api/me")
	var out struct{ Name *string }
	if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil {
		p.t.Fatalf("GET /api/me: %d %s", status, body)
	}
	if out.Name == nil {
		return ""
	}
	return *out.Name
}

func TestLookingAroundNeedsNoName(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob, carol := newPlayer(t, ts), newPlayer(t, ts), newPlayer(t, ts)
	code := alice.create()
	alice.stream(code).state()
	bob.stream(code).state()

	if status, body := carol.get("/api/games"); status != http.StatusOK {
		t.Fatalf("GET /api/games: %d %s", status, body)
	}
	if v := carol.stream(code).state(); v.You != "spectator" {
		t.Fatalf("carol: you=%s", v.You)
	}
	if carol.name() != "" {
		t.Fatal("carol got a name by looking around")
	}
	if alice.name() == "" || bob.name() == "" {
		t.Fatal("creating a game and taking a seat should each give a name")
	}
}

func TestPlayingGivesANameThatShowsInTheGame(t *testing.T) {
	_, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	if alice.name() != "" {
		t.Fatal("alice has a name before playing")
	}
	code := alice.create()
	a := alice.stream(code)
	a.state()
	bob.stream(code).state()
	v := a.state()
	if v.Players.White == "" || v.Players.White != alice.name() || v.Players.Black != bob.name() {
		t.Fatalf("players %+v, alice %q, bob %q", v.Players, alice.name(), bob.name())
	}
}

func TestRerollName(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	status, body := alice.post("/api/me/name", "")
	var out struct{ Name string }
	if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil || out.Name == "" {
		t.Fatalf("first reroll: %d %s", status, body)
	}
	first := out.Name
	_, body = alice.post("/api/me/name", "")
	json.Unmarshal([]byte(body), &out)
	if out.Name == first || alice.name() != out.Name {
		t.Fatalf("second reroll %q (first %q), GET /api/me %q", out.Name, first, alice.name())
	}
}
