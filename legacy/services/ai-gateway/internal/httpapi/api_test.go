package httpapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"homehub.local/go-sdk/identity"
	"homehub.local/services/ai-gateway/internal/config"
	"homehub.local/services/ai-gateway/internal/gateway"
)

func TestHealthDoesNotRequireIdentity(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	response := httptest.NewRecorder()
	New(verifier, nil, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestChatRequiresAIDelegation(t *testing.T) {
	publicKey, _, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"fast","messages":[{"role":"user","content":"hi"}]}`))
	response := httptest.NewRecorder()
	New(verifier, nil, nil).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestModelsAreFilteredBySignedPolicy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	router := newRouter(t, upstream.URL)
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set(identity.HeaderName, sign(t, privateKey, []string{"fast"}))
	response := httptest.NewRecorder()
	New(verifier, router, nil).ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"coding"`) || !strings.Contains(response.Body.String(), `"fast"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChatRewritesAliasAndDoesNotForwardHomeHubHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer provider-secret" {
			t.Errorf("authorization=%q", request.Header.Get("Authorization"))
		}
		if request.Header.Get(identity.HeaderName) != "" || request.Header.Get("Cookie") != "" {
			t.Errorf("untrusted headers reached provider: %v", request.Header)
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"model":"deepseek-v4-flash"`) || !strings.Contains(string(body), `"temperature":0.2`) {
			t.Errorf("rewritten body=%s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chat-1","choices":[]}`))
	}))
	defer upstream.Close()
	router := newRouter(t, upstream.URL)
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"fast","messages":[{"role":"user","content":"private prompt"}],"temperature":0.2}`))
	request.Header.Set(identity.HeaderName, sign(t, privateKey, []string{"fast"}))
	request.Header.Set("Cookie", "must-not-leak=true")
	response := httptest.NewRecorder()
	New(verifier, router, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"id":"chat-1","choices":[]}` {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChatRejectsModelOutsideDelegationBeforeCallingProvider(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	defer upstream.Close()
	router := newRouter(t, upstream.URL)
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"coding","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set(identity.HeaderName, sign(t, privateKey, []string{"fast"}))
	response := httptest.NewRecorder()
	New(verifier, router, nil).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || calls != 0 {
		t.Fatalf("status=%d calls=%d body=%s", response.Code, calls, response.Body.String())
	}
}

func TestChatStreamsSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/event-stream")
		flusher := writer.(http.Flusher)
		_, _ = writer.Write([]byte("data: first\n\n"))
		flusher.Flush()
		_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()
	router := newRouter(t, upstream.URL)
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "ai-gateway")
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"fast","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	request.Header.Set(identity.HeaderName, sign(t, privateKey, []string{"fast"}))
	response := httptest.NewRecorder()
	New(verifier, router, nil).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "text/event-stream" || !strings.Contains(response.Body.String(), "[DONE]") {
		t.Fatalf("status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
}

func newRouter(t *testing.T, baseURL string) *gateway.Router {
	t.Helper()
	keyFile := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(keyFile, []byte("provider-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	router, err := gateway.New(config.Config{
		Providers: []config.Provider{{ID: "test-provider", BaseURL: baseURL, APIKeyFile: keyFile}},
		Models: []config.Model{
			{ID: "fast", Description: "Fast model", Provider: "test-provider", UpstreamModel: "deepseek-v4-flash"},
			{ID: "coding", Description: "Coding model", Provider: "test-provider", UpstreamModel: "kimi-k2.7-code"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func sign(t *testing.T, key ed25519.PrivateKey, models []string) string {
	t.Helper()
	now := time.Now().UTC()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payload, err := json.Marshal(identity.Claims{
		Issuer: identity.Issuer, Audience: "ai-gateway", Subject: "owner-1", Name: "Luna",
		Scopes: []string{"ai.use"}, AuthorizedParty: "assistant", Models: models,
		IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := header + "." + encodedPayload
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
