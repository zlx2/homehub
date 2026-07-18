package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type exchangeResponse struct {
	AccessToken string `json:"access_token"`
}

func TestLiveControlAuthorization(t *testing.T) {
	iamURL := strings.TrimRight(os.Getenv("HOMEHUB_IAM_INTEGRATION_URL"), "/")
	controlURL := strings.TrimRight(os.Getenv("HOMEHUB_CONTROL_INTEGRATION_URL"), "/")
	credentialFile := os.Getenv("HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE")
	if iamURL == "" || controlURL == "" || credentialFile == "" {
		t.Skip("live HomeHub integration environment is not configured")
	}
	credentialBytes, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatal(err)
	}
	credential := strings.TrimSpace(string(credentialBytes))
	if credential == "" {
		t.Fatal("empty machine credential")
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}

	dashboardToken := exchange(t, client, iamURL, credential, "homehub-control", []string{"control.dashboard.read"})
	assertStatus(t, client, controlURL+"/v1/overview", dashboardToken, http.StatusOK)
	assertStatus(t, client, controlURL+"/v1/overview", "", http.StatusUnauthorized)

	nodeToken := exchange(t, client, iamURL, credential, "homehub-control", []string{"control.node.read"})
	assertStatus(t, client, controlURL+"/v1/overview", nodeToken, http.StatusForbidden)

	dropToken := exchange(t, client, iamURL, credential, "homehub-drop", []string{"drop.item.create"})
	assertStatus(t, client, controlURL+"/v1/overview", dropToken, http.StatusUnauthorized)
}

func exchange(t *testing.T, client *http.Client, baseURL, credential, audience string, permissions []string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"audience": audience, "permissions": permissions})
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, baseURL+"/v1/tokens/exchange", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	contents, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("token exchange status = %d, body = %s", response.StatusCode, contents)
	}
	var result exchangeResponse
	if json.Unmarshal(contents, &result) != nil || result.AccessToken == "" {
		t.Fatal("token exchange returned no access token")
	}
	return result.AccessToken
}

func assertStatus(t *testing.T, client *http.Client, url, token string, expected int) {
	t.Helper()
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != expected {
		t.Fatalf("GET %s status = %d, want %d", url, response.StatusCode, expected)
	}
}
