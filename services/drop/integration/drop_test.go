package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

func TestLiveDropAuthorizationAndOriginalFile(t *testing.T) {
	dropURL := strings.TrimRight(os.Getenv("HOMEHUB_DROP_INTEGRATION_URL"), "/")
	iamURL := strings.TrimRight(os.Getenv("HOMEHUB_IAM_INTEGRATION_URL"), "/")
	credentialFile := os.Getenv("HOMEHUB_IAM_INTEGRATION_CREDENTIAL_FILE")
	if dropURL == "" || iamURL == "" || credentialFile == "" {
		t.Skip("live Drop integration environment is not configured")
	}
	credentialBytes, err := os.ReadFile(credentialFile)
	if err != nil {
		t.Fatal(err)
	}
	rootCredential := strings.TrimSpace(string(credentialBytes))
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}}
	rootToken := exchange(t, client, iamURL, rootCredential, "homehub-drop", []string{
		"drop.item.create", "drop.item.read", "drop.item.list", "drop.item.delete",
	})

	original := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0xff, 0x42}
	created := create(t, client, dropURL, rootToken, "root upload", original, "integration-root-upload")
	itemID := stringField(t, created, "id")
	attachments := created["attachments"].([]any)
	attachmentID := stringField(t, attachments[0].(map[string]any), "id")
	assertStatus(t, client, http.MethodGet, dropURL+"/v1/items", rootToken, http.StatusOK)
	assertStatus(t, client, http.MethodGet, dropURL+"/v1/items/"+itemID, rootToken, http.StatusOK)
	request, _ := http.NewRequest(http.MethodGet, dropURL+"/v1/attachments/"+attachmentID, nil)
	request.Header.Set("Authorization", "Bearer "+rootToken)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	downloaded, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK || !bytes.Equal(downloaded, original) || !strings.HasPrefix(response.Header.Get("Content-Disposition"), "inline") {
		t.Fatalf("original download status=%d size=%d", response.StatusCode, len(downloaded))
	}

	adminToken := exchange(t, client, iamURL, rootCredential, "homehub-iam", []string{"iam.principal.manage", "iam.grant.manage"})
	workloadCredential := createWorkload(t, client, iamURL, adminToken)
	createToken := exchange(t, client, iamURL, workloadCredential, "homehub-drop", []string{"drop.item.create"})
	limited := create(t, client, dropURL, createToken, "limited upload", nil, "integration-limited-upload-"+fmt.Sprint(time.Now().UnixNano()))
	limitedID := stringField(t, limited, "id")
	assertStatus(t, client, http.MethodGet, dropURL+"/v1/items", createToken, http.StatusForbidden)
	assertStatus(t, client, http.MethodDelete, dropURL+"/v1/items/"+limitedID, createToken, http.StatusForbidden)
	controlToken := exchange(t, client, iamURL, rootCredential, "homehub-control", []string{"control.dashboard.read"})
	assertStatus(t, client, http.MethodGet, dropURL+"/v1/items", controlToken, http.StatusUnauthorized)

	assertStatus(t, client, http.MethodDelete, dropURL+"/v1/items/"+limitedID, rootToken, http.StatusNoContent)
	assertStatus(t, client, http.MethodDelete, dropURL+"/v1/items/"+itemID, rootToken, http.StatusNoContent)
	assertStatus(t, client, http.MethodGet, dropURL+"/v1/items/"+itemID, rootToken, http.StatusNotFound)
}

func exchange(t *testing.T, client *http.Client, baseURL, credential, audience string, permissions []string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"audience": audience, "permissions": permissions})
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
		t.Fatalf("exchange %s status=%d body=%s", audience, response.StatusCode, contents)
	}
	var result tokenResponse
	if json.Unmarshal(contents, &result) != nil || result.AccessToken == "" {
		t.Fatal("exchange returned no token")
	}
	return result.AccessToken
}

func createWorkload(t *testing.T, client *http.Client, iamURL, adminToken string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"kind": "workload", "display_name": "Drop Integration", "external_subject": fmt.Sprintf("drop-integration-%d", time.Now().UnixNano()),
		"grants": []map[string]string{{"service_id": "drop", "relation": "caller"}},
	})
	request, _ := http.NewRequest(http.MethodPost, iamURL+"/v1/machine-identities", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+adminToken)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	contents, _ := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create workload status=%d body=%s", response.StatusCode, contents)
	}
	var result struct {
		Credential string `json:"credential"`
	}
	if json.Unmarshal(contents, &result) != nil || result.Credential == "" {
		t.Fatal("no workload credential")
	}
	return result.Credential
}

func create(t *testing.T, client *http.Client, baseURL, token, text string, file []byte, key string) map[string]any {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if text != "" {
		_ = writer.WriteField("text", text)
	}
	_ = writer.WriteField("ttl_days", "1")
	if file != nil {
		part, _ := writer.CreateFormFile("files", "原图.png")
		_, _ = part.Write(file)
	}
	_ = writer.Close()
	request, _ := http.NewRequest(http.MethodPost, baseURL+"/v1/items", &body)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", key)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	contents, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create item status=%d body=%s", response.StatusCode, contents)
	}
	var result map[string]any
	if json.Unmarshal(contents, &result) != nil {
		t.Fatal("invalid create response")
	}
	return result
}

func assertStatus(t *testing.T, client *http.Client, method, url, token string, expected int) {
	t.Helper()
	request, _ := http.NewRequest(method, url, nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	if response.StatusCode != expected {
		t.Fatalf("%s %s status=%d want=%d", method, url, response.StatusCode, expected)
	}
}

func stringField(t *testing.T, value map[string]any, field string) string {
	t.Helper()
	result, ok := value[field].(string)
	if !ok || result == "" {
		t.Fatalf("missing %s in %+v", field, value)
	}
	return result
}
