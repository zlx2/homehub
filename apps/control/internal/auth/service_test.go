package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"testing"
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
