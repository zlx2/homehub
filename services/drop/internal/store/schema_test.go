package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestVersionTwoMigrationBackfillsStorageUsage(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	storage, err := Open(ctx, Options{DataDir: dataDir, QuotaBytes: 1024, InlineTextBytes: 64})
	if err != nil {
		t.Fatal(err)
	}
	path := stageFile(t, storage.TmpDir(), "text-*", []byte("hello"))
	if _, err := storage.CreateItem(ctx, CreateItemInput{TextTempPath: path, TextSize: 5, Source: "owner", TTL: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dataDir, "drop.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP TRIGGER storage_usage_item_insert`,
		`DROP TRIGGER storage_usage_item_delete`,
		`DROP TRIGGER storage_usage_attachment_insert`,
		`DROP TRIGGER storage_usage_attachment_delete`,
		`DROP TABLE storage_usage`,
		`DROP TABLE idempotency_keys`,
		`DELETE FROM schema_migrations`,
		`INSERT INTO schema_migrations(version, applied_at) VALUES(2, 0)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			t.Fatalf("legacy setup %q: %v", statement, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(ctx, Options{DataDir: dataDir, QuotaBytes: 1024, InlineTextBytes: 64})
	if err != nil {
		t.Fatalf("Open() migrated database: %v", err)
	}
	defer func() { _ = migrated.Close() }()
	usage, err := migrated.Usage(ctx)
	if err != nil || usage.UsedBytes != 5 || usage.ItemCount != 1 {
		t.Fatalf("migrated usage = %#v, %v", usage, err)
	}
}

func TestVersionThreeMigrationAddsTrustedSessionMetadata(t *testing.T) {
	ctx := context.Background()
	dataDir := t.TempDir()
	storage, err := Open(ctx, Options{DataDir: dataDir, QuotaBytes: 1024, InlineTextBytes: 64})
	if err != nil {
		t.Fatal(err)
	}
	if err := storage.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", filepath.Join(dataDir, "drop.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`DROP TABLE sessions`,
		`CREATE TABLE sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash BLOB NOT NULL UNIQUE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL
		)`,
		`CREATE INDEX idx_sessions_expires_at ON sessions(expires_at)`,
		`INSERT INTO sessions(token_hash, created_at, expires_at) VALUES(X'0102', 1000, 9999999999999)`,
		`DELETE FROM schema_migrations`,
		`INSERT INTO schema_migrations(version, applied_at) VALUES(3, 0)`,
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			_ = db.Close()
			t.Fatalf("legacy setup %q: %v", statement, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	migrated, err := Open(ctx, Options{DataDir: dataDir, QuotaBytes: 1024, InlineTextBytes: 64})
	if err != nil {
		t.Fatalf("Open() migrated database: %v", err)
	}
	defer func() { _ = migrated.Close() }()
	sessions, err := migrated.ListSessions(ctx)
	if err != nil || len(sessions) != 1 || sessions[0].DeviceName != "已授权设备" || sessions[0].LastSeenAt.UnixMilli() != 1000 {
		t.Fatalf("migrated sessions = %#v, %v", sessions, err)
	}
}
