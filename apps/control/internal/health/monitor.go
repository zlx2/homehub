package health

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"homehub.local/control/internal/catalog"
)

type Result struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	LatencyMS int64     `json:"latency_ms"`
	Message   string    `json:"message,omitempty"`
}

type Monitor struct {
	services []catalog.Service
	interval time.Duration
	client   *http.Client

	mu      sync.RWMutex
	results map[string]Result
}

func NewMonitor(services []catalog.Service, interval, timeout time.Duration) *Monitor {
	results := make(map[string]Result, len(services))
	for _, service := range services {
		results[service.ID] = Result{Status: "unknown"}
	}
	return &Monitor{
		services: append([]catalog.Service(nil), services...),
		interval: interval,
		client: &http.Client{
			Timeout:   timeout,
			Transport: &http.Transport{Proxy: nil},
		},
		results: results,
	}
}

func (monitor *Monitor) Run(ctx context.Context) {
	monitor.Check(ctx)
	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			monitor.Check(ctx)
		}
	}
}

func (monitor *Monitor) Check(ctx context.Context) {
	var waitGroup sync.WaitGroup
	for _, service := range monitor.services {
		service := service
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			monitor.set(service.ID, monitor.checkOne(ctx, service.HealthURL))
		}()
	}
	waitGroup.Wait()
}

func (monitor *Monitor) Snapshot() map[string]Result {
	monitor.mu.RLock()
	defer monitor.mu.RUnlock()
	copyOfResults := make(map[string]Result, len(monitor.results))
	for id, result := range monitor.results {
		copyOfResults[id] = result
	}
	return copyOfResults
}

func (monitor *Monitor) checkOne(ctx context.Context, healthURL string) Result {
	started := time.Now()
	result := Result{Status: "unhealthy"}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err == nil {
		var response *http.Response
		response, err = monitor.client.Do(request)
		if response != nil {
			response.Body.Close()
		}
		if err == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
			result.Status = "healthy"
		} else if err == nil {
			err = fmt.Errorf("health endpoint returned %s", response.Status)
		}
	}
	result.CheckedAt = time.Now().UTC()
	result.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Message = err.Error()
	}
	return result
}

func (monitor *Monitor) set(id string, result Result) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.results[id] = result
}
