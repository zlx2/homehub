package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddress   string
	Environment     string
	LogLevel        string
	CatalogFile     string
	IAMJWKSURL      string
	HealthTimeout   time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	config := Config{
		ListenAddress:   value("HOMEHUB_CONTROL_LISTEN_ADDRESS", ":8080"),
		Environment:     value("HOMEHUB_CONTROL_ENVIRONMENT", "development"),
		LogLevel:        value("HOMEHUB_CONTROL_LOG_LEVEL", "info"),
		CatalogFile:     value("HOMEHUB_CONTROL_CATALOG_FILE", "/etc/homehub/catalog.json"),
		IAMJWKSURL:      value("HOMEHUB_CONTROL_IAM_JWKS_URL", "http://iam:8080/.well-known/jwks.json"),
		HealthTimeout:   1500 * time.Millisecond,
		ShutdownTimeout: 10 * time.Second,
	}
	var err error
	if config.HealthTimeout, err = duration("HOMEHUB_CONTROL_HEALTH_TIMEOUT", config.HealthTimeout); err != nil {
		return Config{}, err
	}
	if config.ShutdownTimeout, err = duration("HOMEHUB_CONTROL_SHUTDOWN_TIMEOUT", config.ShutdownTimeout); err != nil {
		return Config{}, err
	}
	if config.ListenAddress == "" || config.CatalogFile == "" || config.IAMJWKSURL == "" || config.HealthTimeout <= 0 || config.ShutdownTimeout <= 0 {
		return Config{}, errors.New("invalid HomeHub Control configuration")
	}
	return config, nil
}

func value(name, fallback string) string {
	if result := strings.TrimSpace(os.Getenv(name)); result != "" {
		return result
	}
	return fallback
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	result, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	return result, nil
}
