package humanauth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/apps/iam/internal/token"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

const (
	passwordTime       = uint32(3)
	passwordMemory     = uint32(64 * 1024)
	passwordThreads    = uint8(2)
	passwordKeyLength  = uint32(32)
	setupTTL           = 15 * time.Minute
	sessionIdleTTL     = 12 * time.Hour
	sessionAbsoluteTTL = 7 * 24 * time.Hour
	maxShares          = 100
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidBootstrap   = errors.New("invalid bootstrap token")
	ErrSetupUnavailable   = errors.New("setup unavailable")
	ErrInvalidTOTP        = errors.New("invalid totp")
	ErrRateLimited        = errors.New("rate limited")
	ErrInvalidSession     = errors.New("invalid session")
	ErrInvalidCSRF        = errors.New("invalid csrf")
	ErrForbidden          = errors.New("forbidden")
	ErrInvalidShare       = errors.New("invalid share")
	ErrPasskey            = errors.New("invalid passkey ceremony")
	usernamePattern       = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,64}$`)
)

type Authorization interface {
	Check(context.Context, storepostgres.AuthorizationState, string, string, string) (bool, error)
	WriteRelationship(context.Context, storepostgres.AuthorizationState, string, string, string) error
	DeleteRelationship(context.Context, storepostgres.AuthorizationState, string, string, string) error
}

type PolicyStore interface {
	AudiencePolicy(context.Context, string) (storepostgres.AudiencePolicy, error)
	ServiceRelationExists(context.Context, string, string) (bool, error)
}

type TokenSigner interface {
	Issue(token.IssueRequest) (string, identity.Claims, error)
}

type Options struct {
	DatabaseURL        string
	EncryptionKeyFile  string
	BootstrapTokenFile string
	Authorization      Authorization
	AuthorizationState storepostgres.AuthorizationState
	Policies           PolicyStore
	Signer             TokenSigner
	PasskeyRPID        string
	PasskeyOrigins     []string
}

type Service struct {
	pool          *pgxpool.Pool
	aead          cipher.AEAD
	authorization Authorization
	state         storepostgres.AuthorizationState
	policies      PolicyStore
	signer        TokenSigner
	dummyPassword string
	hashSlots     chan struct{}
	now           func() time.Time
	passkeys      *webauthn.WebAuthn
}

type Principal struct {
	ID               string    `json:"id"`
	Subject          string    `json:"subject"`
	Kind             string    `json:"kind"`
	Username         string    `json:"username,omitempty"`
	DisplayName      string    `json:"display_name"`
	Realm            string    `json:"realm"`
	SessionID        string    `json:"-"`
	AuthenticationAt time.Time `json:"-"`
	Methods          []string  `json:"-"`
}

type Session struct {
	Principal Principal
	Token     string
	CSRF      string
}

type Setup struct {
	ID              string    `json:"setup_id"`
	ManualSecret    string    `json:"manual_secret"`
	ProvisioningURI string    `json:"provisioning_uri"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Grant struct {
	ServiceID string `json:"service_id"`
	Relation  string `json:"relation"`
}

