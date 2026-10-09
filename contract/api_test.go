package contract

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/rules"
)

// viewKeys is a view's keys, in the order the server sends them.
var viewKeys = []string{"code", "status", "you", "board", "mamdani", "potholes", "turn", "check", "legal", "last", "log", "lost", "taken", "stats", "result", "seq", "clock", "online", "players", "rematch"}

// apiError is an error response's message.
type apiError struct{ Error string }

// seated is a game between alice (White) and bob (Black), both with a
// stream open, at the start.
type seated struct {
	code       string
	alice, bob *player
	a, b       *sseReader
}

func (c *check) seated() seated {
	c.t.Helper()
	ps := c.players(2)
	g := seated{alice: ps[0], bob: ps[1]}
	g.code = g.alice.create()
	g.a = g.alice.stream(g.code)
	g.a.state()
	g.b = g.bob.stream(g.code)
	g.b.state()
	g.a.state()
	return g
}

func TestHealthz(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	r := c.do(http.MethodGet, "/healthz", nil)
	if r.status != 200 || r.header.Get("Content-Type") != "application/json" {
		t.Errorf("/healthz: %d %q", r.status, r.header.Get("Content-Type"))
	}
	if !regexp.MustCompile(`^\{"status":"ok","version":"[^"]+"\}\n$`).MatchString(r.body) {
		t.Errorf("/healthz body %q", r.body)
	}
}

// An unknown API path, or a wrong method on one, is a JSON 404.
func TestUnknownAPIPath(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	for _, r := range []struct{ method, path string }{
		{"GET", "/api/nope"},
		{"DELETE", "/api/me"},
		{"GET", "/api/games/"},
		{"POST", "/api/games/ABCDEF/stream"},
	} {
		res := c.do(r.method, r.path, nil)
		if res.status != 404 || res.body != "{\"error\":\"not found\"}\n" {
			t.Errorf("%s %s: %d %q", r.method, r.path, res.status, res.body)
		}
	}
}

func TestGuestCookie(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	fresh := regexp.MustCompile(`^guest=[0-9a-f]{32}; Path=/; Max-Age=31536000; HttpOnly; SameSite=Lax$`)
	if set := c.do(http.MethodGet, "/api/me", nil).header.Get("Set-Cookie"); !fresh.MatchString(set) {
		t.Errorf("Set-Cookie = %q", set)
	}
	if set := c.do(http.MethodGet, "/api/me", http.Header{"X-Forwarded-Proto": {"https"}}).header.Get("Set-Cookie"); !strings.HasSuffix(set, "; HttpOnly; Secure; SameSite=Lax") {
		t.Errorf("Set-Cookie behind https = %q", set)
	}
	p := c.players(1)[0]
	p.get("/api/me")
	if again := p.fetch(http.MethodGet, "/api/me"); again.header.Values("Set-Cookie") != nil {
		t.Errorf("a guest with a cookie got another: %q", again.header.Values("Set-Cookie"))
	}
	// The app shell sets it too, so a fresh browser's first API requests
	// (the visit beacon and a game's stream, which race) share one guest.
	for _, path := range []string{"/", "/game/ABCDEF"} {
		if set := c.do(http.MethodGet, path, nil).header.Get("Set-Cookie"); !fresh.MatchString(set) {
			t.Errorf("GET %s: Set-Cookie = %q", path, set)
		}
		if again := p.fetch(http.MethodGet, path); again.header.Values("Set-Cookie") != nil {
			t.Errorf("GET %s with a cookie set another: %q", path, again.header.Values("Set-Cookie"))
		}
	}
}

// /api/me before playing, and names need a name.
func TestMeBeforePlaying(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	p := c.players(1)[0]
	if status, body := p.get("/api/me"); status != 200 || body != "{\"name\":null,\"changesLeft\":3,\"changesResetAt\":null}\n" {
		t.Errorf("/api/me: %d %q", status, body)
	}
	if status, body := p.get("/api/me/names"); status != 409 || body != "{\"error\":\"you get a name when you first play\"}\n" {
		t.Errorf("/api/me/names: %d %q", status, body)
	}
}

