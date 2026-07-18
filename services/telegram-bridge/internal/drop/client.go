package drop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

type Attachment struct {
	Name        string
	ContentType string
	Reader      io.Reader
}

type CreateInput struct {
	Text           string
	TTL            int
	IdempotencyKey string
	Attachment     *Attachment
}

type Item struct {
	ID string `json:"id"`
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: timeout},
	}
}

func (c *Client) Create(ctx context.Context, input CreateInput) (Item, error) {
	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	writeResult := make(chan error, 1)
	go func() {
		err := writeMultipart(multipartWriter, input)
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		_ = writer.CloseWithError(err)
		writeResult <- err
	}()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/items", reader)
	if err != nil {
		_ = reader.CloseWithError(err)
		<-writeResult
		return Item{}, fmt.Errorf("build Drop request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Idempotency-Key", input.IdempotencyKey)
	request.Header.Set("Content-Type", multipartWriter.FormDataContentType())
	response, requestErr := c.http.Do(request)
	if requestErr != nil {
		_ = reader.CloseWithError(requestErr)
	}
	writeErr := <-writeResult
	if requestErr != nil {
		return Item{}, fmt.Errorf("send item to Drop: %w", requestErr)
	}
	defer func() { _ = response.Body.Close() }()
	if writeErr != nil {
		return Item{}, fmt.Errorf("stream item to Drop: %w", writeErr)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return Item{}, fmt.Errorf("read Drop response: %w", err)
	}
	if response.StatusCode != http.StatusCreated {
		return Item{}, fmt.Errorf("Drop rejected item (HTTP %d): %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
	var item Item
	if err := json.Unmarshal(body, &item); err != nil || item.ID == "" {
		return Item{}, fmt.Errorf("decode Drop response")
	}
	return item, nil
}

func writeMultipart(writer *multipart.Writer, input CreateInput) error {
	if input.Text != "" {
		if err := writer.WriteField("text", input.Text); err != nil {
			return err
		}
	}
	if err := writer.WriteField("ttl_days", strconv.Itoa(input.TTL)); err != nil {
		return err
	}
	if input.Attachment == nil {
		return nil
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{
		"name": "files", "filename": escapeFilename(input.Attachment.Name),
	}))
	header.Set("Content-Type", input.Attachment.ContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, input.Attachment.Reader)
	return err
}

func escapeFilename(value string) string {
	value = strings.ReplaceAll(value, "\\", "_")
	value = strings.ReplaceAll(value, `"`, "_")
	value = strings.ReplaceAll(value, "\r", "_")
	return strings.ReplaceAll(value, "\n", "_")
}
