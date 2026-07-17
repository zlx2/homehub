package store

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Options struct {
	DataDir         string
	QuotaBytes      int64
	InlineTextBytes int64
	Now             func() time.Time
}

type Store struct {
	db              *sql.DB
	dataDir         string
	blobsDir        string
	tmpDir          string
	quotaBytes      int64
	inlineTextBytes int64
	now             func() time.Time
	mutationMu      sync.Mutex
	trafficMu       sync.Mutex
	trafficFlushMu  sync.Mutex
	trafficPending  map[trafficKey]trafficSample
}

type trafficKey struct {
	hour            int64
	entry, category string
}

type trafficSample struct {
	bytes, requests int64
}

func Open(ctx context.Context, opts Options) (*Store, error) {
	if opts.DataDir == "" || opts.QuotaBytes <= 0 || opts.InlineTextBytes <= 0 {
		return nil, fmt.Errorf("%w: invalid store options", ErrInvalidInput)
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}

	dataDir, err := filepath.Abs(opts.DataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve data directory: %w", err)
	}
	blobsDir := filepath.Join(dataDir, "blobs")
	tmpDir := filepath.Join(dataDir, "tmp")
	for _, dir := range []string{dataDir, blobsDir, tmpDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create %s: %w", dir, err)
		}
	}

	dbPath := filepath.Join(dataDir, "drop.db")
	dsn := (&url.URL{
		Scheme: "file",
		Path:   filepath.ToSlash(dbPath),
		RawQuery: url.Values{
			"_pragma": []string{"busy_timeout(5000)", "foreign_keys(1)", "journal_mode(WAL)"},
		}.Encode(),
	}).String()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{
		db:              db,
		dataDir:         dataDir,
		blobsDir:        blobsDir,
		tmpDir:          tmpDir,
		quotaBytes:      opts.QuotaBytes,
		inlineTextBytes: opts.InlineTextBytes,
		now:             opts.Now,
		trafficPending:  make(map[trafficKey]trafficSample),
	}, nil
}

func (s *Store) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return errors.Join(s.FlushTraffic(ctx), s.db.Close())
}

func (s *Store) TmpDir() string { return s.tmpDir }

func (s *Store) Ready(ctx context.Context) error {
	var value int
	if err := s.db.QueryRowContext(ctx, `SELECT 1`).Scan(&value); err != nil {
		return fmt.Errorf("database readiness check: %w", err)
	}
	if _, err := os.Stat(s.tmpDir); err != nil {
		return fmt.Errorf("storage readiness check: %w", err)
	}
	return nil
}

