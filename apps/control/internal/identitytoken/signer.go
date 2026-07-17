package identitytoken

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	issuer   = "homehub-control"
	tokenTTL = 60 * time.Second
)

type Signer struct {
	key ed25519.PrivateKey
	now func() time.Time
}

type claims struct {
	Issuer   string   `json:"iss"`
	Audience string   `json:"aud"`
	Subject  string   `json:"sub"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
	IssuedAt int64    `json:"iat"`
	Expires  int64    `json:"exp"`
}

func NewFromFile(path string) (*Signer, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read identity signing key: %w", err)
	}
	trimmed := []byte(strings.TrimSpace(string(value)))
	key, err := parsePrivateKey(trimmed)
	if err != nil {
		return nil, err
	}
	return &Signer{key: key, now: time.Now}, nil
}

func parsePrivateKey(value []byte) (ed25519.PrivateKey, error) {
	if block, _ := pem.Decode(value); block != nil {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.New("invalid identity signing key")
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok || len(key) != ed25519.PrivateKeySize {
			return nil, errors.New("identity signing key must be Ed25519")
		}
		return append(ed25519.PrivateKey(nil), key...), nil
	}
	if len(value) < 32 {
		return nil, errors.New("identity signing key must contain at least 32 bytes")
	}
	seed := sha256.Sum256(value)
	return ed25519.NewKeyFromSeed(seed[:]), nil
}

func (s *Signer) Issue(subject, name string, scopes []string, audience string) (string, error) {
	if s == nil || len(s.key) != ed25519.PrivateKeySize || subject == "" || audience == "" {
		return "", errors.New("invalid identity token input")
	}
	now := s.now().UTC()
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT"})
	payload, err := json.Marshal(claims{
		Issuer: issuer, Audience: audience, Subject: subject, Name: name,
		Scopes: append([]string(nil), scopes...), IssuedAt: now.Unix(), Expires: now.Add(tokenTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("encode identity claims: %w", err)
	}
	unsigned := encode(header) + "." + encode(payload)
	return unsigned + "." + encode(ed25519.Sign(s.key, []byte(unsigned))), nil
}

func encode(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }
