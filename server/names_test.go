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

func (p *player) me() meJSON {
	p.t.Helper()
	status, body := p.get("/api/me")
	var out meJSON
	if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil {
		p.t.Fatalf("GET /api/me: %d %s", status, body)
	}
	return out
}

// offers is the caller's GET /api/me/names.
func (p *player) offers() (int, []string) {
	p.t.Helper()
	status, body := p.get("/api/me/names")
	var out struct{ Names []string }
	json.Unmarshal([]byte(body), &out)
	return status, out.Names
}

func TestChangeNameFromOffers(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, _ := alice.offers(); status != http.StatusConflict {
		t.Fatalf("offers before playing: %d, want 409", status)
	}
	alice.create() // playing gives a name
	before := alice.me()
	if before.Name == nil || before.ChangesLeft != 3 || before.ChangesResetAt != nil {
		t.Fatalf("me after playing: %+v", before)
	}
	status, offers := alice.offers()
	if status != http.StatusOK || len(offers) != 3 {
		t.Fatalf("offers: %d %v", status, offers)
	}
	if status, body := alice.post("/api/me/name", `{"name":"not-on-offer"}`); status != http.StatusConflict {
		t.Fatalf("choosing an unoffered name: %d %s, want 409", status, body)
	}
	status, body := alice.post("/api/me/name", `{"name":"`+offers[1]+`"}`)
	var out meJSON
	if status != http.StatusOK || json.Unmarshal([]byte(body), &out) != nil || *out.Name != offers[1] || out.ChangesLeft != 2 || out.ChangesResetAt == nil {
		t.Fatalf("choosing: %d %s", status, body)
	}
	if me := alice.me(); *me.Name != offers[1] || me.ChangesLeft != 2 {
		t.Fatalf("me after choosing: %+v", me)
	}
}

func TestNoChangesLeftIs429(t *testing.T) {
	_, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	alice.create()
	for range 3 {
		_, offers := alice.offers()
		if status, body := alice.post("/api/me/name", `{"name":"`+offers[0]+`"}`); status != http.StatusOK {
			t.Fatalf("change: %d %s", status, body)
		}
	}
	status, body := alice.get("/api/me/names")
	var out struct{ ChangesResetAt int64 }
	if status != http.StatusTooManyRequests || json.Unmarshal([]byte(body), &out) != nil || out.ChangesResetAt == 0 {
		t.Fatalf("a fourth change: %d %s, want 429 with the reset time", status, body)
	}
	if me := alice.me(); me.ChangesLeft != 0 || me.ChangesResetAt == nil {
		t.Fatalf("me with none left: %+v", me)
	}
}
