package config

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type Config struct {
	Providers []Provider `json:"providers"`
	Models    []Model    `json:"models"`
}

type Provider struct {
	ID         string `json:"id"`
	BaseURL    string `json:"base_url"`
	APIKeyFile string `json:"api_key_file"`
}

type Model struct {
	ID            string `json:"id"`
	Description   string `json:"description"`
	Provider      string `json:"provider"`
	UpstreamModel string `json:"upstream_model"`
}

func Load(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("open AI Gateway config: %w", err)
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode AI Gateway config: %w", err)
	}
	if err := ensureEOF(decoder); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (cfg Config) Validate() error {
	if len(cfg.Providers) == 0 || len(cfg.Models) == 0 {
		return fmt.Errorf("AI Gateway requires at least one provider and model")
	}
	providers := make(map[string]struct{}, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		if !idPattern.MatchString(provider.ID) {
			return fmt.Errorf("invalid provider id %q", provider.ID)
		}
		if _, exists := providers[provider.ID]; exists {
			return fmt.Errorf("duplicate provider id %q", provider.ID)
		}
		parsed, err := url.Parse(strings.TrimRight(provider.BaseURL, "/"))
		if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && !isLoopbackHTTP(parsed)) {
			return fmt.Errorf("provider %q base_url must use HTTPS or loopback HTTP", provider.ID)
		}
		if strings.TrimSpace(provider.APIKeyFile) == "" {
			return fmt.Errorf("provider %q api_key_file must not be empty", provider.ID)
		}
		providers[provider.ID] = struct{}{}
	}
	models := make(map[string]struct{}, len(cfg.Models))
	for _, model := range cfg.Models {
		if !idPattern.MatchString(model.ID) {
			return fmt.Errorf("invalid model alias %q", model.ID)
		}
		if _, exists := models[model.ID]; exists {
			return fmt.Errorf("duplicate model alias %q", model.ID)
		}
		if _, exists := providers[model.Provider]; !exists {
			return fmt.Errorf("model %q references unknown provider %q", model.ID, model.Provider)
		}
		if strings.TrimSpace(model.UpstreamModel) == "" {
			return fmt.Errorf("model %q upstream_model must not be empty", model.ID)
		}
		models[model.ID] = struct{}{}
	}
	return nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("AI Gateway config contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing AI Gateway config: %w", err)
	}
	return nil
}

func isLoopbackHTTP(value *url.URL) bool {
	if value.Scheme != "http" {
		return false
	}
	host := value.Hostname()
	return strings.EqualFold(host, "localhost") || net.ParseIP(host).IsLoopback()
}
