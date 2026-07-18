package authz

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
)

//go:embed homehub.json
var modelJSON []byte

type StateStore interface {
	GetAuthorizationState(context.Context, string) (storepostgres.AuthorizationState, bool, error)
	PutAuthorizationState(context.Context, string, storepostgres.AuthorizationState) error
}

type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(rawURL string) (*Client, error) {
	parsed, err := url.Parse(strings.TrimRight(rawURL, "/"))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return nil, errors.New("invalid OpenFGA URL")
	}
	return &Client{
		baseURL: parsed.String(),
		http: &http.Client{
			Timeout:   5 * time.Second,
			Transport: &http.Transport{Proxy: nil},
		},
	}, nil
}

func (client *Client) Ping(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+"/healthz", nil)
	if err != nil {
		return err
	}
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("ping OpenFGA: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("ping OpenFGA: status %d", response.StatusCode)
	}
	return nil
}

func (client *Client) EnsureModel(ctx context.Context, states StateStore, realmSlug string) (storepostgres.AuthorizationState, error) {
	digest := sha256.Sum256(modelJSON)
	modelHash := hex.EncodeToString(digest[:])
	state, found, err := states.GetAuthorizationState(ctx, realmSlug)
	if err != nil {
		return storepostgres.AuthorizationState{}, err
	}
	if found && state.ModelSHA256 == modelHash && client.modelExists(ctx, state.StoreID, state.ModelID) {
		return state, nil
	}

	storeID, err := client.findOrCreateStore(ctx, realmSlug)
	if err != nil {
		return storepostgres.AuthorizationState{}, err
	}
	modelID, err := client.writeModel(ctx, storeID)
	if err != nil {
		return storepostgres.AuthorizationState{}, err
	}
	state = storepostgres.AuthorizationState{StoreID: storeID, ModelID: modelID, ModelSHA256: modelHash}
	if err := states.PutAuthorizationState(ctx, realmSlug, state); err != nil {
		return storepostgres.AuthorizationState{}, err
	}
	return state, nil
}

func (client *Client) WriteRelationship(ctx context.Context, state storepostgres.AuthorizationState, user, relation, object string) error {
	input := map[string]any{
		"authorization_model_id": state.ModelID,
		"writes": map[string]any{"tuple_keys": []map[string]string{{
			"user": user, "relation": relation, "object": object,
		}}},
	}
	path := "/stores/" + url.PathEscape(state.StoreID) + "/write"
	if err := client.doJSON(ctx, http.MethodPost, path, input, nil); err != nil {
		// OpenFGA treats duplicate relationship writes as an error. A positive
		// check makes startup reconciliation idempotent without deleting tuples.
		allowed, checkErr := client.Check(ctx, state, user, relation, object)
		if checkErr == nil && allowed {
			return nil
		}
		return err
	}
	return nil
}

func (client *Client) DeleteRelationship(ctx context.Context, state storepostgres.AuthorizationState, user, relation, object string) error {
	input := map[string]any{
		"authorization_model_id": state.ModelID,
		"deletes": map[string]any{"tuple_keys": []map[string]string{{
			"user": user, "relation": relation, "object": object,
		}}},
	}
	path := "/stores/" + url.PathEscape(state.StoreID) + "/write"
	if err := client.doJSON(ctx, http.MethodPost, path, input, nil); err != nil {
		allowed, checkErr := client.Check(ctx, state, user, relation, object)
		if checkErr == nil && !allowed {
			return nil
		}
		return err
	}
	return nil
}

func (client *Client) Check(ctx context.Context, state storepostgres.AuthorizationState, user, relation, object string) (bool, error) {
	input := map[string]any{
		"authorization_model_id": state.ModelID,
		"tuple_key":              map[string]string{"user": user, "relation": relation, "object": object},
	}
	var result struct {
		Allowed bool `json:"allowed"`
	}
	path := "/stores/" + url.PathEscape(state.StoreID) + "/check"
	if err := client.doJSON(ctx, http.MethodPost, path, input, &result); err != nil {
		return false, err
	}
	return result.Allowed, nil
}

func (client *Client) modelExists(ctx context.Context, storeID, modelID string) bool {
	if storeID == "" || modelID == "" {
		return false
	}
	path := "/stores/" + url.PathEscape(storeID) + "/authorization-models/" + url.PathEscape(modelID)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, client.baseURL+path, nil)
	if err != nil {
		return false
	}
	response, err := client.http.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func (client *Client) findOrCreateStore(ctx context.Context, name string) (string, error) {
	var list struct {
		Stores []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"stores"`
	}
	if err := client.doJSON(ctx, http.MethodGet, "/stores?page_size=100", nil, &list); err != nil {
		return "", err
	}
	for _, store := range list.Stores {
		if store.Name == name {
			return store.ID, nil
		}
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := client.doJSON(ctx, http.MethodPost, "/stores", map[string]string{"name": name}, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", errors.New("OpenFGA returned an empty store ID")
	}
	return created.ID, nil
}

func (client *Client) writeModel(ctx context.Context, storeID string) (string, error) {
	var model any
	if err := json.Unmarshal(modelJSON, &model); err != nil {
		return "", fmt.Errorf("decode embedded authorization model: %w", err)
	}
	var created struct {
		ModelID string `json:"authorization_model_id"`
	}
	path := "/stores/" + url.PathEscape(storeID) + "/authorization-models"
	if err := client.doJSON(ctx, http.MethodPost, path, model, &created); err != nil {
		return "", err
	}
	if created.ModelID == "" {
		return "", errors.New("OpenFGA returned an empty model ID")
	}
	return created.ModelID, nil
}

func (client *Client) doJSON(ctx context.Context, method, path string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	request, err := http.NewRequestWithContext(ctx, method, client.baseURL+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := client.http.Do(request)
	if err != nil {
		return fmt.Errorf("OpenFGA %s %s: %w", method, path, err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("OpenFGA %s %s: status %d", method, path, response.StatusCode)
	}
	if output == nil {
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(output); err != nil {
		return fmt.Errorf("decode OpenFGA response: %w", err)
	}
	return nil
}
