package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pquerna/otp/totp"
)

var (
	ErrInvalidInvitation   = errors.New("invalid or expired invitation")
	ErrInvitationClaimed   = errors.New("invitation enrollment already started")
	ErrUsernameUnavailable = errors.New("username is unavailable")
)

type Invitation struct {
	ID         string     `json:"id"`
	CreatedBy  string     `json:"created_by"`
	ServiceIDs []string   `json:"service_ids"`
	ExpiresAt  time.Time  `json:"expires_at"`
	ConsumedAt *time.Time `json:"consumed_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreatedInvitation struct {
	Invitation
	Token string `json:"token"`
}

func (s *Service) CreateInvitation(ctx context.Context, actor string, serviceIDs []string, expiresAt time.Time, remoteIP string) (CreatedInvitation, error) {
	if !expiresAt.After(time.Now().UTC()) {
		return CreatedInvitation{}, fmt.Errorf("invitation expiry must be in the future")
	}
	token, err := randomToken(32)
	if err != nil {
		return CreatedInvitation{}, err
	}
	digest := tokenHash(token)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return CreatedInvitation{}, err
	}
	defer tx.Rollback(ctx)

	invitation := CreatedInvitation{Token: token}
	err = tx.QueryRow(ctx, `INSERT INTO invitations(token_hash,created_by,expires_at)
		VALUES($1,$2,$3) RETURNING id,created_by,expires_at,created_at`, digest[:], actor, expiresAt).
		Scan(&invitation.ID, &invitation.CreatedBy, &invitation.ExpiresAt, &invitation.CreatedAt)
	if err != nil {
		return CreatedInvitation{}, err
	}
	invitation.ServiceIDs = append([]string(nil), serviceIDs...)
	for _, serviceID := range serviceIDs {
		if _, err := tx.Exec(ctx, `INSERT INTO invitation_services(invitation_id,service_id) VALUES($1,$2)`, invitation.ID, serviceID); err != nil {
			return CreatedInvitation{}, err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'invitation_create','success',$2,$3)`, actor, nullableIP(remoteIP), digest[:]); err != nil {
		return CreatedInvitation{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return CreatedInvitation{}, err
	}
	return invitation, nil
}

func (s *Service) ListInvitations(ctx context.Context) ([]Invitation, error) {
	rows, err := s.pool.Query(ctx, `SELECT i.id,i.created_by,
		COALESCE(array_agg(s.service_id ORDER BY s.service_id) FILTER (WHERE s.service_id IS NOT NULL),'{}'),
		i.expires_at,i.consumed_at,i.revoked_at,i.created_at
		FROM invitations i LEFT JOIN invitation_services s ON s.invitation_id=i.id
		GROUP BY i.id ORDER BY i.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	invitations := make([]Invitation, 0)
	for rows.Next() {
		var invitation Invitation
		if err := rows.Scan(&invitation.ID, &invitation.CreatedBy, &invitation.ServiceIDs, &invitation.ExpiresAt,
			&invitation.ConsumedAt, &invitation.RevokedAt, &invitation.CreatedAt); err != nil {
			return nil, err
		}
		invitations = append(invitations, invitation)
	}
	return invitations, rows.Err()
}

func (s *Service) RevokeInvitation(ctx context.Context, actor, invitationID, remoteIP string) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `UPDATE invitations SET revoked_at=now()
		WHERE id=$1 AND consumed_at IS NULL AND revoked_at IS NULL`, invitationID)
	if err != nil {
		return false, err
	}
	if result.RowsAffected() == 0 {
		return false, nil
	}
	subject := tokenHash(invitationID)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'invitation_revoke','success',$2,$3)`, actor, nullableIP(remoteIP), subject[:]); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s *Service) BeginInvitationEnrollment(ctx context.Context, token, username, password, remoteIP string) (Setup, error) {
	token = strings.TrimSpace(token)
	username = strings.TrimSpace(username)
	if !usernamePattern.MatchString(username) {
		return Setup{}, fmt.Errorf("username must be 3-64 letters, digits, dot, dash, or underscore")
	}
	if len(password) < 12 || len(password) > 256 {
		return Setup{}, fmt.Errorf("password must contain 12-256 characters")
	}
	digest := tokenHash(token)
	var valid bool
	if len(token) >= 32 {
		err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invitations
			WHERE token_hash=$1 AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at>now())`, digest[:]).Scan(&valid)
		if err != nil {
			return Setup{}, err
		}
	}
	if !valid {
		return Setup{}, ErrInvalidInvitation
	}
	var claimed bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM invitation_attempts a
		JOIN invitations i ON i.id=a.invitation_id
		WHERE i.token_hash=$1 AND a.expires_at>now())`, digest[:]).Scan(&claimed); err != nil {
		return Setup{}, err
	}
	if claimed {
		return Setup{}, ErrInvitationClaimed
	}
	var usernameExists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM principals WHERE lower(username)=lower($1))`, username).Scan(&usernameExists); err != nil {
		return Setup{}, err
	}
	if usernameExists {
		return Setup{}, ErrUsernameUnavailable
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
	image, err := key.Image(256, 256)
	if err != nil {
		return Setup{}, fmt.Errorf("render TOTP QR code: %w", err)
	}
	var qr bytes.Buffer
	if err := png.Encode(&qr, image); err != nil {
		return Setup{}, fmt.Errorf("encode TOTP QR code: %w", err)
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Setup{}, err
	}
	defer tx.Rollback(ctx)
	var invitationID string
	err = tx.QueryRow(ctx, `SELECT id FROM invitations WHERE token_hash=$1
		AND consumed_at IS NULL AND revoked_at IS NULL AND expires_at>now() FOR UPDATE`, digest[:]).Scan(&invitationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Setup{}, ErrInvalidInvitation
	}
	if err != nil {
		return Setup{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM invitation_attempts WHERE invitation_id=$1 AND expires_at<=now()`, invitationID); err != nil {
		return Setup{}, err
	}
	expiresAt := time.Now().UTC().Add(setupTTL)
	var setupID string
	err = tx.QueryRow(ctx, `INSERT INTO invitation_attempts
		(invitation_id,username,password_hash,totp_secret_cipher,totp_secret_nonce,expires_at)
		VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (invitation_id) DO NOTHING RETURNING id`,
		invitationID, username, passwordHash, encrypted, nonce, expiresAt).Scan(&setupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Setup{}, ErrInvitationClaimed
	}
	if err != nil {
		return Setup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Setup{}, err
	}

	return Setup{
		ID: setupID, ManualSecret: key.Secret(), ProvisioningURI: key.URL(), ExpiresAt: expiresAt,
		QRCodeDataURL: "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr.Bytes()),
	}, nil
}

