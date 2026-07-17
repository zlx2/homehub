package store

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateReadListAndDeleteItem(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage := openTestStore(t, 1<<20, 8, func() time.Time { return now })

	text := []byte("a text payload that is stored as a blob")
	textPath := stageFile(t, storage.TmpDir(), "text-*", text)
	attachmentBody := []byte{0, 1, 2, 3, 4}
	attachmentPath := stageFile(t, storage.TmpDir(), "upload-*", attachmentBody)

	item, err := storage.CreateItem(ctx, CreateItemInput{
		TextTempPath: textPath,
		TextSize:     int64(len(text)),
		Source:       "owner",
		TTL:          24 * time.Hour,
		Attachments: []PendingAttachment{{
			TempPath: attachmentPath, OriginalName: "example.bin", MIMEType: "application/octet-stream", Size: int64(len(attachmentBody)),
		}},
	})
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}
	if item.TextStorage == "" || len(item.Attachments) != 1 {
		t.Fatalf("CreateItem() item = %#v", item)
	}
	if _, err := os.Stat(textPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged text still exists: %v", err)
	}

	items, err := storage.ListItems(ctx, ListOptions{})
	if err != nil || len(items) != 1 || len(items[0].Attachments) != 1 {
		t.Fatalf("ListItems() = %#v, %v", items, err)
	}
	reader, size, err := storage.ReadText(ctx, item.ID)
	if err != nil {
		t.Fatalf("ReadText() error = %v", err)
	}
	gotText, _ := io.ReadAll(reader)
	if err := reader.Close(); err != nil {
		t.Fatalf("close text reader: %v", err)
	}
	if size != int64(len(text)) || string(gotText) != string(text) {
		t.Fatalf("ReadText() = %q (%d)", gotText, size)
	}
	file, attachment, err := storage.OpenAttachment(ctx, item.Attachments[0].ID)
	if err != nil {
		t.Fatalf("OpenAttachment() error = %v", err)
	}
	gotAttachment, _ := io.ReadAll(file)
	if err := file.Close(); err != nil {
		t.Fatalf("close attachment: %v", err)
	}
	if attachment.OriginalName != "example.bin" || string(gotAttachment) != string(attachmentBody) {
		t.Fatalf("OpenAttachment() = %#v %v", attachment, gotAttachment)
	}

	usage, err := storage.Usage(ctx)
	if err != nil || usage.UsedBytes != int64(len(text)+len(attachmentBody)) || usage.ItemCount != 1 || usage.AttachmentCount != 1 {
		t.Fatalf("Usage() = %#v, %v", usage, err)
	}
	if err := storage.DeleteItem(ctx, item.ID); err != nil {
		t.Fatalf("DeleteItem() error = %v", err)
	}
	if _, err := storage.GetItem(ctx, item.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetItem() after deletion error = %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(filepath.Dir(storage.TmpDir()), "blobs"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("blob directory after deletion = %v, %v", entries, err)
	}
}

func TestInlineTextQuotaAndExpiry(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	storage := openTestStore(t, 5, 1024, func() time.Time { return now })

	path := stageFile(t, storage.TmpDir(), "text-*", []byte("hello"))
	item, err := storage.CreateItem(ctx, CreateItemInput{
		TextTempPath: path, TextSize: 5, Source: "guest", TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("CreateItem() error = %v", err)
	}
	if string(item.TextInline) != "hello" || item.TextStorage != "" {
		t.Fatalf("text was not stored inline: %#v", item)
	}

	overPath := stageFile(t, storage.TmpDir(), "text-*", []byte("x"))
	_, err = storage.CreateItem(ctx, CreateItemInput{TextTempPath: overPath, TextSize: 1, Source: "owner", TTL: time.Hour})
	if !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("quota CreateItem() error = %v", err)
	}
	if _, err := os.Stat(overPath); err != nil {
		t.Fatalf("quota rejection removed caller-owned staged file: %v", err)
	}

	now = now.Add(2 * time.Hour)
	if items, err := storage.ListItems(ctx, ListOptions{}); err != nil || len(items) != 0 {
		t.Fatalf("expired ListItems() = %#v, %v", items, err)
	}
	deleted, err := storage.CleanupExpired(ctx, 10)
	if err != nil || deleted != 1 {
		t.Fatalf("CleanupExpired() = %d, %v", deleted, err)
	}
}

