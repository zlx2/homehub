package auth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"net"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/argon2"
)

//go:embed schema.sql
var schemaFS embed.FS

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidBootstrap   = errors.New("invalid bootstrap token")
	ErrSetupUnavailable   = errors.New("setup is unavailable")
	ErrInvalidTOTP        = errors.New("invalid totp code")
	ErrRateLimited        = errors.New("too many login attempts")
)

const (
	passwordTime    = uint32(3)
	passwordMemory  = uint32(64 * 1024)
	passwordThreads = uint8(2)
	passwordKeyLen  = uint32(32)
	setupTTL        = 15 * time.Minute
	sessionIdleTTL  = 12 * time.Hour
	sessionTTL      = 7 * 24 * time.Hour
)

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,64}$`)

type Config struct {
	Host               string
	Port               string
	Database           string
	User               string
	PasswordFile       string
	EncryptionKeyFile  string
	BootstrapTokenFile string
}

type Service struct {
	pool          *pgxpool.Pool
	aead          cipher.AEAD
	dummyPassword string
	hashSlots     chan struct{}
}

type Principal struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Scopes      []string `json:"scopes"`
}

type PrincipalSummary struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	Scopes      []string  `json:"scopes"`
	CreatedAt   time.Time `json:"created_at"`
}

type ServiceGrant struct {
	ID          string     `json:"id"`
	PrincipalID string     `json:"principal_id"`
	Username    string     `json:"username"`
	ServiceID   string     `json:"service_id"`
	GrantedBy   *string    `json:"granted_by,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Setup struct {
	ID              string    `json:"setup_id"`
	ManualSecret    string    `json:"manual_secret"`
	ProvisioningURI string    `json:"provisioning_uri"`
	QRCodeDataURL   string    `json:"qr_data_url"`
	ExpiresAt       time.Time `json:"expires_at"`
}

type Session struct {
	Principal Principal
	Token     string
	CSRF      string
}

func Open(ctx context.Context, cfg Config) (*Service, error) {
	password, err := readSecret(cfg.PasswordFile, 8)
	if err != nil {
		return nil, fmt.Errorf("read database password: %w", err)
	}
	key, err := readKey(cfg.EncryptionKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read auth encryption key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create auth cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create auth AEAD: %w", err)
	}

	pgConfig, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, fmt.Errorf("create database config: %w", err)
	}
	pgConfig.ConnConfig.Host = cfg.Host
	pgConfig.ConnConfig.Port = parsePort(cfg.Port)
	pgConfig.ConnConfig.Database = cfg.Database
	pgConfig.ConnConfig.User = cfg.User
	pgConfig.ConnConfig.Password = password
	pgConfig.MaxConns = 8
	pgConfig.MinConns = 1
	pgConfig.MaxConnLifetime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, pgConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect database: %w", err)
	}
	service := &Service{pool: pool, aead: aead, hashSlots: make(chan struct{}, 2)}
	service.dummyPassword, err = service.hashPassword("homehub-dummy-password-never-valid")
	if err != nil {
		pool.Close()
		return nil, err
	}
	if err := service.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := service.seedBootstrap(ctx, cfg.BootstrapTokenFile); err != nil {
		pool.Close()
		return nil, err
	}
	return service, nil
}

func (s *Service) Close() { s.pool.Close() }

func (s *Service) migrate(ctx context.Context) error {
	schema, err := schemaFS.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock(468657392)"); err != nil {
		return fmt.Errorf("lock migrations: %w", err)
	}
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock(468657392)")
	if _, err := conn.Conn().PgConn().Exec(ctx, string(schema)).ReadAll(); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	return nil
}

func (s *Service) seedBootstrap(ctx context.Context, path string) error {
	var owners int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM principals WHERE status = 'active'").Scan(&owners); err != nil {
		return err
	}
	if owners > 0 {
		return nil
	}
	token, err := readSecret(path, 32)
	if err != nil {
		return fmt.Errorf("read owner setup token: %w", err)
	}
	hash := tokenHash(token)
	_, err = s.pool.Exec(ctx, `INSERT INTO bootstrap_tokens(token_hash, expires_at)
		VALUES ($1, now() + interval '24 hours') ON CONFLICT (token_hash) DO NOTHING`, hash[:])
	return err
}

