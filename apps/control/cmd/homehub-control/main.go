package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"homehub.local/control/internal/auth"
	"homehub.local/control/internal/catalog"
	"homehub.local/control/internal/config"
	"homehub.local/control/internal/health"
	"homehub.local/control/internal/httpapi"
	"homehub.local/control/internal/identitytoken"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	command := "serve"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	var err error
	switch command {
	case "serve":
		err = serve()
	case "healthcheck":
		err = healthcheck()
	case "version":
		fmt.Printf("homehub-control %s commit=%s built=%s\n", version, commit, buildTime)
	default:
		err = fmt.Errorf("unknown command %q", command)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := newLogger(cfg.LogLevel)
	services, err := catalog.Load(cfg.CatalogFile)
	if err != nil {
		return fmt.Errorf("load service catalog: %w", err)
	}
	authService, err := auth.Open(context.Background(), auth.Config{
		Host:               cfg.DatabaseHost,
		Port:               cfg.DatabasePort,
		Database:           cfg.DatabaseName,
		User:               cfg.DatabaseUser,
		PasswordFile:       cfg.DatabasePasswordFile,
		EncryptionKeyFile:  cfg.AuthKeyFile,
		BootstrapTokenFile: cfg.BootstrapTokenFile,
	})
	if err != nil {
		return fmt.Errorf("initialize authentication: %w", err)
	}
	defer authService.Close()
	identitySigner, err := identitytoken.NewFromFile(cfg.IdentitySigningKeyFile)
	if err != nil {
		return fmt.Errorf("initialize service identity signer: %w", err)
	}

	monitor := health.NewMonitor(services, cfg.HealthInterval, cfg.HealthTimeout)
	handler := httpapi.New(httpapi.Options{
		Logger:         logger,
		Services:       services,
		Statuses:       monitor,
		Version:        version,
		Commit:         commit,
		Environment:    cfg.Environment,
		Auth:           authService,
		AllowedOrigins: cfg.AllowedOrigins,
		SecureCookies:  cfg.SecureCookies,
		IdentityIssuer: identitySigner,
	})

	server := &http.Server{
		Addr:              cfg.ListenAddress,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	go monitor.Run(ctx)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("homehub control listening",
			"address", cfg.ListenAddress,
			"environment", cfg.Environment,
			"services", len(services),
			"version", version,
			"commit", commit,
		)
		if listenErr := server.ListenAndServe(); listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			errCh <- listenErr
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case listenErr := <-errCh:
		if listenErr != nil {
			return fmt.Errorf("serve HTTP: %w", listenErr)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	return nil
}

func healthcheck() error {
	url := strings.TrimSpace(os.Getenv("HOMEHUB_HEALTHCHECK_URL"))
	if url == "" {
		url = "http://127.0.0.1:8080/health/live"
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			Proxy: nil,
		},
	}
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}

func newLogger(levelName string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(levelName) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
