package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestVerifierAcceptsServiceBoundToken(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	verifier, err := NewVerifier(publicKey, "notes")
	if err != nil {
		t.Fatal(err)
	}
	verifier.now = func() time.Time { return now }
	claims, err := verifier.Verify(sign(t, privateKey, Claims{
		Issuer: Issuer, Audience: "notes", Subject: "owner-1", Name: "Luna",
		Scopes: []string{"admin", "portal.view"}, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	}))
	if err != nil || claims.Subject != "owner-1" || !claims.HasScope("admin") {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
}

func TestVerifierRejectsInvalidSecurityClaims(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	otherPublic, otherPrivate, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	verifier, _ := NewVerifier(publicKey, "notes")
	verifier.now = func() time.Time { return now }
	base := Claims{Issuer: Issuer, Audience: "notes", Subject: "guest-1", Scopes: []string{"portal.view"}, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix()}
	tests := map[string]struct {
		claims Claims
		key    ed25519.PrivateKey
	}{
		"wrong issuer":    {mutate(base, func(c *Claims) { c.Issuer = "attacker" }), privateKey},
		"wrong audience":  {mutate(base, func(c *Claims) { c.Audience = "other" }), privateKey},
		"missing subject": {mutate(base, func(c *Claims) { c.Subject = "" }), privateKey},
		"expired":         {mutate(base, func(c *Claims) { c.Expires = now.Add(-time.Second).Unix() }), privateKey},
		"future":          {mutate(base, func(c *Claims) { c.IssuedAt = now.Add(time.Minute).Unix() }), privateKey},
		"too long":        {mutate(base, func(c *Claims) { c.Expires = now.Add(2 * time.Minute).Unix() }), privateKey},
		"wrong key":       {base, otherPrivate},
	}
	_ = otherPublic
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := verifier.Verify(sign(t, test.key, test.claims)); err == nil {
				t.Fatal("expected token rejection")
			}
		})
	}
}

func TestAuthenticateAddsClaimsAndEnforcesScopes(t *testing.T) {
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	verifier, _ := NewVerifier(publicKey, "notes")
	verifier.now = func() time.Time { return now }
	token := sign(t, privateKey, Claims{Issuer: Issuer, Audience: "notes", Subject: "p1", Scopes: []string{"portal.view"}, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix()})
	next := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		claims, ok := FromContext(request.Context())
		if !ok || claims.Subject != "p1" {
			t.Fatal("identity missing from context")
		}
		writer.WriteHeader(http.StatusNoContent)
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(HeaderName, token)
	response := httptest.NewRecorder()
	verifier.Authenticate([]string{"portal.view", "admin"}, next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	verifier.Authenticate([]string{"admin"}, next).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "insufficient_scope") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func mutate(value Claims, change func(*Claims)) Claims { change(&value); return value }

func sign(t *testing.T, key ed25519.PrivateKey, claims Claims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