func TestCreateGame(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	p := c.players(1)[0]
	r := p.fetch(http.MethodPost, "/api/games")
	if r.status != 201 || r.header.Get("Content-Type") != "application/json" {
		t.Errorf("POST /api/games: %d %q", r.status, r.header.Get("Content-Type"))
	}
	if !regexp.MustCompile(`^\{"code":"[ABCDEFGHJKMNPQRSTUVWXYZ2-9]{6}"\}\n$`).MatchString(r.body) {
		t.Errorf("POST /api/games body %q", r.body)
	}
}

// A practice game: one guest moves for both sides, the view says so, and
// the live games list never shows it.
func TestPractice(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	p, other := c.players(2)[0], c.players(1)[0]
	r := p.fetch(http.MethodPost, "/api/practice")
	if r.status != 201 || !regexp.MustCompile(`^\{"code":"[ABCDEFGHJKMNPQRSTUVWXYZ2-9]{6}"\}\n$`).MatchString(r.body) {
		t.Fatalf("POST /api/practice: %d %q", r.status, r.body)
	}
	code := decode[struct{ Code string }](t, r.body).Code
	move := "/api/games/" + code + "/move"
	for i, m := range []string{`{"from":"e2","to":"e4","seq":0}`, `{"from":"e7","to":"e5","seq":1}`} {
		if status, body := p.post(move, m); status != 204 {
			t.Errorf("move %d: %d %q", i, status, body)
		}
	}
	if status, body := other.post(move, `{"from":"g1","to":"f3","seq":2}`); status != 403 {
		t.Errorf("another guest's move: %d %q", status, body)
	}
	_, view := p.get("/api/games/" + code)
	expectKeys(t, "practice view", []byte(view), append(slices.Clone(viewKeys), "practice")...)
	if _, list := p.get("/api/games"); strings.Contains(list, code) {
		t.Errorf("listed: %s", list)
	}
}

// A stream: headers, a random retry, then the state.
func TestStreamFraming(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	p := c.players(1)[0]
	code := p.create()
	r := p.open("/api/games/" + code + "/stream")
	h := r.resp.Header
	if r.resp.StatusCode != 200 || h.Get("Content-Type") != "text/event-stream" || h.Get("Cache-Control") != "no-cache" || h.Get("X-Accel-Buffering") != "no" {
		t.Errorf("stream: %d %q", r.resp.StatusCode, h)
	}
	// The raw bytes, up to the end of the first state event.
	var text string
	buf := make([]byte, 4096)
	for !strings.HasSuffix(text, "\n\n") || !strings.Contains(text, "event: state") {
		n, err := r.br.Read(buf)
		text += string(buf[:n])
		if err != nil {
			t.Fatalf("stream ended after %q: %v", text, err)
		}
	}
	r.close()
	m := regexp.MustCompile(`(?s)^retry: (\d+)\n\nevent: state\ndata: (\{.*\})\n\n$`).FindStringSubmatch(text)
	if m == nil {
		t.Fatalf("stream began %q", text)
	}
	if retry, _ := strconv.Atoi(m[1]); retry < 1000 || retry > 3000 {
		t.Errorf("retry: %d, want 1000 to 3000", retry)
	}
	v := parseView(t, m[2])
	expectKeys(t, "view", v.raw, viewKeys...)
	expectJSON(t, []any{v.Code, v.Status, v.You, v.Seq, v.get("legal"), v.get("result"), v.get("rematch"), v.get("online")},
		`[`+js(code)+`,"waiting","white",0,[],null,{},{"white":true,"black":false}]`)
	expectKeys(t, "clock", v.get("clock"), "whiteMs", "blackMs", "now")
	if v.Players.Black != "" || v.Players.White == "" {
		t.Errorf("players = %+v, want White named and Black empty", v.Players)
	}
}

