package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxTextBytes       = int64(50 << 20)
	defaultMaxAttachmentBytes = int64(500 << 20)
	defaultMaxItemBytes       = int64(1 << 30)
	defaultQuotaBytes         = int64(8 << 30)
)

// Config contains all process settings. Secrets are deliberately loaded only
// from the environment and are never given source-code defaults.
type Config struct {
	ListenAddr            string
	BasePath              string
	IdentityPublicKeyFile string
	AllowedOrigins        map[string]struct{}

	PublicAddr    string
	PublicURL     string
	TailscaleAddr string
	HermesAddr    string
	DataDir       string

	TailscaleUsers   map[string]struct{}
	HermesToken      string
	CookieName       string
	CookieSecure     bool
	AllowNonLoopback bool

	DefaultTTL      time.Duration
	CodeTTL         time.Duration
	SessionTTL      time.Duration
	CleanupInterval time.Duration
	TmpMaxAge       time.Duration

	MaxTextBytes       int64
	MaxAttachmentBytes int64
	MaxItemBytes       int64
	MaxAttachments     int
	QuotaBytes         int64
	InlineTextBytes    int64

	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration

	TrustedPublicProxies []*net.IPNet
}

func Load() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("resolve home directory: %w", err)
	}

	cfg := Config{
		ListenAddr:            env("DROP_LISTEN_ADDRESS", "127.0.0.1:8080"),
		BasePath:              env("DROP_BASE_PATH", "/drop"),
		IdentityPublicKeyFile: env("DROP_IDENTITY_PUBLIC_KEY_FILE", "/run/secrets/identity_public_key"),
		AllowedOrigins:        parseSet(os.Getenv("DROP_ALLOWED_ORIGINS")),
		PublicAddr:            env("DROP_PUBLIC_ADDR", "127.0.0.1:8080"),
		PublicURL:             strings.TrimRight(strings.TrimSpace(os.Getenv("DROP_PUBLIC_URL")), "/"),
		TailscaleAddr:         env("DROP_TAILSCALE_ADDR", "127.0.0.1:8081"),
		HermesAddr:            env("DROP_HERMES_ADDR", "127.0.0.1:8082"),
		DataDir:               env("DROP_DATA_DIR", filepath.Join(home, "drop", "data")),
		TailscaleUsers:        parseSet(os.Getenv("DROP_TAILSCALE_USERS")),
		HermesToken:           strings.TrimSpace(os.Getenv("DROP_HERMES_TOKEN")),
		CookieName:            env("DROP_SESSION_COOKIE", "drop_session"),
		CookieSecure:          true,
		DefaultTTL:            24 * time.Hour,
		CodeTTL:               30 * time.Minute,
		SessionTTL:            180 * 24 * time.Hour,
		CleanupInterval:       time.Minute,
		TmpMaxAge:             2 * time.Hour,
		MaxTextBytes:          defaultMaxTextBytes,
		MaxAttachmentBytes:    defaultMaxAttachmentBytes,
		MaxItemBytes:          defaultMaxItemBytes,
		MaxAttachments:        10,
		QuotaBytes:            defaultQuotaBytes,
		InlineTextBytes:       256 << 10,
		ReadHeaderTimeout:     10 * time.Second,
		ReadTimeout:           15 * time.Minute,
		WriteTimeout:          0, // streaming uploads and SSE use endpoint-level controls
		IdleTimeout:           90 * time.Second,
		ShutdownTimeout:       15 * time.Second,
		TrustedPublicProxies:  nil,
	}

	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) error {
	var err error
	if cfg.CookieSecure, err = boolEnv("DROP_COOKIE_SECURE", cfg.CookieSecure); err != nil {
		return err
	}
	if cfg.AllowNonLoopback, err = boolEnv("DROP_ALLOW_NON_LOOPBACK", cfg.AllowNonLoopback); err != nil {
		return err
	}

	durations := []struct {
		name string
		dst  *time.Duration
	}{
		{"DROP_DEFAULT_TTL", &cfg.DefaultTTL},
		{"DROP_CODE_TTL", &cfg.CodeTTL},
		{"DROP_SESSION_TTL", &cfg.SessionTTL},
		{"DROP_CLEANUP_INTERVAL", &cfg.CleanupInterval},
		{"DROP_TMP_MAX_AGE", &cfg.TmpMaxAge},
		{"DROP_READ_HEADER_TIMEOUT", &cfg.ReadHeaderTimeout},
		{"DROP_READ_TIMEOUT", &cfg.ReadTimeout},
		{"DROP_WRITE_TIMEOUT", &cfg.WriteTimeout},
		{"DROP_IDLE_TIMEOUT", &cfg.IdleTimeout},
		{"DROP_SHUTDOWN_TIMEOUT", &cfg.ShutdownTimeout},
	}
	for _, item := range durations {
		if *item.dst, err = durationEnv(item.name, *item.dst); err != nil {
			return err
		}
	}

	ints64 := []struct {
		name string
		dst  *int64
	}{
		{"DROP_MAX_TEXT_BYTES", &cfg.MaxTextBytes},
		{"DROP_MAX_ATTACHMENT_BYTES", &cfg.MaxAttachmentBytes},
		{"DROP_MAX_ITEM_BYTES", &cfg.MaxItemBytes},
		{"DROP_QUOTA_BYTES", &cfg.QuotaBytes},
		{"DROP_INLINE_TEXT_BYTES", &cfg.InlineTextBytes},
	}
	for _, item := range ints64 {
		if *item.dst, err = int64Env(item.name, *item.dst); err != nil {
			return err
		}
	}

	if cfg.MaxAttachments, err = intEnv("DROP_MAX_ATTACHMENTS", cfg.MaxAttachments); err != nil {
		return err
	}
	cfg.TrustedPublicProxies, err = cidrsEnv("DROP_TRUSTED_PUBLIC_PROXIES")
	return err
}

