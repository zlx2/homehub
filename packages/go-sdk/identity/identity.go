package identity

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	Issuer               = "homehub-iam"
	SystemRootPermission = "system.root"
	maxTokenBytes         = 8192
	clockSkew             = 15 * time.Second
)

var (
	principalID   = regexp.MustCompile(`^(human|guest|device|node|workload|agent):[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	permissionID  = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}$`)
	audienceID    = regexp.MustCompile(`^homehub-[a-z][a-z0-9-]{0,54}$`)
)

type Actor struct {
	Subject string `json:"sub"`
}

type Claims struct {
	Issuer           string   `json:"iss"`
	Audience         string   `json:"aud"`
	Subject          string   `json:"sub"`
	Actor            *Actor   `json:"act,omitempty"`
	AuthorizedParty  string   `json:"azp"`
	Realm            string   `json:"realm"`
	Permissions      []string `json:"permissions"`
	SessionID        string   `json:"sid,omitempty"`
	TokenID          string   `json:"jti"`
	DelegationID     string   `json:"delegation_id,omitempty"`
	Authentication   []string `json:"amr,omitempty"`
	AuthenticationAt int64    `json:"auth_time,omitempty"`
	IssuedAt         int64    `json:"iat"`
	NotBefore        int64    `json:"nbf,omitempty"`
	Expires          int64    `json:"exp"`
}

func (claims Claims) EffectiveActor() string {
	if claims.Actor != nil {
		return claims.Actor.Subject
	}
	return claims.Subject
}

func (claims Claims) HasPermission(expected string) bool {
	for _, permission := range claims.Permissions {
		if permission == expected {
			return true
		}
	}
	return false
}

func (claims Claims) Allows(expected string) bool {
	return claims.HasPermission(expected) || claims.HasPermission(SystemRootPermission)
}

type Verifier struct {
	keys     map[string]ed25519.PublicKey
	audience string
	maxTTL   time.Duration
	now      func() time.Time
}

func NewVerifier(keys map[string]ed25519.PublicKey, audience string, maxTTL time.Duration) (*Verifier, error) {
	if len(keys) == 0 || !audienceID.MatchString(audience) {
		return nil, errors.New("invalid HomeHub verifier configuration")
	}
	if maxTTL < 30*time.Second || maxTTL > 15*time.Minute {
		return nil, errors.New("invalid HomeHub token TTL ceiling")
	}
	keyCopy := make(map[string]ed25519.PublicKey, len(keys))
	for keyID, publicKey := range keys {
		if strings.TrimSpace(keyID) == "" || len(publicKey) != ed25519.PublicKeySize {
			return nil, errors.New("HomeHub verification keys must be named Ed25519 keys")
		}
		keyCopy[keyID] = append(ed25519.PublicKey(nil), publicKey...)
	}
	return &Verifier{keys: keyCopy, audience: audience, maxTTL: maxTTL, now: time.Now}, nil
}

func (verifier *Verifier) Verify(token string) (Claims, error) {
	if verifier == nil || len(token) == 0 || len(token) > maxTokenBytes {
		return Claims{}, errors.New("invalid access token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("invalid access token")
	}

	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
		KeyID     string `json:"kid"`
	}
	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || json.Unmarshal(headerBytes, &header) != nil || header.Algorithm != "EdDSA" || header.Type != "at+jwt" || header.KeyID == "" {
		return Claims{}, errors.New("unsupported access token")
	}
	publicKey, ok := verifier.keys[header.KeyID]
	if !ok {
		return Claims{}, errors.New("unknown access token key")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || len(signature) != ed25519.SignatureSize || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return Claims{}, errors.New("invalid access token signature")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, errors.New("invalid access token")
	}
	var claims Claims
	if json.Unmarshal(payload, &claims) != nil || !claims.valid(verifier.audience, verifier.maxTTL, verifier.now().UTC()) {
		return Claims{}, errors.New("invalid access token claims")
	}
	return claims, nil
}

func (claims Claims) valid(audience string, maxTTL time.Duration, now time.Time) bool {
	issuedAt := time.Unix(claims.IssuedAt, 0)
	notBefore := time.Unix(claims.NotBefore, 0)
	expires := time.Unix(claims.Expires, 0)
	if claims.NotBefore == 0 {
		notBefore = issuedAt
	}
	if claims.Issuer != Issuer || claims.Audience != audience || claims.Realm == "" ||
		!principalID.MatchString(claims.Subject) || claims.AuthorizedParty == "" || claims.TokenID == "" ||
		len(claims.Permissions) == 0 || !expires.After(now) || issuedAt.After(now.Add(clockSkew)) ||
		notBefore.After(now.Add(clockSkew)) || expires.Before(issuedAt) || expires.Sub(issuedAt) > maxTTL {
		return false
	}
	if claims.Actor != nil && !principalID.MatchString(claims.Actor.Subject) {
		return false
	}
	for _, permission := range claims.Permissions {
		if permission != SystemRootPermission && !permissionID.MatchString(permission) {
			return false
		}
	}
	return true
}

func BearerToken(request *http.Request) (string, error) {
	value := strings.TrimSpace(request.Header.Get("Authorization"))
	scheme, token, ok := strings.Cut(value, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" || strings.ContainsAny(token, " \t\r\n") {
		return "", errors.New("missing bearer token")
	}
	return token, nil
}

type principalContextKey struct{}

func FromContext(ctx context.Context) (Claims, bool) {
	claims, ok := ctx.Value(principalContextKey{}).(Claims)
	return claims, ok
}

func (verifier *Verifier) Authenticate(requiredAny []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token, err := BearerToken(request)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "invalid_token")
			return
		}
		claims, err := verifier.Verify(token)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "invalid_token")
			return
		}
		if len(requiredAny) > 0 {
			allowed := false
			for _, permission := range requiredAny {
				if claims.Allows(permission) {
					allowed = true
					break
				}
			}
			if !allowed {
				writeError(response, http.StatusForbidden, "insufficient_permission")
				return
			}
		}
		ctx := context.WithValue(request.Context(), principalContextKey{}, claims)
		next.ServeHTTP(response, request.WithContext(ctx))
	})
}

func writeError(response http.ResponseWriter, status int, code string) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(map[string]string{"error": code})
}
