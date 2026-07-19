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
	sessionIdleTTL     = 30 * 24 * time.Hour
	sessionAbsoluteTTL = 180 * 24 * time.Hour
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
	ErrInvalidAPIKey      = errors.New("invalid api key")
	ErrPasskey            = errors.New("invalid passkey ceremony")
	usernamePattern       = regexp.MustCompile(`^[A-Za-z0-9_.-]{3,64}$`)
)

type TokenSigner interface {
	Issue(token.IssueRequest) (string, identity.Claims, error)
}

type Options struct {
	DatabaseURL        string
	EncryptionKeyFile  string
	BootstrapTokenFile string
	Signer             TokenSigner
	PasskeyRPID        string
	PasskeyOrigins     []string
}

type Service struct {
	pool          *pgxpool.Pool
	store         *storepostgres.Store
	aead          cipher.AEAD
	signer        TokenSigner
	dummyPassword string
	hashSlots     chan struct{}
	now           func() time.Time
	passkeys      *webauthn.WebAuthn
}

type Principal struct {
	ID          string `json:"id"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name"`
	Kind        string `json:"kind"`
	SessionID   string `json:"-"`
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
		return nil, fmt.Errorf("read encryption key: %w", err)
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
	store := &storepostgres.Store{}
	service := &Service{
		pool: pool, store: store, aead: aead, signer: options.Signer,
		hashSlots: make(chan struct{}, 2), now: time.Now,
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
	return service.store.OwnerExists(ctx)
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
	valid, err := service.store.ValidateBootstrapToken(ctx, digest[:])
	if err != nil || !valid {
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
	setupID, err := service.store.InsertPendingSetup(ctx, digest[:], username, strings.ToLower(username), displayName, passwordHash, encrypted, nonce, expiresAt)
	if err != nil {
		return Setup{}, err
	}
	uri := "otpauth://totp/" + url.PathEscape("HomeHub:"+username) + "?secret=" + url.QueryEscape(secret) + "&issuer=HomeHub&algorithm=SHA1&digits=6&period=30"
	return Setup{ID: setupID, ManualSecret: secret, ProvisioningURI: uri, ExpiresAt: expiresAt}, nil
}

func (service *Service) ConfirmSetup(ctx context.Context, setupID, code, remoteIP, userAgent string) (Session, error) {
	tx, err := service.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(92110431)`); err != nil {
		return Session{}, err
	}
	tokenHash, username, normalized, displayName, passwordHash, totpCipher, totpNonce, err := service.store.GetPendingSetup(ctx, tx, setupID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrSetupUnavailable
		}
		return Session{}, err
	}
	secret, err := service.decrypt(totpNonce, totpCipher)
	if err != nil {
		return Session{}, err
	}
	if !validateTOTP(string(secret), code, service.now()) {
		return Session{}, ErrInvalidTOTP
	}
	active, err := service.store.OwnerExists(ctx)
	if err != nil {
		return Session{}, err
	}
	if active {
		return Session{}, ErrSetupUnavailable
	}
	owner, err := service.store.CreateOwner(ctx, tx, username, normalized, displayName, passwordHash, totpCipher, totpNonce)
	if err != nil {
		return Session{}, err
	}
	if err := service.store.ConsumeBootstrapToken(ctx, tx, tokenHash); err != nil {
		return Session{}, err
	}
	if err := service.store.DeletePendingSetup(ctx, tx, setupID); err != nil {
		return Session{}, err
	}
	session, err := service.createSessionTx(ctx, tx, owner.ID, owner.Username, owner.DisplayName, "human", []string{"password", "otp"}, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	service.audit(ctx, "owner.setup", "success", remoteIP, nil)
	return session, nil
}

func (service *Service) Login(ctx context.Context, username, password, code, remoteIP, userAgent string) (Session, error) {
	normalized := strings.ToLower(strings.TrimSpace(username))
	usernameDigest := sha256.Sum256([]byte(normalized))
	var failures int
	err := service.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE event_type='owner.login' AND outcome='denied'
		AND created_at>now()-interval '15 minutes' AND (details->>'username_hash'=$1 OR remote_ip=NULLIF($2,'')::inet)`,
		hex.EncodeToString(usernameDigest[:]), remoteIP).Scan(&failures)
	if err != nil {
		return Session{}, err
	}
	if failures >= 5 {
		return Session{}, ErrRateLimited
	}
	owner, findErr := service.store.GetOwnerByUsername(ctx, normalized)
	passwordHash := service.dummyPassword
	if findErr == nil {
		passwordHash = owner.PasswordHash
	}
	passwordOK := service.verifyPassword(password, passwordHash)
	totpOK := false
	if passwordOK && findErr == nil {
		secret, decryptErr := service.decrypt(owner.TOTPNonce, owner.TOTPCipher)
		if decryptErr != nil {
			return Session{}, decryptErr
		}
		totpOK = validateTOTP(string(secret), code, service.now())
	}
	if !passwordOK || !totpOK || findErr != nil {
		service.audit(ctx, "owner.login", "denied", remoteIP, map[string]any{"username_hash": hex.EncodeToString(usernameDigest[:])})
		return Session{}, ErrInvalidCredentials
	}
	session, err := service.createSession(ctx, owner.ID, owner.Username, owner.DisplayName, "human", []string{"password", "otp"}, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	service.audit(ctx, "owner.login", "success", remoteIP, nil)
	return session, nil
}

func (service *Service) Authenticate(ctx context.Context, sessionToken string) (Principal, error) {
	if len(sessionToken) < 32 {
		return Principal{}, ErrInvalidSession
	}
	digest := hashSecret(sessionToken)
	ownerID, username, displayName, sessionID, err := service.store.AuthenticateSession(ctx, digest[:])
	if err != nil {
		return Principal{}, ErrInvalidSession
	}
	return Principal{ID: ownerID, Username: username, DisplayName: displayName, Kind: "human", SessionID: sessionID}, nil
}

func (service *Service) ValidateCSRF(ctx context.Context, sessionToken, csrf string) bool {
	if len(sessionToken) < 32 || len(csrf) < 32 {
		return false
	}
	sessionHash, csrfHash := hashSecret(sessionToken), hashSecret(csrf)
	valid, _ := service.store.ValidateCSRF(ctx, sessionHash[:], csrfHash[:])
	return valid
}

func (service *Service) Logout(ctx context.Context, sessionToken string) error {
	digest := hashSecret(sessionToken)
	return service.store.RevokeSession(ctx, digest[:])
}

func (service *Service) ListSessions(ctx context.Context, principal Principal) ([]storepostgres.SessionInfo, error) {
	return service.store.ListSessions(ctx, principal.ID)
}

func (service *Service) RevokeSessionByID(ctx context.Context, principal Principal, sessionID string) (bool, error) {
	return service.store.RevokeSessionByID(ctx, principal.ID, sessionID)
}

func (service *Service) RevokeOtherSessions(ctx context.Context, principal Principal, currentSessionID string) (int64, error) {
	return service.store.RevokeOtherSessions(ctx, principal.ID, currentSessionID)
}

// ── API Keys ──

func (service *Service) CreateAPIKey(ctx context.Context, principal Principal, name, kind string, scopes []string, expiresAt *time.Time) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return "", "", ErrInvalidAPIKey
	}
	if kind != "agent" && kind != "device" && kind != "service" {
		return "", "", ErrInvalidAPIKey
	}
	if len(scopes) == 0 {
		return "", "", ErrInvalidAPIKey
	}
	for _, s := range scopes {
		if s == "*" {
			continue
		}
		if !strings.Contains(s, ".") && s != "*" {
			return "", "", ErrInvalidAPIKey
		}
	}
	rawToken, err := randomSecret(32)
	if err != nil {
		return "", "", err
	}
	tokenValue := fmt.Sprintf("hh_%s_%s", kind, rawToken)
	tokenHash := storepostgres.HashCredential(tokenValue)
	keyID, err := service.store.CreateAPIKey(ctx, principal.ID, name, kind, tokenHash[:], scopes, expiresAt)
	if err != nil {
		return "", "", err
	}
	service.audit(ctx, "api_key.create", "success", "", map[string]any{"key_id": keyID, "kind": kind, "name": name})
	return keyID, tokenValue, nil
}

