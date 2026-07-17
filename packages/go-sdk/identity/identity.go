package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	HeaderName = "X-HomeHub-Identity"
	Issuer     = "homehub-control"
	maxToken   = 8192
	maxTTL     = 90 * time.Second
	clockSkew  = 30 * time.Second
)

type Claims struct {
	Issuer          string   `json:"iss"`
	Audience        string   `json:"aud"`
	Subject         string   `json:"sub"`
	Name            string   `json:"name"`
	Scopes          []string `json:"scopes"`
	AuthorizedParty string   `json:"azp,omitempty"`
	Models          []string `json:"models,omitempty"`
	IssuedAt        int64    `json:"iat"`
	Expires         int64    `json:"exp"`
}

func (c Claims) HasScope(expected string) bool {
	for _, scope := range c.Scopes {
		if scope == expected {
			return true
		}
	}
	return false
}

func (c Claims) HasAnyScope(expected ...string) bool {
	for _, scope := range expected {
		if c.HasScope(scope) {
			return true
		}
	}
	return false
}

type Verifier struct {
	publicKey ed25519.PublicKey
	audience  string
	now       func() time.Time
}

func NewVerifier(publicKey ed25519.PublicKey, audience string) (*Verifier, error) {
	if len(publicKey) != ed25519.PublicKeySize {
		return nil, errors.New("HomeHub identity public key must be Ed25519")
	}
	if strings.TrimSpace(audience) == "" {
		return nil, errors.New("HomeHub identity audience must not be empty")
	}
	return &Verifier{publicKey: append(ed25519.PublicKey(nil), publicKey...), audience: audience, now: time.Now}, nil
}

func NewVerifierFromFile(path, audience string) (*Verifier, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read HomeHub identity public key: %w", err)
	}
	publicKey, err := ParsePublicKey(value)
	if err != nil {
		return nil, err
	}
	return NewVerifier(publicKey, audience)
}

func ParsePublicKey(value []byte) (ed25519.PublicKey, error) {
	trimmed := strings.TrimSpace(string(value))
	if block, _ := pem.Decode([]byte(trimmed)); block != nil {
		parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, errors.New("invalid HomeHub identity public key")
		}
		key, ok := parsed.(ed25519.PublicKey)
		if !ok || len(key) != ed25519.PublicKeySize {
			return nil, errors.New("HomeHub identity public key must be Ed25519")
		}
		return append(ed25519.PublicKey(nil), key...), nil
	}
	for _, encoding := range []*base64.Encoding{
		base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding,
	} {
		decoded, err := encoding.DecodeString(trimmed)
		if err == nil && len(decoded) == ed25519.PublicKeySize {
			return ed25519.PublicKey(decoded), nil
		}
	}
	return nil, errors.New("invalid HomeHub identity public key")
}

func (v *Verifier) Verify(token string) (Claims, error) {
	if v == nil || len(v.publicKey) != ed25519.PublicKeySize || len(token) == 0 || len(token) > maxToken {
		return Claims{}, errors.New("invalid identity token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid identity token")
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return Claims{}, errors.New("invalid identity token")
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "EdDSA" || header.Type != "JWT" {
		return Claims{}, errors.New("unsupported identity token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(v.publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return Claims{}, errors.New("invalid identity token signature")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid identity token")
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil {
		return Claims{}, errors.New("invalid identity token")
	}
	now := v.now().UTC()
	issuedAt := time.Unix(claims.IssuedAt, 0)
	expires := time.Unix(claims.Expires, 0)
	if claims.Issuer != Issuer || claims.Audience != v.audience || claims.Subject == "" ||
		!expires.After(now) || issuedAt.After(now.Add(clockSkew)) || expires.Before(issuedAt) || expires.Sub(issuedAt) > maxTTL {
		return Claims{}, errors.New("invalid identity token claims")
	}
	return claims, nil
}

type principalContextKey struct{}

func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(principalContextKey{}).(Claims)
	return claims, ok
}

func (v *Verifier) Authenticate(requiredAny []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		claims, err := v.Verify(request.Header.Get(HeaderName))
		if err != nil {
			writeError(writer, http.StatusUnauthorized, "invalid_identity")
			return
		}
		if len(requiredAny) > 0 && !claims.HasAnyScope(requiredAny...) {
			writeError(writer, http.StatusForbidden, "insufficient_scope")
			return
		}
		ctx := context.WithValue(request.Context(), principalContextKey{}, claims)
		next.ServeHTTP(writer, request.WithContext(ctx))
	})
}

func writeError(writer http.ResponseWriter, status int, code string) {
	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(map[string]string{"error": code})
}
