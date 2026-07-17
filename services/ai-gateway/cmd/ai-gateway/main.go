package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"homehub.local/go-sdk/httpx"
	"homehub.local/go-sdk/identity"
	"homehub.local/services/ai-gateway/internal/httpapi"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		response, err := http.Get("http://127.0.0.1:8080/health/ready")
		if err != nil || response.StatusCode != http.StatusNoContent {
			os.Exit(1)
		}
		_ = response.Body.Close()
		return
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("AI Gateway stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	address := env("AI_GATEWAY_LISTEN_ADDRESS", ":8080")
	keyFile := env("AI_GATEWAY_IDENTITY_PUBLIC_KEY_FILE", "/run/secrets/identity_public_key")
	verifier, err := identity.NewVerifierFromFile(keyFile, "ai-gateway")
	if err != nil {
		return fmt.Errorf("initialize HomeHub identity: %w", err)
	}
	handler := httpx.RequestID(httpx.Recover(logger, httpx.SecurityHeaders(httpapi.New(verifier))))
	server := &http.Server{
		Addr: address, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 30 * time.Second, WriteTimeout: 0, IdleTimeout: 60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("AI Gateway listening", "address", address)
		errorsChannel <- server.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err := <-errorsChannel:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdown)
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
