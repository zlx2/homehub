package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type Owner struct {
	ID           string
	Username     string
	DisplayName  string
	PasswordHash string
	TOTPCipher   []byte
	TOTPNonce    []byte
	Status       string
}

type Session struct {
	ID                   string
	OwnerID              string
	TokenHash            []byte
	CSRFHash             []byte
	AuthenticationMethods []string
	AuthenticatedAt      time.Time
	CreatedAt            time.Time
	LastSeenAt           time.Time
	IdleExpiresAt        time.Time
	AbsoluteExpiresAt    time.Time
	RevokedAt            *time.Time
	RemoteIP             string
	UserAgentHash        []byte
}

type APIKey struct {
	ID         string
	OwnerID    string
	Name       string
	Kind       string
	TokenHash  []byte
	Scopes     []string
	ExpiresAt  *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	LastUsedAt *time.Time
	LastUsedIP string
}

type Share struct {
	ID           string
	OwnerID      string
	TokenHash    []byte
	ShareType    string
	ServiceID    string
	ResourceType string
	ResourceID   string
	Actions      []string
	ExpiresAt    time.Time
	MaxUses      *int
	UseCount     int
	RevokedAt    *time.Time
	CreatedAt    time.Time
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse IAM database configuration: %w", err)
	}
	config.MaxConns = 6
	config.MinConns = 0
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open IAM database: %w", err)
	}
	store := &Store{pool: pool}
	if err := store.Ping(ctx); err != nil {
		store.Close()
		return nil, err
	}
	return store, nil
}

// Migrate runs database migrations
func (store *Store) Migrate(ctx context.Context) error {
	return migrateEmbedded(ctx, store.pool)
}

func (store *Store) Close()     { store.pool.Close() }
func (store *Store) Ping(ctx context.Context) error {
	return store.pool.Ping(ctx)
}

// ── Owner ──

func (store *Store) OwnerExists(ctx context.Context) (bool, error) {
	var exists bool
	err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM owner WHERE status='active')`).Scan(&exists)
	return exists, err
}

func (store *Store) GetOwnerByUsername(ctx context.Context, usernameNormalized string) (Owner, error) {
	var o Owner
	err := store.pool.QueryRow(ctx, `SELECT id::text,username,display_name,password_hash,totp_cipher,totp_nonce,status
		FROM owner WHERE username_normalized=$1 AND status='active'`, usernameNormalized).
		Scan(&o.ID, &o.Username, &o.DisplayName, &o.PasswordHash, &o.TOTPCipher, &o.TOTPNonce, &o.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return Owner{}, fmt.Errorf("owner not found")
	}
	return o, err
}

// ── Bootstrap ──

func (store *Store) ValidateBootstrapToken(ctx context.Context, tokenHash []byte) (bool, error) {
	var valid bool
	err := store.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM owner_bootstrap_tokens WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now()
	) AND NOT EXISTS(SELECT 1 FROM owner WHERE status='active')`, tokenHash).Scan(&valid)
	return valid, err
}

func (store *Store) InsertPendingSetup(ctx context.Context, tokenHash []byte, username, normalized, displayName, passwordHash string, totpCipher, totpNonce []byte, expiresAt time.Time) (string, error) {
	var setupID string
	err := store.pool.QueryRow(ctx, `INSERT INTO pending_owner_setups(
		bootstrap_token_hash,username,username_normalized,display_name,password_hash,totp_cipher,totp_nonce,expires_at
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`,
		tokenHash, username, normalized, displayName, passwordHash, totpCipher, totpNonce, expiresAt).Scan(&setupID)
	return setupID, err
}

func (store *Store) GetPendingSetup(ctx context.Context, transaction pgx.Tx, setupID string) (tokenHash []byte, username, normalized, displayName, passwordHash string, totpCipher, totpNonce []byte, err error) {
	err = transaction.QueryRow(ctx, `SELECT bootstrap_token_hash,username,username_normalized,display_name,password_hash,totp_cipher,totp_nonce
		FROM pending_owner_setups WHERE id=$1::uuid AND expires_at>now() FOR UPDATE`, setupID).
		Scan(&tokenHash, &username, &normalized, &displayName, &passwordHash, &totpCipher, &totpNonce)
	return
}

