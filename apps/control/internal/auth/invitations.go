package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ErrInvalidInvitation = errors.New("invalid or expired invitation")

type Invitation struct {
	ID               string     `json:"id"`
	CreatedBy        string     `json:"created_by"`
	GuestPrincipalID *string    `json:"guest_principal_id,omitempty"`
	ServiceIDs       []string   `json:"service_ids"`
	ExpiresAt        time.Time  `json:"expires_at"`
	ConsumedAt       *time.Time `json:"consumed_at,omitempty"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
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
	rows, err := s.pool.Query(ctx, `SELECT i.id,i.created_by,i.guest_principal_id,
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
		if err := rows.Scan(&invitation.ID, &invitation.CreatedBy, &invitation.GuestPrincipalID, &invitation.ServiceIDs, &invitation.ExpiresAt,
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
	var guestPrincipalID *string
	err = tx.QueryRow(ctx, `UPDATE invitations SET revoked_at=now()
		WHERE id=$1 AND revoked_at IS NULL RETURNING guest_principal_id`, invitationID).Scan(&guestPrincipalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if guestPrincipalID != nil {
		if _, err := tx.Exec(ctx, `UPDATE principals SET status='disabled' WHERE id=$1`, *guestPrincipalID); err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `UPDATE sessions SET revoked_at=now() WHERE principal_id=$1 AND revoked_at IS NULL`, *guestPrincipalID); err != nil {
			return false, err
		}
		if _, err := tx.Exec(ctx, `UPDATE service_grants SET revoked_at=now(),updated_at=now()
			WHERE principal_id=$1 AND revoked_at IS NULL`, *guestPrincipalID); err != nil {
			return false, err
		}
	}
	subject := tokenHash(invitationID)
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'invitation_revoke','success',$2,$3)`, actor, nullableIP(remoteIP), subject[:]); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s *Service) RedeemInvitation(ctx context.Context, token, remoteIP, userAgent string) (Session, error) {
	token = strings.TrimSpace(token)
	if len(token) < 32 {
		return Session{}, ErrInvalidInvitation
	}
	digest := tokenHash(token)
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Session{}, err
	}
	defer tx.Rollback(ctx)

	var invitationID, createdBy string
	var expiresAt time.Time
	var guestPrincipalID *string
	err = tx.QueryRow(ctx, `SELECT id,created_by,expires_at,guest_principal_id
		FROM invitations WHERE token_hash=$1 AND revoked_at IS NULL AND expires_at>now()
		AND (consumed_at IS NULL OR guest_principal_id IS NOT NULL)
		FOR UPDATE`, digest[:]).Scan(&invitationID, &createdBy, &expiresAt, &guestPrincipalID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrInvalidInvitation
	}
	if err != nil {
		return Session{}, err
	}

	principal := Principal{DisplayName: "分享访客", Scopes: []string{"portal.view"}}
	if guestPrincipalID == nil {
		compactID := strings.ReplaceAll(invitationID, "-", "")
		principal.Username = "share-" + compactID[:12]
		err = tx.QueryRow(ctx, `INSERT INTO principals(username,display_name,status)
			VALUES($1,$2,'active') RETURNING id`, principal.Username, principal.DisplayName).Scan(&principal.ID)
		if err != nil {
			return Session{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO principal_scopes(principal_id,scope) VALUES($1,'portal.view')`, principal.ID); err != nil {
			return Session{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO service_grants(principal_id,service_id,granted_by,expires_at)
			SELECT $1,service_id,$2,$3 FROM invitation_services WHERE invitation_id=$4`, principal.ID, createdBy, expiresAt, invitationID); err != nil {
			return Session{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE invitations SET guest_principal_id=$1,consumed_at=COALESCE(consumed_at,now()) WHERE id=$2`, principal.ID, invitationID); err != nil {
			return Session{}, err
		}
	} else {
		principal.ID = *guestPrincipalID
		err = tx.QueryRow(ctx, `SELECT username,display_name FROM principals WHERE id=$1 AND status='active'`, principal.ID).
			Scan(&principal.Username, &principal.DisplayName)
		if errors.Is(err, pgx.ErrNoRows) {
			return Session{}, ErrInvalidInvitation
		}
		if err != nil {
			return Session{}, err
		}
	}

	session, err := createSessionUntilTx(ctx, tx, principal, remoteIP, userAgent, expiresAt)
	if err != nil {
		return Session{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_events(principal_id,event_type,outcome,remote_ip,subject_hash)
		VALUES($1,'share_link_redeem','success',$2,$3)`, principal.ID, nullableIP(remoteIP), digest[:]); err != nil {
		return Session{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Session{}, err
	}
	return session, nil
}
