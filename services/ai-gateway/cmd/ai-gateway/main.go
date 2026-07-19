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

	"gitee.com/zlx23/homehub/packages/go-sdk/httpx"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
	"gitee.com/zlx23/homehub/services/ai-gateway/internal/config"
	"gitee.com/zlx23/homehub/services/ai-gateway/internal/gateway"
	"gitee.com/zlx23/homehub/services/ai-gateway/internal/httpapi"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get("http://127.0.0.1:8080/health/ready")
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
	jwksURL := env("AI_GATEWAY_IAM_JWKS_URL", "http://iam:8080/.well-known/jwks.json")
	configFile := env("AI_GATEWAY_CONFIG_FILE", "/etc/homehub-ai/providers.json")
	identityClient := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	loadContext, cancelLoad := context.WithTimeout(context.Background(), 5*time.Second)
	keys, err := identity.FetchJWKSet(loadContext, identityClient, jwksURL)
	cancelLoad()
	if err != nil {
		return fmt.Errorf("load IAM verification keys: %w", err)
	}
	verifier, err := identity.NewVerifier(keys, "homehub-ai-gateway", 2*time.Minute)
	if err != nil {
		return fmt.Errorf("initialize IAM access token verifier: %w", err)
	}
	cfg, err := config.Load(configFile)
	if err != nil {
		return err
	}
	router, err := gateway.New(cfg)
	if err != nil {
		return fmt.Errorf("initialize provider router: %w", err)
	}
	handler := httpx.RequestID(httpx.Recover(logger, httpx.SecurityHeaders(httpapi.New(verifier, router, logger))))
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
