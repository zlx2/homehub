package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"gitee.com/zlx23/homehub/apps/iam/authz"
	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
)

func EnsureSystemAgent(ctx context.Context, store *storepostgres.Store, authorization *authz.Client, state storepostgres.AuthorizationState, credentialFile string) (storepostgres.MachineIdentity, error) {
	contents, err := os.ReadFile(credentialFile)
	if err != nil {
		return storepostgres.MachineIdentity{}, fmt.Errorf("read system agent credential: %w", err)
	}
	credential := strings.TrimSpace(string(contents))
	if credential == "" {
		return storepostgres.MachineIdentity{}, errors.New("system agent credential is empty")
	}
	identity, err := store.EnsureSystemAgent(ctx, "homehub", "hermes", "Hermes", credential)
	if err != nil {
		return storepostgres.MachineIdentity{}, err
	}
	if err := authorization.WriteRelationship(ctx, state, identity.Subject(), "root", "realm:homehub"); err != nil {
		return storepostgres.MachineIdentity{}, fmt.Errorf("grant system agent root relationship: %w", err)
	}
	return identity, nil
}
