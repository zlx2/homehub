package bridge

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"homehub.local/services/telegram-bridge/internal/config"
	"homehub.local/services/telegram-bridge/internal/drop"
	"homehub.local/services/telegram-bridge/internal/telegram"
)

type Telegram interface {
	GetMe(context.Context) (telegram.Bot, error)
	DeleteWebhook(context.Context) error
	GetUpdates(context.Context, int64, time.Duration) ([]telegram.Update, error)
	GetFile(context.Context, string) (telegram.File, error)
	Download(context.Context, string) (io.ReadCloser, error)
	SendMessage(context.Context, int64, int64, string) error
}

type Drop interface {
	Create(context.Context, drop.CreateInput) (drop.Item, error)
}

type Bridge struct {
	cfg      config.Config
	telegram Telegram
	drop     Drop
	logger   *slog.Logger
	bot      telegram.Bot
}

type media struct {
	fileID      string
	fileSize    int64
	name        string
	contentType string
}

func New(cfg config.Config, telegramClient Telegram, dropClient Drop, logger *slog.Logger) *Bridge {
	return &Bridge{cfg: cfg, telegram: telegramClient, drop: dropClient, logger: logger}
}

func (b *Bridge) Prepare(ctx context.Context) error {
	if err := b.telegram.DeleteWebhook(ctx); err != nil {
		return err
	}
	bot, err := b.telegram.GetMe(ctx)
	if err != nil {
		return err
	}
	b.bot = bot
	b.logger.Info("Telegram bot connected", "bot_id", bot.ID, "username", bot.Username,
		"allowed_users", len(b.cfg.AllowedUserIDs), "allowed_chats", len(b.cfg.AllowedChatIDs))
	if len(b.cfg.AllowedUserIDs) == 0 && len(b.cfg.AllowedChatIDs) == 0 {
		b.logger.Warn("Telegram forwarding is deny-all until an allowed user or chat ID is configured")
	}
	return nil
}

func (b *Bridge) Run(ctx context.Context) error {
	var offset int64
	backoff := time.Second
	for ctx.Err() == nil {
		updates, err := b.telegram.GetUpdates(ctx, offset, b.cfg.PollTimeout)
		if err != nil {
			b.logger.Warn("Telegram polling failed", "error", err, "retry_in", backoff)
			if !wait(ctx, backoff) {
				return ctx.Err()
			}
			backoff = min(backoff*2, 30*time.Second)
			continue
		}
		backoff = time.Second
		for _, update := range updates {
			if err := b.HandleUpdate(ctx, update); err != nil {
				b.logger.Error("Telegram update failed", "update_id", update.UpdateID, "error", err, "retry_in", backoff)
				if !wait(ctx, backoff) {
					return ctx.Err()
				}
				backoff = min(backoff*2, 30*time.Second)
				break
			}
			if update.UpdateID >= offset {
				offset = update.UpdateID + 1
			}
		}
	}
	return ctx.Err()
}

func (b *Bridge) HandleUpdate(ctx context.Context, update telegram.Update) error {
	message := update.Content()
	if message == nil {
		return nil
	}
	if command := parseCommand(message.Text); command != "" {
		return b.handleCommand(ctx, message, command)
	}
	userID := int64(0)
	if message.From != nil {
		userID = message.From.ID
	}
	if !b.cfg.Allowed(userID, message.Chat.ID) {
		return nil
	}
	selected := selectMedia(message)
	if selected == nil && strings.TrimSpace(message.Text) == "" && strings.TrimSpace(message.Caption) == "" {
		return b.reply(ctx, message, "这类 Telegram 消息暂时不能转发。", true)
	}
	if selected != nil && selected.fileSize > b.cfg.MaxTelegramFileSize {
		return b.reply(ctx, message, "文件超过 Telegram Bot API 的 20 MB 下载限制，请直接用 Drop 上传。", true)
	}

	input := drop.CreateInput{
		Text:           formatText(message),
		TTL:            b.cfg.TTL,
		IdempotencyKey: fmt.Sprintf("telegram:%d:%d:%d", b.bot.ID, message.Chat.ID, message.MessageID),
	}
	if selected != nil {
		file, err := b.telegram.GetFile(ctx, selected.fileID)
		if err != nil {
			return fmt.Errorf("resolve Telegram file: %w", err)
		}
		if file.FileSize > b.cfg.MaxTelegramFileSize {
			return b.reply(ctx, message, "文件超过 Telegram Bot API 的 20 MB 下载限制，请直接用 Drop 上传。", true)
		}
		body, err := b.telegram.Download(ctx, file.FilePath)
		if err != nil {
			return fmt.Errorf("download Telegram file: %w", err)
		}
		defer func() { _ = body.Close() }()
		input.Attachment = &drop.Attachment{Name: selected.name, ContentType: selected.contentType, Reader: body}
	}
	item, err := b.drop.Create(ctx, input)
	if err != nil {
		return err
	}
	b.logger.Info("Telegram message forwarded", "update_id", update.UpdateID, "chat_id", message.Chat.ID,
		"message_id", message.MessageID, "drop_item_id", item.ID)
	return b.reply(ctx, message, "已转发到 Drop。", false)
}

