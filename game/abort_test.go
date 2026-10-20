package game

import (
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"mamdani-chess/store"
)

// White misses the first move: the game is aborted, saves no stats row,
// and the stats count one aborted game.
func TestAbortedGameSavesNoStatsRow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		st, err := store.Open(filepath.Join(t.TempDir(), "abort.db"))
		if err != nil {
			t.Fatal(err)
		}
		defer st.Close()
		g, err := NewHub(odd{}, st).CreatePair("white", "black", false)
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(FirstMoveTime + time.Second)
		synctest.Wait()
		has, err := st.HasGameStats(t.Context(), g.Code())
		if err != nil {
			t.Fatal(err)
		}
		// Up to tomorrow, as the page asks: Summarize lists every day.
		tomorrow := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
		days, err := st.Counts(t.Context(), "", tomorrow)
		if err != nil {
			t.Fatal(err)
		}
		section := store.Summarize(days, "", tomorrow)
		if has || len(section.Ends) != 1 || section.Ends[0] != (store.Count{Label: "Aborted", N: 1}) {
			t.Fatalf("stats row %v, ends %v; want none and one aborted", has, section.Ends)
		}
		time.Sleep(DefaultIdle + time.Minute) // evicted, so the bubble's goroutines end
		synctest.Wait()
	})
}
