package bridge

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/config"
	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/drop"
	"gitee.com/zlx23/homehub/services/telegram-bridge/internal/telegram"
)

type fakeTelegram struct {
	file     telegram.File
	fileBody string
	replies  []string
}

func (f *fakeTelegram) GetMe(context.Context) (telegram.Bot, error) {
	return telegram.Bot{ID: 99, Username: "drop_bot"}, nil
}
func (f *fakeTelegram) DeleteWebhook(context.Context) error { return nil }
func (f *fakeTelegram) GetUpdates(context.Context, int64, time.Duration) ([]telegram.Update, error) {
	return nil, nil
}
func (f *fakeTelegram) GetFile(context.Context, string) (telegram.File, error) { return f.file, nil }
func (f *fakeTelegram) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.fileBody)), nil
}
func (f *fakeTelegram) SendMessage(_ context.Context, chatID, replyTo int64, text string) error {
	f.replies = append(f.replies, text)
	return nil
}

type fakeDrop struct {
	calls []drop.CreateInput
	files []string
}

func (f *fakeDrop) Create(_ context.Context, input drop.CreateInput) (drop.Item, error) {
	f.calls = append(f.calls, input)
	if input.Attachment != nil {
		body, _ := io.ReadAll(input.Attachment.Reader)
		f.files = append(f.files, string(body))
	}
	return drop.Item{ID: "item-1"}, nil
}

func testBridge() (*Bridge, *fakeTelegram, *fakeDrop) {
	tg := &fakeTelegram{file: telegram.File{FilePath: "photos/a.jpg", FileSize: 8}, fileBody: "original"}
	dr := &fakeDrop{}
	cfg := config.Config{
		AllowedUserIDs: map[int64]struct{}{23: {}}, AllowedChatIDs: map[int64]struct{}{},
		TTL: 1, AckMode: "private", MaxTelegramFileSize: 20 << 20,
	}
	b := New(cfg, tg, dr, slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.bot = telegram.Bot{ID: 99, Username: "drop_bot"}
	return b, tg, dr
}

func TestAuthorizedPrivateTextForwardsAndAcknowledges(t *testing.T) {
	b, tg, dr := testBridge()
	update := telegram.Update{UpdateID: 7, Message: &telegram.Message{
		MessageID: 8, From: &telegram.User{ID: 23}, Chat: telegram.Chat{ID: 23, Type: "private"}, Text: "hello",
	}}
	if err := b.HandleUpdate(context.Background(), update); err != nil {
		t.Fatal(err)
	}
	if len(dr.calls) != 1 || dr.calls[0].Text != "hello" || dr.calls[0].IdempotencyKey != "telegram:99:23:8" {
		t.Fatalf("drop calls = %#v", dr.calls)
	}
	if len(tg.replies) != 1 || tg.replies[0] != "已转发到 Drop。" {
		t.Fatalf("replies = %#v", tg.replies)
	}
}

func TestUnauthorizedMessageIsIgnoredButWhoamiWorks(t *testing.T) {
	b, tg, dr := testBridge()
	message := &telegram.Message{MessageID: 8, From: &telegram.User{ID: 55}, Chat: telegram.Chat{ID: 55, Type: "private"}, Text: "hello"}
	if err := b.HandleUpdate(context.Background(), telegram.Update{UpdateID: 1, Message: message}); err != nil {
		t.Fatal(err)
	}
	if len(dr.calls) != 0 || len(tg.replies) != 0 {
		t.Fatal("unauthorized content was handled")
	}
	message.Text = "/whoami"
	if err := b.HandleUpdate(context.Background(), telegram.Update{UpdateID: 2, Message: message}); err != nil {
		t.Fatal(err)
	}
	if len(tg.replies) != 1 || !strings.Contains(tg.replies[0], "user_id: 55") {
		t.Fatalf("whoami reply = %#v", tg.replies)
	}
}

func TestGroupPhotoUsesLargestRenditionAndPreservesBytes(t *testing.T) {
	b, tg, dr := testBridge()
	b.cfg.AllowedChatIDs[-1001] = struct{}{}
	message := &telegram.Message{
		MessageID: 12, From: &telegram.User{ID: 55, Username: "friend"},
		Chat: telegram.Chat{ID: -1001, Type: "supergroup", Title: "Home"}, Caption: "screen",
		Photo: []telegram.PhotoSize{
			{FileID: "small", Width: 100, Height: 100, FileSize: 1000},
			{FileID: "large", Width: 1000, Height: 1000, FileSize: 8000},
		},
	}
	if err := b.HandleUpdate(context.Background(), telegram.Update{UpdateID: 3, Message: message}); err != nil {
		t.Fatal(err)
	}
	if len(dr.calls) != 1 || dr.calls[0].Attachment == nil || dr.calls[0].Attachment.Name != "telegram-photo-12.jpg" {
		t.Fatalf("drop call = %#v", dr.calls)
	}
	if dr.files[0] != "original" || dr.calls[0].Text != "Telegram · Home · @friend\n\nscreen" {
		t.Fatalf("file=%q text=%q", dr.files[0], dr.calls[0].Text)
	}
	if len(tg.replies) != 0 {
		t.Fatalf("group acknowledgement should be quiet: %#v", tg.replies)
	}
}

func TestOversizedFileDoesNotRetryOrReachDrop(t *testing.T) {
	b, tg, dr := testBridge()
	message := &telegram.Message{
		MessageID: 12, From: &telegram.User{ID: 23}, Chat: telegram.Chat{ID: 23, Type: "private"},
		Document: &telegram.FileMedia{FileID: "large", FileName: "large.bin", FileSize: 21 << 20},
	}
	if err := b.HandleUpdate(context.Background(), telegram.Update{UpdateID: 4, Message: message}); err != nil {
		t.Fatal(err)
	}
	if len(dr.calls) != 0 || len(tg.replies) != 1 || !strings.Contains(tg.replies[0], "20 MB") {
		t.Fatalf("calls=%d replies=%#v", len(dr.calls), tg.replies)
	}
}
