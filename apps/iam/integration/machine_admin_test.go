package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRootCreatesBoundedWorkloadIdentity(t *testing.T) {
	baseURL := strings.TrimRight(os.Getenv("HOMEHUB_IAM_INTEGRATION_URL"), "/")
	credentialFile := os.Getenv("HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE")
	if baseURL == "" || credentialFile == "" {
		t.Skip("IAM integration environment is not configured")
	}
	rootBytes, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatal(err)
	}
	rootCredential := strings.TrimSpace(string(rootBytes))
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	adminToken := accessToken(t, exchangeFor(t, client, baseURL, rootCredential, "homehub-iam", []string{"iam.principal.manage", "iam.grant.manage"}))

	externalSubject := fmt.Sprintf("integration-worker-%d", time.Now().UnixNano())
	body, _ := json.Marshal(map[string]any{
		"kind": "workload", "display_name": "Integration Worker", "external_subject": externalSubject,
		"grants": []map[string]string{{"service_id": "drop", "relation": "caller"}},
	})
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/machine-identities", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+adminToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	contents, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	if response.StatusCode != http.StatusCreated || response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("create workload status=%d body=%s", response.StatusCode, contents)
	}
	var created struct {
		Subject    string `json:"subject"`
		Credential string `json:"credential"`
	}
	if json.Unmarshal(contents, &created) != nil || !strings.HasPrefix(created.Subject, "workload:") || !strings.HasPrefix(created.Credential, "hhm_") {
		t.Fatal("invalid machine creation response")
	}

	allowed := exchangeFor(t, client, baseURL, created.Credential, "homehub-drop", []string{"drop.item.create"})
	allowed.Body.Close()
	if allowed.StatusCode != http.StatusOK {
		t.Fatalf("caller create exchange status=%d", allowed.StatusCode)
	}
	for name, attempt := range map[string]*http.Response{
		"read":          exchangeFor(t, client, baseURL, created.Credential, "homehub-drop", []string{"drop.item.read"}),
		"delete":        exchangeFor(t, client, baseURL, created.Credential, "homehub-drop", []string{"drop.item.delete"}),
		"root":          exchangeFor(t, client, baseURL, created.Credential, "homehub-drop", []string{"system.root"}),
		"other service": exchangeFor(t, client, baseURL, created.Credential, "homehub-control", []string{"control.dashboard.read"}),
	} {
		attempt.Body.Close()
		if attempt.StatusCode != http.StatusForbidden {
			t.Fatalf("%s exchange status=%d, want 403", name, attempt.StatusCode)
		}
	}
}

func accessToken(t *testing.T, response *http.Response) string {
	t.Helper()
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		contents, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		t.Fatalf("admin exchange status=%d body=%s", response.StatusCode, contents)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if json.NewDecoder(response.Body).Decode(&result) != nil || result.AccessToken == "" {
		t.Fatal("admin exchange returned no access token")
	}
	return result.AccessToken
}
