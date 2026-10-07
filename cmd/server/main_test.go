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

func TestRestoreAfterARestart(t *testing.T) {
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), "games.db")
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := game.NewHub(odd{}, st)
	playing, _ := h.Create("alice")
	sub, _ := playing.Join("bob")
	defer playing.Leave(sub)
	m, _ := rules.ParseMove("e2e4")
	if err := playing.Move("alice", m, 0); err != nil {
		t.Fatal(err)
	}
	waiting, _ := h.Create("carol") // nobody joins
	st.Close()

	st, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	h2 := game.NewHub(odd{}, st)
	if err := restore(ctx, st, h2, time.Now()); err != nil {
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
	if err := restore(ctx, st, h3, time.Now().Add(game.DefaultIdle+time.Minute)); err != nil {
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

func TestNewLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	newLogger("json", &buf).Info("listening", "addr", ":8080")
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("not JSON: %q: %v", buf.String(), err)
	}
	if line["msg"] != "listening" || line["addr"] != ":8080" || line["level"] != "INFO" {
		t.Errorf("got %v", line)
	}
}

func TestNewLoggerTextByDefault(t *testing.T) {
	var buf bytes.Buffer
	newLogger("", &buf).Info("listening", "addr", ":8080")
	if got := buf.String(); !strings.Contains(got, "msg=listening addr=:8080") {
		t.Errorf("got %q, want text output", got)
	}
}
