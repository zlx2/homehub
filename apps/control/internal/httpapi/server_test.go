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
		{name: "root agent owner service", scopes: []string{auth.ScopeAgentRoot}, service: catalog.Service{Visibility: "owner"}, wantAllow: true},
		{name: "root agent internal service", scopes: []string{auth.ScopeAgentRoot}, service: catalog.Service{Visibility: "internal"}, wantAllow: true},
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

func TestAPITokenOnlyAllowsDropCreateItem(t *testing.T) {
	identity := auth.APITokenIdentity{
		Principal: auth.Principal{Scopes: []string{auth.ScopeDropUpload}},
		ServiceID: "drop",
	}
	service := catalog.Service{ID: "drop"}
	tests := []struct {
		method string
		uri    string
		allow  bool
	}{
		{method: http.MethodPost, uri: "/drop/api/v1/items", allow: true},
		{method: http.MethodGet, uri: "/drop/api/v1/items", allow: false},
		{method: http.MethodDelete, uri: "/drop/api/v1/items/abc", allow: false},
		{method: http.MethodPost, uri: "/drop/api/v1/items/abc", allow: false},
		{method: http.MethodPost, uri: "/drop/api/v1/items?ttl=1", allow: true},
	}
	for _, test := range tests {
		if got := apiTokenRequestAllowed(identity, service, test.method, test.uri); got != test.allow {
			t.Fatalf("%s %s allowed=%v, want %v", test.method, test.uri, got, test.allow)
		}
	}
	if apiTokenRequestAllowed(identity, catalog.Service{ID: "chat"}, http.MethodPost, "/drop/api/v1/items") {
		t.Fatal("token crossed service boundary")
	}
}

func TestRootAgentTokenAllowsEveryRegisteredServiceRoute(t *testing.T) {
	identity := auth.APITokenIdentity{
		Principal: auth.Principal{Scopes: []string{auth.ScopeAgentRoot}},
		ServiceID: auth.APITokenServiceAll,
	}
	for _, test := range []struct {
		service catalog.Service
		method  string
		uri     string
	}{
		{service: catalog.Service{ID: "drop"}, method: http.MethodDelete, uri: "/drop/api/v1/items/abc"},
		{service: catalog.Service{ID: "server-monitor"}, method: http.MethodGet, uri: "/server/api/systems"},
		{service: catalog.Service{ID: "future-service"}, method: http.MethodPatch, uri: "/future-service/api/v1/config"},
	} {
		if !apiTokenRequestAllowed(identity, test.service, test.method, test.uri) {
			t.Fatalf("root agent denied %s %s for %s", test.method, test.uri, test.service.ID)
		}
	}
}

func TestRootAgentBypassesHumanAdminChecks(t *testing.T) {
	api := &server{}
	principal := auth.Principal{ID: "hermes", Username: "hermes", Scopes: []string{auth.ScopeAgentRoot}}
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/invitations/example", nil)
	request = request.WithContext(context.WithValue(request.Context(), principalContextKey{}, principal))
	response := httptest.NewRecorder()
	api.requireAdmin(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !api.validMutation(request) {
			t.Fatal("root agent should not require Origin or CSRF")
		}
		writer.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAuthCheckIssuesIdentityForOptedInService(t *testing.T) {
	issuer := &recordingIdentityIssuer{token: "signed-identity"}
	api := &server{identityIssuer: issuer}
	response := httptest.NewRecorder()
	err := api.setServiceIdentity(response, auth.Principal{
		ID: "owner-1", Username: "owner", DisplayName: "Luna", Scopes: []string{"admin", "portal.view"},
	}, catalog.Service{ID: "notes", IdentityEnabled: true})
	if err != nil || response.Header().Get("X-HomeHub-Identity") != "signed-identity" {
		t.Fatalf("identity=%q err=%v", response.Header().Get("X-HomeHub-Identity"), err)
	}
	if issuer.audience != "notes" {
		t.Fatalf("audience=%q", issuer.audience)
	}
}

func TestAuthCheckIssuesSeparateAIDelegation(t *testing.T) {
	issuer := &recordingIdentityIssuer{token: "service-token", aiToken: "ai-token"}
	api := &server{identityIssuer: issuer}
	response := httptest.NewRecorder()
	principal := auth.Principal{ID: "guest-1", DisplayName: "Guest", Scopes: []string{"portal.view"}}
	service := catalog.Service{
		ID: "assistant", IdentityEnabled: true, AIEnabled: true, AIModels: []string{"fast", "coding"},
	}
	if err := api.setServiceIdentity(response, principal, service); err != nil {
		t.Fatal(err)
	}
	if err := api.setAIIdentity(response, principal, service); err != nil {
		t.Fatal(err)
	}
	if response.Header().Get("X-HomeHub-Identity") != "service-token" || response.Header().Get("X-HomeHub-AI-Identity") != "ai-token" {
		t.Fatalf("headers=%v", response.Header())
	}
	if issuer.sourceService != "assistant" || strings.Join(issuer.models, ",") != "fast,coding" {
		t.Fatalf("source=%q models=%v", issuer.sourceService, issuer.models)
	}
}

type recordingIdentityIssuer struct {
	token         string
	aiToken       string
	audience      string
	sourceService string
	models        []string
}

func (issuer *recordingIdentityIssuer) IssueAI(_, _, sourceService string, _ []string, models []string) (string, error) {
	issuer.sourceService = sourceService
	issuer.models = append([]string(nil), models...)
	return issuer.aiToken, nil
}

func (issuer *recordingIdentityIssuer) Issue(_, _ string, _ []string, audience string) (string, error) {
	issuer.audience = audience
	return issuer.token, nil
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

func TestNormalizeInvitationExpiry(t *testing.T) {
	now := time.Date(2026, 7, 17, 6, 0, 0, 0, time.UTC)
	defaultExpiry, err := normalizeInvitationExpiry(now, nil)
	if err != nil || !defaultExpiry.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("default expiry = %v, error = %v", defaultExpiry, err)
	}
	tooSoon := now.Add(4 * time.Minute)
	if _, err := normalizeInvitationExpiry(now, &tooSoon); err == nil {
		t.Fatal("expected short invitation expiry to fail")
	}
	tooLong := now.Add(8 * 24 * time.Hour)
	if _, err := normalizeInvitationExpiry(now, &tooLong); err == nil {
		t.Fatal("expected long invitation expiry to fail")
	}
}

func TestInvitationOnlyIncludesShareableServices(t *testing.T) {
	api := &server{services: []catalog.Service{
		{ID: "chat", Visibility: "shared", ShareEnabled: true},
		{ID: "server", Visibility: "owner", ShareEnabled: false},
	}}
	services, err := api.validateInvitationServices([]string{"chat"})
	if err != nil || len(services) != 1 || services[0] != "chat" {
		t.Fatalf("services = %#v, error = %v", services, err)
	}
	if _, err := api.validateInvitationServices([]string{"server"}); err == nil {
		t.Fatal("expected owner-only service selection to fail")
	}
	if _, err := api.validateInvitationServices([]string{"chat", "chat"}); err == nil {
		t.Fatal("expected duplicate service selection to fail")
	}
	if _, err := api.validateInvitationServices(nil); err == nil {
		t.Fatal("expected empty service selection to fail")
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
