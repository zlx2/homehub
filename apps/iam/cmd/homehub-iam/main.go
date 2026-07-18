package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gitee.com/zlx23/homehub/apps/iam/authz"
	"gitee.com/zlx23/homehub/apps/iam/internal/bootstrap"
	"gitee.com/zlx23/homehub/apps/iam/internal/exchange"
	"gitee.com/zlx23/homehub/apps/iam/internal/httpapi"
	"gitee.com/zlx23/homehub/apps/iam/internal/machineadmin"
	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/apps/iam/internal/token"
	"gitee.com/zlx23/homehub/apps/iam/manifests"
	"homehub.local/go-sdk/identity"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		healthcheck()
		return
	}

	address := os.Getenv("HOMEHUB_IAM_LISTEN_ADDRESS")
	if address == "" {
		address = ":8080"
	}
	databaseURL, err := databaseURLFromEnvironment()
	if err != nil {
		slog.Error("IAM database configuration is invalid", "error", err)
		os.Exit(1)
	}

	startupCtx, startupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	store, err := storepostgres.Open(startupCtx, databaseURL)
	if err == nil {
		err = store.Migrate(startupCtx)
	}
	startupCancel()
	if err != nil {
		slog.Error("IAM database initialization failed", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	openFGA, err := authz.NewClient(strings.TrimSpace(os.Getenv("HOMEHUB_IAM_OPENFGA_URL")))
	if err != nil {
		slog.Error("OpenFGA configuration is invalid", "error", err)
		os.Exit(1)
	}
	authzCtx, authzCancel := context.WithTimeout(context.Background(), 15*time.Second)
	authorizationState, err := openFGA.EnsureModel(authzCtx, store, "homehub")
	authzCancel()
	if err != nil {
		slog.Error("OpenFGA authorization model initialization failed", "error", err)
		os.Exit(1)
	}
	slog.Info("OpenFGA authorization model ready", "store_id", authorizationState.StoreID, "model_id", authorizationState.ModelID)

	builtinManifests, err := manifests.Builtin()
	if err != nil {
		slog.Error("built-in service manifests are invalid", "error", err)
		os.Exit(1)
	}
	for _, manifest := range builtinManifests {
		if err := store.SyncManifest(context.Background(), manifest); err != nil {
			slog.Error("service manifest synchronization failed", "service", manifest.ServiceID, "error", err)
			os.Exit(1)
		}
	}

	signingKeyFile := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_SIGNING_KEY_FILE"))
	signingKeyID := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_SIGNING_KEY_ID"))
	if signingKeyFile == "" || signingKeyID == "" {
		slog.Error("IAM signing key configuration is required")
		os.Exit(1)
	}
	signer, err := token.NewSignerFromFile(signingKeyID, signingKeyFile, 2*time.Minute)
	if err != nil {
		slog.Error("IAM signing key initialization failed", "error", err)
		os.Exit(1)
	}
	iamVerifier, err := identity.NewVerifier(signer.VerificationKeys(), "homehub-iam", 2*time.Minute)
	if err != nil {
		slog.Error("IAM access token verifier initialization failed", "error", err)
		os.Exit(1)
	}
	rootCredentialFile := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_ROOT_AGENT_TOKEN_FILE"))
	if rootCredentialFile == "" {
		slog.Error("IAM root agent credential file is required")
		os.Exit(1)
	}
	bootstrapCtx, bootstrapCancel := context.WithTimeout(context.Background(), 15*time.Second)
	rootAgent, err := bootstrap.EnsureSystemAgent(bootstrapCtx, store, openFGA, authorizationState, rootCredentialFile)
	bootstrapCancel()
	if err != nil {
		slog.Error("root agent bootstrap failed", "error", err)
		os.Exit(1)
	}
	slog.Info("root agent ready", "subject", rootAgent.Subject())
	tokenExchange := exchange.New(store, openFGA, authorizationState, signer)
	machineAdministrator := machineadmin.New(store, openFGA, authorizationState)

	server := &http.Server{
		Addr: address,
		Handler: httpapi.New(httpapi.Options{
			Version: version,
			Readiness: func(ctx context.Context) error {
				if err := store.Ping(ctx); err != nil {
					return err
				}
				return openFGA.Ping(ctx)
			},
			JWKSet:    signer.JWKSet(),
			Exchanger: tokenExchange,
			Verifier:  iamVerifier,
			Machines:  machineAdministrator,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("IAM shutdown failed", "error", err)
		}
	}()

	slog.Info("HomeHub IAM starting", "address", address, "version", version)
	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		slog.Error("HomeHub IAM stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func databaseURLFromEnvironment() (string, error) {
	if path := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_DATABASE_URL_FILE")); path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		if value := strings.TrimSpace(string(contents)); value != "" {
			return value, nil
		}
		return "", errors.New("IAM database URL file is empty")
	}
	if value := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_DATABASE_URL")); value != "" {
		return value, nil
	}
	return "", errors.New("HOMEHUB_IAM_DATABASE_URL_FILE or HOMEHUB_IAM_DATABASE_URL is required")
}

func healthcheck() {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health/ready")
	if err != nil || response.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	_ = response.Body.Close()
}
