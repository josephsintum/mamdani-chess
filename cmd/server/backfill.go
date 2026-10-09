package main

import (
	"context"
	"log/slog"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/store"
)

// backfillStats writes a stats row for every finished game that has none,
// by replaying its saved dice: games that ended before stats were kept.
// Each game is written on its own, so a game that won't replay is skipped
// and logged, and a run cut short leaves the rest for the next start.
func backfillStats(ctx context.Context, st *store.Store) (done, failed int, err error) {
	games, err := st.GamesWithoutStats(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, sg := range games {
		stats, err := game.StatsOf(sg.Turns, *sg.Result)
		if err != nil {
			slog.Warn("game stats not backfilled", "code", sg.Code, "err", err)
			failed++
			continue
		}
		if err := st.AddGameStats(ctx, sg.Code, stats); err != nil {
			return done, failed, err
		}
		done++
	}
	return done, failed, nil
}

// restartEvent counts this start on the stats page's Health section.
func restartEvent(ctx context.Context, st *store.Store) error {
	return st.AddEvent(ctx, "restart", time.Now())
}