func (s *Store) CreateItem(ctx context.Context, input CreateItemInput) (Item, error) {
	if input.TTL <= 0 || input.Source == "" || input.TextSize < 0 {
		return Item{}, fmt.Errorf("%w: invalid item metadata", ErrInvalidInput)
	}
	if input.TextSize == 0 && len(input.Attachments) == 0 {
		return Item{}, fmt.Errorf("%w: text and attachments cannot both be empty", ErrInvalidInput)
	}
	if (input.TextSize > 0) != (input.TextTempPath != "") {
		return Item{}, fmt.Errorf("%w: text staging metadata is inconsistent", ErrInvalidInput)
	}

	total := input.TextSize
	for _, attachment := range input.Attachments {
		if attachment.TempPath == "" || attachment.OriginalName == "" || attachment.Size < 0 {
			return Item{}, fmt.Errorf("%w: invalid attachment metadata", ErrInvalidInput)
		}
		if attachment.Size > int64(^uint64(0)>>1)-total {
			return Item{}, fmt.Errorf("%w: item size overflow", ErrInvalidInput)
		}
		total += attachment.Size
	}

	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	if len(input.IdempotencyKey) > 0 {
		if len(input.IdempotencyKey) != 32 {
			return Item{}, ErrInvalidInput
		}
		existing, found, err := s.itemByIdempotencyKey(ctx, input.IdempotencyKey)
		if err != nil {
			return Item{}, err
		}
		if found {
			return existing, nil
		}
	}

	usage, err := s.usageBytes(ctx)
	if err != nil {
		return Item{}, err
	}
	if total > s.quotaBytes || usage > s.quotaBytes-total {
		return Item{}, ErrQuotaExceeded
	}

	now := s.now().UTC()
	item := Item{
		ID:        randomID(18),
		TextSize:  input.TextSize,
		Source:    input.Source,
		CreatedAt: now,
		ExpiresAt: now.Add(input.TTL),
		TotalSize: total,
	}

	type movedFile struct{ from, to string }
	var moved []movedFile
	cleanupMoved := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			_ = os.Remove(moved[i].to)
		}
	}

	if input.TextSize > 0 {
		if input.TextSize <= s.inlineTextBytes {
			content, err := readExactFile(input.TextTempPath, input.TextSize)
			if err != nil {
				return Item{}, fmt.Errorf("read staged text: %w", err)
			}
			item.TextInline = content
		} else {
			item.TextStorage = "text-" + randomID(24)
			destination := s.blobPath(item.TextStorage)
			if err := os.Rename(input.TextTempPath, destination); err != nil {
				return Item{}, fmt.Errorf("store text blob: %w", err)
			}
			moved = append(moved, movedFile{input.TextTempPath, destination})
		}
	}

	item.Attachments = make([]Attachment, 0, len(input.Attachments))
	for _, pending := range input.Attachments {
		attachment := Attachment{
			ID:           randomID(18),
			ItemID:       item.ID,
			OriginalName: pending.OriginalName,
			StorageName:  "blob-" + randomID(24),
			MIMEType:     pending.MIMEType,
			Size:         pending.Size,
			CreatedAt:    now,
		}
		destination := s.blobPath(attachment.StorageName)
		if err := os.Rename(pending.TempPath, destination); err != nil {
			cleanupMoved()
			return Item{}, fmt.Errorf("store attachment: %w", err)
		}
		moved = append(moved, movedFile{pending.TempPath, destination})
		item.Attachments = append(item.Attachments, attachment)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		cleanupMoved()
		return Item{}, fmt.Errorf("begin item transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var inline any
	if item.TextInline != nil {
		inline = item.TextInline
	}
	var storage any
	if item.TextStorage != "" {
		storage = item.TextStorage
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO items
		(id, text_inline, text_storage, text_size, source, created_at, expires_at, total_size)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, inline, storage, item.TextSize, item.Source, millis(item.CreatedAt), millis(item.ExpiresAt), item.TotalSize,
	)
	if err != nil {
		cleanupMoved()
		return Item{}, fmt.Errorf("insert item: %w", err)
	}
	for _, attachment := range item.Attachments {
		_, err = tx.ExecContext(ctx, `INSERT INTO attachments
			(id, item_id, original_name, storage_name, mime_type, size, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			attachment.ID, attachment.ItemID, attachment.OriginalName, attachment.StorageName,
			attachment.MIMEType, attachment.Size, millis(attachment.CreatedAt),
		)
		if err != nil {
			cleanupMoved()
			return Item{}, fmt.Errorf("insert attachment: %w", err)
		}
	}
	if len(input.IdempotencyKey) > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO idempotency_keys(key_hash, item_id, created_at) VALUES(?, ?, ?)`,
			input.IdempotencyKey, item.ID, millis(now)); err != nil {
			cleanupMoved()
			return Item{}, fmt.Errorf("store idempotency key: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		cleanupMoved()
		return Item{}, fmt.Errorf("commit item: %w", err)
	}
	if item.TextInline != nil {
		_ = os.Remove(input.TextTempPath)
	}
	return item, nil
}

func (s *Store) ListItems(ctx context.Context, opts ListOptions) ([]Item, error) {
	limit := opts.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	before := opts.Before
	now := s.now().UTC()
	query := `SELECT id, text_inline, text_storage, text_size, source, created_at, expires_at, total_size
		FROM items WHERE expires_at > ? ORDER BY created_at DESC, id DESC LIMIT ?`
	args := []any{millis(now), limit}
	if !before.IsZero() {
		beforeID := opts.BeforeID
		if beforeID == "" {
			beforeID = "\U0010ffff"
		}
		query = `SELECT id, text_inline, text_storage, text_size, source, created_at, expires_at, total_size
			FROM items WHERE expires_at > ? AND (created_at < ? OR (created_at = ? AND id < ?))
			ORDER BY created_at DESC, id DESC LIMIT ?`
		args = []any{millis(now), millis(before), millis(before), beforeID, limit}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []Item
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	if err := s.loadAttachments(ctx, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (s *Store) GetItem(ctx context.Context, id string) (Item, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, text_inline, text_storage, text_size, source, created_at, expires_at, total_size
		FROM items WHERE id = ? AND expires_at > ?`, id, millis(s.now().UTC()))
	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, err
	}
	items := []Item{item}
	if err := s.loadAttachments(ctx, items); err != nil {
		return Item{}, err
	}
	return items[0], nil
}

func (s *Store) ReadText(ctx context.Context, itemID string) (io.ReadCloser, int64, error) {
	var inline []byte
	var storage sql.NullString
	var size int64
	err := s.db.QueryRowContext(ctx, `SELECT text_inline, text_storage, text_size FROM items
		WHERE id = ? AND expires_at > ?`, itemID, millis(s.now().UTC())).Scan(&inline, &storage, &size)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && size == 0) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, fmt.Errorf("read item text metadata: %w", err)
	}
	if storage.Valid {
		file, err := os.Open(s.blobPath(storage.String))
		if err != nil {
			return nil, 0, fmt.Errorf("open text blob: %w", err)
		}
		return file, size, nil
	}
	return io.NopCloser(bytes.NewReader(inline)), size, nil
}

func (s *Store) OpenAttachment(ctx context.Context, id string) (*os.File, Attachment, error) {
	var attachment Attachment
	var created int64
	err := s.db.QueryRowContext(ctx, `SELECT a.id, a.item_id, a.original_name, a.storage_name, a.mime_type, a.size, a.created_at
		FROM attachments a JOIN items i ON i.id = a.item_id
		WHERE a.id = ? AND i.expires_at > ?`, id, millis(s.now().UTC())).Scan(
		&attachment.ID, &attachment.ItemID, &attachment.OriginalName, &attachment.StorageName,
		&attachment.MIMEType, &attachment.Size, &created,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, Attachment{}, ErrNotFound
	}
	if err != nil {
		return nil, Attachment{}, fmt.Errorf("read attachment: %w", err)
	}
	attachment.CreatedAt = fromMillis(created)
	file, err := os.Open(s.blobPath(attachment.StorageName))
	if err != nil {
		return nil, Attachment{}, fmt.Errorf("open attachment blob: %w", err)
	}
	return file, attachment, nil
}

func (s *Store) UpdateExpiry(ctx context.Context, id string, ttl time.Duration) (time.Time, error) {
	if ttl <= 0 {
		return time.Time{}, ErrInvalidInput
	}
	expires := s.now().UTC().Add(ttl)
	result, err := s.db.ExecContext(ctx, `UPDATE items SET expires_at = ? WHERE id = ? AND expires_at > ?`,
		millis(expires), id, millis(s.now().UTC()))
	if err != nil {
		return time.Time{}, fmt.Errorf("update expiry: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return time.Time{}, fmt.Errorf("read update result: %w", err)
	}
	if changed == 0 {
		return time.Time{}, ErrNotFound
	}
	return expires, nil
}

func (s *Store) DeleteItem(ctx context.Context, id string) error {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	return s.deleteItemLocked(ctx, id)
}

func (s *Store) deleteItemLocked(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var textStorage sql.NullString
	if err := tx.QueryRowContext(ctx, `SELECT text_storage FROM items WHERE id = ?`, id).Scan(&textStorage); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("read item for delete: %w", err)
	}
	var names []string
	if textStorage.Valid {
		names = append(names, textStorage.String)
	}
	rows, err := tx.QueryContext(ctx, `SELECT storage_name FROM attachments WHERE item_id = ?`, id)
	if err != nil {
		return fmt.Errorf("read attachment paths: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			_ = rows.Close()
			return fmt.Errorf("scan attachment path: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close attachment rows: %w", err)
	}

	type renamedFile struct{ original, trash string }
	var renamed []renamedFile
	restore := func() {
		for i := len(renamed) - 1; i >= 0; i-- {
			_ = os.Rename(renamed[i].trash, renamed[i].original)
		}
	}
	for _, name := range names {
		original := s.blobPath(name)
		trash := filepath.Join(s.tmpDir, "delete-"+randomID(24))
		if err := os.Rename(original, trash); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			restore()
			return fmt.Errorf("stage blob deletion: %w", err)
		}
		renamed = append(renamed, renamedFile{original, trash})
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id); err != nil {
		restore()
		return fmt.Errorf("delete item row: %w", err)
	}
	if err := tx.Commit(); err != nil {
		restore()
		return fmt.Errorf("commit delete: %w", err)
	}
	for _, file := range renamed {
		_ = os.Remove(file.trash)
	}
	return nil
}

func (s *Store) CleanupExpired(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM items WHERE expires_at <= ? ORDER BY expires_at LIMIT ?`,
		millis(s.now().UTC()), limit)
	if err != nil {
		return 0, fmt.Errorf("list expired items: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return 0, fmt.Errorf("scan expired item: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired items: %w", err)
	}
	deleted := 0
	for _, id := range ids {
		if err := s.DeleteItem(ctx, id); err != nil && !errors.Is(err, ErrNotFound) {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

func (s *Store) CleanupTmp(olderThan time.Time) (int, error) {
	entries, err := os.ReadDir(s.tmpDir)
	if err != nil {
		return 0, fmt.Errorf("read tmp directory: %w", err)
	}
	removed := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(olderThan) {
			if err := os.Remove(filepath.Join(s.tmpDir, entry.Name())); err == nil || errors.Is(err, os.ErrNotExist) {
				removed++
			}
		}
	}
	return removed, nil
}

func (s *Store) Usage(ctx context.Context) (Usage, error) {
	var usage Usage
	usage.QuotaBytes = s.quotaBytes
	err := s.db.QueryRowContext(ctx, `SELECT used_bytes, item_count, attachment_count FROM storage_usage WHERE id = 1`).Scan(
		&usage.UsedBytes, &usage.ItemCount, &usage.AttachmentCount,
	)
	if err != nil {
		return Usage{}, fmt.Errorf("read storage usage: %w", err)
	}
	return usage, nil
}

func (s *Store) RecordTraffic(_ context.Context, at time.Time, entry, category string, bytes int64) error {
	if entry == "" || category == "" || bytes < 0 {
		return fmt.Errorf("%w: invalid traffic sample", ErrInvalidInput)
	}
	key := trafficKey{hour: at.UTC().Truncate(time.Hour).Unix(), entry: entry, category: category}
	s.trafficMu.Lock()
	sample := s.trafficPending[key]
	sample.bytes += bytes
	sample.requests++
	s.trafficPending[key] = sample
	s.trafficMu.Unlock()
	return nil
}

func (s *Store) FlushTraffic(ctx context.Context) error {
	s.trafficFlushMu.Lock()
	defer s.trafficFlushMu.Unlock()

	s.trafficMu.Lock()
	if len(s.trafficPending) == 0 {
		s.trafficMu.Unlock()
		return nil
	}
	pending := s.trafficPending
	s.trafficPending = make(map[trafficKey]trafficSample)
	s.trafficMu.Unlock()

	restore := func() {
		s.trafficMu.Lock()
		for key, sample := range pending {
			current := s.trafficPending[key]
			current.bytes += sample.bytes
			current.requests += sample.requests
			s.trafficPending[key] = current
		}
		s.trafficMu.Unlock()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		restore()
		return fmt.Errorf("begin traffic flush: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for key, sample := range pending {
		if _, err := tx.ExecContext(ctx, `INSERT INTO traffic_hourly(hour, entry, category, bytes, requests)
			VALUES(?, ?, ?, ?, ?)
			ON CONFLICT(hour, entry, category) DO UPDATE SET
				bytes = bytes + excluded.bytes,
				requests = requests + excluded.requests`,
			key.hour, key.entry, key.category, sample.bytes, sample.requests); err != nil {
			restore()
			return fmt.Errorf("flush traffic: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		restore()
		return fmt.Errorf("commit traffic flush: %w", err)
	}
	return nil
}

func (s *Store) PurgeTrafficBefore(ctx context.Context, before time.Time) (int64, error) {
	if before.IsZero() {
		return 0, ErrInvalidInput
	}
	s.trafficFlushMu.Lock()
	defer s.trafficFlushMu.Unlock()
	result, err := s.db.ExecContext(ctx, `DELETE FROM traffic_hourly WHERE hour < ?`, before.UTC().Truncate(time.Hour).Unix())
	if err != nil {
		return 0, fmt.Errorf("purge traffic history: %w", err)
	}
	removed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("read purged traffic history: %w", err)
	}
	return removed, nil
}

func (s *Store) TrafficReport(ctx context.Context, now time.Time) (TrafficReport, error) {
	s.trafficFlushMu.Lock()
	defer s.trafficFlushMu.Unlock()
	now = now.UTC()
	hourStart := now.Truncate(time.Hour)
	since30 := hourStart.Add(-30 * 24 * time.Hour)
	rows, err := s.db.QueryContext(ctx, `SELECT hour, entry, SUM(bytes), SUM(requests)
		FROM traffic_hourly WHERE hour >= ?
		GROUP BY hour, entry ORDER BY hour`, since30.Unix())
	if err != nil {
		return TrafficReport{}, fmt.Errorf("read traffic report: %w", err)
	}
	defer func() { _ = rows.Close() }()

	report := TrafficReport{Hourly: make([]HourlyTraffic, 24)}
	for i := range report.Hourly {
		report.Hourly[i].Hour = hourStart.Add(time.Duration(i-23) * time.Hour)
	}
	hourlyIndex := make(map[int64]int, len(report.Hourly))
	for i := range report.Hourly {
		hourlyIndex[report.Hourly[i].Hour.Unix()] = i
	}
	since24 := hourStart.Add(-23 * time.Hour).Unix()
	for rows.Next() {
		var hour int64
		var entry string
		var bytes, requests int64
		if err := rows.Scan(&hour, &entry, &bytes, &requests); err != nil {
			return TrafficReport{}, fmt.Errorf("scan traffic report: %w", err)
		}
		addTraffic(&report.Last30Days, entry, bytes, requests)
		if hour >= since24 {
			addTraffic(&report.Last24Hours, entry, bytes, requests)
			if index, ok := hourlyIndex[hour]; ok {
				addTraffic(&report.Hourly[index].TrafficTotals, entry, bytes, requests)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return TrafficReport{}, fmt.Errorf("iterate traffic report: %w", err)
	}
	s.trafficMu.Lock()
	pending := make(map[trafficKey]trafficSample, len(s.trafficPending))
	for key, sample := range s.trafficPending {
		pending[key] = sample
	}
	s.trafficMu.Unlock()
	for key, sample := range pending {
		if key.hour < since30.Unix() {
			continue
		}
		addTraffic(&report.Last30Days, key.entry, sample.bytes, sample.requests)
		if key.hour >= since24 {
			addTraffic(&report.Last24Hours, key.entry, sample.bytes, sample.requests)
			if index, ok := hourlyIndex[key.hour]; ok {
				addTraffic(&report.Hourly[index].TrafficTotals, key.entry, sample.bytes, sample.requests)
			}
		}
	}
	return report, nil
}

func addTraffic(total *TrafficTotals, entry string, bytes, requests int64) {
	switch entry {
	case "public", "homehub":
		total.PublicBytes += bytes
	case "tailscale":
		total.TailscaleBytes += bytes
	case "hermes":
		total.HermesBytes += bytes
	}
	total.TotalBytes += bytes
	total.Requests += requests
}

func (s *Store) usageBytes(ctx context.Context) (int64, error) {
	var used int64
	if err := s.db.QueryRowContext(ctx, `SELECT used_bytes FROM storage_usage WHERE id = 1`).Scan(&used); err != nil {
		return 0, fmt.Errorf("read storage usage: %w", err)
	}
	return used, nil
}

type scanner interface{ Scan(...any) error }

func scanItem(row scanner) (Item, error) {
	var item Item
	var inline []byte
	var storage sql.NullString
	var created, expires int64
	err := row.Scan(&item.ID, &inline, &storage, &item.TextSize, &item.Source, &created, &expires, &item.TotalSize)
	if err != nil {
		return Item{}, err
	}
	item.TextInline = inline
	if storage.Valid {
		item.TextStorage = storage.String
	}
	item.CreatedAt = fromMillis(created)
	item.ExpiresAt = fromMillis(expires)
	return item, nil
}

func (s *Store) itemByIdempotencyKey(ctx context.Context, key []byte) (Item, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT i.id, i.text_inline, i.text_storage, i.text_size, i.source, i.created_at, i.expires_at, i.total_size
		FROM idempotency_keys k JOIN items i ON i.id = k.item_id WHERE k.key_hash = ?`, key)
	item, err := scanItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, false, nil
	}
	if err != nil {
		return Item{}, false, fmt.Errorf("read idempotent item: %w", err)
	}
	items := []Item{item}
	if err := s.loadAttachments(ctx, items); err != nil {
		return Item{}, false, err
	}
	return items[0], true, nil
}

func (s *Store) loadAttachments(ctx context.Context, items []Item) error {
	if len(items) == 0 {
		return nil
	}
	index := make(map[string]int, len(items))
	args := make([]any, len(items))
	marks := make([]string, len(items))
	for i := range items {
		index[items[i].ID] = i
		args[i] = items[i].ID
		marks[i] = "?"
	}
	query := `SELECT id, item_id, original_name, storage_name, mime_type, size, created_at
		FROM attachments WHERE item_id IN (` + strings.Join(marks, ",") + `) ORDER BY created_at, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("list attachments: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var attachment Attachment
		var created int64
		if err := rows.Scan(&attachment.ID, &attachment.ItemID, &attachment.OriginalName, &attachment.StorageName,
			&attachment.MIMEType, &attachment.Size, &created); err != nil {
			return fmt.Errorf("scan attachment: %w", err)
		}
		attachment.CreatedAt = fromMillis(created)
		i := index[attachment.ItemID]
		items[i].Attachments = append(items[i].Attachments, attachment)
	}
	return rows.Err()
}

func (s *Store) blobPath(name string) string {
	if name == "" || filepath.Base(name) != name {
		panic("invalid internal blob name")
	}
	return filepath.Join(s.blobsDir, name)
}

func readExactFile(path string, expected int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	content, err := io.ReadAll(io.LimitReader(file, expected+1))
	if err != nil {
		return nil, err
	}
	if int64(len(content)) != expected {
		return nil, fmt.Errorf("staged size mismatch: expected %d, got %d", expected, len(content))
	}
	return content, nil
}

func randomID(bytesCount int) string {
	buffer := make([]byte, bytesCount)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		panic(fmt.Sprintf("cryptographic random source failed: %v", err))
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func millis(value time.Time) int64     { return value.UnixMilli() }
func fromMillis(value int64) time.Time { return time.UnixMilli(value).UTC() }
