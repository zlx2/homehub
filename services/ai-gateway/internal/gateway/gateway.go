package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"gitee.com/zlx23/homehub/services/ai-gateway/internal/config"
)

var (
	ErrInvalidRequest = errors.New("invalid chat request")
	ErrModelForbidden = errors.New("model is not allowed by delegation")
	ErrModelNotFound  = errors.New("model alias was not found")
)

type Router struct {
	models    map[string]modelRoute
	modelList []Model
	providers map[string]provider
	client    *http.Client
}

type provider struct {
	id       string
	endpoint string
	apiKey   string
}

type modelRoute struct {
	Model
	upstreamModel string
	providerID    string
}

type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	OwnedBy     string `json:"owned_by"`
	Description string `json:"description,omitempty"`
}

type Result struct {
	Response *http.Response
	Alias    string
	Provider string
	Stream   bool
}

func New(cfg config.Config) (*Router, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	providers := make(map[string]provider, len(cfg.Providers))
	for _, item := range cfg.Providers {
		value, err := os.ReadFile(item.APIKeyFile)
		if err != nil {
			return nil, fmt.Errorf("read provider %q API key: %w", item.ID, err)
		}
		key := strings.TrimSpace(string(value))
		if key == "" {
			return nil, fmt.Errorf("provider %q API key is empty", item.ID)
		}
		providers[item.ID] = provider{
			id: item.ID, endpoint: strings.TrimRight(item.BaseURL, "/") + "/chat/completions", apiKey: key,
		}
	}
	models := make(map[string]modelRoute, len(cfg.Models))
	list := make([]Model, 0, len(cfg.Models))
	for _, item := range cfg.Models {
		public := Model{ID: item.ID, Object: "model", OwnedBy: "homehub", Description: item.Description}
		models[item.ID] = modelRoute{Model: public, upstreamModel: item.UpstreamModel, providerID: item.Provider}
		list = append(list, public)
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = http.ProxyFromEnvironment
	transport.DialContext = (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.ResponseHeaderTimeout = 2 * time.Minute
	transport.IdleConnTimeout = 90 * time.Second
	transport.MaxIdleConns = 100
	transport.MaxIdleConnsPerHost = 20
	return &Router{
		models: models, modelList: list, providers: providers,
		client: &http.Client{Transport: transport, Timeout: 30 * time.Minute},
	}, nil
}

func (router *Router) SetHTTPClient(client *http.Client) {
	if client != nil {
		router.client = client
	}
}

func (router *Router) Models(allowed []string) []Model {
	policy := stringSet(allowed)
	result := make([]Model, 0, len(router.modelList))
	for _, model := range router.modelList {
		if _, ok := policy[model.ID]; ok {
			result = append(result, model)
		}
	}
	return result
}

func (router *Router) ModelIDs() []string {
	result := make([]string, 0, len(router.modelList))
	for _, model := range router.modelList {
		result = append(result, model.ID)
	}
	return result
}

func (router *Router) Complete(ctx context.Context, body []byte, allowed []string, requestID string) (*Result, error) {
	alias, stream, rewritten, err := router.rewrite(body, allowed)
	if err != nil {
		return nil, err
	}
	route := router.models[alias]
	upstream := router.providers[route.providerID]
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, upstream.endpoint, bytes.NewReader(rewritten))
	if err != nil {
		return nil, fmt.Errorf("create provider request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+upstream.apiKey)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "HomeHub-AI-Gateway/0.1")
	if stream {
		request.Header.Set("Accept", "text/event-stream")
	}
	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}
	response, err := router.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("call provider %q: %w", upstream.id, err)
	}
	return &Result{Response: response, Alias: route.ID, Provider: upstream.id, Stream: stream}, nil
}

func (router *Router) rewrite(body []byte, allowed []string) (string, bool, []byte, error) {
	var envelope map[string]json.RawMessage
	if len(body) == 0 || json.Unmarshal(body, &envelope) != nil || envelope == nil {
		return "", false, nil, ErrInvalidRequest
	}
	var alias string
	if json.Unmarshal(envelope["model"], &alias) != nil || strings.TrimSpace(alias) == "" {
		return "", false, nil, ErrInvalidRequest
	}
	var messages []json.RawMessage
	if json.Unmarshal(envelope["messages"], &messages) != nil || len(messages) == 0 {
		return "", false, nil, ErrInvalidRequest
	}
	if _, ok := stringSet(allowed)[alias]; !ok {
		return "", false, nil, ErrModelForbidden
	}
	route, ok := router.models[alias]
	if !ok {
		return "", false, nil, ErrModelNotFound
	}
	stream := false
	if value, exists := envelope["stream"]; exists && json.Unmarshal(value, &stream) != nil {
		return "", false, nil, ErrInvalidRequest
	}
	envelope["model"], _ = json.Marshal(route.upstreamModel)
	rewritten, err := json.Marshal(envelope)
	if err != nil {
		return "", false, nil, ErrInvalidRequest
	}
	return alias, stream, rewritten, nil
}

func stringSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

func DrainAndClose(response *http.Response) {
	if response == nil || response.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	_ = response.Body.Close()
}
