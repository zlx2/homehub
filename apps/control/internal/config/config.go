package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddress   string
	CatalogFile     string
	Environment     string
	LogLevel        string
	HealthInterval  time.Duration
	HealthTimeout   time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		ListenAddress:   envOrDefault("HOMEHUB_LISTEN_ADDRESS", ":8080"),
		CatalogFile:     envOrDefault("HOMEHUB_CATALOG_FILE", "/etc/homehub/services.json"),
		Environment:     envOrDefault("HOMEHUB_ENVIRONMENT", "development"),
		LogLevel:        envOrDefault("HOMEHUB_LOG_LEVEL", "info"),
		HealthInterval:  10 * time.Second,
		HealthTimeout:   2 * time.Second,
		ShutdownTimeout: 10 * time.Second,
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
	return cfg, nil
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
