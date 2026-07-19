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
	"gitee.com/zlx23/homehub/apps/iam/internal/humanauth"
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
		if err := openFGA.WriteRelationship(context.Background(), authorizationState, "realm:homehub", "realm", "service:"+manifest.ServiceID); err != nil {
			slog.Error("service realm relationship synchronization failed", "service", manifest.ServiceID, "error", err)
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
	humanAuthCtx, humanAuthCancel := context.WithTimeout(context.Background(), 15*time.Second)
	humanAuthentication, err := humanauth.Open(humanAuthCtx, humanauth.Options{
		DatabaseURL:        databaseURL,
		EncryptionKeyFile:  strings.TrimSpace(os.Getenv("HOMEHUB_IAM_AUTH_ENCRYPTION_KEY_FILE")),
		BootstrapTokenFile: strings.TrimSpace(os.Getenv("HOMEHUB_IAM_OWNER_SETUP_TOKEN_FILE")),
		Authorization:      openFGA,
		AuthorizationState: authorizationState,
		Policies:           store,
		Signer:             signer,
		PasskeyRPID:        environmentOrDefault("HOMEHUB_IAM_PASSKEY_RP_ID", "zlx2.com"),
		PasskeyOrigins:     splitCSV(environmentOrDefault("HOMEHUB_IAM_PASSKEY_ORIGINS", "https://zlx2.com")),
	})
	humanAuthCancel()
	if err != nil {
		slog.Error("human authentication initialization failed", "error", err)
		os.Exit(1)
	}
	defer humanAuthentication.Close()
	allowedOrigins := splitCSV(environmentOrDefault("HOMEHUB_IAM_ALLOWED_ORIGINS", "https://zlx2.com,https://www.zlx2.com,https://111.229.205.99,http://127.0.0.1:18080"))
	secureCookies := !strings.EqualFold(strings.TrimSpace(os.Getenv("HOMEHUB_IAM_SECURE_COOKIES")), "false")

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
			JWKSet:         signer.JWKSet(),
			Exchanger:      tokenExchange,
			Verifier:       iamVerifier,
			Machines:       machineAdministrator,
			Humans:         humanAuthentication,
			AllowedOrigins: allowedOrigins,
			SecureCookies:  secureCookies,
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

func environmentOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
}

func databaseURLFromEnvironment() (string, error) {
	// Full URL from a file (existing behaviour)
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
	// Full URL from environment (existing behaviour, deprecated in favour of file)
	if value := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_DATABASE_URL")); value != "" {
		return value, nil
	}
	// Password-file based URL construction
	if passwordFile := strings.TrimSpace(os.Getenv("HOMEHUB_IAM_DATABASE_PASSWORD_FILE")); passwordFile != "" {
		passwordBytes, err := os.ReadFile(passwordFile)
		if err != nil {
			return "", err
		}
		password := strings.TrimSpace(string(passwordBytes))
		if password == "" {
			return "", errors.New("IAM database password file is empty")
		}
		host := environmentOrDefault("HOMEHUB_IAM_DATABASE_HOST", "postgres")
		port := environmentOrDefault("HOMEHUB_IAM_DATABASE_PORT", "5432")
		user := environmentOrDefault("HOMEHUB_IAM_DATABASE_USER", "homehub_iam")
		dbname := environmentOrDefault("HOMEHUB_IAM_DATABASE_NAME", "homehub_iam")
		sslmode := environmentOrDefault("HOMEHUB_IAM_DATABASE_SSLMODE", "disable")
		url := "postgres://" + user + ":" + password + "@" + host + ":" + port + "/" + dbname + "?sslmode=" + sslmode
		return url, nil
	}
	return "", errors.New("HOMEHUB_IAM_DATABASE_PASSWORD_FILE, HOMEHUB_IAM_DATABASE_URL_FILE, or HOMEHUB_IAM_DATABASE_URL is required")
}

func healthcheck() {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:8080/health/ready")
	if err != nil || response.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	_ = response.Body.Close()
}
