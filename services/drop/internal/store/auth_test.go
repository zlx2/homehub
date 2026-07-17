package store

import (
	"context"
	"testing"
	"time"
)

func TestRedeemAuthCodeRollsBackConsumptionWhenSessionInsertFails(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage := openTestStore(t, 1024, 64, func() time.Time { return now })
	codeOne := []byte("code-one-hash")
	codeTwo := []byte("code-two-hash")
	sessionHash := []byte("duplicate-session-hash")
	for _, code := range [][]byte{codeOne, codeTwo} {
		if err := storage.CreateAuthCode(ctx, code, now, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	metadata := SessionMetadata{DeviceName: "Via · iPhone", LastIP: "203.0.113.5"}
	if err := storage.RedeemAuthCode(ctx, codeOne, sessionHash, now, now.Add(time.Hour), metadata); err != nil {
		t.Fatalf("first RedeemAuthCode() error = %v", err)
	}
	if err := storage.RedeemAuthCode(ctx, codeTwo, sessionHash, now, now.Add(time.Hour), metadata); err == nil {
		t.Fatal("RedeemAuthCode() accepted a duplicate session hash")
	}
	if err := storage.RedeemAuthCode(ctx, codeTwo, []byte("new-session-hash"), now, now.Add(time.Hour), metadata); err != nil {
		t.Fatalf("code was consumed by rolled-back redemption: %v", err)
	}
}

func TestTrustedSessionsCanBeListedTouchedAndRevoked(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage := openTestStore(t, 1024, 64, func() time.Time { return now })
	codeHash := []byte("device-code-hash")
	tokenHash := []byte("device-token-hash")
	if err := storage.CreateAuthCode(ctx, codeHash, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := storage.RedeemAuthCode(ctx, codeHash, tokenHash, now, now.Add(24*time.Hour), SessionMetadata{
		DeviceName: "Via · iPhone", LastIP: "203.0.113.5",
	}); err != nil {
		t.Fatal(err)
	}

	session, valid, err := storage.SessionByToken(ctx, tokenHash, now.Add(6*time.Minute), "198.51.100.9")
	if err != nil || !valid || session.DeviceName != "Via · iPhone" || session.LastIP != "198.51.100.9" {
		t.Fatalf("touched session = %#v, %t, %v", session, valid, err)
	}
	sessions, err := storage.ListSessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].LastIP != "198.51.100.9" {
		t.Fatalf("listed sessions = %#v, %v", sessions, err)
	}
	revoked, err := storage.RevokeSession(ctx, sessions[0].ID)
	if err != nil || !revoked {
		t.Fatalf("revoke session = %t, %v", revoked, err)
	}
	if _, valid, err := storage.SessionByToken(ctx, tokenHash, now, ""); err != nil || valid {
		t.Fatalf("revoked session valid = %t, %v", valid, err)
	}
}
