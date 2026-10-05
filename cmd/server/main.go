// Command server runs Pothole Chess: Mamdani Edition.
package main

import (
	"context"
	"errors"
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

	handler := server.New(st, game.NewHub(game.CryptoDice{}, st), web.Assets())
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
