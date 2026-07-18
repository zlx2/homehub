package store

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/0001_core.sql
var coreMigration string

type Options struct {
	Host, Port, Database, User, Password string
	DataDirectory                        string
	QuotaBytes                           int64
	Now                                  func() time.Time
}

type Store struct {
	pool       *pgxpool.Pool
	blobs      string
	temporary  string
	quotaBytes int64
	now        func() time.Time
}

func Open(ctx context.Context, options Options) (*Store, error) {
	if options.Host == "" || options.Port == "" || options.Database == "" || options.User == "" || options.Password == "" ||
		options.DataDirectory == "" || options.QuotaBytes <= 0 {
		return nil, ErrInvalidInput
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	configuration, err := pgxpool.ParseConfig("")
	if err != nil {
		return nil, err
	}
	configuration.ConnConfig.Host = options.Host
	configuration.ConnConfig.Port = parsePort(options.Port)
	configuration.ConnConfig.Database = options.Database
	configuration.ConnConfig.User = options.User
	configuration.ConnConfig.Password = options.Password
	configuration.MaxConns = 8
	configuration.MinConns = 1
	configuration.MaxConnLifetime = time.Hour
	pool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		return nil, fmt.Errorf("open Drop database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping Drop database: %w", err)
	}
	if _, err := pool.Exec(ctx, coreMigration); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate Drop database: %w", err)
	}
	root, err := filepath.Abs(options.DataDirectory)
	if err != nil {
		pool.Close()
		return nil, err
	}
	blobs := filepath.Join(root, "blobs")
	temporary := filepath.Join(root, "tmp")
	for _, directory := range []string{root, blobs, temporary} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			pool.Close()
			return nil, fmt.Errorf("create Drop data directory: %w", err)
		}
	}
	return &Store{pool: pool, blobs: blobs, temporary: temporary, quotaBytes: options.QuotaBytes, now: options.Now}, nil
}

func parsePort(value string) uint16 {
	var port uint16
	_, _ = fmt.Sscanf(value, "%d", &port)
	return port
}

func (store *Store) Close()                     { store.pool.Close() }
func (store *Store) TemporaryDirectory() string { return store.temporary }

