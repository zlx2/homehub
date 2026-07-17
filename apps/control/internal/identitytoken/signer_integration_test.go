package identitytoken

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"
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

func TestRunningAIGatewayCompletes(t *testing.T) {
	target := os.Getenv("HOMEHUB_AI_COMPLETION_SMOKE_URL")
	model := os.Getenv("HOMEHUB_AI_COMPLETION_SMOKE_MODEL")
	keyFile := os.Getenv("HOMEHUB_IDENTITY_SMOKE_KEY_FILE")
	if target == "" || model == "" || keyFile == "" {
		t.Skip("running AI completion smoke test is not configured")
	}
	signer, err := NewFromFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.IssueAI(
		"ai-completion-smoke", "AI Completion Smoke Test", "smoke-service",
		[]string{"portal.view"}, []string{model},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"model":      model,
		"messages":   []map[string]string{{"role": "user", "content": "Reply with OK."}},
		"max_tokens": 16,
		"stream":     os.Getenv("HOMEHUB_AI_COMPLETION_SMOKE_STREAM") == "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-HomeHub-Identity", token)
	client := &http.Client{Timeout: 2 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var gatewayError struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		_ = json.NewDecoder(response.Body).Decode(&gatewayError)
		t.Fatalf("AI Gateway model %q returned status %d code %q", model, response.StatusCode, gatewayError.Error.Code)
	}
	if os.Getenv("HOMEHUB_AI_COMPLETION_SMOKE_STREAM") == "true" {
		scanner := bufio.NewScanner(response.Body)
		seenData := false
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") && line != "data: [DONE]" {
				seenData = true
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		if !seenData {
			t.Fatal("AI provider returned no SSE data")
		}
		return
	}
	var completion struct {
		Choices []json.RawMessage `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&completion); err != nil {
		t.Fatal(err)
	}
	if len(completion.Choices) == 0 {
		t.Fatal("AI provider returned no choices")
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
