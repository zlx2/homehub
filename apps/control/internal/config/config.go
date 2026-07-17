package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddress          string
	CatalogFile            string
	Environment            string
	LogLevel               string
	DatabaseHost           string
	DatabasePort           string
	DatabaseName           string
	DatabaseUser           string
	DatabasePasswordFile   string
	AuthKeyFile            string
	BootstrapTokenFile     string
	IdentitySigningKeyFile string
	AllowedOrigins         []string
	SecureCookies          bool
	HealthInterval         time.Duration
	HealthTimeout          time.Duration
	ShutdownTimeout        time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:          envOrDefault("HOMEHUB_LISTEN_ADDRESS", ":8080"),
		CatalogFile:            envOrDefault("HOMEHUB_CATALOG_FILE", "/etc/homehub/services.json"),
		Environment:            envOrDefault("HOMEHUB_ENVIRONMENT", "development"),
		LogLevel:               envOrDefault("HOMEHUB_LOG_LEVEL", "info"),
		DatabaseHost:           envOrDefault("HOMEHUB_DATABASE_HOST", "postgres"),
		DatabasePort:           envOrDefault("HOMEHUB_DATABASE_PORT", "5432"),
		DatabaseName:           envOrDefault("HOMEHUB_DATABASE_NAME", "homehub_control"),
		DatabaseUser:           envOrDefault("HOMEHUB_DATABASE_USER", "homehub_control"),
		DatabasePasswordFile:   envOrDefault("HOMEHUB_DATABASE_PASSWORD_FILE", "/run/secrets/control_db_password"),
		AuthKeyFile:            envOrDefault("HOMEHUB_AUTH_KEY_FILE", "/run/secrets/auth_encryption_key"),
		BootstrapTokenFile:     envOrDefault("HOMEHUB_BOOTSTRAP_TOKEN_FILE", "/run/secrets/owner_setup_token"),
		IdentitySigningKeyFile: envOrDefault("HOMEHUB_IDENTITY_SIGNING_KEY_FILE", "/run/secrets/identity_signing_key"),
		AllowedOrigins:         splitCSV(envOrDefault("HOMEHUB_ALLOWED_ORIGINS", "http://127.0.0.1:18080")),
		SecureCookies:          strings.EqualFold(envOrDefault("HOMEHUB_SECURE_COOKIES", "false"), "true"),
		HealthInterval:         10 * time.Second,
		HealthTimeout:          2 * time.Second,
		ShutdownTimeout:        10 * time.Second,
	}

	var err error
	if cfg.HealthInterval, err = durationFromEnv("HOMEHUB_HEALTH_INTERVAL", cfg.HealthInterval); err != nil {
		return Config{}, err
	}
	if cfg.HealthTimeout, err = durationFromEnv("HOMEHUB_HEALTH_TIMEOUT", cfg.HealthTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationFromEnv("HOMEHUB_SHUTDOWN_TIMEOUT", cfg.ShutdownTimeout); err != nil {
		return Config{}, err
	}

	if cfg.HealthInterval <= 0 || cfg.HealthTimeout <= 0 || cfg.ShutdownTimeout <= 0 {
		return Config{}, fmt.Errorf("duration settings must be positive")
	}
	if strings.TrimSpace(cfg.ListenAddress) == "" {
		return Config{}, fmt.Errorf("HOMEHUB_LISTEN_ADDRESS must not be empty")
	}
	if strings.TrimSpace(cfg.CatalogFile) == "" {
		return Config{}, fmt.Errorf("HOMEHUB_CATALOG_FILE must not be empty")
	}
	if len(cfg.AllowedOrigins) == 0 {
		return Config{}, fmt.Errorf("HOMEHUB_ALLOWED_ORIGINS must not be empty")
	}
	return cfg, nil
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return duration, nil
}
