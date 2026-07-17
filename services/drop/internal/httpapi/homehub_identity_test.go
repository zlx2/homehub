package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestIdentityVerifierAcceptsValidToken(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	verifier := &identityVerifier{key: []byte(strings.Repeat("k", 32)), now: func() time.Time { return now }}
	token := signTestIdentity(t, verifier.key, identityClaims{
		Issuer: "homehub-control", Audience: "drop", Subject: "owner-1", Name: "Luna",
		Scopes: []string{"admin", "portal.view"}, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	})
	claims, err := verifier.verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "owner-1" || !containsScope(claims.Scopes, "admin") {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestIdentityVerifierRejectsInvalidSecurityClaims(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	key := []byte(strings.Repeat("k", 32))
	verifier := &identityVerifier{key: key, now: func() time.Time { return now }}
	base := identityClaims{
		Issuer: "homehub-control", Audience: "drop", Subject: "guest-1", Name: "Guest",
		Scopes: []string{"portal.view"}, IssuedAt: now.Unix(), Expires: now.Add(time.Minute).Unix(),
	}
	tests := map[string]identityClaims{
		"wrong issuer":   withIdentity(base, func(c *identityClaims) { c.Issuer = "attacker" }),
		"wrong audience": withIdentity(base, func(c *identityClaims) { c.Audience = "other" }),
		"expired":        withIdentity(base, func(c *identityClaims) { c.Expires = now.Add(-time.Second).Unix() }),
		"future":         withIdentity(base, func(c *identityClaims) { c.IssuedAt = now.Add(time.Minute).Unix() }),
		"too long":       withIdentity(base, func(c *identityClaims) { c.Expires = now.Add(2 * time.Minute).Unix() }),
		"missing scope":  withIdentity(base, func(c *identityClaims) { c.Scopes = []string{"unrelated"} }),
	}
	for name, claims := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := verifier.verify(signTestIdentity(t, key, claims)); err == nil {
				t.Fatal("expected token rejection")
			}
		})
	}
	valid := signTestIdentity(t, key, base)
	tampered := valid[:len(valid)-1] + map[bool]string{true: "A", false: "B"}[valid[len(valid)-1] != 'A']
	if _, err := verifier.verify(tampered); err == nil {
		t.Fatal("expected tampered token rejection")
	}
}

func withIdentity(value identityClaims, mutate func(*identityClaims)) identityClaims {
	mutate(&value)
	return value
}

func signTestIdentity(t *testing.T, key []byte, claims identityClaims) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