// Black sits down, and both see the game start.
func TestBlackSitsDown(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	g := c.seated()
	v := g.alice.stream(g.code).state() // a second tab
	expectJSON(t, []any{v.You, v.Status, len(v.Legal), v.get("online")}, `["white","playing",33,{"white":true,"black":true}]`)
	expectKeys(t, "clock", v.get("clock"), "whiteMs", "blackMs", "now", "firstMoveDeadline")
	if want := g.alice.name(); v.Players.White != want {
		t.Errorf("players.white = %q, want %q", v.Players.White, want)
	}
	if want := g.bob.name(); v.Players.Black != want {
		t.Errorf("players.black = %q, want %q", v.Players.Black, want)
	}
	if code := g.alice.me().Game; code == nil || *code != g.code {
		t.Errorf("alice's /api/me game = %v, want %s", code, g.code)
	}
	carol := c.players(1)[0]
	if you := carol.stream(g.code).state().You; you != "spectator" {
		t.Errorf("carol is %q, want spectator", you)
	}
	if name := carol.name(); name != "" {
		t.Errorf("carol got the name %q by watching; watching needs no name", name)
	}
	g.a.close()
}

// Move errors: status codes and messages.
func TestMoveErrors(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	g := c.seated()
	carol := c.players(1)[0]
	move := "/api/games/" + g.code + "/move"
	if status, body := carol.post(move, `{"from":"e2","to":"e4","seq":0}`); status != 403 || body != `{"error":"you are not playing in this game"}` {
		t.Errorf("carol moves: %d %s", status, body)
	}
	for _, r := range []struct {
		who         *player
		body, error string
	}{
		{g.bob, `{"from":"e7","to":"e5","seq":0}`, "not your turn"},
		{g.alice, `{"from":"e2","to":"e4","seq":5}`, "the game has moved on; reload the position"},
		{g.alice, `{"from":"e2","to":"e5","seq":0}`, "illegal move"},
	} {
		status, text := r.who.post(move, r.body)
		var out struct {
			Error string
			State game.View
		}
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Errorf("%s: %d %.200s: %v", r.body, status, text, err)
			continue
		}
		expectJSON(t, []any{r.body, status, keys(t, []byte(text)), out.Error, out.State.Seq},
			fmt.Sprintf(`[%s,409,["error","state"],%s,0]`, js(r.body), js(r.error)))
	}
	for _, r := range []struct{ body, text string }{
		{`{"from":"e9","to":"e4","seq":0}`, `{"error":"bad move"}`},
		{`{"from":"e2","to":"e4","promo":"k","seq":0}`, `{"error":"bad move"}`},
		{`{"from":"e2","to":"e4"}`, `{"error":"bad request body"}`},
		{`{"from":"e2","to":"e4","seq":"0"}`, `{"error":"bad request body"}`},
		{`not json`, `{"error":"bad request body"}`},
		{`[]`, `{"error":"bad request body"}`},
		{`{"from":"` + strings.Repeat("x", 5000) + `","seq":0}`, `{"error":"bad request body"}`},
	} {
		if status, text := g.alice.post(move, r.body); status != 400 || text != r.text {
			t.Errorf("%.30s: %d %s, want 400 %s", r.body, status, text, r.text)
		}
	}
	for _, r := range []struct{ path, body string }{
		{"/api/games/NOPE99/move", `{"from":"e2","to":"e4","seq":0}`},
		{"/api/games/NOPE99/resign", ""},
	} {
		if status, text := g.alice.post(r.path, r.body); status != 404 || text != `{"error":"game not found"}` {
			t.Errorf("POST %s: %d %s", r.path, status, text)
		}
	}
	if status, text := g.alice.get("/api/games/NOPE99"); status != 404 || text != "{\"error\":\"game not found\"}\n" {
		t.Errorf("GET /api/games/NOPE99: %d %q", status, text)
	}
	if status := g.alice.fetch(http.MethodGet, "/api/games/NOPE99/stream").status; status != 404 {
		t.Errorf("an unknown game's stream: %d, want 404", status)
	}
}