func (service *Service) ListAPIKeys(ctx context.Context, principal Principal) ([]storepostgres.APIKeyInfo, error) {
	return service.store.ListAPIKeys(ctx, principal.ID)
}

func (service *Service) RevokeAPIKey(ctx context.Context, principal Principal, keyID string) (bool, error) {
	revoked, err := service.store.RevokeAPIKey(ctx, principal.ID, keyID)
	if revoked {
		service.audit(ctx, "api_key.revoke", "success", "", map[string]any{"key_id": keyID})
	}
	return revoked, err
}

func (service *Service) AuthenticateAPIKey(ctx context.Context, tokenHash [32]byte) (*storepostgres.APIKey, error) {
	return service.store.AuthenticateAPIKey(ctx, tokenHash[:])
}

// ── Shares ──

type ShareInput struct {
	ShareType    string   `json:"share_type"`
	ServiceID    string   `json:"service_id"`
	ResourceType string   `json:"resource_type,omitempty"`
	ResourceID   string   `json:"resource_id,omitempty"`
	Actions      []string `json:"actions"`
	ExpiresAt    time.Time `json:"expires_at"`
	MaxUses      *int     `json:"max_uses,omitempty"`
}

func (service *Service) CreateShare(ctx context.Context, principal Principal, input ShareInput) (string, string, error) {
	if input.ServiceID == "" || len(input.Actions) == 0 || !input.ExpiresAt.After(service.now()) {
		return "", "", ErrInvalidShare
	}
	if input.ExpiresAt.After(service.now().Add(365 * 24 * time.Hour)) {
		return "", "", ErrInvalidShare
	}
	if input.ShareType != "service" && input.ShareType != "resource" {
		return "", "", ErrInvalidShare
	}
	rawToken, err := randomSecret(32)
	if err != nil {
		return "", "", err
	}
	tokenHash := storepostgres.HashCredential(rawToken)
	shareID, err := service.store.CreateShare(ctx, principal.ID, tokenHash[:], input.ShareType, input.ServiceID, input.ResourceType, input.ResourceID, input.Actions, input.ExpiresAt, input.MaxUses)
	if err != nil {
		return "", "", err
	}
	service.audit(ctx, "share.create", "success", "", map[string]any{"share_id": shareID, "share_type": input.ShareType, "service_id": input.ServiceID})
	return shareID, rawToken, nil
}

