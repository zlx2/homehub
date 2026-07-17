package httpapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"drop/internal/config"
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

func TestAuthenticateHomeHubAllowsUploadTokenOnlyOnCreateItem(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	verifier, _ := identity.NewVerifier(publicKey, "drop")
	api := &API{identity: verifier}
	now := time.Now().UTC()
	token := signHomeHubIdentity(t, privateKey, identity.Claims{
		Issuer: identity.Issuer, Audience: "drop", Subject: "iphone", Scopes: []string{"drop.upload"},
		IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})
	handler := api.authenticateHomeHub(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !principalFrom(request).HasScope("drop.upload") || principalFrom(request).Role != RoleGuest {
			t.Fatalf("unexpected principal: %+v", principalFrom(request))
		}
		writer.WriteHeader(http.StatusNoContent)
	}))

	allowed := httptest.NewRequest(http.MethodPost, "/api/v1/items", nil)
	allowed.Header.Set(identity.HeaderName, token)
	allowedResponse := httptest.NewRecorder()
	handler.ServeHTTP(allowedResponse, allowed)
	if allowedResponse.Code != http.StatusNoContent {
		t.Fatalf("upload status=%d body=%s", allowedResponse.Code, allowedResponse.Body.String())
	}

	denied := httptest.NewRequest(http.MethodGet, "/api/v1/items", nil)
	denied.Header.Set(identity.HeaderName, token)
	deniedResponse := httptest.NewRecorder()
	handler.ServeHTTP(deniedResponse, denied)
	if deniedResponse.Code != http.StatusForbidden {
		t.Fatalf("list status=%d body=%s", deniedResponse.Code, deniedResponse.Body.String())
	}
}

func TestUploadTokenBypassesBrowserOriginCheckOnlyForUpload(t *testing.T) {
	api := &API{cfg: config.Config{AllowedOrigins: map[string]struct{}{}}}
	handler := api.requireAllowedOrigin(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/items", nil)
	request = withPrincipal(request, principal{Role: RoleGuest, Subject: "iphone", Scopes: []string{"drop.upload"}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
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