type Share struct {
	ID        string     `json:"id"`
	Grants    []Grant    `json:"grants"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

type CreatedShare struct {
	Share
	Token string `json:"token"`
}

type TokenResponse struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ExpiresIn   int      `json:"expires_in"`
	Audience    string   `json:"audience"`
	Permissions []string `json:"permissions"`
}

func Open(ctx context.Context, options Options) (*Service, error) {
	key, err := readKey(options.EncryptionKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read human auth encryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	config, err := pgxpool.ParseConfig(options.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse human auth database URL: %w", err)
	}
	config.MaxConns = 4
	config.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	service := &Service{
		pool: pool, aead: aead, authorization: options.Authorization, state: options.AuthorizationState,
		policies: options.Policies, signer: options.Signer, hashSlots: make(chan struct{}, 2), now: time.Now,
	}
	if options.PasskeyRPID != "" {
		service.passkeys, err = webauthn.New(&webauthn.Config{
			RPDisplayName: "HomeHub",
			RPID:          options.PasskeyRPID,
			RPOrigins:     append([]string(nil), options.PasskeyOrigins...),
		})
		if err != nil {
			pool.Close()
			return nil, fmt.Errorf("initialize passkeys: %w", err)
		}
	}
	service.dummyPassword, err = service.hashPassword("homehub-dummy-password-never-valid")
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := service.ensureBootstrap(ctx, options.BootstrapTokenFile); err != nil {
		pool.Close()
		return nil, err
	}
	return service, nil
}

func (service *Service) Close() { service.pool.Close() }

func (service *Service) SetupRequired(ctx context.Context) (bool, error) {
	var exists bool
	err := service.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals WHERE kind='human' AND status='active' AND deleted_at IS NULL)`).Scan(&exists)
	return !exists, err
}

