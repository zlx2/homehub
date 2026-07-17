package auth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"drop/internal/store"
)

func TestCodeIsSingleUseAndSessionHasFixedExpiry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage, err := store.Open(ctx, store.Options{
		DataDir: t.TempDir(), QuotaBytes: 1024, InlineTextBytes: 64, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	service, err := NewService(storage, Options{
		CodeTTL: 30 * time.Minute, SessionTTL: 12 * time.Hour, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}

	code, err := service.GenerateCode(ctx)
	if err != nil {
		t.Fatalf("GenerateCode() error = %v", err)
	}
	if code.ExpiresAt.Sub(now) != 30*time.Minute || code.Value == "" {
		t.Fatalf("GenerateCode() = %#v", code)
	}

	var successes atomic.Int32
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			if _, err := service.RedeemCode(ctx, code.Value, store.SessionMetadata{DeviceName: "Safari · iPhone"}); err == nil {
				successes.Add(1)
			} else if !errors.Is(err, store.ErrCodeInvalid) {
				t.Errorf("RedeemCode() unexpected error = %v", err)
			}
		})
	}
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successful redemptions = %d, want 1", successes.Load())
	}

	secondCode, err := service.GenerateCode(ctx)
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.RedeemCode(ctx, secondCode.Value, store.SessionMetadata{DeviceName: "Via · iPhone", LastIP: "203.0.113.5"})
	if err != nil {
		t.Fatal(err)
	}
	if session.ExpiresAt.Sub(now) != 12*time.Hour {
		t.Fatalf("session expiry = %v", session.ExpiresAt)
	}
	trusted, valid, err := service.ValidateSession(ctx, session.Token, "203.0.113.5")
	if err != nil || !valid {
		t.Fatalf("ValidateSession() = %t, %v", valid, err)
	}
	if trusted.DeviceName != "Via · iPhone" || trusted.LastIP != "203.0.113.5" {
		t.Fatalf("trusted session = %#v", trusted)
	}
	now = now.Add(12 * time.Hour)
	_, valid, err = service.ValidateSession(ctx, session.Token, "203.0.113.5")
	if err != nil || valid {
		t.Fatalf("expired ValidateSession() = %t, %v", valid, err)
	}
}

func TestExpiredCodeIsRejected(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage, err := store.Open(ctx, store.Options{
		DataDir: t.TempDir(), QuotaBytes: 1024, InlineTextBytes: 64, Now: func() time.Time { return now },
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	service, _ := NewService(storage, Options{
		CodeTTL: time.Minute, SessionTTL: time.Hour, Now: func() time.Time { return now },
	})
	code, _ := service.GenerateCode(ctx)
	now = now.Add(time.Minute)
	if _, err := service.RedeemCode(ctx, code.Value, store.SessionMetadata{}); !errors.Is(err, store.ErrCodeInvalid) {
		t.Fatalf("RedeemCode() error = %v", err)
	}
}
