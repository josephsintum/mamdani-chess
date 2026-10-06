// Command server runs Pothole Chess: Mamdani Edition.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mamdani-chess/game"
	"mamdani-chess/server"
	"mamdani-chess/store"
	"mamdani-chess/web"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run() error {
	// Set before server.New, which keeps slog.Default() for request logs.
	slog.SetDefault(newLogger(os.Getenv("LOG_FORMAT"), os.Stderr))
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data/mamdani.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	hub := game.NewHub(game.CryptoDice{}, st)
	if err := restore(context.Background(), st, hub, time.Now()); err != nil {
		return err
	}
	handler := server.New(st, hub, web.Assets())
	// Railway sets this to the deployed commit; locally it's empty ("dev").
	handler.Version = os.Getenv("RAILWAY_GIT_COMMIT_SHA")
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	// Ends SSE streams when Shutdown starts; other in-flight requests finish.
	srv.RegisterOnShutdown(handler.Close)

	sig, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "db", dbPath)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		return err
	case <-sig.Done():
	}
	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// restore ends games nobody joined within a day, then rebuilds every
// unfinished game, and every game that ended within the last day, so open
// tabs carry on after a restart or a deploy.
func restore(ctx context.Context, st *store.Store, hub *game.Hub, now time.Time) error {
	start := time.Now()
	expired, err := st.ExpireWaiting(ctx, now.Add(-game.DefaultIdle), now)
	if err != nil {
		return fmt.Errorf("expire waiting games: %w", err)
	}
	saved, err := st.LoadForRestore(ctx, now.Add(-game.DefaultIdle))
	if err != nil {
		return fmt.Errorf("load saved games: %w", err)
	}
	n := hub.Restore(saved)
	// One line for the whole restore (each game logs at DEBUG): after a busy
	// day, a line per game would pass Railway's 500 lines/s at once.
	slog.Info("games restored", "restored", n, "failed", len(saved)-n, "expired", expired, "duration", time.Since(start))
	return nil
}

// newLogger writes JSON when format is "json" (the production image sets
// LOG_FORMAT=json, so the host's log search can filter on fields) and
// readable text otherwise.
func newLogger(format string, w io.Writer) *slog.Logger {
	if format == "json" {
		return slog.New(slog.NewJSONHandler(w, nil))
	}
	return slog.New(slog.NewTextHandler(w, nil))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