// A move, as both players and a spectator see it.
func TestMoveSeenByAll(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	var (
		g          seated
		carol      *player
		va, vb, vc view
	)
	// The server rolls real dice, and now and then (about 1 game in 70)
	// the roll after e4 opens a pothole under the pawn and it falls. That
	// is the dice, not the contract, so the check plays again in a new
	// game.
	for try := 1; ; try++ {
		g = c.seated()
		carol = c.players(1)[0]
		cs := carol.stream(g.code)
		cs.state()
		if status, body := g.alice.post("/api/games/"+g.code+"/move", `{"from":"e2","to":"e4","seq":0}`); status != 204 || body != "" {
			t.Fatalf("e2e4: %d %q", status, body)
		}
		va, vb, vc = g.a.state(), g.b.state(), cs.state()
		if !slices.ContainsFunc(vc.Last, func(e game.EventJSON) bool { return e.Kind == rules.Fell && e.Sq == "e4" }) {
			break
		}
		if try == 3 {
			t.Fatal("the pawn on e4 fell into a pothole in three games running")
		}
		t.Logf("game %s: the pawn on e4 fell into a pothole; playing again", g.code)
	}
	for _, v := range []view{va, vb, vc} {
		if len(v.Log) == 0 {
			t.Fatalf("%s: empty log after a move", v.You)
		}
		expectJSON(t, []any{v.Seq, v.Turn, v.Board[28], v.Board[12], len(v.Log), v.Log[0].SAN, v.Log[0].Color}, `[1,"black","wP","",1,"e4","white"]`)
		expectJSON(t, v.get("last", "0"), `{"kind":"moved","from":"e2","to":"e4","piece":"wP","color":"white"}`)
		if kind := v.get("last", "1", "kind"); string(kind) != `"rolled_pothole"` {
			t.Errorf("%s: last[1].kind = %s, want rolled_pothole", v.You, kind)
		}
	}
	if len(va.Legal) != 0 || len(vc.Legal) != 0 {
		t.Errorf("legal moves for alice %d, carol %d; want none", len(va.Legal), len(vc.Legal))
	}
	if len(vb.Legal) == 0 {
		t.Error("bob, to move, has no legal moves")
	}
	// The same JSON for every spectator, and for a fresh GET.
	status, text := carol.get("/api/games/" + g.code)
	if status != 200 {
		t.Fatalf("GET: %d %s", status, text)
	}
	var fresh, streamed map[string]any
	if json.Unmarshal([]byte(text), &fresh) != nil || json.Unmarshal(vc.raw, &streamed) != nil {
		t.Fatalf("GET %.200s, stream %.200s", text, vc.raw)
	}
	fresh["clock"], streamed["clock"] = nil, nil
	if !reflect.DeepEqual(fresh, streamed) {
		t.Errorf("GET differs from the stream:\n%s\n%s", text, vc.raw)
	}
}

// A whole game played to a result, with random legal moves.
func TestWholeGame(t *testing.T) {
	c := newCheck(t, 60*time.Second)
	g := c.seated()
	path := "/api/games/" + g.code
	players := []*player{g.alice, g.bob}
	var v view
	for seq := range 400 {
		who := players[seq%2]
		_, text := who.get(path)
		v = parseView(t, text)
		if v.Status == game.Over {
			break
		}
		if v.Seq != seq {
			t.Fatalf("seq = %d, want %d", v.Seq, seq)
		}
		if len(v.Legal) == 0 {
			t.Fatalf("seq %d: no legal moves for the side to move", seq)
		}
		m := v.Legal[rand.N(len(v.Legal))]
		body, _ := json.Marshal(struct {
			game.MoveJSON
			Seq int `json:"seq"`
		}{m, seq})
		if status, text := who.post(path+"/move", string(body)); status != 204 {
			t.Fatalf("%s: %d %s", body, status, text)
		}
	}
	if v.Status != game.Over {
		if status, text := g.alice.post(path+"/resign", ""); status != 204 {
			t.Fatalf("resign: %d %s", status, text)
		}
	}
	_, text := g.alice.get(path)
	v = parseView(t, text)
	if v.Status != game.Over || v.Result == nil {
		t.Errorf("status %s, result %v; want over with a result", v.Status, v.Result)
	}
	if len(v.Log) != v.Seq {
		t.Errorf("%d log entries after %d turns", len(v.Log), v.Seq)
	}
	expectJSON(t, v.get("legal"), `[]`)
	status, text := g.alice.post(path+"/move", fmt.Sprintf(`{"from":"a2","to":"a3","seq":%d}`, v.Seq))
	if status != 409 || decode[apiError](t, text).Error != "game is over" {
		t.Errorf("a move after the end: %d %s", status, text)
	}
}

