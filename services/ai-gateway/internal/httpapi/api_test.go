package httpapi

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"homehub.local/go-sdk/identity"
)

func TestHealthDoesNotRequireIdentity(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	response := httptest.NewRecorder()
	New(verifier).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestChatRequiresIdentityBeforeProviderSelection(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"default","messages":[{"role":"user","content":"hi"}]}`))
	response := httptest.NewRecorder()
	New(verifier).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