func (s *Service) SetupRequired(ctx context.Context) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM principals WHERE status = 'active')").Scan(&exists)
	return !exists, err
}

func (s *Service) BeginSetup(ctx context.Context, bootstrapToken, username, password string) (Setup, error) {
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return Setup{}, fmt.Errorf("username must be 3-64 letters, digits, dot, dash, or underscore")
	}
	if len(password) < 12 || len(password) > 256 {
		return Setup{}, fmt.Errorf("password must contain 12-256 characters")
	}
	required, err := s.SetupRequired(ctx)
	if err != nil {
		return Setup{}, err
	}
	if !required {
		return Setup{}, ErrSetupUnavailable
	}
	bootstrapHash := tokenHash(strings.TrimSpace(bootstrapToken))
	var valid bool
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM bootstrap_tokens
		WHERE token_hash=$1 AND consumed_at IS NULL AND expires_at > now())`, bootstrapHash[:]).Scan(&valid)
	if err != nil || !valid {
		return Setup{}, ErrInvalidBootstrap
	}

	passwordHash, err := s.hashPassword(password)
	if err != nil {
		return Setup{}, err
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "HomeHub", AccountName: username})
	if err != nil {
		return Setup{}, fmt.Errorf("generate TOTP secret: %w", err)
	}
	nonce, encrypted, err := s.encrypt([]byte(key.Secret()))
	if err != nil {
		return Setup{}, err
	}
	expiresAt := time.Now().UTC().Add(setupTTL)
	var setupID string
	err = s.pool.QueryRow(ctx, `INSERT INTO setup_attempts
		(bootstrap_token_hash, username, password_hash, totp_secret_cipher, totp_secret_nonce, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id`, bootstrapHash[:], username, passwordHash, encrypted, nonce, expiresAt).Scan(&setupID)
	if err != nil {
		return Setup{}, err
	}
	image, err := key.Image(256, 256)
	if err != nil {
		return Setup{}, fmt.Errorf("render TOTP QR code: %w", err)
	}
	var qr bytes.Buffer
	if err := png.Encode(&qr, image); err != nil {
		return Setup{}, fmt.Errorf("encode TOTP QR code: %w", err)
	}
	return Setup{
		ID: setupID, ManualSecret: key.Secret(), ProvisioningURI: key.URL(), ExpiresAt: expiresAt,
		QRCodeDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr.Bytes()),
	}, nil
}

func (s *Service) ConfirmSetup(ctx context.Context, setupID, code, remoteIP, userAgent string) (Session, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var username, passwordHash string
	var encrypted, nonce, bootstrapHash []byte
	err = tx.QueryRow(ctx, `SELECT username,password_hash,totp_secret_cipher,totp_secret_nonce,bootstrap_token_hash
		FROM setup_attempts WHERE id=$1 AND expires_at > now() FOR UPDATE`, setupID).
		Scan(&username, &passwordHash, &encrypted, &nonce, &bootstrapHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSetupUnavailable
	}
	if err != nil {
		return Session{}, err
	}
	var ownerExists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM principals WHERE status='active')").Scan(&ownerExists); err != nil {
		return Session{}, err
	}
	if ownerExists {
		return Session{}, ErrSetupUnavailable
	}
	secret, err := s.decrypt(nonce, encrypted)
	if err != nil {
		return Session{}, err
	}
	if !totp.Validate(strings.TrimSpace(code), string(secret)) {
		return Session{}, ErrInvalidTOTP
	}

	var principal Principal
	err = tx.QueryRow(ctx, `INSERT INTO principals(username,display_name,status) VALUES($1,$1,'active') RETURNING id,username,display_name`, username).
		Scan(&principal.ID, &principal.Username, &principal.DisplayName)
	if err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO credentials(principal_id,password_hash,totp_secret_cipher,totp_secret_nonce)
		VALUES($1,$2,$3,$4)`, principal.ID, passwordHash, encrypted, nonce); err != nil {
		return Session{}, err
	}
	principal.Scopes = []string{"admin", "portal.view"}
	for _, scope := range principal.Scopes {
		if _, err := tx.Exec(ctx, "INSERT INTO principal_scopes(principal_id,scope) VALUES($1,$2)", principal.ID, scope); err != nil {
			return Session{}, err
		}
	}
	if _, err := tx.Exec(ctx, "UPDATE bootstrap_tokens SET consumed_at=now() WHERE token_hash=$1", bootstrapHash); err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM setup_attempts WHERE id=$1", setupID); err != nil {
		return Session{}, err
	}
	session, err := createSessionTx(ctx, tx, principal, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip) VALUES($1,'owner_setup','success',$2)`, principal.ID, nullableIP(remoteIP)); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Login(ctx context.Context, username, password, code, remoteIP, userAgent string) (Session, error) {
	username = strings.TrimSpace(username)
	subject := sha256.Sum256([]byte(strings.ToLower(username)))
	var recent int
	if err := s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE event_type='login' AND outcome='failure'
		AND created_at > now() - interval '15 minutes' AND (subject_hash=$1 OR remote_ip=$2)`, subject[:], nullableIP(remoteIP)).Scan(&recent); err != nil {
		return Session{}, err
	}
	if recent >= 5 {
		return Session{}, ErrRateLimited
	}

	var principal Principal
	var storedHash string
	var encrypted, nonce []byte
	err := s.pool.QueryRow(ctx, `SELECT p.id,p.username,p.display_name,c.password_hash,c.totp_secret_cipher,c.totp_secret_nonce
		FROM principals p JOIN credentials c ON c.principal_id=p.id
		WHERE lower(p.username)=lower($1) AND p.status='active'`, username).
		Scan(&principal.ID, &principal.Username, &principal.DisplayName, &storedHash, &encrypted, &nonce)
	if errors.Is(err, pgx.ErrNoRows) {
		storedHash = s.dummyPassword
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return Session{}, err
	}
	passwordOK := s.verifyPassword(password, storedHash)
	totpOK := false
	if passwordOK && principal.ID != "" {
		secret, decryptErr := s.decrypt(nonce, encrypted)
		if decryptErr != nil {
			return Session{}, decryptErr
		}
		totpOK = totp.Validate(strings.TrimSpace(code), string(secret))
	}
	if !passwordOK || !totpOK || principal.ID == "" {
		_, _ = s.pool.Exec(ctx, `INSERT INTO audit_events(event_type,outcome,remote_ip,subject_hash) VALUES('login','failure',$1,$2)`, nullableIP(remoteIP), subject[:])
		return Session{}, ErrInvalidCredentials
	}
	rows, err := s.pool.Query(ctx, "SELECT scope FROM principal_scopes WHERE principal_id=$1 ORDER BY scope", principal.ID)
	if err != nil {
		return Session{}, err
	}
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			rows.Close()
			return Session{}, err
		}
		principal.Scopes = append(principal.Scopes, scope)
	}
	rows.Close()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)
	session, err := createSessionTx(ctx, tx, principal, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash) VALUES($1,'login','success',$2,$3)`, principal.ID, nullableIP(remoteIP), subject[:]); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}

func (s *Service) Authenticate(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrInvalidCredentials
	}
	hash := tokenHash(token)
	var principal Principal
	err := s.pool.QueryRow(ctx, `UPDATE sessions s SET last_seen_at=now(), idle_expires_at=LEAST(now()+interval '12 hours', absolute_expires_at)
		FROM principals p WHERE s.principal_id=p.id AND s.token_hash=$1 AND s.revoked_at IS NULL
		AND s.idle_expires_at>now() AND s.absolute_expires_at>now() AND p.status='active'
		RETURNING p.id,p.username,p.display_name`, hash[:]).Scan(&principal.ID, &principal.Username, &principal.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return Principal{}, ErrInvalidCredentials
	}
	if err != nil {
		return Principal{}, err
	}
	rows, err := s.pool.Query(ctx, "SELECT scope FROM principal_scopes WHERE principal_id=$1 ORDER BY scope", principal.ID)
	if err != nil {
		return Principal{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return Principal{}, err
		}
		principal.Scopes = append(principal.Scopes, scope)
	}
	return principal, rows.Err()
}

func (s *Service) ValidateCSRF(ctx context.Context, token, csrf string) bool {
	if token == "" || csrf == "" {
		return false
	}
	tokenDigest, csrfDigest := tokenHash(token), tokenHash(csrf)
	var valid bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM sessions WHERE token_hash=$1 AND csrf_hash=$2
		AND revoked_at IS NULL AND idle_expires_at>now() AND absolute_expires_at>now())`, tokenDigest[:], csrfDigest[:]).Scan(&valid)
	return err == nil && valid
}

