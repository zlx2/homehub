package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
	"time"
)

func TestPasswordHashAndVerification(t *testing.T) {
	service := &Service{hashSlots: make(chan struct{}, 1)}
	hash, err := service.hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if hash == "correct horse battery staple" {
		t.Fatal("password was stored directly")
	}
	if !service.verifyPassword("correct horse battery staple", hash) {
		t.Fatal("correct password was rejected")
	}
	if service.verifyPassword("wrong password", hash) {
		t.Fatal("wrong password was accepted")
	}
}

func TestTOTPSecretEncryption(t *testing.T) {
	block, err := aes.NewCipher(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	service := &Service{aead: aead}
	nonce, ciphertext, err := service.encrypt([]byte("totp-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ciphertext) == "totp-secret" {
		t.Fatal("TOTP secret was not encrypted")
	}
	plaintext, err := service.decrypt(nonce, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "totp-secret" {
		t.Fatalf("decrypted value = %q", plaintext)
	}
}

func TestSessionDeadlinesAreCappedByShareLink(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	linkExpiry := now.Add(90 * time.Minute)
	idle, absolute, err := sessionDeadlines(now, linkExpiry)
	if err != nil {
		t.Fatal(err)
	}
	if !idle.Equal(linkExpiry) || !absolute.Equal(linkExpiry) {
		t.Fatalf("deadlines = (%v, %v), want link expiry %v", idle, absolute, linkExpiry)
	}
	if _, _, err := sessionDeadlines(now, now); err == nil {
		t.Fatal("expected expired share link to reject session creation")
	}
}

func TestOpaqueTokensUseDifferentHashes(t *testing.T) {
	first, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	second, err := randomToken(32)
	if err != nil {
		t.Fatal(err)
	}
	if first == second || tokenHash(first) == tokenHash(second) {
		t.Fatal("independent tokens collided")
	}
}

func TestAPITokenSpecIsNarrowAndTimeBound(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	if err := validateAPITokenSpec("iPhone", "drop", []string{"drop.upload"}, now.Add(365*24*time.Hour), now); err != nil {
		t.Fatalf("valid token rejected: %v", err)
	}
	for _, test := range []struct {
		name      string
		serviceID string
		scopes    []string
		expiresAt time.Time
	}{
		{name: "wrong service", serviceID: "rolechat", scopes: []string{"drop.upload"}, expiresAt: now.Add(time.Hour)},
		{name: "broad scope", serviceID: "drop", scopes: []string{"admin"}, expiresAt: now.Add(time.Hour)},
		{name: "too long", serviceID: "drop", scopes: []string{"drop.upload"}, expiresAt: now.Add(367 * 24 * time.Hour)},
	} {
		if err := validateAPITokenSpec(test.name, test.serviceID, test.scopes, test.expiresAt, now); err == nil {
			t.Fatalf("%s should fail", test.name)
		}
	}
}
