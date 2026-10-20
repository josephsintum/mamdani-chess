package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/rules"
	"mamdani-chess/store"
)

func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.DiscardHandler))
	os.Exit(m.Run())
}

type odd struct{}

func (odd) D8() int { return 1 }

// openStore opens the database at path, closed when the test ends.
func openStore(t *testing.T, path string) *store.Store {
	t.Helper()
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

// captureLog sends the default logger's text to the buffer it returns
// until the test ends.
func captureLog(t *testing.T) *bytes.Buffer {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(old) })
	return &buf
}

func TestRestoreAfterARestart(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "games.db")
	st := openStore(t, path)
	h := game.NewHub(odd{}, st)
	playing, _ := h.Create("alice", false)
	sub, _ := playing.Join("bob")
	defer playing.Leave(sub)
	m, _ := rules.ParseMove("e2e4")
	if err := playing.Move("alice", m, 0); err != nil {
		t.Fatal(err)
	}
	waiting, _ := h.Create("carol", false) // nobody joins
	st.Close()

	st = openStore(t, path)
	h2 := game.NewHub(odd{}, st)
	if err := start(ctx, st, h2, time.Now()); err != nil {
		t.Fatal(err)
	}
	g, ok := h2.Get(playing.Code())
	if !ok {
		t.Fatal("the game in progress wasn't restored")
	}
	if v, _ := g.View("bob"); v.Seq != 1 || v.You != "black" {
		t.Fatalf("restored: seq=%d you=%s", v.Seq, v.You)
	}
	if _, ok := h2.Get(waiting.Code()); !ok {
		t.Fatal("a game waiting less than a day should be restored")
	}

	// A day later, the waiting game expires. It ended just now, so it is
	// still restored, to show that it's over.
	h3 := game.NewHub(odd{}, st)
	if err := start(ctx, st, h3, time.Now().Add(game.DefaultIdle+time.Minute)); err != nil {
		t.Fatal(err)
	}
	g3, ok := h3.Get(waiting.Code())
	if !ok {
		t.Fatal("the expired game should be restored for its last day")
	}
	if v, _ := g3.View("carol"); v.Result == nil || v.Result.Reason != game.Expired {
		t.Fatalf("result %+v, want expired", v.Result)
	}
}

// The stats work finishes before any game is restored (each restored
// game's grace starts then), and a first start fills the counters from the
// games already saved.
func TestStartReadiesStatsBeforeRestoring(t *testing.T) {
	ctx := t.Context()
	st := openStore(t, filepath.Join(t.TempDir(), "games.db"))
	h := game.NewHub(odd{}, st)
	playing, _ := h.Create("alice", false)
	sub, _ := playing.Join("bob")
	defer playing.Leave(sub)
	// A new database has no counters version yet, as on the first start
	// with counters: the start rebuilds.
	buf := captureLog(t)
	if err := start(ctx, st, game.NewHub(odd{}, st), time.Now()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	rebuilt, restored := strings.Index(out, "stats rebuilt"), strings.Index(out, "games restored")
	if rebuilt < 0 || restored < 0 || rebuilt > restored {
		t.Fatalf("want the rebuild logged before the restore:\n%s", out)
	}
	days, err := st.Counts(ctx, "", "9999-12-31")
	if err != nil {
		t.Fatal(err)
	}
	sum := map[string]int{}
	for _, d := range days {
		for k, n := range d.N {
			sum[k] += n
		}
	}
	if sum["games.friend"] != 1 || sum["friend.joined"] != 1 || sum["event.restart"] != 1 {
		t.Fatalf("counters after the first start: %v", sum)
	}
}

// LOG_FORMAT=json logs JSON; anything else, readable text.
func TestNewLogger(t *testing.T) {
	for _, c := range []struct {
		format string
		ok     func(out []byte) bool
	}{
		{"json", func(out []byte) bool {
			var line map[string]any
			return json.Unmarshal(out, &line) == nil && line["msg"] == "listening" && line["addr"] == ":8080" && line["level"] == "INFO"
		}},
		{"", func(out []byte) bool { return strings.Contains(string(out), "msg=listening addr=:8080") }},
	} {
		var buf bytes.Buffer
		newLogger(c.format, &buf).Info("listening", "addr", ":8080")
		if !c.ok(buf.Bytes()) {
			t.Errorf("LOG_FORMAT=%q logged %q", c.format, buf.String())
		}
	}
}
