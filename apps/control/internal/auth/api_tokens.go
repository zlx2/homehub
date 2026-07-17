package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const apiTokenPrefix = "hht_"

var (
	ErrInvalidAPIToken  = errors.New("invalid API token specification")
	ErrTooManyAPITokens = errors.New("too many active API tokens")
)

type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	ServiceID  string     `json:"service_id"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreatedAPIToken struct {
	APIToken
	Token string `json:"token"`
}

type APITokenIdentity struct {
	Principal Principal
	TokenID   string
	ServiceID string
}

func (s *Service) CreateAPIToken(ctx context.Context, principalID, name, serviceID string, scopes []string, expiresAt time.Time, remoteIP string) (CreatedAPIToken, error) {
	name = strings.TrimSpace(name)
	serviceID = strings.TrimSpace(serviceID)
	scopes = normalizeTokenScopes(scopes)
	if err := validateAPITokenSpec(name, serviceID, scopes, expiresAt, time.Now().UTC()); err != nil {
		return CreatedAPIToken{}, err
	}
	var active int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM api_tokens
		WHERE principal_id=$1 AND revoked_at IS NULL AND expires_at>now()`, principalID).Scan(&active); err != nil {
		return CreatedAPIToken{}, err
	}
	if active >= 20 {
		return CreatedAPIToken{}, ErrTooManyAPITokens
	}
	random, err := randomToken(32)
	if err != nil {
		return CreatedAPIToken{}, err
	}
	token := apiTokenPrefix + random
	digest := tokenHash(token)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreatedAPIToken{}, err
	}
	defer tx.Rollback(ctx)
	var created CreatedAPIToken
	err = tx.QueryRow(ctx, `INSERT INTO api_tokens(principal_id,name,token_hash,service_id,scopes,expires_at)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id,name,service_id,scopes,expires_at,last_used_at,created_at`,
		principalID, name, digest[:], serviceID, scopes, expiresAt.UTC()).Scan(
		&created.ID, &created.Name, &created.ServiceID, &created.Scopes, &created.ExpiresAt, &created.LastUsedAt, &created.CreatedAt,
	)
	if err != nil {
		return CreatedAPIToken{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'api_token_create','success',$2,$3)`, principalID, nullableIP(remoteIP), digest[:]); err != nil {
		return CreatedAPIToken{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreatedAPIToken{}, err
	}
	created.Token = token
	return created, nil
}

func (s *Service) ListAPITokens(ctx context.Context, principalID string) ([]APIToken, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,name,service_id,scopes,expires_at,last_used_at,created_at
		FROM api_tokens WHERE principal_id=$1 AND revoked_at IS NULL
		ORDER BY created_at DESC LIMIT 100`, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tokens := make([]APIToken, 0)
	for rows.Next() {
		var token APIToken
		if err := rows.Scan(&token.ID, &token.Name, &token.ServiceID, &token.Scopes, &token.ExpiresAt, &token.LastUsedAt, &token.CreatedAt); err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, rows.Err()
}

func (s *Service) RevokeAPIToken(ctx context.Context, principalID, tokenID, remoteIP string) (bool, error) {
	command, err := s.pool.Exec(ctx, `UPDATE api_tokens SET revoked_at=now()
		WHERE id=$1 AND principal_id=$2 AND revoked_at IS NULL`, tokenID, principalID)
	if err != nil {
		return false, err
	}
	if command.RowsAffected() == 0 {
		return false, nil
	}
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip)
		VALUES($1,'api_token_revoke','success',$2)`, principalID, nullableIP(remoteIP))
	return true, nil
}

func (s *Service) AuthenticateAPIToken(ctx context.Context, token string) (APITokenIdentity, error) {
	if !strings.HasPrefix(token, apiTokenPrefix) || len(token) < len(apiTokenPrefix)+32 {
		return APITokenIdentity{}, ErrInvalidCredentials
	}
	digest := tokenHash(token)
	var identity APITokenIdentity
	err := s.pool.QueryRow(ctx, `UPDATE api_tokens t SET last_used_at=now()
		FROM principals p
		WHERE t.principal_id=p.id AND t.token_hash=$1 AND t.revoked_at IS NULL
		AND t.expires_at>now() AND p.status='active'
		RETURNING p.id,p.username,p.display_name,t.scopes,t.id,t.service_id`, digest[:]).Scan(
		&identity.Principal.ID, &identity.Principal.Username, &identity.Principal.DisplayName,
		&identity.Principal.Scopes, &identity.TokenID, &identity.ServiceID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return APITokenIdentity{}, ErrInvalidCredentials
	}
	if err != nil {
		return APITokenIdentity{}, err
	}
	return identity, nil
}

func normalizeTokenScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes))
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope == "" {
			continue
		}
		if _, exists := seen[scope]; exists {
			continue
		}
		seen[scope] = struct{}{}
		result = append(result, scope)
	}
	return result
}

func validateAPITokenSpec(name, serviceID string, scopes []string, expiresAt, now time.Time) error {
	if len(name) < 1 || len(name) > 80 {
		return fmt.Errorf("%w: token name must contain 1-80 characters", ErrInvalidAPIToken)
	}
	if serviceID != "drop" || len(scopes) != 1 || scopes[0] != "drop.upload" {
		return fmt.Errorf("%w: unsupported permission", ErrInvalidAPIToken)
	}
	if !expiresAt.After(now.Add(5 * time.Minute)) {
		return fmt.Errorf("%w: token must remain valid for at least 5 minutes", ErrInvalidAPIToken)
	}
	if expiresAt.After(now.Add(366 * 24 * time.Hour)) {
		return fmt.Errorf("%w: token cannot remain valid for more than 366 days", ErrInvalidAPIToken)
	}
	return nil
}
