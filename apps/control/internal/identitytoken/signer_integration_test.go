package identitytoken

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func TestRunningServiceAcceptsControlIdentity(t *testing.T) {
	target := os.Getenv("HOMEHUB_IDENTITY_SMOKE_URL")
	keyFile := os.Getenv("HOMEHUB_IDENTITY_SMOKE_KEY_FILE")
	if target == "" || keyFile == "" {
		t.Skip("running-service identity smoke test is not configured")
	}
	signer, err := NewFromFile(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	token, err := signer.Issue("identity-smoke", "Identity Smoke Test", []string{"portal.view"}, "drop")
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-HomeHub-Identity", token)
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("service returned status %d", response.StatusCode)
	}
}
