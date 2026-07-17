package httpapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"homehub.local/go-sdk/identity"
)

func TestAuthenticateHomeHubMapsIdentityToDropRole(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, err := identity.NewVerifier(publicKey, "drop")
	if err != nil {
		t.Fatal(err)
	}
	api := &API{identity: verifier}
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal := principalFrom(request)
		if principal.Role != RoleOwner || principal.Subject != "owner-1" {
			t.Fatalf("unexpected principal: %+v", principal)
		}
		writer.WriteHeader(http.StatusNoContent)
	})
	now := time.Now().UTC()
	token := signHomeHubIdentity(t, privateKey, identity.Claims{
		Issuer: identity.Issuer, Audience: "drop", Subject: "owner-1", Name: "Luna",
		Scopes: []string{"admin", "portal.view"}, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(identity.HeaderName, token)
	response := httptest.NewRecorder()
	api.authenticateHomeHub(next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAuthenticateHomeHubRejectsMissingPortalScope(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "drop")
	api := &API{identity: verifier}
	now := time.Now().UTC()
	token := signHomeHubIdentity(t, privateKey, identity.Claims{
		Issuer: identity.Issuer, Audience: "drop", Subject: "p1", Scopes: []string{"unrelated"},
		IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(identity.HeaderName, token)
	response := httptest.NewRecorder()
	api.authenticateHomeHub(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("protected handler must not run")
	})).ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func signHomeHubIdentity(t *testing.T, key ed25519.PrivateKey, claims identity.Claims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
