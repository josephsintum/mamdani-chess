package store

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func seedGames(t *testing.T, s *Store) {
	t.Helper()
	ctx := t.Context()
	game := func(code, kind, white, black string, created time.Time, turns int, reason, winner string, st *GameStats, rematchOf string) {
		t.Helper()
		g := Game{Code: code, Kind: kind, White: white, WhiteName: white, CreatedAt: created, RematchOf: rematchOf}
		if kind == "quick" || rematchOf != "" { // both seated from the start
			g.Black, g.BlackName = black, black
		}
		must(t, s.CreateGame(ctx, g))
		if g.Black == "" && black != "" {
			must(t, s.SeatBlack(ctx, code, black, black, created.Add(100*time.Second)))
		}
		for i := 0; i < turns; i++ {
			left := int64(600000 - 10000*i)
			must(t, s.AddTurn(ctx, code, Turn{Ply: i, Move: "e2e4", WhiteMS: left, BlackMS: left - 5000, At: created.Add(time.Duration(i+1) * 20 * time.Second)}))
		}
		if reason != "" {
			must(t, s.EndGame(ctx, code, Result{EndedAt: created.Add(time.Duration(turns+1) * 20 * time.Second), Reason: reason, Winner: winner}, nil, st))
		}
	}
	// Saturday Oct 3, 2026, 8 pm New York: two quick games, one decided by a roll.
	sat := day(3, 20)
	game("Q00001", "quick", "ann", "bob", sat, 30, "checkmate", "white", &GameStats{Moves: 30, Opened: 5, Fell: 2, WhiteLost: 0, BlackLost: 2, SavingRolls: 1, Saved: 1, Repaired: 1, OpenHist: [6]int{5, 10, 10, 5, 0, 0}}, "")
	game("Q00002", "quick", "cat", "dan", sat.Add(time.Hour), 12, "checkmate", "black", &GameStats{Moves: 12, Opened: 3, Fell: 1, WhiteLost: 1, MateByRoll: true, OpenHist: [6]int{2, 5, 5, 0, 0, 0}}, "")
	// Sunday: a friend game resigned (White lost more to the road and still won), its rematch on time, and a link nobody opened.
	sun := day(4, 15)
	game("F00001", "friend", "ann", "eve", sun, 40, "resignation", "white", &GameStats{Moves: 40, Opened: 6, Fell: 3, WhiteLost: 2, BlackLost: 1, Reset: 1, ClosedCap: 1, ClosedRounds: 4, OpenHist: [6]int{10, 10, 10, 5, 3, 2}}, "")
	game("F00002", "friend", "eve", "ann", sun.Add(time.Hour), 20, "timeout", "black", &GameStats{Moves: 20, Opened: 2, OpenHist: [6]int{10, 10, 0, 0, 0, 0}}, "F00001")
	game("F00003", "friend", "fay", "", sun.Add(2*time.Hour), 0, "expired", "", nil, "")
	// An aborted quick game (no first move).
	game("Q00003", "quick", "gus", "hal", sun.Add(3*time.Hour), 0, "aborted", "", nil, "")
	// A playtest's quick game on Saturday evening, finished with a stats row:
	// it changes no number.
	must(t, s.CreateGame(ctx, Game{Code: "P00001", Kind: "quick", White: "pat", WhiteName: "pat", Black: "pam", BlackName: "pam", CreatedAt: sat.Add(30 * time.Minute), Playtest: true}))
	must(t, s.AddTurn(ctx, "P00001", Turn{Ply: 0, Move: "e2e4", WhiteMS: 1000, BlackMS: 600000, At: sat.Add(31 * time.Minute)}))
	must(t, s.EndGame(ctx, "P00001", Result{EndedAt: sat.Add(32 * time.Minute), Reason: "timeout", Winner: "black"}, nil,
		&GameStats{Moves: 1, Opened: 1, Fell: 1, WhiteLost: 1, SavingRolls: 1, OpenHist: [6]int{0, 1, 0, 0, 0, 0}}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sat, EndedAt: sat.Add(19 * time.Second), Guest: "ann000000000", Matched: true, Others: 0}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sat, EndedAt: sat.Add(19 * time.Second), Guest: "bob000000000", Matched: true, Others: 1}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sun, EndedAt: sun.Add(65 * time.Second), Guest: "fay000000000", Matched: false, Others: 0}))
	must(t, s.AddSearch(ctx, Search{StartedAt: sun, EndedAt: sun.Add(150 * time.Second), Guest: "gus000000000", Matched: true, Others: 0}))
	must(t, s.AddEvent(ctx, "practice", sat))
	must(t, s.AddEvent(ctx, "practice", sun))
	must(t, s.AddEvent(ctx, "reconnect", sun))
	must(t, s.AddEvent(ctx, "restart", sun))
}

