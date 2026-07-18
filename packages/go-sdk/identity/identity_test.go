package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVerifierAcceptsDirectAndDelegatedTokens(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	verifier, err := NewVerifier(map[string]ed25519.PublicKey{"key-1": publicKey}, "homehub-drop", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verifier.now = func() time.Time { return now }

	claims := Claims{
		Issuer: Issuer, Audience: "homehub-drop", Subject: "human:luna",
		Actor: &Actor{Subject: "agent:hermes"}, AuthorizedParty: "hermes",
		Realm: "homehub", Permissions: []string{"drop.item.create"}, TokenID: "token-1",
		IssuedAt: now.Unix(), Expires: now.Add(2 * time.Minute).Unix(),
	}
	verified, err := verifier.Verify(sign(t, "key-1", privateKey, claims))
	if err != nil {
		t.Fatal(err)
	}
	if verified.Subject != "human:luna" || verified.EffectiveActor() != "agent:hermes" || !verified.Allows("drop.item.create") {
		t.Fatalf("unexpected claims: %+v", verified)
	}
}

func TestSystemRootAllowsAnyConcretePermission(t *testing.T) {
	t.Parallel()
	claims := Claims{Permissions: []string{SystemRootPermission}}
	if !claims.Allows("server.command.execute") || !claims.Allows("drop.item.delete") {
		t.Fatal("system.root did not imply a concrete permission")
	}
}

func TestVerifierRejectsInvalidClaims(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	verifier, _ := NewVerifier(map[string]ed25519.PublicKey{"key-1": publicKey}, "homehub-drop", 5*time.Minute)
	verifier.now = func() time.Time { return now }
	base := Claims{
		Issuer: Issuer, Audience: "homehub-drop", Subject: "workload:telegram-bridge",
		AuthorizedParty: "telegram-bridge", Realm: "homehub", Permissions: []string{"drop.item.create"},
		TokenID: "token-1", IssuedAt: now.Unix(), Expires: now.Add(2 * time.Minute).Unix(),
	}

	tests := map[string]Claims{
		"wrong audience": mutate(base, func(claims *Claims) { claims.Audience = "homehub-ai" }),
		"bad subject": mutate(base, func(claims *Claims) { claims.Subject = "unknown:thing" }),
		"bad actor": mutate(base, func(claims *Claims) { claims.Actor = &Actor{Subject: "root"} }),
		"bad permission": mutate(base, func(claims *Claims) { claims.Permissions = []string{"drop.*"} }),
		"empty permissions": mutate(base, func(claims *Claims) { claims.Permissions = nil }),
		"expired": mutate(base, func(claims *Claims) { claims.Expires = now.Add(-time.Second).Unix() }),
		"too long": mutate(base, func(claims *Claims) { claims.Expires = now.Add(10 * time.Minute).Unix() }),
	}
	for name, claims := range tests {
		claims := claims
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := verifier.Verify(sign(t, "key-1", privateKey, claims)); err == nil {
				t.Fatal("expected token rejection")
			}
		})
	}
}

func TestAuthenticateUsesBearerAndConcretePermissions(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	now := time.Date(2026, 7, 18, 8, 0, 0, 0, time.UTC)
	verifier, _ := NewVerifier(map[string]ed25519.PublicKey{"key-1": publicKey}, "homehub-drop", 5*time.Minute)
	verifier.now = func() time.Time { return now }
	token := sign(t, "key-1", privateKey, Claims{
		Issuer: Issuer, Audience: "homehub-drop", Subject: "device:iphone",
		AuthorizedParty: "shortcut", Realm: "homehub", Permissions: []string{"drop.item.create"},
		TokenID: "token-1", IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})

	next := http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		claims, ok := FromContext(request.Context())
		if !ok || claims.Subject != "device:iphone" {
			t.Fatal("claims missing from context")
		}
		response.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/items", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	verifier.Authenticate([]string{"drop.item.create"}, next).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func mutate(value Claims, change func(*Claims)) Claims {
	change(&value)
	return value
}

func sign(t *testing.T, keyID string, key ed25519.PrivateKey, claims Claims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "at+jwt", "kid": keyID})
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(key, []byte(unsigned)))
}
