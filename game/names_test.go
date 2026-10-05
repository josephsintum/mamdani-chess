package game

import (
	"context"
	"testing"
	"time"

	"mamdani-chess/names"
	"mamdani-chess/store"
)

func guestName(t *testing.T, st *store.Store, id string) string {
	t.Helper()
	name, err := st.GuestName(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestPlayersGetNamesWhenTheySit(t *testing.T) {
	st := openStore(t)
	g := create(t, NewHub(odd{}, st), "alice")
	join(t, g, "bob")
	alice, bob := guestName(t, st, "alice"), guestName(t, st, "bob")
	if alice == "" || bob == "" || alice == bob {
		t.Fatalf("names alice=%q bob=%q; want two different names", alice, bob)
	}
	if p := recvView(t, g, "carol").Players; p != (PlayersJSON{White: alice, Black: bob}) {
		t.Fatalf("players %+v", p)
	}
	saved := load(t, st)
	if saved[0].WhiteName != alice || saved[0].BlackName != bob {
		t.Fatalf("saved names %q %q", saved[0].WhiteName, saved[0].BlackName)
	}
}

func TestWatchingNeedsNoName(t *testing.T) {
	st := openStore(t)
	g := create(t, NewHub(odd{}, st), "alice")
	join(t, g, "bob")
	join(t, g, "carol") // the game is full: carol watches
	if _, err := g.View("dave"); err != nil {
		t.Fatal(err)
	}
	if name := guestName(t, st, "carol") + guestName(t, st, "dave"); name != "" {
		t.Fatalf("a spectator got a name: %q", name)
	}
}

func TestARerollDoesntRenameAPlayerMidGame(t *testing.T) {
	st := openStore(t)
	g := create(t, NewHub(odd{}, st), "alice")
	join(t, g, "bob")
	before := guestName(t, st, "alice")
	ctx, now := context.Background(), time.Now()
	offers, _, err := st.NameOffers(ctx, "alice", names.Random, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.ChooseName(ctx, "alice", offers[0], now); err != nil || guestName(t, st, "alice") == before {
		t.Fatalf("change name: %v", err)
	}
	if p := recvView(t, g, "bob").Players; p.White != before {
		t.Fatalf("white is %q after the reroll, want %q", p.White, before)
	}
}

func TestCreatePairSeatsBothPlayers(t *testing.T) {
	st := openStore(t)
	g, err := NewHub(odd{}, st).CreatePair("alice", "bob")
	if err != nil {
		t.Fatal(err)
	}
	v := recvView(t, g, "alice")
	if v.You != "white" || v.Status != Playing || v.Clock.FirstMoveDeadline == 0 {
		t.Fatalf("alice: you=%s status=%s clock=%+v; want white, playing, first-move deadline running", v.You, v.Status, v.Clock)
	}
	if want := (PlayersJSON{White: guestName(t, st, "alice"), Black: guestName(t, st, "bob")}); v.Players != want || want.White == "" || want.Black == "" {
		t.Fatalf("players %+v, want %+v", v.Players, want)
	}
	if v := recvView(t, g, "bob"); v.You != "black" {
		t.Fatalf("bob: you=%s", v.You)
	}
}

func TestRematchSwapsTheNames(t *testing.T) {
	st := openStore(t)
	h := NewHub(odd{}, st)
	g, _, _ := over(t, h)
	g.Rematch("alice", false)
	g.Rematch("bob", false)
	next, ok := h.Get(recvView(t, g, "alice").Rematch.Code)
	if !ok {
		t.Fatal("no rematch")
	}
	want := PlayersJSON{White: guestName(t, st, "bob"), Black: guestName(t, st, "alice")}
	if p := recvView(t, next, "carol").Players; p != want {
		t.Fatalf("rematch players %+v, want %+v", p, want)
	}
}

func TestRestoreKeepsTheNames(t *testing.T) {
	st := openStore(t)
	g := create(t, NewHub(odd{}, st), "alice")
	join(t, g, "bob")
	want := recvView(t, g, "carol").Players

	h := NewHub(odd{}, st) // a restart
	h.Restore(load(t, st))
	restored, ok := h.Get(g.Code())
	if !ok {
		t.Fatal("not restored")
	}
	if p := recvView(t, restored, "carol").Players; p != want {
		t.Fatalf("restored players %+v, want %+v", p, want)
	}
}