func (s *Service) ConfirmInvitationEnrollment(ctx context.Context, setupID, code, remoteIP, userAgent string) (Session, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var invitationID, username, passwordHash, createdBy string
	var encrypted, nonce []byte
	err = tx.QueryRow(ctx, `SELECT a.invitation_id,a.username,a.password_hash,a.totp_secret_cipher,a.totp_secret_nonce,i.created_by
		FROM invitation_attempts a JOIN invitations i ON i.id=a.invitation_id
		WHERE a.id=$1 AND a.expires_at>now() AND a.failed_attempts<5
		AND i.consumed_at IS NULL AND i.revoked_at IS NULL AND i.expires_at>now()
		FOR UPDATE OF a,i`, setupID).Scan(&invitationID, &username, &passwordHash, &encrypted, &nonce, &createdBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidInvitation
	}
	if err != nil {
		return Session{}, err
	}
	secret, err := s.decrypt(nonce, encrypted)
	if err != nil {
		return Session{}, err
	}
	if !totp.Validate(strings.TrimSpace(code), string(secret)) {
		if _, err := tx.Exec(ctx, `UPDATE invitation_attempts SET failed_attempts=failed_attempts+1 WHERE id=$1`, setupID); err != nil {
			return Session{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return Session{}, err
		}
		return Session{}, ErrInvalidTOTP
	}

	principal := Principal{Username: username, DisplayName: username, Scopes: []string{"portal.view"}}
	err = tx.QueryRow(ctx, `INSERT INTO principals(username,display_name,status) VALUES($1,$1,'active') RETURNING id`, username).Scan(&principal.ID)
	if err != nil {
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) && pgError.Code == "23505" {
			return Session{}, ErrUsernameUnavailable
		}
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO credentials(principal_id,password_hash,totp_secret_cipher,totp_secret_nonce)
		VALUES($1,$2,$3,$4)`, principal.ID, passwordHash, encrypted, nonce); err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO principal_scopes(principal_id,scope) VALUES($1,'portal.view')`, principal.ID); err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO service_grants(principal_id,service_id,granted_by)
		SELECT $1,service_id,$2 FROM invitation_services WHERE invitation_id=$3`, principal.ID, createdBy, invitationID); err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `UPDATE invitations SET consumed_at=now() WHERE id=$1`, invitationID); err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM invitation_attempts WHERE id=$1`, setupID); err != nil {
		return Session{}, err
	}
	session, err := createSessionTx(ctx, tx, principal, remoteIP, userAgent)
	if err != nil {
		return Session{}, err
	}
	subject := tokenHash(invitationID)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'invitation_registration','success',$2,$3)`, principal.ID, nullableIP(remoteIP), subject[:]); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}