func (service *Service) ListShares(ctx context.Context, principal Principal) ([]storepostgres.ShareInfo, error) {
	return service.store.ListShares(ctx, principal.ID)
}

func (service *Service) RevokeShare(ctx context.Context, principal Principal, shareID string) (bool, error) {
	revoked, err := service.store.RevokeShare(ctx, principal.ID, shareID)
	if revoked {
		service.audit(ctx, "share.revoke", "success", "", map[string]any{"share_id": shareID})
	}
	return revoked, err
}

func (service *Service) RedeemShare(ctx context.Context, shareToken, remoteIP, userAgent string) (Session, error) {
	if len(shareToken) < 32 {
		return Session{}, ErrInvalidShare
	}
	tokenHash := storepostgres.HashCredential(shareToken)
	share, err := service.store.RedeemShare(ctx, tokenHash[:])
	if err != nil {
		return Session{}, ErrInvalidShare
	}
	_ = service.store.IncrementShareUse(ctx, share.ID)
	service.audit(ctx, "share.redeem", "success", remoteIP, map[string]any{"share_id": share.ID, "share_type": share.ShareType})
	return service.createShareSession(ctx, share, remoteIP, userAgent)
}

func (service *Service) createShareSession(ctx context.Context, share *storepostgres.Share, remoteIP, userAgent string) (Session, error) {
	// Create a temporary session for share access
	sessionToken, err := randomSecret(32)
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomSecret(32)
	if err != nil {
		return Session{}, err
	}
	now := service.now().UTC()
	tokenHash := storepostgres.HashCredential(sessionToken)
	csrfHash := storepostgres.HashCredential(csrfToken)
	uaHash := userAgentHash(userAgent)
	// Share sessions are short-lived
	shareSessionTTL := 24 * time.Hour
	if !share.ExpiresAt.IsZero() {
		remaining := time.Until(share.ExpiresAt)
		if remaining < shareSessionTTL {
			shareSessionTTL = remaining
		}
	}
	_, err = service.store.CreateSession(ctx, nil, share.OwnerID, tokenHash[:], csrfHash[:], []string{"share"}, now, shareSessionTTL, shareSessionTTL, remoteIP, uaHash[:])
	if err != nil {
		return Session{}, err
	}
	principal := Principal{
		ID: share.OwnerID, Username: "share", DisplayName: "Shared Access",
		Kind: "guest",
	}
	return Session{Principal: principal, Token: sessionToken, CSRF: csrfToken}, nil
}

