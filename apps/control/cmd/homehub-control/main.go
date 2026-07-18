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

	"homehub.local/control/internal/catalog"
	"homehub.local/control/internal/config"
	"homehub.local/control/internal/httpapi"
	"homehub.local/go-sdk/identity"
)

var version = "dev"
var commit = "unknown"
var buildTime = "unknown"

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
		return err
	}
	identityClient := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
	loadContext, cancelLoad := context.WithTimeout(context.Background(), 5*time.Second)
	keys, err := identity.FetchJWKSet(loadContext, identityClient, cfg.IAMJWKSURL)
	cancelLoad()
	if err != nil {
		return fmt.Errorf("initialize IAM verification keys: %w", err)
	}
	verifier, err := identity.NewVerifier(keys, "homehub-control", 2*time.Minute)
	if err != nil {
		return fmt.Errorf("initialize access token verifier: %w", err)
	}
	healthClient := &http.Client{Timeout: cfg.HealthTimeout, Transport: &http.Transport{Proxy: nil}}
	handler := httpapi.New(httpapi.Options{
		Logger: logger, Verifier: verifier, Services: services, HealthClient: healthClient,
		Version: version, Commit: commit, Environment: cfg.Environment,
	})
	server := &http.Server{
		Addr: cfg.ListenAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("HomeHub Control listening", "address", cfg.ListenAddress, "services", len(services), "version", version)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- err
		}
		close(errorsChannel)
	}()
	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case err := <-errorsChannel:
		if err != nil {
			return err
		}
	}
	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	return server.Shutdown(shutdownContext)
}

func healthcheck() error {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://127.0.0.1:8080/health/live", nil)
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}

func newLogger(name string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(name) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
