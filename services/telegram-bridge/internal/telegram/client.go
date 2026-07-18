package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type apiResponse[T any] struct {
	OK          bool   `json:"ok"`
	Result      T      `json:"result"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) GetMe(ctx context.Context) (Bot, error) {
	return call[Bot](ctx, c, "getMe", map[string]any{})
}

func (c *Client) DeleteWebhook(ctx context.Context) error {
	_, err := call[bool](ctx, c, "deleteWebhook", map[string]any{"drop_pending_updates": false})
	return err
}

func (c *Client) GetUpdates(ctx context.Context, offset int64, timeout time.Duration) ([]Update, error) {
	return call[[]Update](ctx, c, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         int(timeout.Seconds()),
		"allowed_updates": []string{"message", "channel_post"},
	})
}

func (c *Client) GetFile(ctx context.Context, fileID string) (File, error) {
	return call[File](ctx, c, "getFile", map[string]any{"file_id": fileID})
}

func (c *Client) Download(ctx context.Context, filePath string) (io.ReadCloser, error) {
	if filePath == "" || strings.Contains(filePath, "..") {
		return nil, fmt.Errorf("Telegram returned an invalid file path")
	}
	requestURL := c.baseURL + "/file/bot" + url.PathEscape(c.token) + "/" + strings.TrimLeft(filePath, "/")
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build Telegram file request")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download Telegram file")
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("download Telegram file: HTTP %d", response.StatusCode)
	}
	return response.Body, nil
}

func (c *Client) SendMessage(ctx context.Context, chatID, replyTo int64, text string) error {
	payload := map[string]any{"chat_id": chatID, "text": text}
	if replyTo != 0 {
		payload["reply_parameters"] = map[string]any{"message_id": replyTo, "allow_sending_without_reply": true}
	}
	_, err := call[Message](ctx, c, "sendMessage", payload)
	return err
}

func call[T any](ctx context.Context, client *Client, method string, payload any) (T, error) {
	var zero T
	encoded, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("encode Telegram request: %w", err)
	}
	requestURL := client.baseURL + "/bot" + url.PathEscape(client.token) + "/" + method
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL, bytes.NewReader(encoded))
	if err != nil {
		return zero, fmt.Errorf("build Telegram request")
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.http.Do(request)
	if err != nil {
		return zero, fmt.Errorf("Telegram API request failed")
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return zero, fmt.Errorf("read Telegram API response")
	}
	var result apiResponse[T]
	if err := json.Unmarshal(body, &result); err != nil {
		return zero, fmt.Errorf("decode Telegram API response (HTTP %d)", response.StatusCode)
	}
	if !result.OK {
		description := strings.TrimSpace(result.Description)
		if description == "" {
			description = "request rejected"
		}
		return zero, fmt.Errorf("Telegram API %s failed (%s): %s", method, strconv.Itoa(result.ErrorCode), description)
	}
	return result.Result, nil
}
