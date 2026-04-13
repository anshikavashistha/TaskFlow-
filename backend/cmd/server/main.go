package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anshika/taskflow/internal/config"
	"github.com/anshika/taskflow/internal/db"
	"github.com/anshika/taskflow/internal/handlers"
	"github.com/anshika/taskflow/internal/router"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("migrations", "err", err)
		os.Exit(1)
	}

	if cfg.Seed {
		if err := db.RunSeed(ctx, pool); err != nil {
			slog.Error("seed", "err", err)
			os.Exit(1)
		}
	}

	api := &handlers.API{Pool: pool, JWTSecret: cfg.JWTSecret}
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router.New(api),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