func (b *Bridge) handleCommand(ctx context.Context, message *telegram.Message, command string) error {
	switch command {
	case "start", "help":
		return b.telegram.SendMessage(ctx, message.Chat.ID, message.MessageID,
			"把文字、图片、视频或文件发给我，我会原样转发到 Drop。群组使用前请先把群 ID 加入允许列表。发送 /whoami 可查看用户和群 ID。")
	case "whoami":
		userID := int64(0)
		if message.From != nil {
			userID = message.From.ID
		}
		return b.telegram.SendMessage(ctx, message.Chat.ID, message.MessageID,
			fmt.Sprintf("user_id: %d\nchat_id: %d\nchat_type: %s", userID, message.Chat.ID, message.Chat.Type))
	default:
		return nil
	}
}

func (b *Bridge) reply(ctx context.Context, message *telegram.Message, text string, force bool) error {
	if !force && (b.cfg.AckMode == "none" || (b.cfg.AckMode == "private" && message.Chat.Type != "private")) {
		return nil
	}
	return b.telegram.SendMessage(ctx, message.Chat.ID, message.MessageID, text)
}

func parseCommand(text string) string {
	first, _, _ := strings.Cut(strings.TrimSpace(text), " ")
	if !strings.HasPrefix(first, "/") {
		return ""
	}
	command := strings.TrimPrefix(first, "/")
	command, _, _ = strings.Cut(command, "@")
	return strings.ToLower(command)
}

func formatText(message *telegram.Message) string {
	text := message.Text
	if text == "" {
		text = message.Caption
	}
	text = strings.TrimSpace(text)
	if message.Chat.Type == "private" {
		return text
	}
	prefix := "Telegram"
	if chat := chatName(message.Chat); chat != "" {
		prefix += " · " + chat
	}
	if sender := senderName(message); sender != "" {
		prefix += " · " + sender
	}
	if text == "" {
		return prefix
	}
	return prefix + "\n\n" + text
}

func chatName(chat telegram.Chat) string {
	if chat.Title != "" {
		return chat.Title
	}
	if chat.Username != "" {
		return "@" + chat.Username
	}
	return strings.TrimSpace(chat.FirstName + " " + chat.LastName)
}

func senderName(message *telegram.Message) string {
	if message.From != nil {
		if message.From.Username != "" {
			return "@" + message.From.Username
		}
		return strings.TrimSpace(message.From.FirstName + " " + message.From.LastName)
	}
	if message.SenderChat != nil {
		return chatName(*message.SenderChat)
	}
	return ""
}

func selectMedia(message *telegram.Message) *media {
	if len(message.Photo) > 0 {
		photo := message.Photo[0]
		for _, candidate := range message.Photo[1:] {
			if candidate.FileSize > photo.FileSize || (candidate.FileSize == photo.FileSize && candidate.Width*candidate.Height > photo.Width*photo.Height) {
				photo = candidate
			}
		}
		return &media{fileID: photo.FileID, fileSize: photo.FileSize, name: "telegram-photo-" + strconv.FormatInt(message.MessageID, 10) + ".jpg", contentType: "image/jpeg"}
	}
	if message.Animation != nil {
		return fileMedia(message.Animation, "animation-"+strconv.FormatInt(message.MessageID, 10)+".mp4", "video/mp4")
	}
	if message.Document != nil {
		return fileMedia(message.Document, "document-"+strconv.FormatInt(message.MessageID, 10), "application/octet-stream")
	}
	if message.Video != nil {
		return fileMedia(message.Video, "video-"+strconv.FormatInt(message.MessageID, 10)+".mp4", "video/mp4")
	}
	if message.Audio != nil {
		return fileMedia(message.Audio, "audio-"+strconv.FormatInt(message.MessageID, 10)+".mp3", "audio/mpeg")
	}
	if message.Voice != nil {
		return fileMedia(message.Voice, "voice-"+strconv.FormatInt(message.MessageID, 10)+".ogg", "audio/ogg")
	}
	if message.VideoNote != nil {
		return fileMedia(message.VideoNote, "video-note-"+strconv.FormatInt(message.MessageID, 10)+".mp4", "video/mp4")
	}
	if message.Sticker != nil {
		name, contentType := "sticker-"+strconv.FormatInt(message.MessageID, 10)+".webp", "image/webp"
		if message.Sticker.IsAnimated {
			name, contentType = strings.TrimSuffix(name, ".webp")+".tgs", "application/x-tgsticker"
		} else if message.Sticker.IsVideo {
			name, contentType = strings.TrimSuffix(name, ".webp")+".webm", "video/webm"
		}
		return &media{fileID: message.Sticker.FileID, fileSize: message.Sticker.FileSize, name: name, contentType: contentType}
	}
	return nil
}

func fileMedia(value *telegram.FileMedia, fallbackName, fallbackType string) *media {
	name := filepath.Base(strings.TrimSpace(value.FileName))
	if name == "." || name == "" {
		name = fallbackName
	}
	contentType := strings.TrimSpace(value.MIMEType)
	if contentType == "" {
		contentType = fallbackType
	}
	return &media{fileID: value.FileID, fileSize: value.FileSize, name: name, contentType: contentType}
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
