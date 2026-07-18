package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