func TestResigning(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	g := c.seated()
	carol := c.players(1)[0]
	resign := "/api/games/" + g.code + "/resign"
	if status, body := carol.post(resign, ""); status != 403 || body != `{"error":"you are not playing in this game"}` {
		t.Errorf("carol resigns: %d %s", status, body)
	}
	if status, body := g.alice.post(resign, ""); status != 204 || body != "" {
		t.Fatalf("alice resigns: %d %q", status, body)
	}
	v := g.b.state()
	expectJSON(t, []any{v.Status, v.get("result")}, `["over",{"winner":"black","draw":false,"reason":"resignation"}]`)
	if running, ok := lookup(v.raw, "clock", "running"); ok {
		t.Errorf("clock.running = %s after the end, want none", running)
	}
	status, text := g.bob.post(resign, "")
	if status != 409 || decode[apiError](t, text).Error != "game is over" {
		t.Errorf("bob resigns after the end: %d %s", status, text)
	}
	if me := g.alice.me(); me.Game != nil {
		t.Errorf("alice's /api/me still has a game: %s", me.raw)
	} else if _, ok := lookup(me.raw, "game"); ok {
		t.Errorf("alice's /api/me has a game key: %s", me.raw)
	}
}

// Rematch: offer, decline, offer again, accept.
func TestRematch(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	g := c.seated()
	rematch := "/api/games/" + g.code + "/rematch"
	status, text := g.alice.post(rematch, "{}")
	if status != 409 || decode[apiError](t, text).Error != "the game isn't over" {
		t.Errorf("a rematch before the end: %d %s", status, text)
	}
	g.alice.post("/api/games/"+g.code+"/resign", "")
	g.a.state()
	g.b.state()
	if status, text := g.alice.post(rematch, `{"decline":"yes"}`); status != 400 {
		t.Errorf(`{"decline":"yes"}: %d %s, want 400`, status, text)
	}
	expectPost := func(p *player, body string) {
		t.Helper()
		if status, text := p.post(rematch, body); status != 204 || text != "" {
			t.Fatalf("rematch %s: %d %q, want 204", body, status, text)
		}
	}
	expectPost(g.alice, "{}")
	expectJSON(t, g.b.state().get("rematch"), `{"offer":"white"}`)
	expectPost(g.bob, `{"decline":true}`)
	expectJSON(t, g.b.state().get("rematch"), `{"declined":true}`)
	g.a.until(func(v view) bool { return v.Rematch.Declined })
	expectPost(g.bob, "null") // null reads as {}: an offer
	expectJSON(t, g.a.state().get("rematch"), `{"offer":"black"}`)
	g.alice.post(rematch, "{}") // accepts
	next := g.a.state().Rematch.Code
	if !regexp.MustCompile(`^[A-Z2-9]{6}$`).MatchString(next) || next == g.code {
		t.Fatalf("rematch code %q (the game was %s)", next, g.code)
	}
	nb := g.bob.stream(next).state()
	na := g.alice.stream(next).state()
	expectJSON(t, []any{nb.You, na.You, nb.Status}, `["white","black","playing"]`)
	expectJSON(t, nb.get("players"), fmt.Sprintf(`{"white":%s,"black":%s}`, js(g.bob.name()), js(g.alice.name())))
}

