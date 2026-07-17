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
