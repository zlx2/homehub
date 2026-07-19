package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/bridge"
	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/config"
	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/drop"
	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/iam"
	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/telegram"
)

const serviceName = "telegram-bridge"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		response, err := http.Get("http://127.0.0.1:8730/health/ready")
		if err != nil || response.StatusCode != http.StatusNoContent {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("service stopped", "service", serviceName, "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	telegramClient := telegram.NewClient(cfg.TelegramAPIBaseURL, cfg.TelegramToken, 2*time.Minute)
	tokenSource := iam.NewClient(cfg.IAMBaseURL, cfg.IAMCredential, 10*time.Second)
	dropClient := drop.NewClient(cfg.DropBaseURL, tokenSource, cfg.RequestTimeout)
	worker := bridge.New(cfg, telegramClient, dropClient, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var ready atomic.Bool
	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           healthHandler(&ready),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	errorsChannel := make(chan error, 2)
	go func() {
		logger.Info("health server listening", "service", serviceName, "address", cfg.ListenAddress)
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- err
		}
	}()
	go func() {
		if err := worker.Prepare(ctx); err != nil {
			errorsChannel <- fmt.Errorf("prepare Telegram worker: %w", err)
			return
		}
		ready.Store(true)
		errorsChannel <- worker.Run(ctx)
	}()

	select {
	case <-ctx.Done():
	case err := <-errorsChannel:
		if err != nil && !errors.Is(err, context.Canceled) {
			stop()
			return err
		}
	}
	ready.Store(false)
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func healthHandler(ready *atomic.Bool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /health/ready", func(writer http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			http.Error(writer, "not ready", http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	return mux
}
