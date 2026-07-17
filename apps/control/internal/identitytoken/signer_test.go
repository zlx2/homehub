package identitytoken

import (
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
