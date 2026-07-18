package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type AuthorizationState struct {
	StoreID     string
	ModelID     string
	ModelSHA256 string
}

func (store *Store) GetAuthorizationState(ctx context.Context, realmSlug string) (AuthorizationState, bool, error) {
	var state AuthorizationState
	err := store.pool.QueryRow(ctx, `
		SELECT store_id, model_id, model_sha256
		FROM authorization_state
		WHERE realm_slug = $1`, realmSlug).Scan(&state.StoreID, &state.ModelID, &state.ModelSHA256)
	if err == pgx.ErrNoRows {
		return AuthorizationState{}, false, nil
	}
	if err != nil {
		return AuthorizationState{}, false, fmt.Errorf("read authorization state: %w", err)
	}
	return state, true, nil
}

func (store *Store) PutAuthorizationState(ctx context.Context, realmSlug string, state AuthorizationState) error {
	_, err := store.pool.Exec(ctx, `
		INSERT INTO authorization_state(realm_slug, store_id, model_id, model_sha256)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (realm_slug) DO UPDATE SET
			store_id = EXCLUDED.store_id,
			model_id = EXCLUDED.model_id,
			model_sha256 = EXCLUDED.model_sha256,
			updated_at = now()`, realmSlug, state.StoreID, state.ModelID, state.ModelSHA256)
	if err != nil {
		return fmt.Errorf("persist authorization state: %w", err)
	}
	return nil
}
