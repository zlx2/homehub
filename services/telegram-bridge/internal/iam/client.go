package iam

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	dropAudience     = "homehub-drop"
	createPermission = "drop.item.create"
)

type TokenSource interface {
	Token(context.Context) (string, error)
}

type Client struct {
	baseURL    string
	credential string
	http       *http.Client
	now        func() time.Time

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type exchangeResponse struct {
	AccessToken string   `json:"access_token"`
	ExpiresIn   int      `json:"expires_in"`
	Audience    string   `json:"audience"`
	Permissions []string `json:"permissions"`
}

func NewClient(baseURL, credential string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"), credential: credential,
		http: &http.Client{Timeout: timeout, Transport: &http.Transport{Proxy: nil}}, now: time.Now,
	}
}

func (client *Client) Token(ctx context.Context) (string, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.token != "" && client.now().Add(30*time.Second).Before(client.expiresAt) {
		return client.token, nil
	}
	payload, err := json.Marshal(map[string]any{
		"audience": dropAudience, "permissions": []string{createPermission},
	})
	if err != nil {
		return "", fmt.Errorf("encode IAM token request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+"/v1/tokens/exchange", bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build IAM token request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+client.credential)
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return "", fmt.Errorf("exchange HomeHub access token: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	if err != nil {
		return "", fmt.Errorf("read IAM token response: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IAM rejected machine credential (HTTP %d)", response.StatusCode)
	}
	var result exchangeResponse
	if err := json.Unmarshal(body, &result); err != nil || result.AccessToken == "" || result.ExpiresIn < 1 ||
		result.Audience != dropAudience || !contains(result.Permissions, createPermission) {
		return "", fmt.Errorf("IAM returned an invalid access token response")
	}
	client.token = result.AccessToken
	client.expiresAt = client.now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return client.token, nil
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
