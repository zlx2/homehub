package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
)

func ParseJWKSet(contents []byte) (map[string]ed25519.PublicKey, error) {
	var document struct {
		Keys []struct {
			KeyType   string `json:"kty"`
			Curve     string `json:"crv"`
			Use       string `json:"use"`
			Algorithm string `json:"alg"`
			KeyID     string `json:"kid"`
			X         string `json:"x"`
		} `json:"keys"`
	}
	if json.Unmarshal(contents, &document) != nil || len(document.Keys) == 0 {
		return nil, errors.New("invalid HomeHub JWKS")
	}
	keys := make(map[string]ed25519.PublicKey, len(document.Keys))
	for _, key := range document.Keys {
		if key.KeyType != "OKP" || key.Curve != "Ed25519" || key.Use != "sig" ||
			key.Algorithm != "EdDSA" || key.KeyID == "" || key.X == "" {
			return nil, errors.New("unsupported HomeHub JWK")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil || len(decoded) != ed25519.PublicKeySize {
			return nil, errors.New("invalid HomeHub JWK")
		}
		if _, duplicate := keys[key.KeyID]; duplicate {
			return nil, errors.New("duplicate HomeHub JWK key ID")
		}
		keys[key.KeyID] = ed25519.PublicKey(decoded)
	}
	return keys, nil
}