func TestQuickMatch(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	ps := c.players(3)
	alice, bob, carol := ps[0], ps[1], ps[2]
	looking := func(p *player) int {
		t.Helper()
		_, text := p.get("/api/games")
		var out struct{ Looking int }
		if err := json.Unmarshal([]byte(text), &out); err != nil {
			t.Fatalf("/api/games %.200s: %v", text, err)
		}
		return out.Looking
	}
	before := looking(carol)
	a, n := alice.queue()
	if n != before+1 {
		t.Errorf("queued: looking %d, want %d", n, before+1)
	}
	if alice.name() == "" {
		t.Error("queueing gave alice no name")
	}
	if n := looking(alice); n != before {
		t.Errorf("alice sees %d looking, want %d: you aren't counted for yourself", n, before)
	}
	bobStream := bob.open("/api/match")
	rest := a.rest() // alice's stream ends after "matched"
	m := regexp.MustCompile(`^retry: 3600000\nevent: matched\ndata: (\{"code":"[A-Z2-9]{6}"\})\n\n$`).FindStringSubmatch(rest)
	if m == nil {
		t.Fatalf("alice's stream ended with %q", rest)
	}
	code := decode[struct{ Code string }](t, m[1]).Code
	if rest := bobStream.rest(); !strings.Contains(rest, "event: matched\ndata: {\"code\":\""+code+"\"}\n\n") {
		t.Errorf("bob's stream ended with %q, want the match to %s", rest, code)
	}
	va := alice.stream(code).state()
	vb := bob.stream(code).state()
	if va.You == vb.You || (va.You != "white" && va.You != "black") || (vb.You != "white" && vb.You != "black") {
		t.Errorf("alice is %q and bob %q, want white and black", va.You, vb.You)
	}
	if va.Status != game.Playing {
		t.Errorf("status %s, want playing", va.Status)
	}
	if status, text := alice.get("/api/match"); status != 409 || text != `{"code":"`+code+`","error":"you're already in a game"}`+"\n" {
		t.Errorf("queueing again: %d %q", status, text)
	}
	_, text := carol.get("/api/games")
	expectKeys(t, "/api/games", []byte(text), "games", "looking")
	games, _ := lookup([]byte(text), "games")
	var entry json.RawMessage
	for _, e := range decode[[]json.RawMessage](t, string(games)) {
		var l game.Live
		if json.Unmarshal(e, &l) == nil && l.Code == code {
			entry = e
		}
	}
	if entry == nil {
		t.Fatalf("game %s isn't listed: %s", code, text)
	}
	expectKeys(t, "a live game", entry, "code", "white", "black", "move", "board", "mamdani", "potholes", "last", "watching")
	move, _ := lookup(entry, "move")
	last, _ := lookup(entry, "last")
	white, _ := lookup(entry, "white")
	black, _ := lookup(entry, "black")
	expectJSON(t, []any{move, last, white, black}, fmt.Sprintf(`[1,null,%s,%s]`, js(va.Players.White), js(va.Players.Black)))
}

// Names: offers, choosing, the daily limit.
func TestNames(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	p := c.players(1)[0]
	p.create()
	me := p.me()
	expectKeys(t, "/api/me", me.raw, "name", "changesLeft", "changesResetAt")
	if me.ChangesLeft != 3 || me.ChangesResetAt != nil {
		t.Errorf("/api/me after creating a game: %s", me.raw)
	}
	type offers struct {
		Names       []string
		ChangesLeft int
	}
	getOffers := func() (int, offers, []byte) {
		t.Helper()
		status, text := p.get("/api/me/names")
		return status, decode[offers](t, text), []byte(text)
	}
	status, first, raw := getOffers()
	expectJSON(t, []any{status, keys(t, raw), len(first.Names), first.ChangesLeft}, `[200,["changesLeft","changesResetAt","names"],3,3]`)
	if _, again, _ := getOffers(); !reflect.DeepEqual(again.Names, first.Names) {
		t.Errorf("offers changed from %q to %q before one was chosen", first.Names, again.Names)
	}
	if status, text := p.post("/api/me/name", `{"name":"not-on-offer"}`); status != 409 || text != `{"error":"that name isn't on offer"}` {
		t.Errorf("a name not on offer: %d %s", status, text)
	}
	for i := range 3 {
		_, o, _ := getOffers()
		if len(o.Names) == 0 {
			t.Fatal("no names on offer")
		}
		body, _ := json.Marshal(map[string]string{"name": o.Names[0]})
		status, text := p.post("/api/me/name", string(body))
		out := decode[struct {
			Name        string
			ChangesLeft int
		}](t, text)
		expectJSON(t, []any{status, keys(t, []byte(text)), out.Name, out.ChangesLeft},
			fmt.Sprintf(`[200,["name","changesLeft","changesResetAt"],%s,%d]`, js(o.Names[0]), 2-i))
	}
	status, text := p.get("/api/me/names")
	expectJSON(t, []any{status, keys(t, []byte(text)), decode[apiError](t, text).Error}, `[429,["changesResetAt","error"],"no name changes left today"]`)
	if status, text := p.post("/api/me/name", `{"name":"x"}`); status != 429 {
		t.Errorf("a fourth change: %d %s, want 429", status, text)
	}
}

