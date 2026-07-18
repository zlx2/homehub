package humanauth

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/jackc/pgx/v5"
)

const passkeyCeremonyTTL = 5 * time.Minute

type webAuthnUser struct {
	Principal
	Credentials []webauthn.Credential
}

func (user webAuthnUser) WebAuthnID() []byte { return []byte(user.ID) }
func (user webAuthnUser) WebAuthnName() string {
	if user.Username != "" {
		return user.Username
	}
	return user.DisplayName
}
func (user webAuthnUser) WebAuthnDisplayName() string                { return user.DisplayName }
func (user webAuthnUser) WebAuthnCredentials() []webauthn.Credential { return user.Credentials }

type PasskeyCredential struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

func (service *Service) BeginPasskeyRegistration(ctx context.Context, principal Principal) (any, string, error) {
	if service.passkeys == nil || principal.Kind != "human" {
		return nil, "", ErrForbidden
	}
	user, err := service.loadWebAuthnUser(ctx, principal.ID)
	if err != nil {
		return nil, "", err
	}
	creation, session, err := service.passkeys.BeginRegistration(user,
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithAuthenticatorSelection(protocol.AuthenticatorSelection{UserVerification: protocol.VerificationRequired}),
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
	)
	if err != nil {
		return nil, "", err
	}
	tokenValue, err := service.storePasskeyCeremony(ctx, "registration", principal.ID, session)
	return creation, tokenValue, err
}

func (service *Service) FinishPasskeyRegistration(ctx context.Context, principal Principal, ceremonyToken, name string, request *http.Request) error {
	if service.passkeys == nil || principal.Kind != "human" {
		return ErrForbidden
	}
	session, err := service.takePasskeyCeremony(ctx, "registration", principal.ID, ceremonyToken)
	if err != nil {
		return err
	}
	user, err := service.loadWebAuthnUser(ctx, principal.ID)
	if err != nil {
		return err
	}
	credential, err := service.passkeys.FinishRegistration(user, *session, request)
	if err != nil {
		return ErrPasskey
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	nonce, cipherText, err := service.encrypt(encoded)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Bitwarden Passkey"
	}
	if len(name) > 80 {
		return ErrPasskey
	}
	_, err = service.pool.Exec(ctx, `INSERT INTO webauthn_credentials(credential_id,principal_id,name,credential_cipher,credential_nonce)
		VALUES($1,$2::uuid,$3,$4,$5)`, credential.ID, principal.ID, name, cipherText, nonce)
	if err != nil {
		return ErrPasskey
	}
	service.audit(ctx, principal.ID, "passkey.register", "success", "", nil)
	return nil
}

func (service *Service) BeginPasskeyLogin(ctx context.Context) (any, string, error) {
	if service.passkeys == nil {
		return nil, "", ErrPasskey
	}
	assertion, session, err := service.passkeys.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return nil, "", err
	}
	tokenValue, err := service.storePasskeyCeremony(ctx, "login", "", session)
	return assertion, tokenValue, err
}

func (service *Service) FinishPasskeyLogin(ctx context.Context, ceremonyToken, remoteIP, userAgent string, request *http.Request) (Session, error) {
	if service.passkeys == nil {
		return Session{}, ErrPasskey
	}
	sessionData, err := service.takePasskeyCeremony(ctx, "login", "", ceremonyToken)
	if err != nil {
		return Session{}, err
	}
	var selected *webAuthnUser
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		user, loadErr := service.loadWebAuthnUser(ctx, string(userHandle))
		if loadErr != nil {
			return nil, loadErr
		}
		matched := false
		for index := range user.Credentials {
			if subtle.ConstantTimeCompare(user.Credentials[index].ID, rawID) == 1 {
				matched = true
				break
			}
		}
		if !matched {
			return nil, ErrInvalidCredentials
		}
		selected = &user
		return user, nil
	}
	credential, err := service.passkeys.FinishDiscoverableLogin(handler, *sessionData, request)
	if err != nil || selected == nil {
		service.audit(ctx, "", "passkey.login", "denied", remoteIP, nil)
		return Session{}, ErrInvalidCredentials
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return Session{}, err
	}
	nonce, cipherText, err := service.encrypt(encoded)
	if err != nil {
		return Session{}, err
	}
	transaction, err := service.pool.Begin(ctx)
	if err != nil {
		return Session{}, err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	result, err := transaction.Exec(ctx, `UPDATE webauthn_credentials SET credential_cipher=$2,credential_nonce=$3,last_used_at=now()
		WHERE credential_id=$1 AND principal_id=$4::uuid`, credential.ID, cipherText, nonce, selected.ID)
	if err != nil || result.RowsAffected() != 1 {
		return Session{}, ErrInvalidCredentials
	}
	created, err := createSession(ctx, transaction, selected.Principal, []string{"passkey"}, service.now().UTC(), sessionAbsoluteTTL, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return Session{}, err
	}
	service.audit(ctx, selected.ID, "passkey.login", "success", remoteIP, nil)
	return created, nil
}

