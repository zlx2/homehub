package identity

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxJWKSetBytes = 64 << 10

// FetchJWKSet loads a bounded HomeHub JWKS document from IAM. Callers should
// use an HTTP client with an explicit timeout and no unintended proxy.
func FetchJWKSet(ctx context.Context, client *http.Client, endpoint string) (map[string]ed25519.PublicKey, error) {
	if client == nil {
		return nil, errors.New("HomeHub JWKS HTTP client is required")
	}
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("invalid HomeHub JWKS URL")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("build HomeHub JWKS request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("fetch HomeHub JWKS: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch HomeHub JWKS: unexpected status %d", response.StatusCode)
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, maxJWKSetBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read HomeHub JWKS: %w", err)
	}
	if len(contents) > maxJWKSetBytes {
		return nil, errors.New("HomeHub JWKS exceeds size limit")
	}
	keys, err := ParseJWKSet(contents)
	if err != nil {
		return nil, err
	}
	result := make(map[string]ed25519.PublicKey, len(keys))
	for keyID, key := range keys {
		result[keyID] = append(ed25519.PublicKey(nil), key...)
	}
	return result, nil
}
