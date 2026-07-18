package identity

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"testing"
)

func TestParseJWKSet(t *testing.T) {
	t.Parallel()
	publicKey, _, _ := ed25519.GenerateKey(nil)
	document := fmt.Sprintf(`{"keys":[{"kty":"OKP","crv":"Ed25519","use":"sig","alg":"EdDSA","kid":"key-1","x":"%s"}]}`,
		base64.RawURLEncoding.EncodeToString(publicKey))
	keys, err := ParseJWKSet([]byte(document))
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 || len(keys["key-1"]) != ed25519.PublicKeySize {
		t.Fatalf("unexpected keys: %+v", keys)
	}
}
