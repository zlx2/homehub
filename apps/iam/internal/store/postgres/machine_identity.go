package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"gitee.com/zlx23/homehub/apps/iam/internal/domain"
	"github.com/jackc/pgx/v5"
)

type MachineIdentity struct {
	PrincipalID  string
	Kind         domain.PrincipalKind
	DisplayName  string
	Realm        string
	CredentialID string
}

type AudiencePolicy struct {
	Audience           string
	ServiceID          string
	MaxTokenTTLSeconds int
	Permissions        map[string]string
}

func HashCredential(value string) [sha256.Size]byte {
	return sha256.Sum256([]byte(value))
}

func (store *Store) EnsureSystemAgent(ctx context.Context, realmSlug, externalSubject, displayName, credential string) (MachineIdentity, error) {
	if len(credential) < 32 {
		return MachineIdentity{}, errors.New("machine credential must contain at least 32 characters")
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return MachineIdentity{}, fmt.Errorf("begin system agent bootstrap: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()

	var identity MachineIdentity
	err = transaction.QueryRow(ctx, `
		SELECT p.id::text, p.kind, p.display_name, r.slug
		FROM external_accounts e
		JOIN principals p ON p.id = e.principal_id
		JOIN realms r ON r.id = p.realm_id
		WHERE e.provider = 'homehub-system' AND e.external_subject = $1`, externalSubject).
		Scan(&identity.PrincipalID, &identity.Kind, &identity.DisplayName, &identity.Realm)
	if err == pgx.ErrNoRows {
		err = transaction.QueryRow(ctx, `
			INSERT INTO principals(realm_id, kind, display_name, status)
			SELECT id, 'agent', $2, 'active' FROM realms WHERE slug = $1
			RETURNING id::text, kind, display_name`, realmSlug, displayName).
			Scan(&identity.PrincipalID, &identity.Kind, &identity.DisplayName)
		if err != nil {
			return MachineIdentity{}, fmt.Errorf("create system agent: %w", err)
		}
		identity.Realm = realmSlug
		if _, err := transaction.Exec(ctx, `
			INSERT INTO external_accounts(provider, external_subject, principal_id)
			VALUES ('homehub-system', $1, $2::uuid)`, externalSubject, identity.PrincipalID); err != nil {
			return MachineIdentity{}, fmt.Errorf("link system agent identity: %w", err)
		}
	} else if err != nil {
		return MachineIdentity{}, fmt.Errorf("find system agent: %w", err)
	}

	hash := HashCredential(credential)
	err = transaction.QueryRow(ctx, `
		SELECT id::text FROM credentials
		WHERE principal_id = $1::uuid AND kind = 'api_key' AND label = 'system-bootstrap' AND revoked_at IS NULL`, identity.PrincipalID).
		Scan(&identity.CredentialID)
	if err == pgx.ErrNoRows {
		err = transaction.QueryRow(ctx, `
			INSERT INTO credentials(principal_id, kind, label, secret_hash)
			VALUES ($1::uuid, 'api_key', 'system-bootstrap', $2)
			RETURNING id::text`, identity.PrincipalID, hash[:]).Scan(&identity.CredentialID)
	} else if err == nil {
		_, err = transaction.Exec(ctx, `
			UPDATE credentials SET secret_hash = $2, status = 'active', revoked_at = NULL
			WHERE id = $1::uuid`, identity.CredentialID, hash[:])
	}
	if err != nil {
		return MachineIdentity{}, fmt.Errorf("upsert system agent credential: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return MachineIdentity{}, fmt.Errorf("commit system agent bootstrap: %w", err)
	}
	return identity, nil
}

func (store *Store) AuthenticateAPIKey(ctx context.Context, credential string) (MachineIdentity, error) {
	hash := HashCredential(credential)
	var identity MachineIdentity
	err := store.pool.QueryRow(ctx, `
		UPDATE credentials c SET last_used_at = now()
		FROM principals p, realms r
		WHERE c.secret_hash = $1 AND c.kind = 'api_key' AND c.status = 'active' AND c.revoked_at IS NULL
		  AND (c.expires_at IS NULL OR c.expires_at > now())
		  AND p.id = c.principal_id AND p.status = 'active' AND p.deleted_at IS NULL
		  AND r.id = p.realm_id AND r.status = 'active'
		RETURNING p.id::text, p.kind, p.display_name, r.slug, c.id::text`, hash[:]).
		Scan(&identity.PrincipalID, &identity.Kind, &identity.DisplayName, &identity.Realm, &identity.CredentialID)
	if err == pgx.ErrNoRows {
		return MachineIdentity{}, errors.New("invalid machine credential")
	}
	if err != nil {
		return MachineIdentity{}, fmt.Errorf("authenticate machine credential: %w", err)
	}
	return identity, nil
}

func (identity MachineIdentity) Subject() string {
	return string(identity.Kind) + ":" + identity.PrincipalID
}

func (store *Store) SyncManifest(ctx context.Context, manifest domain.ServiceManifest) error {
	if err := manifest.Validate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		return err
	}
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin service manifest sync: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `
		INSERT INTO service_audiences(audience, service_id, manifest_version, max_token_ttl_seconds, manifest)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (audience) DO UPDATE SET
			service_id = EXCLUDED.service_id,
			manifest_version = EXCLUDED.manifest_version,
			max_token_ttl_seconds = EXCLUDED.max_token_ttl_seconds,
			manifest = EXCLUDED.manifest,
			enabled = true,
			updated_at = now()`, manifest.Audience, manifest.ServiceID, manifest.Version, manifest.MaxTokenTTLSeconds, encoded); err != nil {
		return fmt.Errorf("sync service audience: %w", err)
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM permissions WHERE audience = $1`, manifest.Audience); err != nil {
		return fmt.Errorf("replace service permissions: %w", err)
	}
	for _, permission := range manifest.Permissions {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO permissions(name, audience, description, risk, required_relation)
			VALUES ($1, $2, $3, $4, $5)`, permission.Name, manifest.Audience, permission.Description, permission.Risk, permission.RequiredRelation); err != nil {
			return fmt.Errorf("sync service permission %q: %w", permission.Name, err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit service manifest sync: %w", err)
	}
	return nil
}

func (store *Store) AudiencePolicy(ctx context.Context, audience string) (AudiencePolicy, error) {
	var policy AudiencePolicy
	err := store.pool.QueryRow(ctx, `
		SELECT audience, service_id, max_token_ttl_seconds
		FROM service_audiences WHERE audience = $1 AND enabled`, audience).
		Scan(&policy.Audience, &policy.ServiceID, &policy.MaxTokenTTLSeconds)
	if err == pgx.ErrNoRows {
		return AudiencePolicy{}, errors.New("unknown token audience")
	}
	if err != nil {
		return AudiencePolicy{}, fmt.Errorf("read audience policy: %w", err)
	}
	policy.Permissions = make(map[string]string)
	rows, err := store.pool.Query(ctx, `SELECT name, required_relation FROM permissions WHERE audience = $1 AND deprecated_at IS NULL`, audience)
	if err != nil {
		return AudiencePolicy{}, fmt.Errorf("read audience permissions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, relation string
		if err := rows.Scan(&name, &relation); err != nil {
			return AudiencePolicy{}, fmt.Errorf("scan audience permission: %w", err)
		}
		policy.Permissions[name] = relation
	}
	if err := rows.Err(); err != nil {
		return AudiencePolicy{}, fmt.Errorf("iterate audience permissions: %w", err)
	}
	return policy, nil
}

func (store *Store) RecordTokenAudit(ctx context.Context, identity MachineIdentity, eventType, outcome, audience, requestID string, details map[string]any) {
	encoded, _ := json.Marshal(details)
	_, _ = store.pool.Exec(ctx, `
		INSERT INTO audit_events(realm_id, subject_id, actor_id, event_type, outcome, audience, request_id, details)
		SELECT r.id, $2::uuid, $2::uuid, $3, $4, $5, $6, $7
		FROM realms r WHERE r.slug = $1`, identity.Realm, identity.PrincipalID, eventType, outcome, audience, requestID, encoded)
}
