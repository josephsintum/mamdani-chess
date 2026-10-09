package server

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/store"
)

func TestPracticeAndReconnectEvents(t *testing.T) {
	s, ts := newTestServer(t)
	alice := newPlayer(t, ts)
	if status, _ := alice.post("/api/practice", ""); status != http.StatusCreated {
		t.Fatalf("practice: %d", status)
	}
	code := alice.create()
	alice.stream(code).state()
	resp, err := alice.c.Get(ts.URL + "/api/games/" + code + "/stream?again=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	counts := eventCounts(t, s)
	if counts["practice"] != 1 || counts["reconnect"] != 1 || counts["restart"] != 0 {
		t.Fatalf("events %v", counts)
	}
}

// eventCounts counts each kind of event logged so far.
func eventCounts(t *testing.T, s *Server) map[string]int {
	t.Helper()
	counts := map[string]int{}
	for _, kind := range []string{"practice", "reconnect", "restart"} {
		n, err := s.store.CountEvents(t.Context(), kind, time.Time{}, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		counts[kind] = n
	}
	return counts
}

// playtestHeader adds the header the playtest script sends to every request.
type playtestHeader struct{}

func (playtestHeader) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("X-Playtest", "1")
	return http.DefaultTransport.RoundTrip(r)
}

// newPlaytester is a browser the playtest script drives.
func newPlaytester(t *testing.T, ts *httptest.Server) *player {
	t.Helper()
	p := newPlayer(t, ts)
	p.c.Transport = playtestHeader{}
	return p
}

// kindOf reads a saved game's kind and playtest mark.
func kindOf(t *testing.T, s *Server, code string) (string, bool) {
	t.Helper()
	kind, playtest, err := s.store.GameKind(t.Context(), code)
	if err != nil {
		t.Fatalf("kind of %s: %v", code, err)
	}
	return kind, playtest
}

func hasStats(t *testing.T, s *Server, code string) bool {
	t.Helper()
	has, err := s.store.HasGameStats(t.Context(), code)
	if err != nil {
		t.Fatal(err)
	}
	return has
}

// rematchOf has both players ask for a rematch of a finished game and
// returns the new game's code, read from white's stream.
func rematchOf(t *testing.T, code string, white, black *player) string {
	t.Helper()
	w := white.stream(code)
	w.state()
	rematch := "/api/games/" + code + "/rematch"
	for _, p := range []*player{white, black} {
		if status, body := p.post(rematch, `{}`); status != http.StatusNoContent {
			t.Fatalf("rematch: %d %s", status, body)
		}
	}
	for {
		if next := w.state().Rematch.Code; next != "" {
			return next
		}
	}
}

// matchPair puts a and b through quick match and returns their game's code
// once both streams have ended (each search is logged as its stream ends).
func matchPair(t *testing.T, a, b *player) string {
	t.Helper()
	as, _ := a.queue()
	bs, _ := b.queue()
	code := matchedCode(as)
	if got := matchedCode(bs); got != code {
		t.Fatalf("matched to %s and %s", code, got)
	}
	as.waitEnd(2*time.Second, "matched")
	bs.waitEnd(2*time.Second, "matched")
	return code
}

// A playtest's games are marked, its searches and events aren't logged,
// and the stats count none of it.
func TestPlaytestGamesAreNotCounted(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob := newPlaytester(t, ts), newPlaytester(t, ts)
	code := matchPair(t, alice, bob)
	if kind, playtest := kindOf(t, s, code); kind != "quick" || !playtest {
		t.Fatalf("quick match: kind %q playtest %v, want quick, true", kind, playtest)
	}
	if rows := searches(t, s); len(rows) != 0 {
		t.Fatalf("a playtest's searches were logged: %+v", rows)
	}
	if status, body := alice.post("/api/games/"+code+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign: %d %s", status, body)
	}
	next := rematchOf(t, code, alice, bob)
	if kind, playtest := kindOf(t, s, next); kind != "quick" || !playtest {
		t.Fatalf("rematch: kind %q playtest %v, want quick, true", kind, playtest)
	}
	friend := newPlaytester(t, ts).create()
	if kind, playtest := kindOf(t, s, friend); kind != "friend" || !playtest {
		t.Fatalf("friend link: kind %q playtest %v, want friend, true", kind, playtest)
	}
	if status, _ := alice.post("/api/practice", ""); status != http.StatusCreated {
		t.Fatalf("practice: %d", status)
	}
	resp, err := bob.c.Get(ts.URL + "/api/games/" + friend + "/stream?again=1")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if counts := eventCounts(t, s); counts["practice"] != 0 || counts["reconnect"] != 0 {
		t.Fatalf("a playtest's events were logged: %v", counts)
	}
	section, err := s.store.GameSection(t.Context(), time.Time{}, time.Now().Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatal(err)
	}
	funnel, err := s.store.FunnelStats(t.Context(), time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if section.Games != 0 || section.Road.Games != 0 || funnel.Games != 0 {
		t.Fatalf("the stats count a playtest: section %d games (%d on the road), funnel %d", section.Games, section.Road.Games, funnel.Games)
	}
}

// A game that ends saves its stats row with the result; every game is
// saved with its kind, and a rematch keeps it.
func TestGameEndSavesStatsAndKind(t *testing.T) {
	s, ts := newTestServer(t)
	alice, bob := newPlayer(t, ts), newPlayer(t, ts)
	code := matchPair(t, alice, bob)
	if status, body := alice.post("/api/games/"+code+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign: %d %s", status, body)
	}
	if kind, playtest := kindOf(t, s, code); kind != "quick" || playtest || !hasStats(t, s, code) {
		t.Fatalf("quick match: kind %q playtest %v stats %v, want quick, false, true", kind, playtest, hasStats(t, s, code))
	}
	next := rematchOf(t, code, alice, bob)
	if kind, _ := kindOf(t, s, next); kind != "quick" {
		t.Fatalf("rematch of a quick match: kind %q", kind)
	}
	carol := newPlayer(t, ts)
	if kind, playtest := kindOf(t, s, carol.create()); kind != "friend" || playtest {
		t.Fatalf("friend link: kind %q playtest %v", kind, playtest)
	}

	// A practice game is never saved, so it has no row of either kind.
	status, body := carol.post("/api/practice", "")
	var out struct{ Code string }
	if status != http.StatusCreated || json.Unmarshal([]byte(body), &out) != nil {
		t.Fatalf("practice: %d %s", status, body)
	}
	practice := out.Code
	if status, body := carol.post("/api/games/"+practice+"/resign", ""); status != http.StatusNoContent {
		t.Fatalf("resign practice: %d %s", status, body)
	}
	if _, _, err := s.store.GameKind(t.Context(), practice); !errors.Is(err, sql.ErrNoRows) || hasStats(t, s, practice) {
		t.Fatalf("practice game saved: %v", err)
	}

	// White misses the first move: aborted, no stats row.
	t.Run("aborted", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			st, err := store.Open(filepath.Join(t.TempDir(), "abort.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer st.Close()
			g, err := game.NewHub(odd{}, st).CreatePair("white", "black", false)
			if err != nil {
				t.Fatal(err)
			}
			time.Sleep(game.FirstMoveTime + time.Second)
			synctest.Wait()
			has, err := st.HasGameStats(t.Context(), g.Code())
			if err != nil {
				t.Fatal(err)
			}
			section, err := st.GameSection(t.Context(), time.Time{}, time.Now().Add(time.Hour), time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			if has || len(section.Ends) != 1 || section.Ends[0] != (store.Count{Label: "Aborted", N: 1}) {
				t.Fatalf("stats row %v, ends %v; want none and one aborted", has, section.Ends)
			}
			time.Sleep(game.DefaultIdle + time.Minute) // evicted, so the bubble's goroutines end
			synctest.Wait()
		})
	})
}