func (s *Service) Logout(ctx context.Context, token string) error {
	hash := tokenHash(token)
	_, err := s.pool.Exec(ctx, "UPDATE sessions SET revoked_at=now() WHERE token_hash=$1 AND revoked_at IS NULL", hash[:])
	return err
}

func HasScope(principal Principal, required string) bool {
	for _, scope := range principal.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func (s *Service) ActiveServiceIDs(ctx context.Context, principalID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `SELECT service_id FROM service_grants
		WHERE principal_id=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>now())`, principalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	serviceIDs := make(map[string]struct{})
	for rows.Next() {
		var serviceID string
		if err := rows.Scan(&serviceID); err != nil {
			return nil, err
		}
		serviceIDs[serviceID] = struct{}{}
	}
	return serviceIDs, rows.Err()
}

func (s *Service) ListPrincipals(ctx context.Context) ([]PrincipalSummary, error) {
	rows, err := s.pool.Query(ctx, `SELECT p.id,p.username,p.display_name,p.status,
		COALESCE(array_agg(s.scope ORDER BY s.scope) FILTER (WHERE s.scope IS NOT NULL),'{}'),p.created_at
		FROM principals p LEFT JOIN principal_scopes s ON s.principal_id=p.id
		GROUP BY p.id ORDER BY lower(p.username)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	principals := make([]PrincipalSummary, 0)
	for rows.Next() {
		var principal PrincipalSummary
		if err := rows.Scan(&principal.ID, &principal.Username, &principal.DisplayName, &principal.Status, &principal.Scopes, &principal.CreatedAt); err != nil {
			return nil, err
		}
		principals = append(principals, principal)
	}
	return principals, rows.Err()
}

func (s *Service) PrincipalExists(ctx context.Context, principalID string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals WHERE id=$1 AND status='active')`, principalID).Scan(&exists)
	return exists, err
}

func (s *Service) ListServiceGrants(ctx context.Context) ([]ServiceGrant, error) {
	rows, err := s.pool.Query(ctx, `SELECT g.id,g.principal_id,p.username,g.service_id,g.granted_by,g.expires_at,g.created_at,g.updated_at
		FROM service_grants g JOIN principals p ON p.id=g.principal_id
		WHERE g.revoked_at IS NULL AND (g.expires_at IS NULL OR g.expires_at>now())
		ORDER BY lower(p.username),g.service_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	grants := make([]ServiceGrant, 0)
	for rows.Next() {
		var grant ServiceGrant
		if err := rows.Scan(&grant.ID, &grant.PrincipalID, &grant.Username, &grant.ServiceID, &grant.GrantedBy, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (s *Service) GrantService(ctx context.Context, actor, principalID, serviceID string, expiresAt *time.Time, remoteIP string) (ServiceGrant, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ServiceGrant{}, err
	}
	defer tx.Rollback(ctx)
	var grant ServiceGrant
	err = tx.QueryRow(ctx, `INSERT INTO service_grants(principal_id,service_id,granted_by,expires_at)
		VALUES($1,$2,$3,$4)
		ON CONFLICT (principal_id,service_id) WHERE revoked_at IS NULL
		DO UPDATE SET granted_by=EXCLUDED.granted_by,expires_at=EXCLUDED.expires_at,updated_at=now()
		RETURNING id,principal_id,service_id,granted_by,expires_at,created_at,updated_at`, principalID, serviceID, actor, expiresAt).
		Scan(&grant.ID, &grant.PrincipalID, &grant.ServiceID, &grant.GrantedBy, &grant.ExpiresAt, &grant.CreatedAt, &grant.UpdatedAt)
	if err != nil {
		return ServiceGrant{}, err
	}
	if err := tx.QueryRow(ctx, "SELECT username FROM principals WHERE id=$1", principalID).Scan(&grant.Username); err != nil {
		return ServiceGrant{}, err
	}
	subject := tokenHash(principalID + ":" + serviceID)
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'service_grant','success',$2,$3)`, actor, nullableIP(remoteIP), subject[:])
	if err != nil {
		return ServiceGrant{}, err
	}
	return grant, tx.Commit(ctx)
}

func (s *Service) RevokeServiceGrant(ctx context.Context, actor, grantID, remoteIP string) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE service_grants SET revoked_at=now(),updated_at=now()
		WHERE id=$1 AND revoked_at IS NULL`, grantID)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 0 {
		return false, nil
	}
	subject := tokenHash(grantID)
	_, err = tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'service_grant_revoke','success',$2,$3)`, actor, nullableIP(remoteIP), subject[:])
	if err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func createSessionTx(ctx context.Context, tx pgx.Tx, principal Principal, remoteIP, userAgent string) (Session, error) {
	token, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	csrf, err := randomToken(32)
	if err != nil {
		return Session{}, err
	}
	tokenDigest, csrfDigest := tokenHash(token), tokenHash(csrf)
	userAgentHash := sha256.Sum256([]byte(userAgent))
	_, err = tx.Exec(ctx, `INSERT INTO sessions(principal_id,token_hash,csrf_hash,idle_expires_at,absolute_expires_at,remote_ip,user_agent_hash)
		VALUES($1,$2,$3,now()+interval '12 hours',now()+interval '7 days',$4,$5)`, principal.ID, tokenDigest[:], csrfDigest[:], nullableIP(remoteIP), userAgentHash[:])
	return Session{Principal: principal, Token: token, CSRF: csrf}, err
}

