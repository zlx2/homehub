package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchJWKSet(t *testing.T) {
	t.Parallel()
	publicKey, _, _ := ed25519.GenerateKey(nil)
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") != "application/json" {
			t.Fatal("missing JSON accept header")
		}
		fmt.Fprintf(response, `{"keys":[{"kty":"OKP","crv":"Ed25519","use":"sig","alg":"EdDSA","kid":"key-1","x":"%s"}]}`,
			base64.RawURLEncoding.EncodeToString(publicKey))
	}))
	defer server.Close()

	client := &http.Client{Timeout: time.Second}
	keys, err := FetchJWKSet(context.Background(), client, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || len(keys["key-1"]) != ed25519.PublicKeySize {
		t.Fatalf("unexpected keys: %+v", keys)
	}
}

func TestFetchJWKSetRejectsBadStatusAndURL(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client := &http.Client{Timeout: time.Second}
	if _, err := FetchJWKSet(context.Background(), client, server.URL); err == nil {
		t.Fatal("expected status rejection")
	}
	if _, err := FetchJWKSet(context.Background(), client, "file:///tmp/jwks.json"); err == nil {
		t.Fatal("expected URL rejection")
	}
}