// The app, static files and link previews.
func TestAppAndPreviews(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	home := c.do(http.MethodGet, "/", http.Header{"Accept-Encoding": {"br, gzip"}})
	html := home.body
	expectJSON(t, []any{home.status, header(home, "Content-Type"), header(home, "Cache-Control"), header(home, "Content-Encoding")},
		`[200,"text/html; charset=utf-8","private, no-cache",null]`)
	for _, want := range []string{
		"<title>Mamdani Chess</title>\n<meta name=\"description\" content=\"Chess where potholes open under your pieces.",
		`<meta property="og:image" content="` + base + `/og.png" />`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the home page lacks %q", want)
		}
	}
	found := regexp.MustCompile(`(_app/immutable/entry/start\.[^"']+\.js)`).FindStringSubmatch(html)
	if found == nil {
		t.Fatal("the home page loads no _app/immutable/entry/start.*.js")
	}
	script := "/" + found[1]
	for _, e := range []struct {
		accept   string
		encoding any // nil for none
	}{{"br, gzip", "br"}, {"gzip", "gzip"}, {"identity", nil}} {
		// An explicit Accept-Encoding also stops Go's client from
		// decompressing, so the body is what the server sent.
		res := c.do(http.MethodGet, script, http.Header{"Accept-Encoding": {e.accept}})
		want, _ := json.Marshal([]any{e.accept, 200, e.encoding, "text/javascript; charset=utf-8", "public, max-age=31536000, immutable", "Accept-Encoding"})
		expectJSON(t, []any{e.accept, res.status, header(res, "Content-Encoding"), header(res, "Content-Type"), header(res, "Cache-Control"), header(res, "Vary")}, string(want))
	}
	gone := c.do(http.MethodGet, "/_app/immutable/nope.js", nil)
	expectJSON(t, []any{gone.status, gone.body, header(gone, "Cache-Control")}, `[404,"404 page not found\n","no-cache"]`)
	route := c.do(http.MethodGet, "/game/ABCDEF", nil)
	if route.status != 200 || !strings.Contains(route.body, "og:title") {
		t.Errorf("/game/ABCDEF: %d, og:title %v", route.status, strings.Contains(route.body, "og:title"))
	}
	if status := c.do(http.MethodPost, "/", nil).status; status != 405 {
		t.Errorf("POST /: %d, want 405", status)
	}
	alice := c.players(1)[0]
	code := alice.create()
	invite := c.do(http.MethodGet, "/game/"+code, nil).body
	for _, want := range []string{
		`<meta property="og:title" content="` + alice.name() + ` invites you to Mamdani Chess" />`,
		`<meta property="og:url" content="` + base + `/game/` + code + `" />`,
	} {
		if !strings.Contains(invite, want) {
			t.Errorf("the invite page lacks %q", want)
		}
	}
	for path, title := range map[string]string{
		"/play":        "Quick match · Mamdani Chess",
		"/how-to-play": "How to play · Mamdani Chess",
		"/rules":       "Rules · Mamdani Chess",
		"/practice":    "Practice · Mamdani Chess",
		"/about":       "About · Mamdani Chess",
	} {
		if body := c.do(http.MethodGet, path, nil).body; !strings.Contains(body, "<title>"+title+"</title>") {
			t.Errorf("%s lacks its title", path)
		}
	}
}

