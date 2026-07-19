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

	"gitee.com/zlx23/homehub/services/drop/internal/config"
	"gitee.com/zlx23/homehub/services/drop/internal/httpapi"
	"gitee.com/zlx23/homehub/services/drop/internal/store"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := serve(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() error {
	configuration, err := config.Load()
	if err != nil {
		return err
	}
	password, err := configuration.DatabasePassword()
	if err != nil {
		return err
	}
	startup, cancelStartup := context.WithTimeout(context.Background(), 15*time.Second)
	storage, err := store.Open(startup, store.Options{
		Host: configuration.DatabaseHost, Port: configuration.DatabasePort, Database: configuration.DatabaseName,
		User: configuration.DatabaseUser, Password: password, DataDirectory: configuration.DataDirectory, QuotaBytes: configuration.QuotaBytes,
	})
	if err != nil {
		cancelStartup()
		return err
	}
	keys, err := identity.FetchJWKSet(startup, &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}, configuration.IAMJWKSURL)
	cancelStartup()
	if err != nil {
		storage.Close()
		return err
	}
	verifier, err := identity.NewVerifier(keys, "homehub-drop", 2*time.Minute)
	if err != nil {
		storage.Close()
		return err
	}
	defer storage.Close()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	server := &http.Server{
		Addr: configuration.ListenAddress, Handler: httpapi.New(configuration, storage, verifier, logger),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Minute, WriteTimeout: 0, IdleTimeout: 60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go cleanupLoop(ctx, storage, logger)
	errorsChannel := make(chan error, 1)
	go func() {
		logger.Info("HomeHub Drop listening", "address", configuration.ListenAddress)
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- err
		}
		close(errorsChannel)
	}()
	select {
	case <-ctx.Done():
	case err := <-errorsChannel:
		if err != nil {
			return err
		}
	}
	shutdown, cancel := context.WithTimeout(context.Background(), configuration.ShutdownTimeout)
	defer cancel()
	return server.Shutdown(shutdown)
}

func cleanupLoop(ctx context.Context, storage *store.Store, logger *slog.Logger) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		cleanup, cancel := context.WithTimeout(ctx, 20*time.Second)
		deleted, err := storage.CleanupExpired(cleanup, 200)
		cancel()
		if err != nil && ctx.Err() == nil {
			logger.Warn("cleanup expired Drop items", "error", err)
		}
		if deleted > 0 {
			logger.Info("expired Drop items removed", "count", deleted)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func healthcheck() error {
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	response, err := client.Get("http://127.0.0.1:8080/health/ready")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %s", response.Status)
	}
	return nil
}
