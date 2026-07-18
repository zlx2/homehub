package postgres

import "testing"

func TestMigrationVersion(t *testing.T) {
	t.Parallel()

	version, err := migrationVersion("migrations/0001_core.sql")
	if err != nil {
		t.Fatalf("migrationVersion() error = %v", err)
	}
	if version != 1 {
		t.Fatalf("migrationVersion() = %d, want 1", version)
	}

	for _, path := range []string{"migrations/core.sql", "migrations/0000_core.sql", "migrations/nope"} {
		if _, err := migrationVersion(path); err == nil {
			t.Errorf("migrationVersion(%q) unexpectedly succeeded", path)
		}
	}
}