func (store *Store) CreateOwner(ctx context.Context, transaction pgx.Tx, username, normalized, displayName, passwordHash string, totpCipher, totpNonce []byte) (Owner, error) {
	var o Owner
	err := transaction.QueryRow(ctx, `INSERT INTO owner(username,username_normalized,display_name,password_hash,totp_cipher,totp_nonce)
		VALUES($1,$2,$3,$4,$5,$6) RETURNING id::text,username,display_name`,
		username, normalized, displayName, passwordHash, totpCipher, totpNonce).
		Scan(&o.ID, &o.Username, &o.DisplayName)
	return o, err
}

func (store *Store) ConsumeBootstrapToken(ctx context.Context, transaction pgx.Tx, tokenHash []byte) error {
	_, err := transaction.Exec(ctx, `UPDATE owner_bootstrap_tokens SET consumed_at=now() WHERE token_hash=$1 AND consumed_at IS NULL`, tokenHash)
	return err
}

func (store *Store) DeletePendingSetup(ctx context.Context, transaction pgx.Tx, setupID string) error {
	_, err := transaction.Exec(ctx, `DELETE FROM pending_owner_setups WHERE id=$1::uuid`, setupID)
	return err
}

// ── Sessions ──

func (store *Store) CreateSession(ctx context.Context, transaction pgx.Tx, ownerID string, tokenHash, csrfHash []byte, methods []string, now time.Time, idleTTL, absoluteTTL time.Duration, remoteIP string, userAgentHash []byte) (string, error) {
	var sessionID string
	err := transaction.QueryRow(ctx, `INSERT INTO sessions(owner_id,token_hash,csrf_hash,authentication_methods,authenticated_at,last_seen_at,idle_expires_at,absolute_expires_at,remote_ip,user_agent_hash)
		VALUES($1::uuid,$2,$3,$4,$5,$5,$6,$7,$8::inet,$9) RETURNING id::text`,
		ownerID, tokenHash, csrfHash, methods, now, now.Add(idleTTL), now.Add(absoluteTTL), remoteIP, userAgentHash).Scan(&sessionID)
	return sessionID, err
}

func (store *Store) AuthenticateSession(ctx context.Context, tokenHash []byte) (ownerID, ownerUsername, ownerDisplayName, sessionID string, err error) {
	err = store.pool.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=now(),idle_expires_at=LEAST(now()+interval '30 days',absolute_expires_at)
		FROM owner o
		WHERE s.token_hash=$1 AND s.owner_id=o.id AND s.revoked_at IS NULL AND s.idle_expires_at>now() AND s.absolute_expires_at>now()
		AND o.status='active'
		RETURNING o.id::text,o.username,o.display_name,s.id::text`, tokenHash).
		Scan(&ownerID, &ownerUsername, &ownerDisplayName, &sessionID)
	return
}

func (store *Store) ValidateCSRF(ctx context.Context, sessionHash, csrfHash []byte) (bool, error) {
	var valid bool
	err := store.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions s JOIN owner o ON o.id=s.owner_id
		WHERE s.token_hash=$1 AND s.csrf_hash=$2 AND s.revoked_at IS NULL AND s.idle_expires_at>now() AND s.absolute_expires_at>now()
		AND o.status='active')`, sessionHash, csrfHash).Scan(&valid)
	return valid, err
}

func (store *Store) RevokeSession(ctx context.Context, tokenHash []byte) error {
	_, err := store.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, tokenHash)
	return err
}

