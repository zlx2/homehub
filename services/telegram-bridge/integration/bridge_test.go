package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/drop"
	bridgeiam "gitee.com/zlx23/homehub/services/telegram-bridge/internal/iam"
)

func TestLiveBridgeIdentityCreatesButCannotReadDrop(t *testing.T) {
	iamURL := strings.TrimRight(os.Getenv("HOMEHUB_IAM_INTEGRATION_URL"), "/")
	dropURL := strings.TrimRight(os.Getenv("HOMEHUB_DROP_INTEGRATION_URL"), "/")
	machineFile := os.Getenv("HOMEHUB_TELEGRAM_INTEGRATION_CREDENTIAL_FILE")
	rootFile := os.Getenv("HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE")
	if iamURL == "" || dropURL == "" || machineFile == "" || rootFile == "" {
		t.Skip("live Telegram Bridge integration environment is not configured")
	}
	machineCredential := readSecret(t, machineFile)
	rootCredential := readSecret(t, rootFile)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	tokens := bridgeiam.NewClient(iamURL, machineCredential, 5*time.Second)
	dropClient := drop.NewClient(dropURL, tokens, 10*time.Second)
	text := fmt.Sprintf("telegram-bridge-integration-%d", time.Now().UnixNano())
	item, err := dropClient.Create(ctx, drop.CreateInput{
		Text: text, TTL: 1, IdempotencyKey: fmt.Sprintf("telegram-integration-%d", time.Now().UnixNano()),
	})
	if err != nil {
		t.Fatal(err)
	}

	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}}
	if status := exchangeStatus(t, client, iamURL, machineCredential, []string{"drop.item.read"}); status != http.StatusForbidden {
		t.Fatalf("machine read-token exchange status=%d, want 403", status)
	}
	rootToken := exchange(t, client, iamURL, rootCredential, []string{"drop.item.read", "drop.item.delete"})
	defer deleteItem(t, client, dropURL, rootToken, item.ID)
	machineToken, err := tokens.Token(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status := requestStatus(t, client, http.MethodGet, dropURL+"/v1/items/"+item.ID, machineToken); status != http.StatusForbidden {
		t.Fatalf("create-only token read status=%d, want 403", status)
	}

	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, dropURL+"/v1/items/"+item.ID, nil)
	request.Header.Set("Authorization", "Bearer "+rootToken)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var stored struct {
		Text           string `json:"text"`
		CreatorSubject string `json:"creator_subject"`
		ActorSubject   string `json:"actor_subject"`
	}
	if response.StatusCode != http.StatusOK || json.NewDecoder(response.Body).Decode(&stored) != nil {
		t.Fatalf("root read status=%d", response.StatusCode)
	}
	if stored.Text != text || !strings.HasPrefix(stored.CreatorSubject, "workload:") || stored.ActorSubject != stored.CreatorSubject {
		t.Fatalf("stored attribution=%+v", stored)
	}
}

func readSecret(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		t.Fatal("credential file is empty")
	}
	return value
}

func exchange(t *testing.T, client *http.Client, baseURL, credential string, permissions []string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"audience": "homehub-drop", "permissions": permissions})
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/tokens/exchange", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	contents, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusOK {
		t.Fatalf("root exchange status=%d body=%s", response.StatusCode, contents)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(contents, &result) != nil || result.AccessToken == "" {
		t.Fatal("root exchange returned no token")
	}
	return result.AccessToken
}

func exchangeStatus(t *testing.T, client *http.Client, baseURL, credential string, permissions []string) int {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"audience": "homehub-drop", "permissions": permissions})
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/tokens/exchange", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+credential)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	return response.StatusCode
}

func requestStatus(t *testing.T, client *http.Client, method, url, token string) int {
	t.Helper()
	request, _ := http.NewRequest(method, url, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	return response.StatusCode
}

func deleteItem(t *testing.T, client *http.Client, baseURL, token, id string) {
	t.Helper()
	if status := requestStatus(t, client, http.MethodDelete, baseURL+"/v1/items/"+id, token); status != http.StatusNoContent && status != http.StatusNotFound {
		t.Errorf("cleanup delete status=%d", status)
	}
}
