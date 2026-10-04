// Command server runs Pothole Chess: Mamdani Edition.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

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
	port := envOr("PORT", "8080")
	dbPath := envOr("DB_PATH", "data/mamdani.db")

	st, err := store.Open(dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// Cancelling base ends open SSE streams so shutdown doesn't hang on them.
	base, stopStreams := context.WithCancel(context.Background())
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           server.New(st, web.Assets()),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return base },
	}

	sig, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errc := make(chan error, 1)
	go func() {
		slog.Info("listening", "addr", srv.Addr, "db", dbPath)
		errc <- srv.ListenAndServe()
	}()

	select {
	case err := <-errc:
		stopStreams()
		return err
	case <-sig.Done():
	}
	slog.Info("shutting down")
	stopStreams()
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

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