func TestTrafficReportPersistsHourlyEntryTotals(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 8, 35, 0, 0, time.UTC)
	storage := openTestStore(t, 1<<20, 1024, func() time.Time { return now })
	for _, sample := range []struct {
		at       time.Time
		entry    string
		category string
		bytes    int64
	}{
		{now.Add(-time.Hour), "public", "attachment", 1500},
		{now.Add(-time.Hour), "public", "api", 500},
		{now, "tailscale", "preview", 250},
		{now.Add(-25 * time.Hour), "hermes", "api", 100},
	} {
		if err := storage.RecordTraffic(ctx, sample.at, sample.entry, sample.category, sample.bytes); err != nil {
			t.Fatal(err)
		}
	}
	report, err := storage.TrafficReport(ctx, now)
	if err != nil {
		t.Fatal(err)
	}
	if report.Last24Hours.PublicBytes != 2000 || report.Last24Hours.TailscaleBytes != 250 || report.Last24Hours.HermesBytes != 0 || report.Last24Hours.Requests != 3 {
		t.Fatalf("last 24 hours = %#v", report.Last24Hours)
	}
	if report.Last30Days.TotalBytes != 2350 || report.Last30Days.Requests != 4 || len(report.Hourly) != 24 {
		t.Fatalf("traffic report = %#v", report)
	}
	if err := storage.FlushTraffic(ctx); err != nil {
		t.Fatalf("FlushTraffic() error = %v", err)
	}
	var requests int64
	if err := storage.db.QueryRowContext(ctx, `SELECT SUM(requests) FROM traffic_hourly`).Scan(&requests); err != nil || requests != 4 {
		t.Fatalf("persisted requests = %d, %v", requests, err)
	}
}

func TestPurgeTrafficBeforeBoundsHistory(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 8, 35, 0, 0, time.UTC)
	storage := openTestStore(t, 1<<20, 1024, func() time.Time { return now })
	for _, at := range []time.Time{now.Add(-40 * 24 * time.Hour), now.Add(-24 * time.Hour), now} {
		if err := storage.RecordTraffic(ctx, at, "public", "api", 100); err != nil {
			t.Fatal(err)
		}
	}
	if err := storage.FlushTraffic(ctx); err != nil {
		t.Fatal(err)
	}
	removed, err := storage.PurgeTrafficBefore(ctx, now.Add(-32*24*time.Hour))
	if err != nil || removed != 1 {
		t.Fatalf("PurgeTrafficBefore() = %d, %v", removed, err)
	}
	var remaining int
	if err := storage.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM traffic_hourly`).Scan(&remaining); err != nil || remaining != 2 {
		t.Fatalf("remaining traffic rows = %d, %v", remaining, err)
	}
}

func TestCompositeCursorKeepsItemsCreatedInSameMillisecond(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 8, 35, 0, 0, time.UTC)
	storage := openTestStore(t, 1<<20, 1024, func() time.Time { return now })
	for _, text := range []string{"first", "second"} {
		path := stageFile(t, storage.TmpDir(), "text-*", []byte(text))
		if _, err := storage.CreateItem(ctx, CreateItemInput{TextTempPath: path, TextSize: int64(len(text)), Source: "owner", TTL: time.Hour}); err != nil {
			t.Fatal(err)
		}
	}
	firstPage, err := storage.ListItems(ctx, ListOptions{Limit: 1})
	if err != nil || len(firstPage) != 1 {
		t.Fatalf("first page = %#v, %v", firstPage, err)
	}
	secondPage, err := storage.ListItems(ctx, ListOptions{Limit: 1, Before: firstPage[0].CreatedAt, BeforeID: firstPage[0].ID})
	if err != nil || len(secondPage) != 1 || secondPage[0].ID == firstPage[0].ID {
		t.Fatalf("second page = %#v, %v", secondPage, err)
	}
}

func TestCreateItemIdempotencyReturnsOriginalItem(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 16, 8, 35, 0, 0, time.UTC)
	storage := openTestStore(t, 1<<20, 1024, func() time.Time { return now })
	key := make([]byte, 32)
	key[0] = 1
	create := func(text string) Item {
		path := stageFile(t, storage.TmpDir(), "text-*", []byte(text))
		item, err := storage.CreateItem(ctx, CreateItemInput{
			TextTempPath: path, TextSize: int64(len(text)), Source: "owner", TTL: time.Hour, IdempotencyKey: key,
		})
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	first := create("first")
	second := create("different retry body")
	if second.ID != first.ID || string(second.TextInline) != "first" {
		t.Fatalf("idempotent retry returned %#v, want %#v", second, first)
	}
	usage, err := storage.Usage(ctx)
	if err != nil || usage.ItemCount != 1 || usage.UsedBytes != int64(len("first")) {
		t.Fatalf("usage after retry = %#v, %v", usage, err)
	}
}

func openTestStore(t *testing.T, quota, inline int64, now func() time.Time) *Store {
	t.Helper()
	storage, err := Open(context.Background(), Options{
		DataDir: t.TempDir(), QuotaBytes: quota, InlineTextBytes: inline, Now: now,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = storage.Close() })
	return storage
}

func stageFile(t *testing.T, dir, pattern string, content []byte) string {
	t.Helper()
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return file.Name()
}