func (service *Service) BeginSetup(ctx context.Context, bootstrapToken, username, displayName, password string) (Setup, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	if !usernamePattern.MatchString(username) || len(password) < 12 || len(password) > 256 {
		return Setup{}, ErrInvalidBootstrap
	}
	if displayName == "" {
		displayName = username
	}
	if len(displayName) > 128 {
		return Setup{}, ErrInvalidBootstrap
	}
	digest := hashSecret(strings.TrimSpace(bootstrapToken))
	var valid bool
	err := service.pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM owner_bootstrap_tokens WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at>now()
	) AND NOT EXISTS(SELECT 1 FROM principals WHERE kind='human' AND status='active' AND deleted_at IS NULL)`, digest[:]).Scan(&valid)
	if err != nil {
		return Setup{}, err
	}
	if !valid {
		return Setup{}, ErrInvalidBootstrap
	}
	passwordHash, err := service.hashPassword(password)
	if err != nil {
		return Setup{}, err
	}
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return Setup{}, err
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)
	nonce, encrypted, err := service.encrypt([]byte(secret))
	if err != nil {
		return Setup{}, err
	}
	expiresAt := service.now().UTC().Add(setupTTL)
	var setupID string
	err = service.pool.QueryRow(ctx, `INSERT INTO pending_owner_setups(
		bootstrap_token_hash,username,username_normalized,display_name,password_hash,totp_cipher,totp_nonce,expires_at
	) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id::text`, digest[:], username, strings.ToLower(username), displayName,
		passwordHash, encrypted, nonce, expiresAt).Scan(&setupID)
	if err != nil {
		return Setup{}, err
	}
	uri := "otpauth://totp/" + url.PathEscape("HomeHub:"+username) + "?secret=" + url.QueryEscape(secret) + "&issuer=HomeHub&algorithm=SHA1&digits=6&period=30"
	return Setup{ID: setupID, ManualSecret: secret, ProvisioningURI: uri, ExpiresAt: expiresAt}, nil
}

func (service *Service) ConfirmSetup(ctx context.Context, setupID, code, remoteIP, userAgent string) (Session, error) {
	transaction, err := service.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock(92110431)`); err != nil {
		return Session{}, err
	}
	var bootstrapHash, encrypted, nonce []byte
	var username, normalized, displayName, passwordHash string
	err = transaction.QueryRow(ctx, `SELECT bootstrap_token_hash,username,username_normalized,display_name,password_hash,totp_cipher,totp_nonce
		FROM pending_owner_setups WHERE id=$1::uuid AND expires_at>now() FOR UPDATE`, setupID).
		Scan(&bootstrapHash, &username, &normalized, &displayName, &passwordHash, &encrypted, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSetupUnavailable
	}
	if err != nil {
		return Session{}, err
	}
	secret, err := service.decrypt(nonce, encrypted)
	if err != nil {
		return Session{}, err
	}
	if !validateTOTP(string(secret), code, service.now()) {
		return Session{}, ErrInvalidTOTP
	}
	var active bool
	if err := transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals WHERE kind='human' AND status='active' AND deleted_at IS NULL)`).Scan(&active); err != nil {
		return Session{}, err
	}
	if active {
		return Session{}, ErrSetupUnavailable
	}
	var principal Principal
	err = transaction.QueryRow(ctx, `INSERT INTO principals(realm_id,kind,display_name,status)
		SELECT id,'human',$1,'pending' FROM realms WHERE slug='homehub'
		RETURNING id::text,display_name`, displayName).Scan(&principal.ID, &principal.DisplayName)
	if err != nil {
		return Session{}, err
	}
	principal.Kind, principal.Username, principal.Realm = "human", username, "homehub"
	principal.Subject = "human:" + principal.ID
	if _, err := transaction.Exec(ctx, `INSERT INTO external_accounts(provider,external_subject,principal_id,attributes)
		VALUES('homehub-username',$1,$2::uuid,jsonb_build_object('username',$3::text))`, normalized, principal.ID, username); err != nil {
		return Session{}, err
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO human_authenticators(principal_id,password_hash,totp_cipher,totp_nonce)
		VALUES($1::uuid,$2,$3,$4)`, principal.ID, passwordHash, encrypted, nonce); err != nil {
		return Session{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Session{}, err
	}
	if err := service.authorization.WriteRelationship(ctx, service.state, principal.Subject, "owner", "realm:homehub"); err != nil {
		_, _ = service.pool.Exec(ctx, `DELETE FROM principals WHERE id=$1::uuid`, principal.ID)
		return Session{}, fmt.Errorf("grant owner relationship: %w", err)
	}
	activation, err := service.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		_ = service.authorization.DeleteRelationship(ctx, service.state, principal.Subject, "owner", "realm:homehub")
		return Session{}, err
	}
	defer func() { _ = activation.Rollback(ctx) }()
	if _, err := activation.Exec(ctx, `SELECT pg_advisory_xact_lock(92110431)`); err != nil {
		return Session{}, err
	}
	if _, err := activation.Exec(ctx, `UPDATE principals SET status='active',updated_at=now() WHERE id=$1::uuid AND status='pending'`, principal.ID); err != nil {
		return Session{}, err
	}
	if _, err := activation.Exec(ctx, `UPDATE owner_bootstrap_tokens SET consumed_at=now() WHERE token_hash=$1 AND consumed_at IS NULL`, bootstrapHash); err != nil {
		return Session{}, err
	}
	if _, err := activation.Exec(ctx, `DELETE FROM pending_owner_setups WHERE id=$1::uuid`, setupID); err != nil {
		return Session{}, err
	}
	session, err := createSession(ctx, activation, principal, []string{"password", "otp"}, service.now().UTC(), sessionAbsoluteTTL, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if err := activation.Commit(ctx); err != nil {
		_ = service.authorization.DeleteRelationship(ctx, service.state, principal.Subject, "owner", "realm:homehub")
		_, _ = service.pool.Exec(ctx, `DELETE FROM principals WHERE id=$1::uuid`, principal.ID)
		return Session{}, err
	}
	service.audit(ctx, principal.ID, "human.setup", "success", remoteIP, nil)
	return session, nil
}