// A stream stays open through a quiet spell, with a heartbeat every 15 s.
func TestHeartbeat(t *testing.T) {
	t.Parallel() // a 15 s wait; before newCheck, whose deadline starts at once
	c := newCheck(t, 25*time.Second)
	p := c.players(1)[0]
	r := p.stream(p.create())
	r.state()
	start := time.Now()
	if event, data := r.next(); event != "comment" || data != "ping" {
		t.Errorf("got %q %q, want the comment ping", event, data)
	}
	if d := time.Since(start); d <= 10*time.Second {
		t.Errorf("a heartbeat after %v, want over 10 s", d)
	}
}

// A missed first move aborts the game after a minute.
func TestFirstMoveAbort(t *testing.T) {
	if !slow {
		t.Skip("takes a minute: set CONTRACT_SLOW=1")
	}
	t.Parallel() // a minute's wait; before newCheck, whose deadline starts at once
	c := newCheck(t, 70*time.Second)
	g := c.seated()
	v := g.a.until(func(v view) bool { return v.Status == game.Over })
	expectJSON(t, []any{v.Status, v.get("result")}, `["over",{"draw":false,"reason":"aborted"}]`)
}

// A page view or an error report is a 204 with no body. The 204s are sent
// as the playtest (X-Playtest), so a run against production (BASE_URL)
// stores nothing there.
func TestVisit(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	res := c.do(http.MethodPost, "/api/visit", http.Header{"Content-Type": {"application/json"}})
	// c.do sends no body: a bad body is a 400 in the API's usual shape.
	expectJSON(t, []any{res.status, res.body}, `[400,"{\"error\":\"bad request body\"}\n"]`)
	p := c.players(1)[0]
	asPlaytest := func(path, body string) reply {
		t.Helper()
		return readReply(t, p.send(c.ctx, http.MethodPost, path, http.Header{"Content-Type": {"application/json"}, "X-Playtest": {"1"}}, body))
	}
	if r := asPlaytest("/api/visit", `{"path":"/game/ABCDEF","w":390,"h":844,"touch":true,"referrer":""}`); r.status != 204 || r.body != "" {
		t.Errorf("POST /api/visit: %d %q", r.status, r.body)
	}
	if r := asPlaytest("/api/error", `{"path":"/","message":"TypeError: x"}`); r.status != 204 || r.body != "" {
		t.Errorf("POST /api/error: %d %q", r.status, r.body)
	}
	if status := c.do(http.MethodGet, "/api/visit", nil).status; status != 404 {
		t.Errorf("GET /api/visit: %d, want 404", status)
	}
}

// /stats is Basic Auth with STATS_PASSWORD, and reads as HTML.
func TestStatsPage(t *testing.T) {
	c := newCheck(t, 5*time.Second)
	// The server the checks start has STATS_PASSWORD=contract; a running
	// one (BASE_URL) needs its own from the environment.
	pass := "contract"
	if os.Getenv("BASE_URL") != "" {
		if pass = os.Getenv("STATS_PASSWORD"); pass == "" {
			t.Skip("BASE_URL without STATS_PASSWORD: can't check the page on a running server")
		}
	}
	res := c.do(http.MethodGet, "/stats", nil)
	expectJSON(t, []any{res.status, header(res, "WWW-Authenticate")}, `[401,"Basic realm=\"stats\""]`)
	res = c.do(http.MethodGet, "/stats?days=7", http.Header{"Authorization": {"Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pass))}})
	if res.status != 200 || !strings.Contains(res.body, "<h1>Stats</h1>") || !strings.Contains(res.body, "7 days") {
		t.Errorf("GET /stats: %d, HTML %v", res.status, strings.Contains(res.body, "<h1>Stats</h1>"))
	}
}