func (store *Store) Ready(ctx context.Context) error {
	if err := store.pool.Ping(ctx); err != nil {
		return err
	}
	probe, err := os.CreateTemp(store.temporary, "ready-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	_ = probe.Close()
	return os.Remove(name)
}

func (store *Store) Create(ctx context.Context, input CreateInput) (Item, error) {
	if input.CreatorSubject == "" || input.ActorSubject == "" || input.TTL <= 0 ||
		(input.Text == "" && len(input.Attachments) == 0) || (len(input.IdempotencyKey) != 0 && len(input.IdempotencyKey) != 32) {
		return Item{}, ErrInvalidInput
	}
	total := int64(len([]byte(input.Text)))
	for _, attachment := range input.Attachments {
		if attachment.TempPath == "" || attachment.OriginalName == "" || attachment.Size < 0 || len(attachment.SHA256) != 32 {
			return Item{}, ErrInvalidInput
		}
		if attachment.Size > store.quotaBytes-total {
			return Item{}, ErrQuotaExceeded
		}
		total += attachment.Size
	}

	transaction, err := store.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Item{}, fmt.Errorf("begin Drop item creation: %w", err)
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock(298734209)`); err != nil {
		return Item{}, err
	}
	if len(input.IdempotencyKey) == 32 {
		var existingID string
		err := transaction.QueryRow(ctx, `SELECT item_id FROM idempotency_keys WHERE key_hash = $1`, input.IdempotencyKey).Scan(&existingID)
		if err == nil {
			_ = transaction.Rollback(ctx)
			return store.Get(ctx, existingID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return Item{}, err
		}
	}
	var used int64
	if err := transaction.QueryRow(ctx, `SELECT COALESCE(SUM(total_size), 0) FROM items WHERE expires_at > now()`).Scan(&used); err != nil {
		return Item{}, err
	}
	if total > store.quotaBytes || used > store.quotaBytes-total {
		return Item{}, ErrQuotaExceeded
	}

	now := store.now().UTC()
	item := Item{
		ID: randomID(), Text: input.Text, CreatorSubject: input.CreatorSubject, ActorSubject: input.ActorSubject,
		CreatedAt: now, ExpiresAt: now.Add(input.TTL), TotalSize: total,
		Attachments: make([]Attachment, 0, len(input.Attachments)),
	}
	type movedFile struct{ destination string }
	var moved []movedFile
	cleanup := func() {
		for _, file := range moved {
			_ = os.Remove(file.destination)
		}
	}
	for _, pending := range input.Attachments {
		attachment := Attachment{
			ID: randomID(), ItemID: item.ID, OriginalName: pending.OriginalName, StorageName: "blob-" + randomID(),
			MediaType: pending.MediaType, Size: pending.Size, SHA256: append([]byte(nil), pending.SHA256...), CreatedAt: now,
		}
		destination := filepath.Join(store.blobs, attachment.StorageName)
		if err := os.Rename(pending.TempPath, destination); err != nil {
			cleanup()
			return Item{}, fmt.Errorf("store original attachment: %w", err)
		}
		moved = append(moved, movedFile{destination: destination})
		item.Attachments = append(item.Attachments, attachment)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO items(id,text_content,creator_subject,actor_subject,created_at,expires_at,total_size)
		VALUES($1,$2,$3,$4,$5,$6,$7)`, item.ID, item.Text, item.CreatorSubject, item.ActorSubject, item.CreatedAt, item.ExpiresAt, item.TotalSize); err != nil {
		cleanup()
		return Item{}, fmt.Errorf("insert Drop item: %w", err)
	}
	for _, attachment := range item.Attachments {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO attachments(id,item_id,original_name,storage_name,media_type,size,sha256,created_at)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, attachment.ID, attachment.ItemID, attachment.OriginalName,
			attachment.StorageName, attachment.MediaType, attachment.Size, attachment.SHA256, attachment.CreatedAt); err != nil {
			cleanup()
			return Item{}, fmt.Errorf("insert Drop attachment: %w", err)
		}
	}
	if len(input.IdempotencyKey) == 32 {
		if _, err := transaction.Exec(ctx, `INSERT INTO idempotency_keys(key_hash,item_id,created_at) VALUES($1,$2,$3)`, input.IdempotencyKey, item.ID, now); err != nil {
			cleanup()
			return Item{}, err
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		cleanup()
		return Item{}, fmt.Errorf("commit Drop item: %w", err)
	}
	return item, nil
}

