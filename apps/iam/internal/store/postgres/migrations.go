package postgres

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

func migrateEmbedded(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin IAM migration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(681246021)`); err != nil {
		return fmt.Errorf("lock IAM migrations: %w", err)
	}
	if _, err := tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS iam_schema_migrations (version bigint PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create IAM migration ledger: %w", err)
	}

	entries, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("list IAM migrations: %w", err)
	}
	sort.Strings(entries)
	for _, entry := range entries {
		version, err := migrationVersion(entry)
		if err != nil {
			return err
		}
		var applied bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM iam_schema_migrations WHERE version = $1)`, version).Scan(&applied); err != nil {
			return fmt.Errorf("check IAM migration %d: %w", version, err)
		}
		if applied {
			continue
		}
		contents, err := migrationFiles.ReadFile(entry)
		if err != nil {
			return fmt.Errorf("read IAM migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, string(contents)); err != nil {
			return fmt.Errorf("apply IAM migration %d: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO iam_schema_migrations(version) VALUES ($1)`, version); err != nil {
			return fmt.Errorf("record IAM migration %d: %w", version, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit IAM migrations: %w", err)
	}
	return nil
}

func migrationVersion(path string) (int64, error) {
	name := path[strings.LastIndexByte(path, '/')+1:]
	prefix, _, ok := strings.Cut(name, "_")
	if !ok {
		return 0, fmt.Errorf("invalid IAM migration filename %q", path)
	}
	version, err := strconv.ParseInt(prefix, 10, 64)
	if err != nil || version < 1 {
		return 0, fmt.Errorf("invalid IAM migration filename %q", path)
	}
	return version, nil
}
