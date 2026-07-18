package token

import (
	"crypto/ed25519"
	"testing"
	"time"

	"homehub.local/go-sdk/identity"
)

func TestSignerIssuesAudienceBoundDelegatedToken(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, _ := ed25519.GenerateKey(nil)
	signer, err := NewSigner("key-1", privateKey, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	signer.now = func() time.Time { return now }

	encoded, issued, err := signer.Issue(IssueRequest{
		Audience: "homehub-drop", Subject: "human:luna", Actor: "agent:hermes",
		AuthorizedParty: "hermes", Realm: "homehub",
		Permissions:  []string{"drop.item.create", "drop.item.create", "drop.item.delete"},
		DelegationID: "delegation-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Expires-issued.IssuedAt != 120 || len(issued.Permissions) != 2 || issued.EffectiveActor() != "agent:hermes" {
		t.Fatalf("unexpected issued claims: %+v", issued)
	}

	verifier, err := identity.NewVerifier(map[string]ed25519.PublicKey{"key-1": publicKey}, "homehub-drop", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	verified, err := verifier.Verify(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Subject != "human:luna" || verified.DelegationID != "delegation-1" {
		t.Fatalf("unexpected verified claims: %+v", verified)
	}
}

func TestSignerRejectsInvalidIdentityAndPermissions(t *testing.T) {
	t.Parallel()
	_, privateKey, _ := ed25519.GenerateKey(nil)
	signer, _ := NewSigner("key-1", privateKey, time.Minute)

	requests := []IssueRequest{
		{Audience: "drop", Subject: "human:luna", AuthorizedParty: "portal", Realm: "homehub", Permissions: []string{"drop.item.read"}},
		{Audience: "homehub-drop", Subject: "root", AuthorizedParty: "portal", Realm: "homehub", Permissions: []string{"drop.item.read"}},
		{Audience: "homehub-drop", Subject: "human:luna", Actor: "root", AuthorizedParty: "portal", Realm: "homehub", Permissions: []string{"drop.item.read"}},
		{Audience: "homehub-drop", Subject: "human:luna", AuthorizedParty: "portal", Realm: "homehub", Permissions: []string{"drop.*"}},
	}
	for index, request := range requests {
		if _, _, err := signer.Issue(request); err == nil {
			t.Errorf("request %d unexpectedly succeeded", index)
		}
	}
}
