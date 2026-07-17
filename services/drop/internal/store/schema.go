package store

import (
	"context"
	"database/sql"
	"fmt"
)

const schemaVersion = 4

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	statements := []string{
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id TEXT PRIMARY KEY,
			text_inline BLOB,
			text_storage TEXT,
			text_size INTEGER NOT NULL CHECK (text_size >= 0),
			source TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			total_size INTEGER NOT NULL CHECK (total_size >= 0),
			CHECK (text_inline IS NULL OR text_storage IS NULL)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_created_at ON items(created_at DESC, id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_items_expires_at ON items(expires_at)`,
		`CREATE TABLE IF NOT EXISTS attachments (
			id TEXT PRIMARY KEY,
			item_id TEXT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			original_name TEXT NOT NULL,
			storage_name TEXT NOT NULL UNIQUE,
			mime_type TEXT NOT NULL,
			size INTEGER NOT NULL CHECK (size >= 0),
			created_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_attachments_item_id ON attachments(item_id)`,
		`CREATE TABLE IF NOT EXISTS storage_usage (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			used_bytes INTEGER NOT NULL CHECK (used_bytes >= 0),
			item_count INTEGER NOT NULL CHECK (item_count >= 0),
			attachment_count INTEGER NOT NULL CHECK (attachment_count >= 0)
		)`,
		`INSERT INTO storage_usage(id, used_bytes, item_count, attachment_count)
			SELECT 1, COALESCE(SUM(total_size), 0), COUNT(*), (SELECT COUNT(*) FROM attachments) FROM items WHERE 1
			ON CONFLICT(id) DO NOTHING`,
		`CREATE TRIGGER IF NOT EXISTS storage_usage_item_insert AFTER INSERT ON items BEGIN
			UPDATE storage_usage SET used_bytes = used_bytes + NEW.total_size, item_count = item_count + 1 WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS storage_usage_item_delete AFTER DELETE ON items BEGIN
			UPDATE storage_usage SET used_bytes = used_bytes - OLD.total_size, item_count = item_count - 1 WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS storage_usage_attachment_insert AFTER INSERT ON attachments BEGIN
			UPDATE storage_usage SET attachment_count = attachment_count + 1 WHERE id = 1;
		END`,
		`CREATE TRIGGER IF NOT EXISTS storage_usage_attachment_delete AFTER DELETE ON attachments BEGIN
			UPDATE storage_usage SET attachment_count = attachment_count - 1 WHERE id = 1;
		END`,
		`CREATE TABLE IF NOT EXISTS auth_codes (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash BLOB NOT NULL UNIQUE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			used_at INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_auth_codes_expires_at ON auth_codes(expires_at)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			token_hash BLOB NOT NULL UNIQUE,
			created_at INTEGER NOT NULL,
			expires_at INTEGER NOT NULL,
			device_name TEXT NOT NULL DEFAULT '已授权设备',
			last_seen_at INTEGER NOT NULL DEFAULT 0,
			last_ip TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at)`,
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			key_hash BLOB PRIMARY KEY,
			item_id TEXT NOT NULL UNIQUE REFERENCES items(id) ON DELETE CASCADE,
			created_at INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS traffic_hourly (
			hour INTEGER NOT NULL,
			entry TEXT NOT NULL,
			category TEXT NOT NULL,
			bytes INTEGER NOT NULL CHECK (bytes >= 0),
			requests INTEGER NOT NULL CHECK (requests >= 0),
			PRIMARY KEY (hour, entry, category)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_traffic_hourly_hour ON traffic_hourly(hour)`,
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply schema: %w", err)
		}
	}

	var current int
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current)
	if err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if current > schemaVersion {
		return fmt.Errorf("database schema %d is newer than supported version %d", current, schemaVersion)
	}
	if current > 0 && current < 4 {
		columns, err := tableColumns(ctx, tx, "sessions")
		if err != nil {
			return fmt.Errorf("inspect trusted sessions: %w", err)
		}
		for name, definition := range map[string]string{
			"device_name":  `TEXT NOT NULL DEFAULT '已授权设备'`,
			"last_seen_at": `INTEGER NOT NULL DEFAULT 0`,
			"last_ip":      `TEXT NOT NULL DEFAULT ''`,
		} {
			if columns[name] {
				continue
			}
			if _, err := tx.ExecContext(ctx, `ALTER TABLE sessions ADD COLUMN `+name+` `+definition); err != nil {
				return fmt.Errorf("migrate trusted sessions: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE sessions SET last_seen_at = created_at WHERE last_seen_at = 0`); err != nil {
			return fmt.Errorf("backfill trusted session activity: %w", err)
		}
	}
	if current < schemaVersion {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations(version, applied_at) VALUES(?, unixepoch('subsec') * 1000)`,
			schemaVersion,
		); err != nil {
			return fmt.Errorf("record schema version: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

func tableColumns(ctx context.Context, tx *sql.Tx, table string) (map[string]bool, error) {
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	columns := make(map[string]bool)
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, columnType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}