func (service *Service) Login(ctx context.Context, username, password, code, remoteIP, userAgent string) (Session, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	usernameDigest := sha256.Sum256([]byte(normalized))
	var failures int
	err := service.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE event_type='human.login' AND outcome='denied'
		AND created_at>now()-interval '15 minutes' AND (details->>'username_hash'=$1 OR remote_ip=NULLIF($2,'')::inet)`, hex.EncodeToString(usernameDigest[:]), remoteIP).Scan(&failures)
	if err != nil {
		return Session{}, err
	}
	if failures >= 5 {
		return Session{}, ErrRateLimited
	}
	var principal Principal
	var passwordHash string
	var encrypted, nonce []byte
	err = service.pool.QueryRow(ctx, `SELECT p.id::text,p.display_name,r.slug,h.password_hash,h.totp_cipher,h.totp_nonce,
		COALESCE(e.attributes->>'username',e.external_subject)
		FROM external_accounts e JOIN principals p ON p.id=e.principal_id JOIN realms r ON r.id=p.realm_id
		JOIN human_authenticators h ON h.principal_id=p.id
		WHERE e.provider='homehub-username' AND e.external_subject=$1 AND p.kind='human' AND p.status='active' AND p.deleted_at IS NULL`, normalized).
		Scan(&principal.ID, &principal.DisplayName, &principal.Realm, &passwordHash, &encrypted, &nonce, &principal.Username)
	if errors.Is(err, pgx.ErrNoRows) {
		passwordHash = service.dummyPassword
	} else if err != nil {
		return Session{}, err
	}
	passwordOK := service.verifyPassword(password, passwordHash)
	totpOK := false
	if passwordOK && principal.ID != "" {
		secret, decryptErr := service.decrypt(nonce, encrypted)
		if decryptErr != nil {
			return Session{}, decryptErr
		}
		totpOK = validateTOTP(string(secret), code, service.now())
	}
	if !passwordOK || !totpOK || principal.ID == "" {
		service.audit(ctx, "", "human.login", "denied", remoteIP, map[string]any{"username_hash": hex.EncodeToString(usernameDigest[:])})
		return Session{}, ErrInvalidCredentials
	}
	principal.Kind, principal.Subject = "human", "human:"+principal.ID
	transaction, err := service.pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	session, err := createSession(ctx, transaction, principal, []string{"password", "otp"}, service.now().UTC(), sessionAbsoluteTTL, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Session{}, err
	}
	service.audit(ctx, principal.ID, "human.login", "success", remoteIP, nil)
	return session, nil
}

func (service *Service) Authenticate(ctx context.Context, sessionToken string) (Principal, error) {
	if len(sessionToken) < 32 {
		return Principal{}, ErrInvalidSession
	}
	digest := hashSecret(sessionToken)
	var principal Principal
	err := service.pool.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=now(),idle_expires_at=LEAST(now()+interval '12 hours',absolute_expires_at)
		FROM principals p JOIN realms r ON r.id=p.realm_id
		LEFT JOIN external_accounts e ON e.principal_id=p.id AND e.provider='homehub-username'
		WHERE s.token_hash=$1 AND s.principal_id=p.id AND s.revoked_at IS NULL AND s.idle_expires_at>now() AND s.absolute_expires_at>now()
		AND p.status='active' AND p.deleted_at IS NULL
		RETURNING p.id::text,p.kind,p.display_name,r.slug,s.id::text,s.authenticated_at,s.authentication_methods,
		COALESCE(e.attributes->>'username',e.external_subject,'')`, digest[:]).
		Scan(&principal.ID, &principal.Kind, &principal.DisplayName, &principal.Realm, &principal.SessionID,
			&principal.AuthenticationAt, &principal.Methods, &principal.Username)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrInvalidSession
	}
	if err != nil {
		return Principal{}, err
	}
	principal.Subject = principal.Kind + ":" + principal.ID
	return principal, nil
}

func (service *Service) ValidateCSRF(ctx context.Context, sessionToken, csrf string) bool {
	if len(sessionToken) < 32 || len(csrf) < 32 {
		return false
	}
	sessionHash, csrfHash := hashSecret(sessionToken), hashSecret(csrf)
	var valid bool
	err := service.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions s JOIN principals p ON p.id=s.principal_id
		WHERE s.token_hash=$1 AND s.csrf_hash=$2 AND s.revoked_at IS NULL AND s.idle_expires_at>now() AND s.absolute_expires_at>now()
		AND p.status='active' AND p.deleted_at IS NULL)`, sessionHash[:], csrfHash[:]).Scan(&valid)
	return err == nil && valid
}

func (service *Service) Logout(ctx context.Context, sessionToken string) error {
	digest := hashSecret(sessionToken)
	_, err := service.pool.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL`, digest[:])
	return err
}

