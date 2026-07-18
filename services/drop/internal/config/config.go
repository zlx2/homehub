package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddress        string
	DatabaseHost         string
	DatabasePort         string
	DatabaseName         string
	DatabaseUser         string
	DatabasePasswordFile string
	DataDirectory        string
	IAMJWKSURL           string
	QuotaBytes           int64
	MaxItemBytes         int64
	MaxAttachmentBytes   int64
	MaxAttachments       int
	ShutdownTimeout      time.Duration
}

func Load() (Config, error) {
	config := Config{
		ListenAddress:        value("HOMEHUB_DROP_LISTEN_ADDRESS", ":8080"),
		DatabaseHost:         value("HOMEHUB_DROP_DATABASE_HOST", "postgres"),
		DatabasePort:         value("HOMEHUB_DROP_DATABASE_PORT", "5432"),
		DatabaseName:         value("HOMEHUB_DROP_DATABASE_NAME", "homehub_drop"),
		DatabaseUser:         value("HOMEHUB_DROP_DATABASE_USER", "homehub_drop"),
		DatabasePasswordFile: value("HOMEHUB_DROP_DATABASE_PASSWORD_FILE", "/run/secrets/drop_db_password"),
		DataDirectory:        value("HOMEHUB_DROP_DATA_DIRECTORY", "/data"),
		IAMJWKSURL:           value("HOMEHUB_DROP_IAM_JWKS_URL", "http://iam:8080/.well-known/jwks.json"),
		QuotaBytes:           8 << 30, MaxItemBytes: 1 << 30, MaxAttachmentBytes: 500 << 20,
		MaxAttachments: 10, ShutdownTimeout: 15 * time.Second,
	}
	if config.ListenAddress == "" || config.DatabaseHost == "" || config.DatabasePort == "" || config.DatabaseName == "" ||
		config.DatabaseUser == "" || config.DatabasePasswordFile == "" || config.DataDirectory == "" || config.IAMJWKSURL == "" {
		return Config{}, errors.New("invalid Drop configuration")
	}
	contents, err := os.ReadFile(config.DatabasePasswordFile)
	if err != nil {
		return Config{}, fmt.Errorf("read Drop database password: %w", err)
	}
	if strings.TrimSpace(string(contents)) == "" {
		return Config{}, errors.New("Drop database password is empty")
	}
	return config, nil
}

func (config Config) DatabasePassword() (string, error) {
	contents, err := os.ReadFile(config.DatabasePasswordFile)
	if err != nil {
		return "", err
	}
	password := strings.TrimSpace(string(contents))
	if password == "" {
		return "", errors.New("Drop database password is empty")
	}
	return password, nil
}

func value(name, fallback string) string {
	if result := strings.TrimSpace(os.Getenv(name)); result != "" {
		return result
	}
	return fallback
}