func TestGameSection(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	seedGames(t, s)
	g, err := s.GameSection(ctx, day(1, 0), day(8, 0), newYork)
	must(t, err)
	check := func(what string, got, want any) {
		t.Helper()
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s = %v, want %v", what, got, want)
		}
	}
	check("games", g.Games, 6)
	check("got going", g.GotGoing, 4)
	check("days", len(g.Days), 7)
	check("Sat", g.Days[2], KindDay{"2026-10-03", 2, 0, 1})
	check("Sun", g.Days[3], KindDay{"2026-10-04", 1, 3, 1}) // Q00003; F00001, its rematch F00002, F00003
	check("ends", g.Ends, []Count{{"Checkmate", 1}, {"Mate by a roll", 1}, {"Resignation", 1}, {"Out of time", 1}, {"Aborted", 1}, {"Nobody came", 1}})
	check("lengths", g.Lengths, []Count{{"1–10", 0}, {"11–20", 2}, {"21–30", 1}, {"31–40", 1}, {"41–50", 0}, {"51–60", 0}, {"61–80", 0}, {"81+", 0}})
	check("median moves", g.MedianMoves, 25) // 12, 20, 30, 40 → (20+30)/2
	check("on clock", g.OnClock, 1)
	check("winner clock", g.WinnerClockLeft, "5:58") // 210, 310, 405, 485 s → median 357.5 s
	check("heat Sat 8pm", g.Heat[5][10], 2)          // Saturday, the 20–22 block
	// Sunday: F00001 (3 pm) and F00002 (4 pm) got going; F00003 (5 pm,
	// expired) and Q00003 (6 pm, aborted) didn't.
	check("heat Sun", g.Heat[6][7]+g.Heat[6][8]+g.Heat[6][9], 2)
	// A rematch isn't a link: F00002 (seated from the start) is left out,
	// so links are F00001 and F00003 = 2, joined F00001 = 1, finished
	// F00001 = 1, median join F00001's 100 s = 1:40; rematches of friend
	// games: F00002 = 1.
	check("friend", fmt.Sprint(g.Friend.Links, g.Friend.Joined, g.Friend.Finished, g.Friend.Rematches, " ", g.Friend.MedianJoin), "2 1 1 1 1:40")
	check("quick", fmt.Sprint(g.Quick.Searches, g.Quick.Matched, g.Quick.GaveUp, g.Quick.Alone, " ", g.Quick.MedianWait, " ", g.Quick.MedianGaveUp), "4 3 1 3 0:19 1:05")
	check("waits", g.Quick.Waits, []Count{{"under 10s", 0}, {"10–30s", 2}, {"30–60s", 0}, {"1–2 min", 0}, {"2 min+", 1}})
	check("road", fmt.Sprint(g.Road.Games, g.Road.Opened, g.Road.Fell, g.Road.SavingRolls, g.Road.Saved, g.Road.Repaired, g.Road.Reset, g.Road.MateByRoll, g.Road.ClosedRounds, g.Road.ClosedCap, g.Road.NoFall), "4 16 6 1 1 1 1 1 4 1 1")
	check("open hist", g.Road.OpenHist, [6]int{27, 35, 25, 10, 3, 2})
	// Q00001: white lost 0 < black 2 → less; Q00002: white 1 > black 0, black won → less; F00001: white 2 > black 1, white won → more; F00002: 0 = 0 → same.
	check("decided", fmt.Sprint(g.Road.LessWon, g.Road.MoreWon, g.Road.Same), "2 1 1")
	check("health", fmt.Sprint(g.Reconnects, g.Restarts), "1 1")
}

