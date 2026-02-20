package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clwclw-monitor/coordinator/internal/config"
	"clwclw-monitor/coordinator/internal/httpapi"
	"clwclw-monitor/coordinator/internal/logger"
	"clwclw-monitor/coordinator/internal/store"
	"clwclw-monitor/coordinator/internal/store/memory"
	"clwclw-monitor/coordinator/internal/store/postgres"
)

func main() {
	cfg := config.Load()

	if err := logger.Init(cfg.LogLevel, cfg.LogFormat, cfg.LogFile); err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Close()

	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	var st store.Store
	var closer func()

	if cfg.DatabaseURL != "" {
		pg, err := postgres.NewStore(cfg.DatabaseURL)
		if err != nil {
			slog.Error("failed to init postgres store", "error", err)
			os.Exit(1)
		}
		st = pg
		closer = pg.Close
		slog.Info("using postgres store")
	} else {
		st = memory.NewStore()
		slog.Info("using memory store")
	}

	if closer != nil {
		defer closer()
	}

	if cfg.EventRetentionDays > 0 {
		if purger, ok := st.(interface {
			PurgeEventsBefore(ctx context.Context, before time.Time) (int, error)
		}); ok {
			go runEventRetentionLoop(rootCtx, purger, cfg.EventRetentionDays, cfg.RetentionIntervalHours)
		} else {
			slog.Info("event retention enabled but store does not support purge")
		}
	}

	srv := httpapi.NewServer(cfg, st)

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr(),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("coordinator listening", "addr", cfg.ListenAddr())
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		slog.Info("shutdown requested")
	case err := <-errCh:
		slog.Error("server error", "error", err)
	}

	cancelRoot()

	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	_ = httpServer.Shutdown(ctxShutdown)
}

func runEventRetentionLoop(
	ctx context.Context,
	purger interface {
		PurgeEventsBefore(ctx context.Context, before time.Time) (int, error)
	},
	retentionDays int,
	intervalHours int,
) {
	retention := time.Duration(retentionDays) * 24 * time.Hour
	interval := time.Duration(intervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	runOnce := func() {
		before := time.Now().UTC().Add(-retention)
		ctxPurge, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		n, err := purger.PurgeEventsBefore(ctxPurge, before)
		if err != nil {
			slog.Error("retention purge failed", "error", err)
			return
		}
		if n > 0 {
			slog.Info("retention purged events", "count", n, "before", before.Format(time.RFC3339))
		}
	}

	runOnce()

	t := time.NewTicker(interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			runOnce()
		}
	}
}