func (s *Service) encrypt(plaintext []byte) ([]byte, []byte, error) {
	nonce := make([]byte, s.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	return nonce, s.aead.Seal(nil, nonce, plaintext, nil), nil
}

func (s *Service) decrypt(nonce, ciphertext []byte) ([]byte, error) {
	return s.aead.Open(nil, nonce, ciphertext, nil)
}

func (s *Service) hashPassword(password string) (string, error) {
	s.hashSlots <- struct{}{}
	defer func() { <-s.hashSlots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, passwordTime, passwordMemory, passwordThreads, passwordKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", passwordMemory, passwordTime, passwordThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(hash)), nil
}

func (s *Service) verifyPassword(password, encoded string) bool {
	s.hashSlots <- struct{}{}
	defer func() { <-s.hashSlots }()
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	var memory, iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false
	}
	if memory > passwordMemory || iterations > 10 || threads > 8 {
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
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(expected)))
	return subtleEqual(actual, expected)
}

func subtleEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := range a {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

func randomToken(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func tokenHash(value string) [32]byte { return sha256.Sum256([]byte(value)) }

func readSecret(path string, minimum int) (string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(value))
	if len(secret) < minimum {
		return "", fmt.Errorf("secret in %s is too short", path)
	}
	return secret, nil
}

func readKey(path string) ([]byte, error) {
	encoded, err := readSecret(path, 43)
	if err != nil {
		return nil, err
	}
	key, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		key, err = base64.RawURLEncoding.DecodeString(encoded)
	}
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("auth key must be 32-byte unpadded base64")
	}
	return key, nil
}

func parsePort(value string) uint16 {
	var port uint16 = 5432
	_, _ = fmt.Sscanf(value, "%d", &port)
	return port
}

func nullableIP(value string) any {
	ip := net.ParseIP(value)
	if ip == nil {
		return nil
	}
	return ip.String()
}
