package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"drop/internal/auth"
	"drop/internal/config"
	"drop/internal/httpapi"
	"drop/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("drop stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	storage, err := store.Open(ctx, store.Options{
		DataDir: cfg.DataDir, QuotaBytes: cfg.QuotaBytes, InlineTextBytes: cfg.InlineTextBytes,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := storage.Close(); err != nil {
			logger.Error("close storage", "error", err)
		}
	}()
	authService, err := auth.NewService(storage, auth.Options{CodeTTL: cfg.CodeTTL, SessionTTL: cfg.SessionTTL})
	if err != nil {
		return err
	}
	hub := httpapi.NewHub()
	api := httpapi.New(cfg, storage, authService, hub, logger)
	if err := api.EnableHomeHubIdentity(); err != nil {
		return fmt.Errorf("initialize HomeHub identity: %w", err)
	}

	var background sync.WaitGroup
	startBackground := func(task func()) {
		background.Add(1)
		go func() {
			defer background.Done()
			task()
		}()
	}
	startBackground(func() { cleanupLoop(ctx, storage, hub, cfg, logger) })
	startBackground(func() { trafficMaintenanceLoop(ctx, storage, logger) })

	server := &http.Server{
		Addr: cfg.ListenAddr, Handler: api.HomeHubHandler(),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout, ReadTimeout: cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("listener ready", "entry", "homehub", "address", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-serverErrors:
		logger.Error("HTTP server failed", "error", err)
	}
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	shutdownErr := server.Shutdown(shutdownCtx)
	cancel()
	background.Wait()
	flushCtx, flushCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := storage.FlushTraffic(flushCtx); err != nil {
		shutdownErr = errors.Join(shutdownErr, err)
	}
	flushCancel()
	return shutdownErr
}

const trafficRetention = 32 * 24 * time.Hour

func trafficMaintenanceLoop(ctx context.Context, storage *store.Store, logger *slog.Logger) {
	flushTicker := time.NewTicker(10 * time.Second)
	pruneTicker := time.NewTicker(24 * time.Hour)
	defer flushTicker.Stop()
	defer pruneTicker.Stop()
	pruneTraffic(ctx, storage, logger)
	for {
		select {
		case <-ctx.Done():
			return
		case <-flushTicker.C:
			flushCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := storage.FlushTraffic(flushCtx)
			cancel()
			if err != nil && ctx.Err() == nil {
				logger.Warn("flush traffic metrics", "error", err)
			}
		case <-pruneTicker.C:
			pruneTraffic(ctx, storage, logger)
		}
	}
}

func pruneTraffic(ctx context.Context, storage *store.Store, logger *slog.Logger) {
	pruneCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err := storage.PurgeTrafficBefore(pruneCtx, time.Now().UTC().Add(-trafficRetention))
	cancel()
	if err != nil && ctx.Err() == nil {
		logger.Warn("prune traffic metrics", "error", err)
	}
}

func cleanupLoop(ctx context.Context, storage *store.Store, hub *httpapi.Hub, cfg config.Config, logger *slog.Logger) {
	if err := cleanup(ctx, storage, hub, cfg, logger); err != nil && ctx.Err() == nil {
		logger.Error("initial cleanup failed", "error", err)
	}
	ticker := time.NewTicker(cfg.CleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := cleanup(ctx, storage, hub, cfg, logger); err != nil && ctx.Err() == nil {
				logger.Error("background cleanup failed", "error", err)
			}
		}
	}
}

func cleanup(ctx context.Context, storage *store.Store, hub *httpapi.Hub, cfg config.Config, logger *slog.Logger) error {
	deleted, err := storage.CleanupExpired(ctx, 500)
	if err != nil {
		return err
	}
	if deleted > 0 {
		hub.Publish("expired", "")
		logger.Info("expired items removed", "count", deleted)
	}
	if _, err := storage.CleanupTmp(time.Now().Add(-cfg.TmpMaxAge)); err != nil {
		return err
	}
	if err := storage.PurgeExpiredAuth(ctx, time.Now().UTC()); err != nil {
		return err
	}
	return nil
}
