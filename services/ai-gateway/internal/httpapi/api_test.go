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

	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
	"gitee.com/zlx23/homehub/services/ai-gateway/internal/config"
	"gitee.com/zlx23/homehub/services/ai-gateway/internal/gateway"
)

func TestHealthDoesNotRequireIdentity(t *testing.T) {
	router := newRouter(t, "http://127.0.0.1:1")
	verifier, _, _ := newVerifier(t)
	response := httptest.NewRecorder()
	New(verifier, router, nil).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d", response.Code)
	}
}

func TestChatRequiresAccessToken(t *testing.T) {
	router := newRouter(t, "http://127.0.0.1:1")
	verifier, _, _ := newVerifier(t)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"fast","messages":[{"role":"user","content":"hi"}]}`))
	response := httptest.NewRecorder()
	New(verifier, router, nil).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestModelsAreFilteredBySignedPermissions(t *testing.T) {
	router := newRouter(t, "http://127.0.0.1:1")
	verifier, privateKey, keyID := newVerifier(t)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+sign(t, privateKey, keyID, []string{"ai.model.fast"}))
	response := httptest.NewRecorder()
	New(verifier, router, nil).ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), `"coding"`) || !strings.Contains(response.Body.String(), `"fast"`) {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestChatRewritesAliasAndDoesNotForwardCallerHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer provider-secret" || request.Header.Get("Cookie") != "" {
			t.Errorf("unexpected upstream headers: %v", request.Header)
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"model":"deepseek-v4-flash"`) {
			t.Errorf("rewritten body=%s", body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"chat-1","choices":[]}`))
	}))
	defer upstream.Close()
	router := newRouter(t, upstream.URL)
	verifier, privateKey, keyID := newVerifier(t)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"fast","messages":[{"role":"user","content":"hi"}]}`))
	request.Header.Set("Authorization", "Bearer "+sign(t, privateKey, keyID, []string{"ai.model.fast"}))
	request.Header.Set("Cookie", "must-not-leak=true")
	response := httptest.NewRecorder()
	New(verifier, router, slog.New(slog.NewTextHandler(io.Discard, nil))).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
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
			{ID: "fast", Provider: "test-provider", UpstreamModel: "deepseek-v4-flash"},
			{ID: "coding", Provider: "test-provider", UpstreamModel: "code-model"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return router
}

func newVerifier(t *testing.T) (*identity.Verifier, ed25519.PrivateKey, string) {
	t.Helper()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	const keyID = "test-key"
	verifier, err := identity.NewVerifier(map[string]ed25519.PublicKey{keyID: publicKey}, "homehub-ai-gateway", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return verifier, privateKey, keyID
}

func sign(t *testing.T, key ed25519.PrivateKey, keyID string, permissions []string) string {
	t.Helper()
	now := time.Now().UTC()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "at+jwt", "kid": keyID})
	payload, err := json.Marshal(identity.Claims{
		Issuer: identity.Issuer, Audience: "homehub-ai-gateway", Subject: "agent:test",
		AuthorizedParty: "test-client", Realm: "homehub", Permissions: permissions,
		TokenID: "test-token", IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})
	if err != nil {
		t.Fatal(err)
	}
	encodedHeader := base64.RawURLEncoding.EncodeToString(header)
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	unsigned := encodedHeader + "." + encodedPayload
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
