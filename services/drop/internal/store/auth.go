package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateAuthCode(ctx context.Context, hash []byte, createdAt, expiresAt time.Time) error {
	if len(hash) == 0 || !expiresAt.After(createdAt) {
		return ErrInvalidInput
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO auth_codes(token_hash, created_at, expires_at) VALUES(?, ?, ?)`,
		hash, millis(createdAt), millis(expiresAt))
	if err != nil {
		return fmt.Errorf("create auth code: %w", err)
	}
	return nil
}

func (s *Store) RedeemAuthCode(ctx context.Context, codeHash, sessionHash []byte, now, sessionExpiresAt time.Time, metadata SessionMetadata) error {
	if len(codeHash) == 0 || len(sessionHash) == 0 || !sessionExpiresAt.After(now) {
		return ErrInvalidInput
	}
	if metadata.DeviceName == "" {
		metadata.DeviceName = "已授权设备"
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth code consumption: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE auth_codes SET used_at = ?
		WHERE token_hash = ? AND used_at IS NULL AND expires_at > ?`, millis(now), codeHash, millis(now))
	if err != nil {
		return fmt.Errorf("consume auth code: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read auth code result: %w", err)
	}
	if changed != 1 {
		return ErrCodeInvalid
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO sessions
		(token_hash, created_at, expires_at, device_name, last_seen_at, last_ip) VALUES(?, ?, ?, ?, ?, ?)`,
		sessionHash, millis(now), millis(sessionExpiresAt), metadata.DeviceName, millis(now), metadata.LastIP); err != nil {
		return fmt.Errorf("create redeemed session: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth code redemption: %w", err)
	}
	return nil
}

func (s *Store) SessionByToken(ctx context.Context, hash []byte, now time.Time, lastIP string) (TrustedSession, bool, error) {
	var session TrustedSession
	var created, lastSeen, expires int64
	err := s.db.QueryRowContext(ctx, `SELECT id, device_name, created_at, last_seen_at, expires_at, last_ip
		FROM sessions WHERE token_hash = ? AND expires_at > ?`, hash, millis(now)).Scan(
		&session.ID, &session.DeviceName, &created, &lastSeen, &expires, &session.LastIP,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedSession{}, false, nil
	}
	if err != nil {
		return TrustedSession{}, false, fmt.Errorf("validate session: %w", err)
	}
	session.CreatedAt = fromMillis(created)
	session.LastSeenAt = fromMillis(lastSeen)
	session.ExpiresAt = fromMillis(expires)
	if now.Sub(session.LastSeenAt) >= 5*time.Minute {
		if _, err := s.db.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, last_ip = ? WHERE id = ?`,
			millis(now), lastIP, session.ID); err != nil {
			return TrustedSession{}, false, fmt.Errorf("update session activity: %w", err)
		}
		session.LastSeenAt = now.UTC()
		session.LastIP = lastIP
	}
	return session, true, nil
}

func (s *Store) ListSessions(ctx context.Context) ([]TrustedSession, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, device_name, created_at, last_seen_at, expires_at, last_ip
		FROM sessions WHERE expires_at > ? ORDER BY last_seen_at DESC, id DESC`, millis(s.now().UTC()))
	if err != nil {
		return nil, fmt.Errorf("list trusted sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var sessions []TrustedSession
	for rows.Next() {
		var session TrustedSession
		var created, lastSeen, expires int64
		if err := rows.Scan(&session.ID, &session.DeviceName, &created, &lastSeen, &expires, &session.LastIP); err != nil {
			return nil, fmt.Errorf("scan trusted session: %w", err)
		}
		session.CreatedAt = fromMillis(created)
		session.LastSeenAt = fromMillis(lastSeen)
		session.ExpiresAt = fromMillis(expires)
		sessions = append(sessions, session)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted sessions: %w", err)
	}
	return sessions, nil
}

func (s *Store) RevokeSession(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, ErrInvalidInput
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("revoke trusted session: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("read revoke result: %w", err)
	}
	return changed == 1, nil
}

func (s *Store) RevokeAllSessions(ctx context.Context) (int64, error) {
	result, err := s.db.ExecContext(ctx, `DELETE FROM sessions`)
	if err != nil {
		return 0, fmt.Errorf("revoke all trusted sessions: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read revoke all result: %w", err)
	}
	return changed, nil
}

func (s *Store) PurgeExpiredAuth(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin auth cleanup: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM auth_codes WHERE expires_at <= ? OR used_at IS NOT NULL`, millis(now)); err != nil {
		return fmt.Errorf("delete expired auth codes: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, millis(now)); err != nil {
		return fmt.Errorf("delete expired sessions: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit auth cleanup: %w", err)
	}
	return nil
}