func (service *Service) IsAdministrator(ctx context.Context, principal Principal) (bool, error) {
	return service.authorization.Check(ctx, service.state, principal.Subject, "administrator", "realm:"+principal.Realm)
}

func (service *Service) Issue(ctx context.Context, principal Principal, audience string, requested []string, allowedOnly bool) (TokenResponse, error) {
	policy, err := service.policies.AudiencePolicy(ctx, audience)
	if err != nil {
		return TokenResponse{}, ErrForbidden
	}
	permissions := append([]string(nil), requested...)
	if len(permissions) == 0 {
		for permission := range policy.Permissions {
			permissions = append(permissions, permission)
		}
		sort.Strings(permissions)
	}
	allowed := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		relation, known := policy.Permissions[permission]
		if !known {
			return TokenResponse{}, ErrForbidden
		}
		ok, checkErr := service.authorization.Check(ctx, service.state, principal.Subject, relation, "service:"+policy.ServiceID)
		if checkErr != nil {
			return TokenResponse{}, checkErr
		}
		if ok {
			allowed = append(allowed, permission)
			continue
		}
		if !allowedOnly {
			return TokenResponse{}, ErrForbidden
		}
	}
	if len(allowed) == 0 {
		return TokenResponse{}, ErrForbidden
	}
	encoded, claims, err := service.signer.Issue(token.IssueRequest{
		Audience: audience, Subject: principal.Subject, AuthorizedParty: principal.SessionID,
		Realm: principal.Realm, Permissions: allowed, SessionID: principal.SessionID,
		Authentication: principal.Methods, AuthenticationAt: principal.AuthenticationAt,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	service.audit(ctx, principal.ID, "session.token", "success", "", map[string]any{"audience": audience, "permissions": allowed})
	return TokenResponse{AccessToken: encoded, TokenType: "Bearer", ExpiresIn: int(claims.Expires - claims.IssuedAt), Audience: audience, Permissions: allowed}, nil
}

func (service *Service) CreateShare(ctx context.Context, actor Principal, grants []Grant, expiresAt time.Time, remoteIP string) (CreatedShare, error) {
	if len(grants) == 0 || len(grants) > 16 || !expiresAt.After(service.now()) || expiresAt.After(service.now().Add(7*24*time.Hour)) {
		return CreatedShare{}, ErrInvalidShare
	}
	seen := make(map[string]struct{})
	normalized := make([]Grant, 0, len(grants))
	for _, grant := range grants {
		grant.ServiceID, grant.Relation = strings.TrimSpace(grant.ServiceID), strings.TrimSpace(grant.Relation)
		if grant.Relation != "viewer" && grant.Relation != "editor" {
			return CreatedShare{}, ErrInvalidShare
		}
		exists, err := service.policies.ServiceRelationExists(ctx, grant.ServiceID, grant.Relation)
		if err != nil || !exists {
			return CreatedShare{}, ErrInvalidShare
		}
		key := grant.ServiceID + "\x00" + grant.Relation
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, grant)
	}
	tokenValue, err := randomSecret(32)
	if err != nil {
		return CreatedShare{}, err
	}
	digest := hashSecret(tokenValue)
	transaction, err := service.pool.Begin(ctx)
	if err != nil {
		return CreatedShare{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var share CreatedShare
	share.Token, share.Grants, share.ExpiresAt = tokenValue, normalized, expiresAt.UTC()
	err = transaction.QueryRow(ctx, `INSERT INTO share_links(realm_id,token_hash,created_by,expires_at)
		SELECT realm_id,$2,$1::uuid,$3 FROM principals WHERE id=$1::uuid
		RETURNING id::text,created_at`, actor.ID, digest[:], share.ExpiresAt).Scan(&share.ID, &share.CreatedAt)
	if err != nil {
		return CreatedShare{}, err
	}
	for _, grant := range normalized {
		if _, err := transaction.Exec(ctx, `INSERT INTO share_grants(share_id,service_id,relation) VALUES($1::uuid,$2,$3)`, share.ID, grant.ServiceID, grant.Relation); err != nil {
			return CreatedShare{}, err
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return CreatedShare{}, err
	}
	service.audit(ctx, actor.ID, "share.create", "success", remoteIP, map[string]any{"share_id": share.ID, "grants": normalized})
	return share, nil
}

func (service *Service) ListShares(ctx context.Context) ([]Share, error) {
	rows, err := service.pool.Query(ctx, `SELECT s.id::text,s.expires_at,s.revoked_at,s.created_at,g.service_id,g.relation
		FROM share_links s JOIN share_grants g ON g.share_id=s.id ORDER BY s.created_at DESC,g.service_id,g.relation LIMIT $1`, maxShares*16)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byID := make(map[string]*Share)
	order := make([]string, 0)
	for rows.Next() {
		var id, serviceID, relation string
		var expiresAt, createdAt time.Time
		var revokedAt *time.Time
		if err := rows.Scan(&id, &expiresAt, &revokedAt, &createdAt, &serviceID, &relation); err != nil {
			return nil, err
		}
		share := byID[id]
		if share == nil {
			share = &Share{ID: id, ExpiresAt: expiresAt, RevokedAt: revokedAt, CreatedAt: createdAt}
			byID[id], order = share, append(order, id)
		}
		share.Grants = append(share.Grants, Grant{ServiceID: serviceID, Relation: relation})
	}
	result := make([]Share, 0, len(order))
	for _, id := range order {
		result = append(result, *byID[id])
	}
	return result, rows.Err()
}

func (service *Service) RedeemShare(ctx context.Context, capability, remoteIP, userAgent string) (Session, error) {
	if len(capability) < 32 {
		return Session{}, ErrInvalidShare
	}
	digest := hashSecret(capability)
	transaction, err := service.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var shareID, realmID, realm, guestID string
	var expiresAt time.Time
	var existingGuest *string
	err = transaction.QueryRow(ctx, `SELECT s.id::text,s.realm_id::text,r.slug,s.guest_principal_id::text,s.expires_at
		FROM share_links s JOIN realms r ON r.id=s.realm_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>now() FOR UPDATE`, digest[:]).
		Scan(&shareID, &realmID, &realm, &existingGuest, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidShare
	}
	if err != nil {
		return Session{}, err
	}
	if existingGuest == nil {
		err = transaction.QueryRow(ctx, `INSERT INTO principals(realm_id,kind,display_name,status,attributes)
			VALUES($1::uuid,'guest','分享访客','active',jsonb_build_object('share_id',$2::text)) RETURNING id::text`, realmID, shareID).Scan(&guestID)
		if err != nil {
			return Session{}, err
		}
		if _, err := transaction.Exec(ctx, `UPDATE share_links SET guest_principal_id=$2::uuid WHERE id=$1::uuid`, shareID, guestID); err != nil {
			return Session{}, err
		}
	} else {
		guestID = *existingGuest
	}
	rows, err := transaction.Query(ctx, `SELECT service_id,relation FROM share_grants WHERE share_id=$1::uuid ORDER BY service_id,relation`, shareID)
	if err != nil {
		return Session{}, err
	}
	var grants []Grant
	for rows.Next() {
		var grant Grant
		if err := rows.Scan(&grant.ServiceID, &grant.Relation); err != nil {
			rows.Close()
			return Session{}, err
		}
		grants = append(grants, grant)
	}
	rows.Close()
	if err := transaction.Commit(ctx); err != nil {
		return Session{}, err
	}
	subject := "guest:" + guestID
	written := make([]Grant, 0, len(grants))
	for _, grant := range grants {
		if err := service.authorization.WriteRelationship(ctx, service.state, subject, grant.Relation, "service:"+grant.ServiceID); err != nil {
			for _, prior := range written {
				_ = service.authorization.DeleteRelationship(ctx, service.state, subject, prior.Relation, "service:"+prior.ServiceID)
			}
			return Session{}, err
		}
		written = append(written, grant)
	}
	principal := Principal{ID: guestID, Subject: subject, Kind: "guest", DisplayName: "分享访客", Realm: realm}
	sessionTTL := time.Until(expiresAt)
	if sessionTTL > sessionAbsoluteTTL {
		sessionTTL = sessionAbsoluteTTL
	}
	sessionTx, err := service.pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = sessionTx.Rollback(ctx) }()
	session, err := createSession(ctx, sessionTx, principal, []string{"share"}, service.now().UTC(), sessionTTL, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if err := sessionTx.Commit(ctx); err != nil {
		return Session{}, err
	}
	service.audit(ctx, guestID, "share.redeem", "success", remoteIP, map[string]any{"share_id": shareID})
	return session, nil
}

func (service *Service) RevokeShare(ctx context.Context, actor Principal, shareID, remoteIP string) (bool, error) {
	transaction, err := service.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	var guestID *string
	err = transaction.QueryRow(ctx, `UPDATE share_links SET revoked_at=now() WHERE id=$1::uuid AND revoked_at IS NULL RETURNING guest_principal_id::text`, shareID).Scan(&guestID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var grants []Grant
	rows, err := transaction.Query(ctx, `SELECT service_id,relation FROM share_grants WHERE share_id=$1::uuid`, shareID)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var grant Grant
		if err := rows.Scan(&grant.ServiceID, &grant.Relation); err != nil {
			rows.Close()
			return false, err
		}
		grants = append(grants, grant)
	}
	rows.Close()
	if guestID != nil {
		if _, err := transaction.Exec(ctx, `UPDATE principals SET status='revoked',updated_at=now() WHERE id=$1::uuid`, *guestID); err != nil {
			return false, err
		}
		if _, err := transaction.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE principal_id=$1::uuid AND revoked_at IS NULL`, *guestID); err != nil {
			return false, err
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, err
	}
	if guestID != nil {
		subject := "guest:" + *guestID
		for _, grant := range grants {
			_ = service.authorization.DeleteRelationship(ctx, service.state, subject, grant.Relation, "service:"+grant.ServiceID)
		}
	}
	service.audit(ctx, actor.ID, "share.revoke", "success", remoteIP, map[string]any{"share_id": shareID})
	return true, nil
}

func (service *Service) ensureBootstrap(ctx context.Context, path string) error {
	required, err := service.SetupRequired(ctx)
	if err != nil || !required {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read owner bootstrap token: %w", err)
	}
	value := strings.TrimSpace(string(contents))
	if len(value) < 32 {
		return errors.New("owner bootstrap token is too short")
	}
	digest := hashSecret(value)
	_, err = service.pool.Exec(ctx, `INSERT INTO owner_bootstrap_tokens(token_hash,expires_at)
		VALUES($1,now()+interval '30 days') ON CONFLICT(token_hash) DO UPDATE SET expires_at=GREATEST(owner_bootstrap_tokens.expires_at,EXCLUDED.expires_at)
		WHERE owner_bootstrap_tokens.consumed_at IS NULL`, digest[:])
	return err
}

func createSession(ctx context.Context, transaction pgx.Tx, principal Principal, methods []string, authenticatedAt time.Time, ttl time.Duration, remoteIP, userAgent string) (Session, error) {
	tokenValue, err := randomSecret(32)
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomSecret(32)
	if err != nil {
		return Session{}, err
	}
	tokenHash, csrfHash := hashSecret(tokenValue), hashSecret(csrf)
	absolute := authenticatedAt.Add(ttl)
	idle := authenticatedAt.Add(sessionIdleTTL)
	if idle.After(absolute) {
		idle = absolute
	}
	var sessionID string
	err = transaction.QueryRow(ctx, `INSERT INTO sessions(principal_id,token_hash,csrf_hash,authentication_methods,authenticated_at,idle_expires_at,absolute_expires_at,remote_ip,user_agent_hash)
		VALUES($1::uuid,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::inet,$9) RETURNING id::text`, principal.ID, tokenHash[:], csrfHash[:], methods,
		authenticatedAt, idle, absolute, remoteIP, hashBytes(userAgent)).Scan(&sessionID)
	if err != nil {
		return Session{}, err
	}
	principal.SessionID, principal.AuthenticationAt, principal.Methods = sessionID, authenticatedAt, append([]string(nil), methods...)
	return Session{Principal: principal, Token: tokenValue, CSRF: csrf}, nil
}

func (service *Service) audit(ctx context.Context, principalID, eventType, outcome, remoteIP string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	_, _ = service.pool.Exec(ctx, `INSERT INTO audit_events(realm_id,subject_id,actor_id,event_type,outcome,audience,remote_ip,details)
		SELECT id,NULLIF($1,'')::uuid,NULLIF($1,'')::uuid,$2,$3,'homehub-iam',NULLIF($4,'')::inet,$5 FROM realms WHERE slug='homehub'`, principalID, eventType, outcome, remoteIP, details)
}

func (service *Service) hashPassword(password string) (string, error) {
	select {
	case service.hashSlots <- struct{}{}:
		defer func() { <-service.hashSlots }()
	case <-time.After(5 * time.Second):
		return "", ErrRateLimited
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordTime, passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func (service *Service) verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil || memory > 128*1024 || iterations > 10 || threads > 8 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	expected, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil || len(expected) < 16 || len(expected) > 64 {
		return false
	}
	select {
	case service.hashSlots <- struct{}{}:
		defer func() { <-service.hashSlots }()
	case <-time.After(5 * time.Second):
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func validateTOTP(secret, rawCode string, now time.Time) bool {
	code := strings.TrimSpace(rawCode)
	if len(code) != 6 {
		return false
	}
	for _, char := range code {
		if char < '0' || char > '9' {
			return false
		}
	}
	decoded, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	counter := now.Unix() / 30
	for offset := int64(-1); offset <= 1; offset++ {
		var value [8]byte
		binary.BigEndian.PutUint64(value[:], uint64(counter+offset))
		mac := hmac.New(sha1.New, decoded)
		_, _ = mac.Write(value[:])
		digest := mac.Sum(nil)
		index := digest[len(digest)-1] & 0x0f
		number := (uint32(digest[index])&0x7f)<<24 | uint32(digest[index+1])<<16 | uint32(digest[index+2])<<8 | uint32(digest[index+3])
		candidate := fmt.Sprintf("%06d", number%1_000_000)
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

func (service *Service) encrypt(plain []byte) ([]byte, []byte, error) {
	nonce := make([]byte, service.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, service.aead.Seal(nil, nonce, plain, nil), nil
}

func (service *Service) decrypt(nonce, encrypted []byte) ([]byte, error) {
	plain, err := service.aead.Open(nil, nonce, encrypted, nil)
	if err != nil {
		return nil, errors.New("invalid encrypted authenticator")
	}
	return plain, nil
}

func readKey(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(string(contents))
	for _, encoding := range []*base64.Encoding{base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if len(contents) == 32 {
		return contents, nil
	}
	return nil, errors.New("auth encryption key must contain 32 bytes")
}

func randomSecret(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashSecret(value string) [sha256.Size]byte { return sha256.Sum256([]byte(value)) }
func hashBytes(value string) []byte {
	hash := sha256.Sum256([]byte(value))
	return hash[:]
}
