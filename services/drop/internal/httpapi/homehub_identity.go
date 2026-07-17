package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const homeHubIdentityHeader = "X-HomeHub-Identity"

type identityVerifier struct {
	key []byte
	now func() time.Time
}

type identityClaims struct {
	Issuer   string   `json:"iss"`
	Audience string   `json:"aud"`
	Subject  string   `json:"sub"`
	Name     string   `json:"name"`
	Scopes   []string `json:"scopes"`
	IssuedAt int64    `json:"iat"`
	Expires  int64    `json:"exp"`
}

func newIdentityVerifier(path string) (*identityVerifier, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HomeHub identity key: %w", err)
	}
	key := []byte(strings.TrimSpace(string(value)))
	if len(key) < 32 {
		return nil, errors.New("HomeHub identity key must contain at least 32 bytes")
	}
	return &identityVerifier{key: key, now: time.Now}, nil
}

func (v *identityVerifier) verify(token string) (identityClaims, error) {
	if len(token) == 0 || len(token) > 8192 {
		return identityClaims{}, errors.New("invalid token size")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return identityClaims{}, errors.New("invalid token format")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return identityClaims{}, errors.New("invalid token header")
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return identityClaims{}, errors.New("unsupported token header")
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(provided) != sha256.Size {
		return identityClaims{}, errors.New("invalid token signature")
	}
	mac := hmac.New(sha256.New, v.key)
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(provided, expected) != 1 {
		return identityClaims{}, errors.New("invalid token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return identityClaims{}, errors.New("invalid token claims")
	}
	var claims identityClaims
	if json.Unmarshal(payload, &claims) != nil {
		return identityClaims{}, errors.New("invalid token claims")
	}
	now := v.now().UTC().Unix()
	if claims.Issuer != "homehub-control" || claims.Audience != "drop" || claims.Subject == "" || claims.Expires <= now || claims.IssuedAt > now+30 || claims.Expires-claims.IssuedAt > 90 {
		return identityClaims{}, errors.New("invalid token claims")
	}
	if !containsScope(claims.Scopes, "portal.view") && !containsScope(claims.Scopes, "admin") {
		return identityClaims{}, errors.New("required scope is missing")
	}
	return claims, nil
}

func containsScope(scopes []string, expected string) bool {
	for _, scope := range scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func (a *API) authenticateHomeHub(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := a.identity.verify(r.Header.Get(homeHubIdentityHeader))
		if err != nil {
			writeAPIError(w, unauthorized())
			return
		}
		role := RoleGuest
		if containsScope(claims.Scopes, "admin") {
			role = RoleOwner
		}
		value := principal{Role: role, Subject: claims.Subject}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, value)))
	})
}

func (a *API) requireAllowedOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := strings.ToLower(strings.TrimSpace(r.Header.Get("Origin")))
			if _, ok := a.cfg.AllowedOrigins[origin]; !ok {
				writeAPIError(w, &apiError{Status: http.StatusForbidden, Code: "invalid_origin", Message: "Request origin is not allowed"})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
