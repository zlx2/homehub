package catalog

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
)

var serviceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)
var modelAliasPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type Service struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Description     string   `json:"description"`
	Icon            string   `json:"icon"`
	Route           string   `json:"route"`
	Visibility      string   `json:"visibility"`
	ShareEnabled    bool     `json:"share_enabled"`
	IdentityEnabled bool     `json:"identity_enabled"`
	AIEnabled       bool     `json:"ai_enabled"`
	AIModels        []string `json:"ai_models"`
	HealthURL       string   `json:"health_url"`
}

type fileFormat struct {
	Services []Service `json:"services"`
}

func Load(path string) ([]Service, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file fileFormat
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	if len(file.Services) == 0 {
		return nil, fmt.Errorf("catalog must define at least one service")
	}
	seen := make(map[string]struct{}, len(file.Services))
	for index := range file.Services {
		if err := validate(file.Services[index]); err != nil {
			return nil, fmt.Errorf("service %d: %w", index, err)
		}
		if _, exists := seen[file.Services[index].ID]; exists {
			return nil, fmt.Errorf("duplicate service id %q", file.Services[index].ID)
		}
		seen[file.Services[index].ID] = struct{}{}
	}
	return file.Services, nil
}

func validate(service Service) error {
	if !serviceIDPattern.MatchString(service.ID) {
		return fmt.Errorf("invalid id %q", service.ID)
	}
	if strings.TrimSpace(service.Name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	if service.Route != "" && !strings.HasPrefix(service.Route, "/") {
		return fmt.Errorf("route must be empty or start with /")
	}
	switch service.Visibility {
	case "owner", "shared", "internal":
	default:
		return fmt.Errorf("invalid visibility %q", service.Visibility)
	}
	if service.AIEnabled && !service.IdentityEnabled {
		return fmt.Errorf("ai_enabled requires identity_enabled")
	}
	if service.AIEnabled && len(service.AIModels) == 0 {
		return fmt.Errorf("ai_enabled requires at least one ai_models entry")
	}
	if !service.AIEnabled && len(service.AIModels) != 0 {
		return fmt.Errorf("ai_models requires ai_enabled")
	}
	seenModels := make(map[string]struct{}, len(service.AIModels))
	for _, model := range service.AIModels {
		if !modelAliasPattern.MatchString(model) {
			return fmt.Errorf("invalid AI model alias %q", model)
		}
		if _, exists := seenModels[model]; exists {
			return fmt.Errorf("duplicate AI model alias %q", model)
		}
		seenModels[model] = struct{}{}
	}
	healthURL, err := url.Parse(service.HealthURL)
	if err != nil || healthURL.Host == "" || (healthURL.Scheme != "http" && healthURL.Scheme != "https") {
		return fmt.Errorf("health_url must be an absolute HTTP URL")
	}
	return nil
}

func MatchRoute(services []Service, requestURI string) (Service, bool) {
	path := strings.TrimSpace(strings.SplitN(requestURI, "?", 2)[0])
	var matched Service
	matchedLength := -1
	for _, service := range services {
		base := strings.TrimSuffix(service.Route, "/")
		if base == "" {
			continue
		}
		if path != base && !strings.HasPrefix(path, base+"/") {
			continue
		}
		if len(base) > matchedLength {
			matched = service
			matchedLength = len(base)
		}
	}
	return matched, matchedLength >= 0
}
