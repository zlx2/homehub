package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"homehub.local/control/internal/auth"
	"homehub.local/control/internal/catalog"
	"homehub.local/control/internal/health"
)

type staticStatuses map[string]health.Result

func (statuses staticStatuses) Snapshot() map[string]health.Result {
	result := make(map[string]health.Result, len(statuses))
	for id, status := range statuses {
		result[id] = status
	}
	return result
}

func TestServiceAccessPolicy(t *testing.T) {
	tests := []struct {
		name      string
		scopes    []string
		service   catalog.Service
		hasGrant  bool
		wantAllow bool
	}{
		{name: "admin owner service", scopes: []string{"portal.view", "admin"}, service: catalog.Service{Visibility: "owner"}, wantAllow: true},
		{name: "friend owner service", scopes: []string{"portal.view"}, service: catalog.Service{Visibility: "owner"}, hasGrant: true, wantAllow: false},
		{name: "friend shared with grant", scopes: []string{"portal.view"}, service: catalog.Service{Visibility: "shared", ShareEnabled: true}, hasGrant: true, wantAllow: true},
		{name: "friend shared without grant", scopes: []string{"portal.view"}, service: catalog.Service{Visibility: "shared", ShareEnabled: true}, wantAllow: false},
		{name: "sharing disabled", scopes: []string{"portal.view"}, service: catalog.Service{Visibility: "shared", ShareEnabled: false}, hasGrant: true, wantAllow: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			principal := auth.Principal{Username: "owner", Scopes: test.scopes}
			if got := serviceAccessAllowed(principal, test.service, test.hasGrant); got != test.wantAllow {
				t.Fatalf("serviceAccessAllowed() = %v, want %v", got, test.wantAllow)
			}
		})
	}
}

func TestRequireAdminDeniesNonAdminPrincipal(t *testing.T) {
	api := &server{}
	handler := api.requireAdmin(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/principals", nil)
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, auth.Principal{
		ID: "friend", Scopes: []string{"portal.view"},
	}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestServicesDoNotExposeInternalHealthURL(t *testing.T) {
	handler := New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Services: []catalog.Service{{
			ID: "demo-service", Name: "Demo", Route: "/demo", Visibility: "owner",
			HealthURL: "http://secret-internal-name:8080/health/live",
		}},
		Statuses: staticStatuses{
			"demo-service": {
				Status:    "unhealthy",
				CheckedAt: time.Now(),
				Message:   `Get "http://secret-internal-name:8080/health/live": connection refused`,
			},
		},
		Version:             "test",
		DisableAuthForTests: true,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/services", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret-internal-name") || strings.Contains(response.Body.String(), "health_url") {
		t.Fatalf("internal health URL leaked: %s", response.Body.String())
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID missing")
	}
}

func TestUnknownService(t *testing.T) {
	handler := New(Options{
		Logger:              slog.New(slog.NewTextHandler(io.Discard, nil)),
		Statuses:            staticStatuses{},
		DisableAuthForTests: true,
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/services/missing", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d", response.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "service_not_found" {
		t.Fatalf("body = %#v", body)
	}
}
