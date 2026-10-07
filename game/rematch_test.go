package game

import (
	"errors"
	"testing"
)

// over starts a game between alice (White) and bob (Black), and has alice
// resign at once.
func over(t *testing.T, h *Hub) (*Game, *Sub, *Sub) {
	t.Helper()
	g, a, b := seated(t, h)
	if err := g.Resign("alice"); err != nil {
		t.Fatal(err)
	}
	return g, a, b
}

// askRematch offers, accepts or (with decline) declines a rematch, which
// must not fail.
func askRematch(t *testing.T, g *Game, guest string, decline bool) {
	t.Helper()
	if err := g.Rematch(guest, decline); err != nil {
		t.Fatalf("rematch by %s: %v", guest, err)
	}
}

// rematched ends a game between alice and bob on h, and both ask for a
// rematch. It returns the old game and the rematch's code.
func rematched(t *testing.T, h *Hub) (*Game, string) {
	t.Helper()
	g, _, _ := over(t, h)
	askRematch(t, g, "alice", false)
	askRematch(t, g, "bob", false)
	code := recvView(t, g, "alice").Rematch.Code
	if code == "" {
		t.Fatal("two offers should start the rematch")
	}
	return g, code
}

func TestRematchOfferAndAccept(t *testing.T) {
	h := NewHub(odd{}, nil)
	g, _, _ := over(t, h)
	if err := g.Rematch("alice", false); err != nil {
		t.Fatal(err)
	}
	if r := recvView(t, g, "bob").Rematch; r != (RematchJSON{Offer: "white"}) {
		t.Fatalf("after alice offers: %+v", r)
	}
	if err := g.Rematch("bob", false); err != nil { // accepting
		t.Fatal(err)
	}
	code := recvView(t, g, "carol").Rematch.Code // spectators see it too
	if code == "" || code == g.Code() {
		t.Fatalf("rematch code %q", code)
	}
	next, ok := h.Get(code)
	if !ok {
		t.Fatal("the rematch isn't in the hub")
	}
	if v := recvView(t, next, "bob"); v.You != "white" || v.Status != Playing || v.Clock.FirstMoveDeadline == 0 {
		t.Fatalf("rematch for bob: you=%s status=%s clock=%+v; want white, playing, first-move deadline running",
			v.You, v.Status, v.Clock)
	}
	if v := recvView(t, next, "alice"); v.You != "black" {
		t.Fatalf("rematch for alice: you=%s, want black", v.You)
	}
	if err := g.Rematch("alice", false); err != nil { // a late click changes nothing
		t.Fatal(err)
	}
	if got := recvView(t, g, "bob").Rematch.Code; got != code {
		t.Fatalf("rematch code changed to %q", got)
	}
}

func TestRematchDecline(t *testing.T) {
	g, _, _ := over(t, NewHub(odd{}, nil))
	askRematch(t, g, "alice", false)
	if err := g.Rematch("bob", true); err != nil {
		t.Fatal(err)
	}
	if r := recvView(t, g, "alice").Rematch; r != (RematchJSON{Declined: true}) {
		t.Fatalf("after bob declines: %+v", r)
	}
	askRematch(t, g, "alice", true) // declining your own offer does nothing
	askRematch(t, g, "bob", false)  // a new offer after a decline
	if r := recvView(t, g, "alice").Rematch; r != (RematchJSON{Offer: "black"}) {
		t.Fatalf("after bob offers: %+v", r)
	}
}

func TestRematchOfferEndsWhenTheOffererLeaves(t *testing.T) {
	g, a, _ := over(t, NewHub(odd{}, nil))
	a2 := join(t, g, "alice") // a second tab
	askRematch(t, g, "alice", false)
	g.Leave(a)
	if r := recvView(t, g, "bob").Rematch; r.Offer != "white" {
		t.Fatalf("offer gone while alice still has a tab open: %+v", r)
	}
	g.Leave(a2)
	if r := recvView(t, g, "bob").Rematch; r != (RematchJSON{}) {
		t.Fatalf("offer still there after alice left: %+v", r)
	}
}

func TestRematchErrors(t *testing.T) {
	g, _, _ := seated(t, NewHub(odd{}, nil))
	if err := g.Rematch("alice", false); !errors.Is(err, ErrNotOver) {
		t.Errorf("rematch during the game: %v, want ErrNotOver", err)
	}
	if err := g.Resign("alice"); err != nil {
		t.Fatal(err)
	}
	if err := g.Rematch("carol", false); !errors.Is(err, ErrNotPlayer) {
		t.Errorf("rematch by a spectator: %v, want ErrNotPlayer", err)
	}
}

func TestOnlineFollowsThePlayersStreams(t *testing.T) {
	g, _, b := seated(t, NewHub(odd{}, nil))
	if v := recvView(t, g, "alice"); v.Online != (OnlineJSON{White: true, Black: true}) {
		t.Fatalf("online %+v, want both", v.Online)
	}
	g.Leave(b)
	if v := recvView(t, g, "alice"); v.Online != (OnlineJSON{White: true}) {
		t.Fatalf("online %+v after bob left", v.Online)
	}
	join(t, g, "bob")
	if v := recvView(t, g, "alice"); v.Online != (OnlineJSON{White: true, Black: true}) {
		t.Fatalf("online %+v after bob came back", v.Online)
	}
}
