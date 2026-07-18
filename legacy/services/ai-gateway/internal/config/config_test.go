package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidatesProviderAndModels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	contents := `{"providers":[{"id":"deepseek","base_url":"https://api.deepseek.com","api_key_file":"/run/secrets/deepseek"}],"models":[{"id":"fast","description":"Fast","provider":"deepseek","upstream_model":"deepseek-v4-flash"}]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Models[0].ID != "fast" {
		t.Fatalf("config=%#v error=%v", cfg, err)
	}
}

func TestLoadRejectsUnsafeOrUnknownConfig(t *testing.T) {
	tests := map[string]string{
		"plaintext external provider": `{"providers":[{"id":"bad","base_url":"http://example.com","api_key_file":"/key"}],"models":[{"id":"fast","provider":"bad","upstream_model":"x"}]}`,
		"unknown provider":            `{"providers":[{"id":"good","base_url":"https://example.com","api_key_file":"/key"}],"models":[{"id":"fast","provider":"missing","upstream_model":"x"}]}`,
		"unknown field":               `{"providers":[{"id":"good","base_url":"https://example.com","api_key_file":"/key","token":"secret"}],"models":[{"id":"fast","provider":"good","upstream_model":"x"}]}`,
	}
	for name, contents := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "providers.json")
			if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Load(path); err == nil || strings.Contains(err.Error(), "secret") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