// ── JWT Issue ──

func (service *Service) IssueJWT(ctx context.Context, principal Principal, audience string, scopes []string) (TokenResponse, error) {
	if audience == "" {
		audience = "homehub"
	}
	now := service.now().UTC()
	encoded, claims, err := service.signer.Issue(token.IssueRequest{
		Audience:         audience,
		Subject:          "human:" + principal.ID,
		AuthorizedParty:  principal.SessionID,
		Realm:            "homehub",
		Permissions:      scopes,
		SessionID:        principal.SessionID,
		Authentication:   []string{"session"},
		AuthenticationAt: now,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: encoded, TokenType: "Bearer",
		ExpiresIn: int(claims.Expires - claims.IssuedAt),
		Audience: audience, Permissions: claims.Permissions,
	}, nil
}

func (service *Service) IssueAPIKeyJWT(ctx context.Context, key *storepostgres.APIKey, audience string) (TokenResponse, error) {
	if audience == "" {
		audience = "homehub"
	}
	now := service.now().UTC()
	encoded, claims, err := service.signer.Issue(token.IssueRequest{
		Audience:         audience,
		Subject:          "human:" + key.OwnerID,
		AuthorizedParty:  key.ID,
		Realm:            "homehub",
		Permissions:      key.Scopes,
		Authentication:   []string{"api_key"},
		AuthenticationAt: now,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: encoded, TokenType: "Bearer",
		ExpiresIn: int(claims.Expires - claims.IssuedAt),
		Audience: audience, Permissions: claims.Permissions,
	}, nil
}

func (service *Service) IssueShareJWT(ctx context.Context, share *storepostgres.Share, audience string) (TokenResponse, error) {
	if audience == "" {
		audience = "homehub-" + share.ServiceID
	}
	now := service.now().UTC()
	encoded, claims, err := service.signer.Issue(token.IssueRequest{
		Audience:         audience,
		Subject:          "share:" + share.ID,
		AuthorizedParty:  share.ID,
		Realm:            "homehub",
		Permissions:      share.Actions,
		Authentication:   []string{"share"},
		AuthenticationAt: now,
	})
	if err != nil {
		return TokenResponse{}, err
	}
	return TokenResponse{
		AccessToken: encoded, TokenType: "Bearer",
		ExpiresIn: int(claims.Expires - claims.IssuedAt),
		Audience: audience, Permissions: claims.Permissions,
	}, nil
}

// ── Internal helpers ──

func (service *Service) createSession(ctx context.Context, ownerID, username, displayName, kind string, methods []string, remoteIP, userAgent string) (Session, error) {
	tx, err := service.pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	session, err := service.createSessionTx(ctx, tx, ownerID, username, displayName, kind, methods, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (service *Service) createSessionTx(ctx context.Context, tx pgx.Tx, ownerID, username, displayName, kind string, methods []string, remoteIP, userAgent string) (Session, error) {
	sessionToken, err := randomSecret(32)
	if err != nil {
		return Session{}, err
	}
	csrfToken, err := randomSecret(32)
	if err != nil {
		return Session{}, err
	}
	now := service.now().UTC()
	tokenHash := storepostgres.HashCredential(sessionToken)
	csrfHash := storepostgres.HashCredential(csrfToken)
	uaHash := userAgentHash(userAgent)
	_, err = service.store.CreateSession(ctx, tx, ownerID, tokenHash[:], csrfHash[:], methods, now, sessionIdleTTL, sessionAbsoluteTTL, remoteIP, uaHash[:])
	if err != nil {
		return Session{}, err
	}
	principal := Principal{ID: ownerID, Username: username, DisplayName: displayName, Kind: kind}
	return Session{Principal: principal, Token: sessionToken, CSRF: csrfToken}, nil
}

func (service *Service) ensureBootstrap(ctx context.Context, tokenFile string) error {
	if tokenFile == "" {
		return nil
	}
	contents, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil // Token file is optional at startup
	}
	token := strings.TrimSpace(string(contents))
	if token == "" {
		return nil
	}
	digest := hashSecret(token)
	_, err = service.pool.Exec(ctx, `INSERT INTO owner_bootstrap_tokens(token_hash,expires_at)
		VALUES($1,now()+interval '168 hours') ON CONFLICT DO NOTHING`, digest[:])
	return err
}

func (service *Service) audit(ctx context.Context, eventType, outcome, remoteIP string, details map[string]any) {
	if details == nil {
		details = map[string]any{}
	}
	service.store.RecordAudit(ctx, eventType, outcome, remoteIP, details)
}

// ── Encryption ──

func (service *Service) encrypt(plaintext []byte) ([]byte, []byte, error) {
	nonce := make([]byte, service.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, service.aead.Seal(nil, nonce, plaintext, nil), nil
}

func (service *Service) decrypt(nonce, ciphertext []byte) ([]byte, error) {
	return service.aead.Open(nil, nonce, ciphertext, nil)
}

// ── Password ──

func (service *Service) hashPassword(password string) (string, error) {
	service.hashSlots <- struct{}{}
	defer func() { <-service.hashSlots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLength)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		passwordMemory, passwordTime, passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key)), nil
}

func (service *Service) verifyPassword(password, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, timeVal, threads int
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &timeVal, &threads); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	key := argon2.IDKey([]byte(password), salt, uint32(timeVal), uint32(memory), uint8(threads), uint32(len(expected)))
	return subtle.ConstantTimeCompare(key, expected) == 1
}

// ── Token signing relaxed ──

// We override the strict audience validation from the token package

func readKey(path string) ([]byte, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(strings.TrimSpace(string(contents)))
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption key must be 32 bytes")
	}
	return key, nil
}

func validateTOTP(secret, code string, now time.Time) bool {
	if len(code) != 6 {
		return false
	}
	secretBytes, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return false
	}
	counter := uint64(now.Unix() / 30)
	for offset := int64(-1); offset <= 1; offset++ {
		if hotp(secretBytes, counter+uint64(offset)) == code {
			return true
		}
	}
	return false
}

func hotp(key []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	mac.Write(buf[:])
	hash := mac.Sum(nil)
	offset := hash[len(hash)-1] & 0x0f
	value := int(binary.BigEndian.Uint32(hash[offset:offset+4]) & 0x7fffffff)
	return fmt.Sprintf("%06d", value%1_000_000)
}

func hashSecret(value string) [sha256.Size]byte {
	return sha256.Sum256([]byte(value))
}

func randomSecret(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func userAgentHash(userAgent string) [sha256.Size]byte {
	return sha256.Sum256([]byte(userAgent))
}