func (service *Service) ListPasskeys(ctx context.Context, principal Principal) ([]PasskeyCredential, error) {
	rows, err := service.pool.Query(ctx, `SELECT credential_id,name,created_at,last_used_at FROM webauthn_credentials
		WHERE principal_id=$1::uuid ORDER BY created_at`, principal.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]PasskeyCredential, 0)
	for rows.Next() {
		var id []byte
		var item PasskeyCredential
		if err := rows.Scan(&id, &item.Name, &item.CreatedAt, &item.LastUsedAt); err != nil {
			return nil, err
		}
		item.ID = base64.RawURLEncoding.EncodeToString(id)
		result = append(result, item)
	}
	return result, rows.Err()
}

func (service *Service) DeletePasskey(ctx context.Context, principal Principal, encodedID string) (bool, error) {
	id, err := base64.RawURLEncoding.DecodeString(encodedID)
	if err != nil || len(id) == 0 || len(id) > 1024 {
		return false, ErrPasskey
	}
	result, err := service.pool.Exec(ctx, `DELETE FROM webauthn_credentials WHERE credential_id=$1 AND principal_id=$2::uuid`, id, principal.ID)
	if err == nil && result.RowsAffected() == 1 {
		service.audit(ctx, principal.ID, "passkey.delete", "success", "", nil)
		return true, nil
	}
	return false, err
}

func (service *Service) loadWebAuthnUser(ctx context.Context, principalID string) (webAuthnUser, error) {
	var user webAuthnUser
	err := service.pool.QueryRow(ctx, `SELECT p.id::text,p.kind,p.display_name,r.slug,COALESCE(e.attributes->>'username',e.external_subject,'')
		FROM principals p JOIN realms r ON r.id=p.realm_id LEFT JOIN external_accounts e ON e.principal_id=p.id AND e.provider='homehub-username'
		WHERE p.id=$1::uuid AND p.kind='human' AND p.status='active' AND p.deleted_at IS NULL`, principalID).
		Scan(&user.ID, &user.Kind, &user.DisplayName, &user.Realm, &user.Username)
	if errors.Is(err, pgx.ErrNoRows) {
		return webAuthnUser{}, ErrInvalidCredentials
	}
	if err != nil {
		return webAuthnUser{}, err
	}
	user.Subject = "human:" + user.ID
	rows, err := service.pool.Query(ctx, `SELECT credential_cipher,credential_nonce FROM webauthn_credentials WHERE principal_id=$1::uuid`, user.ID)
	if err != nil {
		return webAuthnUser{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var cipherText, nonce []byte
		if err := rows.Scan(&cipherText, &nonce); err != nil {
			return webAuthnUser{}, err
		}
		plain, err := service.decrypt(nonce, cipherText)
		if err != nil {
			return webAuthnUser{}, err
		}
		var credential webauthn.Credential
		if err := json.Unmarshal(plain, &credential); err != nil {
			return webAuthnUser{}, err
		}
		user.Credentials = append(user.Credentials, credential)
	}
	return user, rows.Err()
}

func (service *Service) storePasskeyCeremony(ctx context.Context, kind, principalID string, session *webauthn.SessionData) (string, error) {
	encoded, err := json.Marshal(session)
	if err != nil {
		return "", err
	}
	nonce, cipherText, err := service.encrypt(encoded)
	if err != nil {
		return "", err
	}
	tokenValue, err := randomSecret(32)
	if err != nil {
		return "", err
	}
	digest := hashSecret(tokenValue)
	_, _ = service.pool.Exec(ctx, `DELETE FROM webauthn_ceremonies WHERE expires_at<=now()`)
	_, err = service.pool.Exec(ctx, `INSERT INTO webauthn_ceremonies(token_hash,principal_id,kind,session_cipher,session_nonce,expires_at)
		VALUES($1,NULLIF($2,'')::uuid,$3,$4,$5,$6)`, digest[:], principalID, kind, cipherText, nonce, service.now().UTC().Add(passkeyCeremonyTTL))
	return tokenValue, err
}

func (service *Service) takePasskeyCeremony(ctx context.Context, kind, principalID, tokenValue string) (*webauthn.SessionData, error) {
	if len(tokenValue) < 32 {
		return nil, ErrPasskey
	}
	digest := hashSecret(tokenValue)
	var cipherText, nonce []byte
	var err error
	if principalID == "" {
		err = service.pool.QueryRow(ctx, `DELETE FROM webauthn_ceremonies WHERE token_hash=$1 AND kind=$2 AND principal_id IS NULL AND expires_at>now()
			RETURNING session_cipher,session_nonce`, digest[:], kind).Scan(&cipherText, &nonce)
	} else {
		err = service.pool.QueryRow(ctx, `DELETE FROM webauthn_ceremonies WHERE token_hash=$1 AND kind=$2 AND principal_id=$3::uuid AND expires_at>now()
			RETURNING session_cipher,session_nonce`, digest[:], kind, principalID).Scan(&cipherText, &nonce)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrPasskey
	}
	if err != nil {
		return nil, err
	}
	plain, err := service.decrypt(nonce, cipherText)
	if err != nil {
		return nil, ErrPasskey
	}
	var session webauthn.SessionData
	if json.Unmarshal(plain, &session) != nil {
		return nil, ErrPasskey
	}
	return &session, nil
}
