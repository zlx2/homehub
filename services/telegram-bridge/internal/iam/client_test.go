package iam

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTokenExchangesCredentialAndCachesAccessToken(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		calls++
		if request.Method != http.MethodPost || request.URL.Path != "/v1/tokens/exchange" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer machine-secret" {
			t.Fatal("machine credential was not sent")
		}
		var body struct {
			Audience    string   `json:"audience"`
			Permissions []string `json:"permissions"`
		}
		if json.NewDecoder(request.Body).Decode(&body) != nil || body.Audience != dropAudience ||
			len(body.Permissions) != 1 || body.Permissions[0] != createPermission {
			t.Fatalf("exchange body = %#v", body)
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"access_token":"short-lived-token","expires_in":120,"audience":"homehub-drop","permissions":["drop.item.create"]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "machine-secret", time.Second)
	first, err := client.Token(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := client.Token(context.Background())
	if err != nil || first != "short-lived-token" || second != first || calls != 1 {
		t.Fatalf("first=%q second=%q calls=%d err=%v", first, second, calls, err)
	}
}

func TestTokenRejectsUnexpectedGrant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte(`{"access_token":"wrong-token","expires_in":120,"audience":"homehub-drop","permissions":["drop.item.read"]}`))
	}))
	defer server.Close()
	client := NewClient(server.URL, "machine-secret", time.Second)
	if _, err := client.Token(context.Background()); err == nil {
		t.Fatal("unexpected IAM permission was accepted")
	}
}