func (c Config) Validate() error {
	if _, _, err := net.SplitHostPort(c.ListenAddr); err != nil {
		return fmt.Errorf("DROP_LISTEN_ADDRESS: %w", err)
	}
	if !strings.HasPrefix(c.BasePath, "/") || c.BasePath == "/" || strings.HasSuffix(c.BasePath, "/") {
		return errors.New("DROP_BASE_PATH must be an absolute path without a trailing slash")
	}
	if strings.TrimSpace(c.IdentityPublicKeyFile) == "" {
		return errors.New("DROP_IDENTITY_PUBLIC_KEY_FILE must not be empty")
	}
	for name, addr := range map[string]string{
		"DROP_PUBLIC_ADDR":    c.PublicAddr,
		"DROP_TAILSCALE_ADDR": c.TailscaleAddr,
		"DROP_HERMES_ADDR":    c.HermesAddr,
	} {
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if !c.AllowNonLoopback && host != "127.0.0.1" && host != "::1" && host != "localhost" {
			return fmt.Errorf("%s must listen on localhost, got %q", name, addr)
		}
	}
	if strings.TrimSpace(c.DataDir) == "" {
		return errors.New("DROP_DATA_DIR must not be empty")
	}
	if c.PublicURL != "" {
		publicURL, err := url.Parse(c.PublicURL)
		if err != nil || publicURL.Scheme != "https" || publicURL.Host == "" || publicURL.User != nil || publicURL.RawQuery != "" || publicURL.Fragment != "" {
			return errors.New("DROP_PUBLIC_URL must be an absolute HTTPS URL without credentials, query, or fragment")
		}
	}
	if c.CookieName == "" {
		return errors.New("DROP_SESSION_COOKIE must not be empty")
	}
	if c.DefaultTTL <= 0 || c.CodeTTL <= 0 || c.SessionTTL <= 0 || c.CleanupInterval <= 0 || c.TmpMaxAge <= 0 {
		return errors.New("TTL and cleanup durations must be positive")
	}
	if c.MaxTextBytes <= 0 || c.MaxAttachmentBytes <= 0 || c.MaxItemBytes <= 0 || c.QuotaBytes <= 0 || c.InlineTextBytes <= 0 {
		return errors.New("size limits must be positive")
	}
	if c.MaxAttachments <= 0 || c.MaxAttachments > 100 {
		return errors.New("DROP_MAX_ATTACHMENTS must be between 1 and 100")
	}
	if c.MaxTextBytes > c.MaxItemBytes || c.MaxAttachmentBytes > c.MaxItemBytes {
		return errors.New("per-part size limits must not exceed DROP_MAX_ITEM_BYTES")
	}
	if c.InlineTextBytes > c.MaxTextBytes {
		return errors.New("DROP_INLINE_TEXT_BYTES must not exceed DROP_MAX_TEXT_BYTES")
	}
	if c.ReadHeaderTimeout <= 0 || c.ReadTimeout <= 0 || c.IdleTimeout <= 0 || c.ShutdownTimeout <= 0 || c.WriteTimeout < 0 {
		return errors.New("HTTP timeouts must be positive (write timeout may be zero)")
	}
	if c.HermesToken != "" && len(c.HermesToken) < 32 {
		return errors.New("DROP_HERMES_TOKEN must contain at least 32 characters when enabled")
	}
	return nil
}

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func parseSet(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, part := range strings.Split(value, ",") {
		if item := strings.TrimSpace(strings.ToLower(part)); item != "" {
			result[item] = struct{}{}
		}
	}
	return result
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func int64Env(name string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func intEnv(name string, fallback int) (int, error) {
	value, err := int64Env(name, int64(fallback))
	return int(value), err
}

func boolEnv(name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func cidrsEnv(name string) ([]*net.IPNet, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return nil, nil
	}
	var result []*net.IPNet
	for _, part := range strings.Split(value, ",") {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		if !strings.Contains(item, "/") {
			if ip := net.ParseIP(item); ip != nil {
				if ip.To4() != nil {
					item += "/32"
				} else {
					item += "/128"
				}
			}
		}
		_, network, err := net.ParseCIDR(item)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid CIDR %q: %w", name, item, err)
		}
		result = append(result, network)
	}
	return result, nil
}
