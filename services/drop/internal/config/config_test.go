package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DROP_PUBLIC_ADDR", "")
	t.Setenv("DROP_TAILSCALE_ADDR", "")
	t.Setenv("DROP_HERMES_ADDR", "")
	t.Setenv("DROP_HERMES_TOKEN", "")
	t.Setenv("DROP_PUBLIC_URL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.PublicAddr != "127.0.0.1:8080" || cfg.TailscaleAddr != "127.0.0.1:8081" || cfg.HermesAddr != "127.0.0.1:8082" {
		t.Fatalf("unexpected listeners: %#v", cfg)
	}
	if cfg.DefaultTTL != 24*time.Hour || cfg.CodeTTL != 30*time.Minute || cfg.SessionTTL != 180*24*time.Hour {
		t.Fatalf("unexpected TTL defaults: %#v", cfg)
	}
	if cfg.QuotaBytes != 8<<30 || cfg.MaxAttachments != 10 {
		t.Fatalf("unexpected quota defaults: %#v", cfg)
	}
}

func TestLoadOverridesAndRejectsPublicBind(t *testing.T) {
	t.Setenv("DROP_PUBLIC_ADDR", "127.0.0.1:9000")
	t.Setenv("DROP_TAILSCALE_USERS", "Alice@Example.com, bob@example.com")
	t.Setenv("DROP_QUOTA_BYTES", "12345")
	t.Setenv("DROP_COOKIE_SECURE", "false")
	t.Setenv("DROP_PUBLIC_URL", "https://drop.example.test/")
	t.Setenv("DROP_TRUSTED_PUBLIC_PROXIES", "127.0.0.1,10.0.0.0/8")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if _, ok := cfg.TailscaleUsers["alice@example.com"]; !ok {
		t.Fatal("normalized Tailscale user missing")
	}
	if cfg.QuotaBytes != 12345 || cfg.CookieSecure || cfg.PublicURL != "https://drop.example.test" || len(cfg.TrustedPublicProxies) != 2 {
		t.Fatalf("overrides not applied: %#v", cfg)
	}

	t.Setenv("DROP_PUBLIC_URL", "http://drop.example.test")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted an insecure public URL")
	}
	t.Setenv("DROP_PUBLIC_URL", "https://drop.example.test")

	t.Setenv("DROP_PUBLIC_ADDR", "0.0.0.0:8080")
	if _, err := Load(); err == nil {
		t.Fatal("Load() accepted a public bind")
	}
	t.Setenv("DROP_ALLOW_NON_LOOPBACK", "true")
	if cfg, err := Load(); err != nil || !cfg.AllowNonLoopback {
		t.Fatalf("Load() rejected explicit container bind: %#v, %v", cfg, err)
	}
}
