package identitytoken

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
	key []byte
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
	key := []byte(strings.TrimSpace(string(value)))
	if len(key) < 32 {
		return nil, errors.New("identity signing key must contain at least 32 bytes")
	}
	return &Signer{key: key, now: time.Now}, nil
}

func (s *Signer) Issue(subject, name string, scopes []string, audience string) (string, error) {
	if s == nil || len(s.key) < 32 || subject == "" || audience == "" {
		return "", errors.New("invalid identity token input")
	}
	now := s.now().UTC()
	header, _ := json.Marshal(map[string]string{"alg": "HS256", "typ": "JWT"})
	payload, err := json.Marshal(claims{
		Issuer: issuer, Audience: audience, Subject: subject, Name: name,
		Scopes: append([]string(nil), scopes...), IssuedAt: now.Unix(), Expires: now.Add(tokenTTL).Unix(),
	})
	if err != nil {
		return "", fmt.Errorf("encode identity claims: %w", err)
	}
	unsigned := encode(header) + "." + encode(payload)
	mac := hmac.New(sha256.New, s.key)
	_, _ = mac.Write([]byte(unsigned))
	return unsigned + "." + encode(mac.Sum(nil)), nil
}

func encode(value []byte) string { return base64.RawURLEncoding.EncodeToString(value) }
