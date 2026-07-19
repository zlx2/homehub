package humanauth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gitee.com/zlx23/homehub/apps/iam/authz"
	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/apps/iam/internal/token"
)

func TestLiveOwnerAndShareLifecycle(t *testing.T) {
	if os.Getenv("HOMEHUB_IAM_LIVE_TEST") != "1" {
		t.Skip("live IAM test is disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	databaseURL := fmt.Sprintf("postgres://homehub_iam:%s@postgres:5432/homehub_iam?sslmode=disable", os.Getenv("V2_IAM_DB_PASSWORD"))
	store, err := storepostgres.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	openFGA, err := authz.NewClient("http://openfga:8080")
	if err != nil {
		t.Fatal(err)
	}
	state, err := openFGA.EnsureModel(ctx, store, "homehub")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.NewSignerFromFile("live-test", "/run/secrets/iam_signing_key", 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	service, err := Open(ctx, Options{DatabaseURL: databaseURL, EncryptionKeyFile: "/run/secrets/auth_encryption_key", BootstrapTokenFile: "/run/secrets/owner_setup_token", Authorization: openFGA, AuthorizationState: state, Policies: store, Signer: signer})
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	required, err := service.SetupRequired(ctx)
	if err != nil || !required {
		t.Fatalf("expected clean owner setup state: required=%v err=%v", required, err)
	}
	bootstrapBytes, err := os.ReadFile("/run/secrets/owner_setup_token")
	if err != nil {
		t.Fatal(err)
	}
	username := fmt.Sprintf("smoke-%d", time.Now().Unix())
	password := "live-smoke-password-only-2026"
	setup, err := service.BeginSetup(ctx, strings.TrimSpace(string(bootstrapBytes)), username, "Smoke Owner", password)
	if err != nil {
		t.Fatal(err)
	}
	code := totpCodeForTest(setup.ManualSecret, time.Now())
	owner, err := service.ConfirmSetup(ctx, setup.ID, code, "127.0.0.1", "homehub-live-test")
	if err != nil {
		t.Fatal(err)
	}
	var guestID string
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		_ = openFGA.DeleteRelationship(cleanupCtx, state, owner.Principal.Subject, "owner", "realm:homehub")
		if guestID != "" {
			_, _ = service.pool.Exec(cleanupCtx, `DELETE FROM principals WHERE id=$1::uuid`, guestID)
		}
		_, _ = service.pool.Exec(cleanupCtx, `DELETE FROM principals WHERE id=$1::uuid`, owner.Principal.ID)
		digest := hashSecret(strings.TrimSpace(string(bootstrapBytes)))
		_, _ = service.pool.Exec(cleanupCtx, `UPDATE owner_bootstrap_tokens SET consumed_at=NULL,expires_at=now()+interval '30 days' WHERE token_hash=$1`, digest[:])
	}()
	authenticated, err := service.Authenticate(ctx, owner.Token)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := service.IsAdministrator(ctx, authenticated)
	if err != nil || !admin {
		t.Fatalf("owner is not administrator: %v", err)
	}
	dropToken, err := service.Issue(ctx, authenticated, "homehub-drop", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, permission := range []string{"drop.item.create", "drop.item.read", "drop.item.list", "drop.item.delete"} {
		if !contains(dropToken.Permissions, permission) {
			t.Fatalf("owner token lacks %s", permission)
		}
	}
	share, err := service.CreateShare(ctx, authenticated, []Grant{{ServiceID: "drop", Relation: "viewer"}}, time.Now().Add(time.Hour), "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	guest, err := service.RedeemShare(ctx, share.Token, "127.0.0.1", "homehub-live-test")
	if err != nil {
		t.Fatal(err)
	}
	guestID = guest.Principal.ID
	guestToken, err := service.Issue(ctx, guest.Principal, "homehub-drop", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(guestToken.Permissions, "drop.item.read") || !contains(guestToken.Permissions, "drop.item.list") || contains(guestToken.Permissions, "drop.item.delete") {
		t.Fatalf("unexpected guest permissions: %v", guestToken.Permissions)
	}
	if _, err := service.RevokeShare(ctx, authenticated, share.ID, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, guest.Token); err == nil {
		t.Fatal("revoked guest session remains valid")
	}
	if err := service.Logout(ctx, owner.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, owner.Token); err == nil {
		t.Fatal("logged out owner session remains valid")
	}
	login, err := service.Login(ctx, username, password, totpCodeForTest(setup.ManualSecret, time.Now()), "127.0.0.1", "homehub-live-test")
	if err != nil {
		t.Fatal(err)
	}
	_ = service.Logout(ctx, login.Token)
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func totpCodeForTest(secret string, now time.Time) string {
	for code := 0; code < 1_000_000; code++ {
		candidate := fmt.Sprintf("%06d", code)
		if validateTOTP(secret, candidate, now) {
			return candidate
		}
	}
	return ""
}
