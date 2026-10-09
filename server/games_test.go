package server

import (
	"net/http"
	"testing"
	"time"
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
	counts := map[string]int{}
	for _, kind := range []string{"practice", "reconnect", "restart"} {
		n, err := s.store.CountEvents(t.Context(), kind, time.Time{}, time.Now().Add(time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		counts[kind] = n
	}
	if counts["practice"] != 1 || counts["reconnect"] != 1 || counts["restart"] != 0 {
		t.Fatalf("events %v", counts)
	}
}