// Only "timeout" is decided on the clock ("timeout_vs_insufficient" is a
// draw), and the heat map counts only the games that got going.
func TestGameSectionOnClockAndHeat(t *testing.T) {
	ctx := t.Context()
	s, _ := openTemp(t)
	sat := day(3, 20)
	for i, reason := range []string{"timeout", "timeout_vs_insufficient", "aborted"} {
		code := fmt.Sprintf("C0000%d", i+1)
		must(t, s.CreateGame(ctx, Game{Code: code, Kind: "quick", White: "w", WhiteName: "w", Black: "b", BlackName: "b", CreatedAt: sat}))
		if reason != "aborted" {
			must(t, s.AddTurn(ctx, code, Turn{Ply: 0, Move: "e2e4", WhiteMS: 600000, BlackMS: 600000, At: sat.Add(time.Minute)}))
		}
		must(t, s.EndGame(ctx, code, Result{EndedAt: sat.Add(time.Hour), Reason: reason, Winner: "white"}, nil, nil))
	}
	g, err := s.GameSection(ctx, day(1, 0), day(8, 0), newYork)
	must(t, err)
	if g.OnClock != 1 || g.GotGoing != 2 || g.Heat[5][10] != 2 {
		t.Fatalf("on clock %d, got going %d, heat %d; want 1, 2, 2", g.OnClock, g.GotGoing, g.Heat[5][10])
	}
}

func TestGameSectionEmpty(t *testing.T) {
	s, _ := openTemp(t)
	g, err := s.GameSection(t.Context(), day(1, 0), day(3, 0), newYork)
	must(t, err)
	if g.Games != 0 || len(g.Days) != 2 || g.MedianMoves != 0 || g.WinnerClockLeft != "" || g.Quick.MedianWait != "" || len(g.Lengths) != 8 {
		t.Fatalf("empty: %+v", g)
	}
	g, err = s.GameSection(t.Context(), time.Time{}, day(3, 0), newYork)
	must(t, err)
	if len(g.Days) != 0 {
		t.Fatalf("all time with no games has days: %v", g.Days)
	}
}

// Nothing the page reads carries a game code.
func TestGameSectionHasNoCodes(t *testing.T) {
	s, _ := openTemp(t)
	seedGames(t, s)
	g, err := s.GameSection(t.Context(), day(1, 0), day(8, 0), newYork)
	must(t, err)
	if text := fmt.Sprintf("%+v", g); strings.Contains(text, "Q00001") || strings.Contains(text, "F00001") {
		t.Fatalf("a code leaked: %s", text)
	}
}

func TestIsFinishedMatchesTheSQLList(t *testing.T) {
	reasons := []string{"checkmate", "stalemate", "fifty_moves", "repetition", "insufficient_material", "timeout", "timeout_vs_insufficient", "resignation"}
	for _, r := range reasons {
		if !isFinished(r) || !strings.Contains(finishedReasons, "'"+r+"'") {
			t.Errorf("%s: isFinished and finishedReasons disagree", r)
		}
	}
	if got := strings.Count(finishedReasons, "'") / 2; got != len(reasons) {
		t.Errorf("finishedReasons has %d reasons, want %d", got, len(reasons))
	}
	if isFinished("aborted") || isFinished("") {
		t.Error("an unfinished reason counts as finished")
	}
}
