package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddress       string
	TelegramAPIBaseURL  string
	TelegramToken       string
	DropBaseURL         string
	DropToken           string
	AllowedUserIDs      map[int64]struct{}
	AllowedChatIDs      map[int64]struct{}
	TTL                 int
	AckMode             string
	PollTimeout         time.Duration
	RequestTimeout      time.Duration
	MaxTelegramFileSize int64
}

func Load() (Config, error) {
	telegramToken, err := readSecret(env("TELEGRAM_BRIDGE_BOT_TOKEN_FILE", "/run/secrets/telegram_bot_token"))
	if err != nil {
		return Config{}, fmt.Errorf("load Telegram bot token: %w", err)
	}
	dropToken, err := readSecret(env("TELEGRAM_BRIDGE_DROP_TOKEN_FILE", "/run/secrets/telegram_drop_token"))
	if err != nil {
		return Config{}, fmt.Errorf("load Drop token: %w", err)
	}
	allowedUsers, err := parseIDs(os.Getenv("TELEGRAM_BRIDGE_ALLOWED_USER_IDS"))
	if err != nil {
		return Config{}, fmt.Errorf("parse allowed user IDs: %w", err)
	}
	allowedChats, err := parseIDs(os.Getenv("TELEGRAM_BRIDGE_ALLOWED_CHAT_IDS"))
	if err != nil {
		return Config{}, fmt.Errorf("parse allowed chat IDs: %w", err)
	}
	ttl, err := strconv.Atoi(env("TELEGRAM_BRIDGE_TTL_DAYS", "1"))
	if err != nil || (ttl != 1 && ttl != 3 && ttl != 7) {
		return Config{}, fmt.Errorf("TELEGRAM_BRIDGE_TTL_DAYS must be 1, 3, or 7")
	}
	ackMode := env("TELEGRAM_BRIDGE_ACK_MODE", "private")
	if ackMode != "none" && ackMode != "private" && ackMode != "all" {
		return Config{}, fmt.Errorf("TELEGRAM_BRIDGE_ACK_MODE must be none, private, or all")
	}
	return Config{
		ListenAddress:       env("TELEGRAM_BRIDGE_LISTEN_ADDRESS", "127.0.0.1:8730"),
		TelegramAPIBaseURL:  strings.TrimRight(env("TELEGRAM_BRIDGE_API_BASE_URL", "https://api.telegram.org"), "/"),
		TelegramToken:       telegramToken,
		DropBaseURL:         strings.TrimRight(env("TELEGRAM_BRIDGE_DROP_BASE_URL", "https://111.229.205.99/drop"), "/"),
		DropToken:           dropToken,
		AllowedUserIDs:      allowedUsers,
		AllowedChatIDs:      allowedChats,
		TTL:                 ttl,
		AckMode:             ackMode,
		PollTimeout:         50 * time.Second,
		RequestTimeout:      5 * time.Minute,
		MaxTelegramFileSize: 20 << 20,
	}, nil
}

func (c Config) Allowed(userID, chatID int64) bool {
	if _, ok := c.AllowedChatIDs[chatID]; ok {
		return true
	}
	_, ok := c.AllowedUserIDs[userID]
	return ok
}

func parseIDs(value string) (map[int64]struct{}, error) {
	result := make(map[int64]struct{})
	for _, raw := range strings.Split(value, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("invalid Telegram ID %q", raw)
		}
		result[id] = struct{}{}
	}
	return result, nil
}

func readSecret(path string) (string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(value))
	if secret == "" {
		return "", fmt.Errorf("secret file is empty")
	}
	return secret, nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
