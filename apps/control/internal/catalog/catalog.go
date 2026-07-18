package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxCatalogBytes = 256 << 10

var serviceID = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

type Document struct {
	Version  int       `json:"version"`
	Services []Service `json:"services"`
}

type Service struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	HealthURL   string `json:"health_url"`
	Path        string `json:"path"`
	Visibility  string `json:"visibility"`
}

type Status struct {
	State      string    `json:"state"`
	CheckedAt  time.Time `json:"checked_at"`
	LatencyMS  int64     `json:"latency_ms"`
	StatusCode int       `json:"status_code,omitempty"`
}

type View struct {
	Service
	Status Status `json:"status"`
}

func Load(path string) ([]Service, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open service catalog: %w", err)
	}
	defer file.Close()
	contents, err := io.ReadAll(io.LimitReader(file, maxCatalogBytes+1))
	if err != nil || len(contents) > maxCatalogBytes {
		return nil, errors.New("read service catalog")
	}
	var document Document
	if json.Unmarshal(contents, &document) != nil || document.Version != 1 || len(document.Services) == 0 {
		return nil, errors.New("invalid service catalog")
	}
	seen := make(map[string]struct{}, len(document.Services))
	for index := range document.Services {
		service := &document.Services[index]
		parsed, parseErr := url.Parse(service.HealthURL)
		if !serviceID.MatchString(service.ID) || strings.TrimSpace(service.Name) == "" || strings.TrimSpace(service.Kind) == "" ||
			parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil ||
			!strings.HasPrefix(service.Path, "/") || (service.Visibility != "owner" && service.Visibility != "shared" && service.Visibility != "public") {
			return nil, fmt.Errorf("invalid catalog service at index %d", index)
		}
		if _, duplicate := seen[service.ID]; duplicate {
			return nil, fmt.Errorf("duplicate catalog service %q", service.ID)
		}
		seen[service.ID] = struct{}{}
	}
	sort.Slice(document.Services, func(i, j int) bool { return document.Services[i].ID < document.Services[j].ID })
	return document.Services, nil
}

func Probe(ctx context.Context, client *http.Client, services []Service) []View {
	views := make([]View, len(services))
	var wait sync.WaitGroup
	for index, service := range services {
		index, service := index, service
		wait.Add(1)
		go func() {
			defer wait.Done()
			views[index] = View{Service: service, Status: probeOne(ctx, client, service.HealthURL)}
		}()
	}
	wait.Wait()
	return views
}

func probeOne(ctx context.Context, client *http.Client, endpoint string) Status {
	started := time.Now()
	status := Status{State: "unavailable", CheckedAt: started.UTC()}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return status
	}
	response, err := client.Do(request)
	status.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		return status
	}
	defer response.Body.Close()
	status.StatusCode = response.StatusCode
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		status.State = "healthy"
	} else {
		status.State = "degraded"
	}
	return status
}
