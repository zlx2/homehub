package token

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gitee.com/zlx23/homehub/apps/iam/internal/domain"
	"homehub.local/go-sdk/identity"
)

var (
	audienceName = regexp.MustCompile(`^homehub-[a-z][a-z0-9-]{0,54}$`)
	partyName    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

type Signer struct {
	keyID string
	key   ed25519.PrivateKey
	ttl   time.Duration
	now   func() time.Time
}

type IssueRequest struct {
	Audience         string
	Subject          string
	Actor            string
	AuthorizedParty  string
	Realm            string
	Permissions      []string
	SessionID        string
	DelegationID     string
	Authentication   []string
	AuthenticationAt time.Time
}

func NewSigner(keyID string, key ed25519.PrivateKey, ttl time.Duration) (*Signer, error) {
	if !partyName.MatchString(keyID) || len(key) != ed25519.PrivateKeySize || ttl < 30*time.Second || ttl > 15*time.Minute {
		return nil, errors.New("invalid access token signer configuration")
	}
	return &Signer{keyID: keyID, key: append(ed25519.PrivateKey(nil), key...), ttl: ttl, now: time.Now}, nil
}

func NewSignerFromFile(keyID, path string, ttl time.Duration) (*Signer, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read IAM signing key: %w", err)
	}
	key, err := parsePrivateKey(contents)
	if err != nil {
		return nil, err
	}
	return NewSigner(keyID, key, ttl)
}

func (signer *Signer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), signer.key.Public().(ed25519.PublicKey)...)
}

func (signer *Signer) JWKSet() map[string]any {
	return map[string]any{"keys": []map[string]string{{
		"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA",
		"kid": signer.keyID, "x": base64.RawURLEncoding.EncodeToString(signer.PublicKey()),
	}}}
}

func (signer *Signer) Issue(request IssueRequest) (string, identity.Claims, error) {
	if signer == nil || !audienceName.MatchString(request.Audience) || !partyName.MatchString(request.AuthorizedParty) ||
		!partyName.MatchString(request.Realm) || len(request.Permissions) == 0 {
		return "", identity.Claims{}, errors.New("invalid access token request")
	}
	if _, err := domain.ParsePrincipalID(request.Subject); err != nil {
		return "", identity.Claims{}, errors.New("invalid access token subject")
	}
	if request.Actor != "" {
		if _, err := domain.ParsePrincipalID(request.Actor); err != nil {
			return "", identity.Claims{}, errors.New("invalid access token actor")
		}
	}
	permissions := make([]string, 0, len(request.Permissions))
	seen := make(map[string]struct{}, len(request.Permissions))
	for _, value := range request.Permissions {
		if _, err := domain.ParsePermission(value); err != nil {
			return "", identity.Claims{}, err
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		permissions = append(permissions, value)
	}

	now := signer.now().UTC()
	claims := identity.Claims{
		Issuer: identity.Issuer, Audience: request.Audience, Subject: request.Subject,
		AuthorizedParty: request.AuthorizedParty, Realm: request.Realm, Permissions: permissions,
		SessionID: request.SessionID, TokenID: randomID(), DelegationID: request.DelegationID,
		Authentication: append([]string(nil), request.Authentication...),
		IssuedAt:       now.Unix(), NotBefore: now.Add(-5 * time.Second).Unix(), Expires: now.Add(signer.ttl).Unix(),
	}
	if request.Actor != "" && request.Actor != request.Subject {
		claims.Actor = &identity.Actor{Subject: request.Actor}
	}
	if !request.AuthenticationAt.IsZero() {
		claims.AuthenticationAt = request.AuthenticationAt.UTC().Unix()
	}

	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "at+jwt", "kid": signer.keyID})
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", identity.Claims{}, fmt.Errorf("encode access token claims: %w", err)
	}
	unsigned := encode(header) + "." + encode(payload)
	return unsigned + "." + encode(ed25519.Sign(signer.key, []byte(unsigned))), claims, nil
}

func parsePrivateKey(contents []byte) (ed25519.PrivateKey, error) {
	trimmed := strings.TrimSpace(string(contents))
	if block, _ := pem.Decode([]byte(trimmed)); block != nil {
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, errors.New("invalid IAM signing key")
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok || len(key) != ed25519.PrivateKeySize {
			return nil, errors.New("IAM signing key must be Ed25519")
		}
		return append(ed25519.PrivateKey(nil), key...), nil
	}
	for _, encoding := range []*base64.Encoding{base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding} {
		decoded, err := encoding.DecodeString(trimmed)
		if err != nil {
			continue
		}
		switch len(decoded) {
		case ed25519.SeedSize:
			return ed25519.NewKeyFromSeed(decoded), nil
		case ed25519.PrivateKeySize:
			return ed25519.PrivateKey(append([]byte(nil), decoded...)), nil
		}
	}
	return nil, errors.New("invalid IAM signing key")
}

func randomID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(value[:])
}

func encode(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
