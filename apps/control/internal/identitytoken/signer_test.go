package identitytoken

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIssueUsesShortLivedServiceBoundClaims(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(strings.Repeat("k", 32)), 0o600); err != nil {
		t.Fatal(err)
	}
	signer, err := NewFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	issuedAt := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	signer.now = func() time.Time { return issuedAt }
	token, err := signer.Issue("principal-1", "Luna", []string{"admin", "portal.view"}, "drop")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("token parts = %d", len(parts))
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var got claims
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatal(err)
	}
	if got.Issuer != issuer || got.Audience != "drop" || got.Subject != "principal-1" || got.Name != "Luna" {
		t.Fatalf("unexpected claims: %+v", got)
	}
	if got.Expires-got.IssuedAt != 60 {
		t.Fatalf("token lifetime = %d seconds", got.Expires-got.IssuedAt)
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	var header map[string]string
	if err := json.Unmarshal(headerBytes, &header); err != nil || header["alg"] != "EdDSA" {
		t.Fatalf("unexpected header: %s", headerBytes)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	seed := sha256.Sum256([]byte(strings.Repeat("k", 32)))
	publicKey := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
	if !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		t.Fatal("token signature is invalid")
	}
}

func TestNewFromFileRejectsShortKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte("too-short"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := NewFromFile(path); err == nil {
		t.Fatal("expected short key to be rejected")
	}
}
