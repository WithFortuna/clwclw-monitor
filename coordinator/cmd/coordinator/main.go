package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"clwclw-monitor/coordinator/internal/config"
	"clwclw-monitor/coordinator/internal/httpapi"
	"clwclw-monitor/coordinator/internal/store"
	"clwclw-monitor/coordinator/internal/store/memory"
	"clwclw-monitor/coordinator/internal/store/postgres"
)

func main() {
	cfg := config.Load()
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	defer cancelRoot()

	var st store.Store
	var closer func()

	if cfg.DatabaseURL != "" {
		pg, err := postgres.NewStore(cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("failed to init postgres store: %v", err)
		}
		st = pg
		closer = pg.Close
		log.Printf("using postgres store")
	} else {
		st = memory.NewStore()
		log.Printf("using memory store")
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
			log.Printf("event retention enabled but store does not support purge")
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
		log.Printf("coordinator listening on %s", cfg.ListenAddr())
		errCh <- httpServer.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-stop:
		log.Printf("shutdown requested")
	case err := <-errCh:
		log.Printf("server error: %v", err)
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
			log.Printf("retention purge failed: %v", err)
			return
		}
		if n > 0 {
			log.Printf("retention purged %d events (< %s)", n, before.Format(time.RFC3339))
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
