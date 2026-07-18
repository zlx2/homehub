package httpapi

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homehub.local/go-sdk/identity"
)

type stubVerifier struct {
	claims identity.Claims
	err    error
}

func (stub stubVerifier) Verify(string) (identity.Claims, error) { return stub.claims, stub.err }

func TestProtectedRoutesEnforceTokenAndPermission(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	baseClaims := identity.Claims{
		Subject: "agent:hermes", AuthorizedParty: "hermes", Realm: "homehub",
		Permissions: []string{dashboardRead}, Expires: time.Now().Add(time.Minute).Unix(),
	}

	tests := []struct {
		name       string
		verifier   TokenVerifier
		authorize  bool
		wantStatus int
	}{
		{name: "missing bearer", verifier: stubVerifier{claims: baseClaims}, wantStatus: http.StatusUnauthorized},
		{name: "invalid token", verifier: stubVerifier{err: errors.New("bad token")}, authorize: true, wantStatus: http.StatusUnauthorized},
		{name: "missing permission", verifier: stubVerifier{claims: identity.Claims{Permissions: []string{nodeRead}}}, authorize: true, wantStatus: http.StatusForbidden},
		{name: "allowed", verifier: stubVerifier{claims: baseClaims}, authorize: true, wantStatus: http.StatusOK},
		{name: "root allowed", verifier: stubVerifier{claims: identity.Claims{Permissions: []string{identity.SystemRootPermission}}}, authorize: true, wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			handler := New(Options{Logger: logger, Verifier: test.verifier, HealthClient: &http.Client{Timeout: time.Second}})
			request := httptest.NewRequest(http.MethodGet, "/v1/overview", nil)
			if test.authorize {
				request.Header.Set("Authorization", "Bearer test-token")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
			if len(response.Header().Get("X-Request-ID")) != 24 {
				t.Fatal("missing generated request ID")
			}
		})
	}
}

func TestHealthRoutesAreAnonymous(t *testing.T) {
	t.Parallel()
	handler := New(Options{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Verifier: stubVerifier{err: errors.New("must not be called")},
		HealthClient: &http.Client{Timeout: time.Second},
	})
	for _, path := range []string{"/health/live", "/health/ready"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
}