func (store *Store) RevokeSessionByID(ctx context.Context, ownerID, sessionID string) (bool, error) {
	result, err := store.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE id=$1::uuid AND owner_id=$2::uuid AND revoked_at IS NULL`, sessionID, ownerID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

func (store *Store) RevokeOtherSessions(ctx context.Context, ownerID, currentSessionID string) (int64, error) {
	result, err := store.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE owner_id=$1::uuid AND id!=$2::uuid AND revoked_at IS NULL`, ownerID, currentSessionID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

type SessionInfo struct {
	ID                string     `json:"id"`
	CreatedAt         time.Time  `json:"created_at"`
	LastSeenAt        time.Time  `json:"last_seen_at"`
	RemoteIP          string     `json:"remote_ip"`
	UserAgent         string     `json:"user_agent"`
	AuthMethods       []string   `json:"auth_methods"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
}

func (store *Store) ListSessions(ctx context.Context, ownerID string) ([]SessionInfo, error) {
	rows, err := store.pool.Query(ctx, `SELECT id::text,created_at,last_seen_at,COALESCE(host(remote_ip),''),authentication_methods,revoked_at
		FROM sessions WHERE owner_id=$1::uuid ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []SessionInfo
	for rows.Next() {
		var s SessionInfo
		if err := rows.Scan(&s.ID, &s.CreatedAt, &s.LastSeenAt, &s.RemoteIP, &s.AuthMethods, &s.RevokedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

// ── API Keys ──

func HashCredential(value string) [sha256.Size]byte {
	return sha256.Sum256([]byte(value))
}

func (store *Store) CreateAPIKey(ctx context.Context, ownerID, name, kind string, tokenHash []byte, scopes []string, expiresAt *time.Time) (string, error) {
	var id string
	err := store.pool.QueryRow(ctx, `INSERT INTO api_keys(owner_id,name,kind,token_hash,scopes,expires_at)
		VALUES($1::uuid,$2,$3,$4,$5,$6) RETURNING id::text`,
		ownerID, name, kind, tokenHash, scopes, expiresAt).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create api key: %w", err)
	}
	return id, nil
}

func (store *Store) RevokeAPIKey(ctx context.Context, ownerID, keyID string) (bool, error) {
	result, err := store.pool.Exec(ctx, `UPDATE api_keys SET revoked_at=now() WHERE id=$1::uuid AND owner_id=$2::uuid AND revoked_at IS NULL`, keyID, ownerID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

type APIKeyInfo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	Scopes     []string   `json:"scopes"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	LastUsedIP string     `json:"last_used_ip,omitempty"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

func (store *Store) ListAPIKeys(ctx context.Context, ownerID string) ([]APIKeyInfo, error) {
	rows, err := store.pool.Query(ctx, `SELECT id::text,name,kind,scopes,created_at,last_used_at,COALESCE(host(last_used_ip),''),expires_at,revoked_at
		FROM api_keys WHERE owner_id=$1::uuid ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []APIKeyInfo
	for rows.Next() {
		var k APIKeyInfo
		if err := rows.Scan(&k.ID, &k.Name, &k.Kind, &k.Scopes, &k.CreatedAt, &k.LastUsedAt, &k.LastUsedIP, &k.ExpiresAt, &k.RevokedAt); err != nil {
			return nil, err
		}
		result = append(result, k)
	}
	return result, rows.Err()
}

func (store *Store) AuthenticateAPIKey(ctx context.Context, tokenHash []byte) (*APIKey, error) {
	var key APIKey
	err := store.pool.QueryRow(ctx, `UPDATE api_keys SET last_used_at=now()
		FROM owner o WHERE api_keys.token_hash=$1 AND api_keys.owner_id=o.id AND api_keys.revoked_at IS NULL
		AND (api_keys.expires_at IS NULL OR api_keys.expires_at>now()) AND o.status='active'
		RETURNING api_keys.id::text,api_keys.owner_id::text,api_keys.name,api_keys.kind,api_keys.scopes,api_keys.expires_at,api_keys.created_at`, tokenHash).
		Scan(&key.ID, &key.OwnerID, &key.Name, &key.Kind, &key.Scopes, &key.ExpiresAt, &key.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid api key")
	}
	return &key, err
}

// ── Shares ──

func (store *Store) CreateShare(ctx context.Context, ownerID string, tokenHash []byte, shareType, serviceID, resourceType, resourceID string, actions []string, expiresAt time.Time, maxUses *int) (string, error) {
	var id string
	err := store.pool.QueryRow(ctx, `INSERT INTO shares(owner_id,token_hash,share_type,service_id,resource_type,resource_id,actions,expires_at,max_uses)
		VALUES($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id::text`,
		ownerID, tokenHash, shareType, serviceID, resourceType, resourceID, actions, expiresAt, maxUses).Scan(&id)
	return id, err
}

func (store *Store) RevokeShare(ctx context.Context, ownerID, shareID string) (bool, error) {
	result, err := store.pool.Exec(ctx, `UPDATE shares SET revoked_at=now() WHERE id=$1::uuid AND owner_id=$2::uuid AND revoked_at IS NULL`, shareID, ownerID)
	if err != nil {
		return false, err
	}
	return result.RowsAffected() > 0, nil
}

type ShareInfo struct {
	ID           string     `json:"id"`
	ShareType    string     `json:"share_type"`
	ServiceID    string     `json:"service_id"`
	ResourceType string     `json:"resource_type,omitempty"`
	ResourceID   string     `json:"resource_id,omitempty"`
	Actions      []string   `json:"actions"`
	ExpiresAt    time.Time  `json:"expires_at"`
	MaxUses      *int       `json:"max_uses,omitempty"`
	UseCount     int        `json:"use_count"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (store *Store) ListShares(ctx context.Context, ownerID string) ([]ShareInfo, error) {
	rows, err := store.pool.Query(ctx, `SELECT id::text,share_type,service_id,COALESCE(resource_type,''),COALESCE(resource_id,''),actions,expires_at,max_uses,use_count,revoked_at,created_at
		FROM shares WHERE owner_id=$1::uuid ORDER BY created_at DESC`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ShareInfo
	for rows.Next() {
		var s ShareInfo
		if err := rows.Scan(&s.ID, &s.ShareType, &s.ServiceID, &s.ResourceType, &s.ResourceID, &s.Actions, &s.ExpiresAt, &s.MaxUses, &s.UseCount, &s.RevokedAt, &s.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

func (store *Store) RedeemShare(ctx context.Context, tokenHash []byte) (*Share, error) {
	var s Share
	err := store.pool.QueryRow(ctx, `SELECT id::text,owner_id::text,token_hash,share_type,service_id,COALESCE(resource_type,''),COALESCE(resource_id,''),actions,expires_at,max_uses,use_count,revoked_at,created_at
		FROM shares WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()
		AND (max_uses IS NULL OR use_count < max_uses)`, tokenHash).
		Scan(&s.ID, &s.OwnerID, &s.TokenHash, &s.ShareType, &s.ServiceID, &s.ResourceType, &s.ResourceID, &s.Actions, &s.ExpiresAt, &s.MaxUses, &s.UseCount, &s.RevokedAt, &s.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("invalid share")
	}
	return &s, err
}

func (store *Store) IncrementShareUse(ctx context.Context, shareID string) error {
	_, err := store.pool.Exec(ctx, `UPDATE shares SET use_count=use_count+1 WHERE id=$1::uuid`, shareID)
	return err
}

// ── Passkeys ──

func (store *Store) PasskeyQueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	return store.pool.QueryRow(ctx, query, args...)
}

func (store *Store) PasskeyExec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := store.pool.Exec(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

func (store *Store) PasskeyQuery(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	return store.pool.Query(ctx, query, args...)
}

// ── Audit ──

func (store *Store) RecordAudit(ctx context.Context, eventType, outcome, remoteIP string, details map[string]any) {
	encoded, _ := json.Marshal(details)
	_, _ = store.pool.Exec(ctx, `INSERT INTO audit_events(event_type,outcome,remote_ip,details)
		VALUES($1,$2,$3::inet,$4)`, eventType, outcome, remoteIP, encoded)
}
