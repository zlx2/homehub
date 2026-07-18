package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsDuplicateServices(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "catalog.json")
	contents := `{"version":1,"services":[` +
		`{"id":"iam","name":"IAM","kind":"control-plane","health_url":"http://iam/health","path":"/iam","visibility":"owner"},` +
		`{"id":"iam","name":"Again","kind":"control-plane","health_url":"http://iam/health","path":"/iam","visibility":"owner"}]}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected duplicate rejection")
	}
}
