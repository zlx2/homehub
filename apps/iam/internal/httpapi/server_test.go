package httpapi

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitee.com/zlx23/homehub/apps/iam/internal/machineadmin"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

type fakeVerifier struct {
	claims identity.Claims
	err    error
}

func (verifier fakeVerifier) Verify(string) (identity.Claims, error) {
	return verifier.claims, verifier.err
}

type fakeMachineAdministrator struct {
	response machineadmin.CreateResponse
	err      error
}

func (administrator fakeMachineAdministrator) Create(context.Context, identity.Claims, string, machineadmin.CreateRequest) (machineadmin.CreateResponse, error) {
	return administrator.response, administrator.err
}

func TestHealth(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	New(Options{Version: "test"}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("X-Request-ID") == "" {
		t.Fatal("X-Request-ID was not generated")
	}
}

func TestMetadata(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/v1/metadata", nil)
	response := httptest.NewRecorder()
	New(Options{Version: "test-version"}).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q", contentType)
	}
}

func TestJWKS(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil)
	response := httptest.NewRecorder()
	New(Options{Version: "test", JWKSet: map[string]any{"keys": []any{}}}).ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Header().Get("Cache-Control") == "" {
		t.Fatalf("status = %d, cache-control = %q", response.Code, response.Header().Get("Cache-Control"))
	}
}

func TestReadinessFailure(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	New(Options{
		Version: "test",
		Readiness: func(context.Context) error {
			return errors.New("database unavailable")
		},
	}).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

func TestMachineIdentityCreationRequiresIAMPermission(t *testing.T) {
	t.Parallel()
	body := []byte(`{"kind":"workload","display_name":"Telegram","external_subject":"telegram-bridge","grants":[{"service_id":"drop","relation":"caller"}]}`)
	tests := []struct {
		name       string
		verifier   TokenVerifier
		authorize  bool
		wantStatus int
	}{
		{name: "missing token", verifier: fakeVerifier{}, wantStatus: http.StatusUnauthorized},
		{name: "invalid token", verifier: fakeVerifier{err: errors.New("invalid")}, authorize: true, wantStatus: http.StatusUnauthorized},
		{name: "missing permissions", verifier: fakeVerifier{claims: identity.Claims{Permissions: []string{"iam.principal.read"}}}, authorize: true, wantStatus: http.StatusForbidden},
		{name: "missing grant permission", verifier: fakeVerifier{claims: identity.Claims{Permissions: []string{principalManagePermission}}}, authorize: true, wantStatus: http.StatusForbidden},
		{name: "allowed", verifier: fakeVerifier{claims: identity.Claims{Subject: "agent:root", Realm: "homehub", Permissions: []string{principalManagePermission, grantManagePermission}}}, authorize: true, wantStatus: http.StatusCreated},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			handler := New(Options{
				Version: "test", Verifier: test.verifier,
				Machines: fakeMachineAdministrator{response: machineadmin.CreateResponse{Subject: "workload:worker", Credential: "one-time-secret"}},
			})
			request := httptest.NewRequest(http.MethodPost, "/v1/machine-identities", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			if test.authorize {
				request.Header.Set("Authorization", "Bearer test-token")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if response.Code == http.StatusCreated && response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("machine credential response must not be cached")
			}
		})
	}
}