func (store *Store) List(ctx context.Context, options ListOptions) ([]Item, error) {
	if options.Limit < 1 || options.Limit > 100 {
		options.Limit = 50
	}
	query := `SELECT id,text_content,creator_subject,actor_subject,created_at,expires_at,total_size
		FROM items WHERE expires_at > now() ORDER BY created_at DESC,id DESC LIMIT $1`
	arguments := []any{options.Limit}
	if !options.Before.IsZero() && options.BeforeID != "" {
		query = `SELECT id,text_content,creator_subject,actor_subject,created_at,expires_at,total_size
			FROM items WHERE expires_at > now() AND (created_at,id) < ($1,$2) ORDER BY created_at DESC,id DESC LIMIT $3`
		arguments = []any{options.Before, options.BeforeID, options.Limit}
	}
	rows, err := store.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Text, &item.CreatorSubject, &item.ActorSubject, &item.CreatedAt, &item.ExpiresAt, &item.TotalSize); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := store.loadAttachments(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (store *Store) Get(ctx context.Context, id string) (Item, error) {
	var item Item
	err := store.pool.QueryRow(ctx, `SELECT id,text_content,creator_subject,actor_subject,created_at,expires_at,total_size
		FROM items WHERE id=$1 AND expires_at > now()`, id).
		Scan(&item.ID, &item.Text, &item.CreatorSubject, &item.ActorSubject, &item.CreatedAt, &item.ExpiresAt, &item.TotalSize)
	if errors.Is(err, pgx.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	items := []Item{item}
	if err := store.loadAttachments(ctx, items); err != nil {
		return Item{}, err
	}
	return items[0], nil
}

func (store *Store) loadAttachments(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	index := make(map[string]int, len(items))
	for position := range items {
		ids[position] = items[position].ID
		index[items[position].ID] = position
		items[position].Attachments = []Attachment{}
	}
	rows, err := store.pool.Query(ctx, `SELECT id,item_id,original_name,storage_name,media_type,size,sha256,created_at
		FROM attachments WHERE item_id=ANY($1) ORDER BY created_at,id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var attachment Attachment
		if err := rows.Scan(&attachment.ID, &attachment.ItemID, &attachment.OriginalName, &attachment.StorageName,
			&attachment.MediaType, &attachment.Size, &attachment.SHA256, &attachment.CreatedAt); err != nil {
			return err
		}
		position, ok := index[attachment.ItemID]
		if ok {
			items[position].Attachments = append(items[position].Attachments, attachment)
		}
	}
	return rows.Err()
}

func (store *Store) OpenAttachment(ctx context.Context, id string) (*os.File, Attachment, error) {
	var attachment Attachment
	err := store.pool.QueryRow(ctx, `SELECT a.id,a.item_id,a.original_name,a.storage_name,a.media_type,a.size,a.sha256,a.created_at
		FROM attachments a JOIN items i ON i.id=a.item_id WHERE a.id=$1 AND i.expires_at > now()`, id).
		Scan(&attachment.ID, &attachment.ItemID, &attachment.OriginalName, &attachment.StorageName,
			&attachment.MediaType, &attachment.Size, &attachment.SHA256, &attachment.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, Attachment{}, ErrNotFound
	}
	if err != nil {
		return nil, Attachment{}, err
	}
	file, err := os.Open(filepath.Join(store.blobs, attachment.StorageName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, Attachment{}, ErrNotFound
	}
	return file, attachment, err
}

func (store *Store) Delete(ctx context.Context, id string) error {
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = transaction.Rollback(ctx) }()
	rows, err := transaction.Query(ctx, `SELECT a.storage_name FROM attachments a JOIN items i ON i.id=a.item_id WHERE i.id=$1 FOR UPDATE`, id)
	if err != nil {
		return err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		names = append(names, name)
	}
	rows.Close()
	command, err := transaction.Exec(ctx, `DELETE FROM items WHERE id=$1`, id)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return ErrNotFound
	}
	type stagedFile struct{ original, temporary string }
	var staged []stagedFile
	restore := func() {
		for index := len(staged) - 1; index >= 0; index-- {
			_ = os.Rename(staged[index].temporary, staged[index].original)
		}
	}
	for _, name := range names {
		original := filepath.Join(store.blobs, name)
		temporary := filepath.Join(store.temporary, "delete-"+randomID())
		if err := os.Rename(original, temporary); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			restore()
			return err
		}
		staged = append(staged, stagedFile{original: original, temporary: temporary})
	}
	if err := transaction.Commit(ctx); err != nil {
		restore()
		return err
	}
	for _, file := range staged {
		_ = os.Remove(file.temporary)
	}
	return nil
}

func (store *Store) CleanupExpired(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		limit = 100
	}
	rows, err := store.pool.Query(ctx, `SELECT id FROM items WHERE expires_at <= now() ORDER BY expires_at LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	deleted := 0
	for _, id := range ids {
		if err := store.Delete(ctx, id); err != nil && !errors.Is(err, ErrNotFound) {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func randomID() string {
	var value [18]byte
	if _, err := rand.Read(value[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(value[:])
}
