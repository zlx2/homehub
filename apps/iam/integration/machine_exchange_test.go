package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

func TestMachineCredentialExchange(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("HOMEHUB_IAM_INTEGRATION_URL"), "/")
	credentialFile := os.Getenv("HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE")
	if baseURL == "" || credentialFile == "" {
		t.Skip("IAM integration environment is not configured")
	}
	credentialBytes, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatal(err)
	}
	credential := strings.TrimSpace(string(credentialBytes))
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}

	response := exchange(t, client, baseURL, credential, []string{"system.root", "drop.item.delete"})
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		response.Body.Close()
		t.Fatalf("exchange status=%d body=%s", response.StatusCode, body)
	}
	var exchanged struct {
		AccessToken string   `json:"access_token"`
		TokenType   string   `json:"token_type"`
		ExpiresIn   int      `json:"expires_in"`
		Audience    string   `json:"audience"`
		Permissions []string `json:"permissions"`
	}
	if err := json.NewDecoder(response.Body).Decode(&exchanged); err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if exchanged.TokenType != "Bearer" || exchanged.ExpiresIn != 120 || exchanged.Audience != "homehub-drop" {
		t.Fatalf("unexpected exchange metadata: %+v", exchanged)
	}

	jwksResponse, err := client.Get(baseURL + "/.well-known/jwks.json")
	if err != nil {
		t.Fatal(err)
	}
	jwks, err := io.ReadAll(io.LimitReader(jwksResponse.Body, 64<<10))
	jwksResponse.Body.Close()
	if err != nil || jwksResponse.StatusCode != http.StatusOK {
		t.Fatalf("JWKS status=%d err=%v", jwksResponse.StatusCode, err)
	}
	keys, err := identity.ParseJWKSet(jwks)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := identity.NewVerifier(keys, "homehub-drop", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := verifier.Verify(exchanged.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(claims.Subject, "agent:") || claims.EffectiveActor() != claims.Subject ||
		!claims.Allows("drop.item.delete") || !claims.HasPermission(identity.SystemRootPermission) {
		t.Fatalf("unexpected verified claims: %+v", claims)
	}

	invalid := exchange(t, client, baseURL, "hhk_invalid-credential-that-is-long-enough", []string{"drop.item.read"})
	invalid.Body.Close()
	if invalid.StatusCode != http.StatusUnauthorized {
		t.Fatalf("invalid credential status=%d", invalid.StatusCode)
	}
	unknown := exchange(t, client, baseURL, credential, []string{"drop.item.rename"})
	unknown.Body.Close()
	if unknown.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown permission status=%d", unknown.StatusCode)
	}
}

func exchange(t *testing.T, client *http.Client, baseURL, credential string, permissions []string) *http.Response {
	return exchangeFor(t, client, baseURL, credential, "homehub-drop", permissions)
}

func exchangeFor(t *testing.T, client *http.Client, baseURL, credential, audience string, permissions []string) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"audience": audience, "permissions": permissions})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, baseURL+"/v1/tokens/exchange", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
