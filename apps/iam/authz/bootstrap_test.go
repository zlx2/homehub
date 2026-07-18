package authz

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
)

type memoryState struct {
	state storepostgres.AuthorizationState
	found bool
}

func (state *memoryState) GetAuthorizationState(context.Context, string) (storepostgres.AuthorizationState, bool, error) {
	return state.state, state.found, nil
}

func (state *memoryState) PutAuthorizationState(_ context.Context, _ string, value storepostgres.AuthorizationState) error {
	state.state, state.found = value, true
	return nil
}

func TestEnsureModelCreatesStoreAndWritesModel(t *testing.T) {
	t.Parallel()
	writes := 0
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodGet && request.URL.Path == "/stores":
			_ = json.NewEncoder(response).Encode(map[string]any{"stores": []any{}})
		case request.Method == http.MethodPost && request.URL.Path == "/stores":
			_ = json.NewEncoder(response).Encode(map[string]string{"id": "store-1"})
		case request.Method == http.MethodPost && request.URL.Path == "/stores/store-1/authorization-models":
			writes++
			_ = json.NewEncoder(response).Encode(map[string]string{"authorization_model_id": "model-1"})
		default:
			http.NotFound(response, request)
		}
	}))
	defer server.Close()

	client, err := NewClient(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	states := &memoryState{}
	state, err := client.EnsureModel(context.Background(), states, "homehub")
	if err != nil {
		t.Fatal(err)
	}
	if state.StoreID != "store-1" || state.ModelID != "model-1" || state.ModelSHA256 == "" || writes != 1 {
		t.Fatalf("unexpected authorization state: %+v, writes=%d", state, writes)
	}
}

func TestEnsureModelReusesMatchingLiveModel(t *testing.T) {
	t.Parallel()
	digest := sha256.Sum256(modelJSON)
	states := &memoryState{found: true, state: storepostgres.AuthorizationState{
		StoreID: "store-1", ModelID: "model-1", ModelSHA256: hex.EncodeToString(digest[:]),
	}}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodGet && request.URL.Path == "/stores/store-1/authorization-models/model-1" {
			_ = json.NewEncoder(response).Encode(map[string]any{"schema_version": "1.1"})
			return
		}
		t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
	}))
	defer server.Close()

	client, _ := NewClient(server.URL)
	if _, err := client.EnsureModel(context.Background(), states, "homehub"); err != nil {
		t.Fatal(err)
	}
}
