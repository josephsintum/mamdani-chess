package server

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A deploy check reads the version from /healthz instead of creating a game.
func TestHealthzReportsTheVersion(t *testing.T) {
	s, ts := newTestServer(t)
	s.Version = "05cd63f"
	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct{ Status, Version string }
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Status != "ok" || body.Version != "05cd63f" {
		t.Fatalf("healthz: %+v (%v), want status ok and version 05cd63f", body, err)
	}
}
