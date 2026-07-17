package identitytoken

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestRunningServiceAcceptsControlIdentity(t *testing.T) {
	target := os.Getenv("HOMEHUB_IDENTITY_SMOKE_URL")
	keyFile := os.Getenv("HOMEHUB_IDENTITY_SMOKE_KEY_FILE")
	if target == "" || keyFile == "" {
		t.Skip("running-service identity smoke test is not configured")
	}
	signer, err := NewFromFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.Issue("identity-smoke", "Identity Smoke Test", []string{"portal.view"}, "drop")
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-HomeHub-Identity", token)
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("service returned status %d", response.StatusCode)
	}
}

func TestRunningAIGatewayAcceptsDelegation(t *testing.T) {
	target := os.Getenv("HOMEHUB_AI_IDENTITY_SMOKE_URL")
	keyFile := os.Getenv("HOMEHUB_IDENTITY_SMOKE_KEY_FILE")
	if target == "" || keyFile == "" {
		t.Skip("running AI Gateway identity smoke test is not configured")
	}
	signer, err := NewFromFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.IssueAI(
		"ai-identity-smoke", "AI Identity Smoke Test", "smoke-service",
		[]string{"portal.view"}, []string{"fast"},
	)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-HomeHub-Identity", token)
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("AI Gateway returned status %d", response.StatusCode)
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&models); err != nil {
		t.Fatal(err)
	}
	if len(models.Data) != 1 || models.Data[0].ID != "fast" {
		t.Fatalf("unexpected delegated models: %#v", models.Data)
	}
}
