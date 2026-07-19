package machineadmin

import (
	"context"
	"errors"
	"testing"

	"gitee.com/zlx23/homehub/apps/iam/internal/domain"
	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

type fakeStore struct {
	relations map[string]bool
	created   storepostgres.MachineIdentity
	deleted   bool
}

func (store *fakeStore) CreateMachineIdentity(context.Context, string, domain.PrincipalKind, string, string, string) (storepostgres.MachineIdentity, error) {
	return store.created, nil
}
func (store *fakeStore) DeleteMachineIdentity(context.Context, string) error {
	store.deleted = true
	return nil
}
func (store *fakeStore) ServiceRelationExists(_ context.Context, serviceID, relation string) (bool, error) {
	return store.relations[serviceID+":"+relation], nil
}
func (*fakeStore) RecordMachineAdminAudit(context.Context, string, string, string, string, string, map[string]any) {
}

type fakeAuthorization struct{ fail bool }

func (writer fakeAuthorization) WriteRelationship(context.Context, storepostgres.AuthorizationState, string, string, string) error {
	if writer.fail {
		return errors.New("unavailable")
	}
	return nil
}
func (fakeAuthorization) DeleteRelationship(context.Context, storepostgres.AuthorizationState, string, string, string) error {
	return nil
}

func TestCreateWorkloadWithBoundedGrant(t *testing.T) {
	t.Parallel()
	store := &fakeStore{
		relations: map[string]bool{"drop:caller": true},
		created:   storepostgres.MachineIdentity{PrincipalID: "00000000-0000-0000-0000-000000000010", Kind: domain.PrincipalWorkload, DisplayName: "Telegram", Realm: "homehub"},
	}
	service := New(store, fakeAuthorization{}, storepostgres.AuthorizationState{})
	response, err := service.Create(context.Background(), identity.Claims{Subject: "agent:00000000-0000-0000-0000-000000000001", Realm: "homehub"}, "request-1", CreateRequest{
		Kind: domain.PrincipalWorkload, DisplayName: "Telegram", ExternalSubject: "telegram-bridge", Grants: []Grant{{ServiceID: "drop", Relation: "caller"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Subject == "" || len(response.Credential) < 40 || response.Grants[0].Relation != "caller" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestCreateRejectsHumanAndUnknownRelation(t *testing.T) {
	t.Parallel()
	service := New(&fakeStore{relations: map[string]bool{}}, fakeAuthorization{}, storepostgres.AuthorizationState{})
	for _, request := range []CreateRequest{
		{Kind: domain.PrincipalHuman, DisplayName: "Human", ExternalSubject: "human-machine", Grants: []Grant{{ServiceID: "drop", Relation: "caller"}}},
		{Kind: domain.PrincipalWorkload, DisplayName: "Worker", ExternalSubject: "worker-one", Grants: []Grant{{ServiceID: "drop", Relation: "administrator"}}},
	} {
		if _, err := service.Create(context.Background(), identity.Claims{Realm: "homehub"}, "request", request); !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestCreateCompensatesWhenGrantWriteFails(t *testing.T) {
	t.Parallel()
	store := &fakeStore{relations: map[string]bool{"drop:caller": true}, created: storepostgres.MachineIdentity{PrincipalID: "id", Kind: domain.PrincipalWorkload}}
	service := New(store, fakeAuthorization{fail: true}, storepostgres.AuthorizationState{})
	_, err := service.Create(context.Background(), identity.Claims{Realm: "homehub"}, "request", CreateRequest{
		Kind: domain.PrincipalWorkload, DisplayName: "Worker", ExternalSubject: "worker-one", Grants: []Grant{{ServiceID: "drop", Relation: "caller"}},
	})
	if err == nil || !store.deleted {
		t.Fatalf("error=%v deleted=%v", err, store.deleted)
	}
}
