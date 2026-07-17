package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homehub.local/control/internal/catalog"
)

func TestMonitorCheck(t *testing.T) {
	healthy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer healthy.Close()

	monitor := NewMonitor([]catalog.Service{
		{ID: "healthy-service", HealthURL: healthy.URL},
		{ID: "broken-service", HealthURL: "http://127.0.0.1:1/health"},
	}, time.Minute, 200*time.Millisecond)

	monitor.Check(context.Background())
	results := monitor.Snapshot()
	if results["healthy-service"].Status != "healthy" {
		t.Fatalf("healthy result = %#v", results["healthy-service"])
	}
	if results["broken-service"].Status != "unhealthy" {
		t.Fatalf("broken result = %#v", results["broken-service"])
	}
}
