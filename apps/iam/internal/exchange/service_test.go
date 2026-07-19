package exchange

import (
	"context"
	"errors"
	"testing"

	"gitee.com/zlx23/homehub/apps/iam/internal/domain"
	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/apps/iam/internal/token"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

type fakeStore struct {
	identity storepostgres.MachineIdentity
	policy   storepostgres.AudiencePolicy
	audits   []string
}

func (store *fakeStore) AuthenticateAPIKey(context.Context, string) (storepostgres.MachineIdentity, error) {
	if store.identity.PrincipalID == "" {
		return storepostgres.MachineIdentity{}, errors.New("invalid")
	}
	return store.identity, nil
}

func (store *fakeStore) AudiencePolicy(context.Context, string) (storepostgres.AudiencePolicy, error) {
	if store.policy.Audience == "" {
		return storepostgres.AudiencePolicy{}, errors.New("unknown")
	}
	return store.policy, nil
}

func (store *fakeStore) RecordTokenAudit(_ context.Context, _ storepostgres.MachineIdentity, _ string, outcome, _ string, _ string, _ map[string]any) {
	store.audits = append(store.audits, outcome)
}

type fakeAuthorization struct {
	root    bool
	service bool
}

func (authorization fakeAuthorization) Check(_ context.Context, _ storepostgres.AuthorizationState, _ string, relation, _ string) (bool, error) {
	if relation == "root" {
		return authorization.root, nil
	}
	return authorization.service, nil
}

type fakeSigner struct{}

func (fakeSigner) Issue(request token.IssueRequest) (string, identity.Claims, error) {
	return "signed", identity.Claims{
		Audience: request.Audience, Permissions: request.Permissions, IssuedAt: 100, Expires: 220,
	}, nil
}

func machineStore() *fakeStore {
	return &fakeStore{
		identity: storepostgres.MachineIdentity{
			PrincipalID: "00000000-0000-0000-0000-000000000001", Kind: domain.PrincipalAgent,
			DisplayName: "Hermes", Realm: "homehub", CredentialID: "00000000-0000-0000-0000-000000000002",
		},
		policy: storepostgres.AudiencePolicy{
			Audience: "homehub-drop", ServiceID: "drop", MaxTokenTTLSeconds: 120,
			Permissions: map[string]string{"drop.item.create": "caller", "drop.item.delete": "manager"},
		},
	}
}

func TestRootAgentCanExchangeSystemRootToken(t *testing.T) {
	t.Parallel()
	store := machineStore()
	service := New(store, fakeAuthorization{root: true}, storepostgres.AuthorizationState{}, fakeSigner{})
	response, err := service.Exchange(context.Background(), "credential", "request-1", Request{
		Audience: "homehub-drop", Permissions: []string{"system.root", "drop.item.delete"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.AccessToken != "signed" || response.ExpiresIn != 120 || len(store.audits) != 1 || store.audits[0] != "success" {
		t.Fatalf("unexpected response=%+v audits=%v", response, store.audits)
	}
}

func TestNonRootAgentCannotRequestSystemRoot(t *testing.T) {
	t.Parallel()
	store := machineStore()
	service := New(store, fakeAuthorization{service: true}, storepostgres.AuthorizationState{}, fakeSigner{})
	_, err := service.Exchange(context.Background(), "credential", "request-1", Request{
		Audience: "homehub-drop", Permissions: []string{"system.root"},
	})
	if !errors.Is(err, ErrInsufficientPermission) || len(store.audits) != 1 || store.audits[0] != "denied" {
		t.Fatalf("error=%v audits=%v", err, store.audits)
	}
}

func TestUnknownConcretePermissionIsRejected(t *testing.T) {
	t.Parallel()
	store := machineStore()
	service := New(store, fakeAuthorization{root: true}, storepostgres.AuthorizationState{}, fakeSigner{})
	_, err := service.Exchange(context.Background(), "credential", "request-1", Request{
		Audience: "homehub-drop", Permissions: []string{"drop.item.rename"},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("error = %v", err)
	}
	if len(store.audits) != 1 || store.audits[0] != "denied" {
		t.Fatalf("audits = %v", store.audits)
	}
}
