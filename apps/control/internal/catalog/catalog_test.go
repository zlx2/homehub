package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	path := writeCatalog(t, `{"services":[{"id":"demo-service","name":"Demo","description":"test","icon":"box","route":"/demo","visibility":"owner","share_enabled":true,"health_url":"http://demo:8080/health/live"}]}`)
	services, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(services) != 1 || services[0].ID != "demo-service" {
		t.Fatalf("unexpected services: %#v", services)
	}
}

func TestLoadRejectsDuplicateIDs(t *testing.T) {
	path := writeCatalog(t, `{"services":[{"id":"demo-service","name":"One","visibility":"owner","health_url":"http://one/health"},{"id":"demo-service","name":"Two","visibility":"owner","health_url":"http://two/health"}]}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("Load() error = %v, want duplicate error", err)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeCatalog(t, `{"services":[{"id":"demo-service","name":"Demo","visibility":"owner","health_url":"http://demo/health","secret":"nope"}]}`)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Load() error = %v, want unknown field error", err)
	}
}

func TestLoadValidatesAIPolicy(t *testing.T) {
	valid := writeCatalog(t, `{"services":[{"id":"assistant","name":"Assistant","visibility":"owner","identity_enabled":true,"ai_enabled":true,"ai_models":["fast","coding"],"health_url":"http://assistant/health"}]}`)
	services, err := Load(valid)
	if err != nil || !services[0].AIEnabled || len(services[0].AIModels) != 2 {
		t.Fatalf("Load() services=%#v error=%v", services, err)
	}

	for name, contents := range map[string]string{
		"identity required": `{"services":[{"id":"assistant","name":"Assistant","visibility":"owner","ai_enabled":true,"ai_models":["fast"],"health_url":"http://assistant/health"}]}`,
		"models required":   `{"services":[{"id":"assistant","name":"Assistant","visibility":"owner","identity_enabled":true,"ai_enabled":true,"health_url":"http://assistant/health"}]}`,
		"duplicate model":   `{"services":[{"id":"assistant","name":"Assistant","visibility":"owner","identity_enabled":true,"ai_enabled":true,"ai_models":["fast","fast"],"health_url":"http://assistant/health"}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Load(writeCatalog(t, contents)); err == nil {
				t.Fatal("expected invalid AI policy to fail")
			}
		})
	}
}

func TestMatchRouteUsesSegmentBoundaryAndLongestRoute(t *testing.T) {
	services := []Service{
		{ID: "chat", Route: "/chat/"},
		{ID: "chat-admin", Route: "/chat/admin/"},
	}
	tests := []struct {
		uri    string
		wantID string
		wantOK bool
	}{
		{uri: "/chat", wantID: "chat", wantOK: true},
		{uri: "/chat/room?x=1", wantID: "chat", wantOK: true},
		{uri: "/chat/admin/users", wantID: "chat-admin", wantOK: true},
		{uri: "/chatter", wantOK: false},
	}
	for _, test := range tests {
		service, ok := MatchRoute(services, test.uri)
		if ok != test.wantOK || service.ID != test.wantID {
			t.Fatalf("MatchRoute(%q) = (%q, %v), want (%q, %v)", test.uri, service.ID, ok, test.wantID, test.wantOK)
		}
	}
}

func writeCatalog(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "services.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
