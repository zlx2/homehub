package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"drop/internal/store"
)

const previewBytes = 4096

type attachmentResponse struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	MIMEType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	Previewable  bool   `json:"previewable"`
	DownloadURL  string `json:"download_url"`
	PreviewURL   string `json:"preview_url,omitempty"`
}

type itemResponse struct {
	ID            string               `json:"id"`
	TextPreview   string               `json:"text_preview,omitempty"`
	TextTruncated bool                 `json:"text_truncated"`
	TextSize      int64                `json:"text_size"`
	HasText       bool                 `json:"has_text"`
	Source        string               `json:"source"`
	CreatedAt     time.Time            `json:"created_at"`
	ExpiresAt     time.Time            `json:"expires_at"`
	TotalSize     int64                `json:"total_size"`
	Attachments   []attachmentResponse `json:"attachments"`
	FullTextURL   string               `json:"full_text_url,omitempty"`
}

type listCursor struct {
	CreatedAt int64  `json:"created_at"`
	ID        string `json:"id"`
}

func (a *API) listItems(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 200 {
			writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_limit", Message: "Limit must be between 1 and 200"})
			return
		}
		limit = parsed
	}
	var before time.Time
	var beforeID string
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		cursor, err := decodeListCursor(raw)
		if err != nil {
			writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_cursor", Message: "Cursor is invalid"})
			return
		}
		before = time.UnixMilli(cursor.CreatedAt).UTC()
		beforeID = cursor.ID
	} else if raw := r.URL.Query().Get("before"); raw != "" {
		parsed, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_cursor", Message: "Before must be an RFC3339 timestamp"})
			return
		}
		before = parsed
	}
	items, err := a.store.ListItems(r.Context(), store.ListOptions{Limit: limit, Before: before, BeforeID: beforeID})
	if err != nil {
		a.logInternal("list items", err)
		writeAPIError(w, err)
		return
	}
	responses := make([]itemResponse, 0, len(items))
	for _, item := range items {
		response, err := a.itemResponse(r, item)
		if err != nil {
			a.logInternal("build item response", err)
			writeAPIError(w, err)
			return
		}
		responses = append(responses, response)
	}
	var nextCursor string
	if len(items) == limit {
		nextCursor = encodeListCursor(items[len(items)-1])
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": responses, "next_cursor": nextCursor})
}

func encodeListCursor(item store.Item) string {
	encoded, _ := json.Marshal(listCursor{CreatedAt: item.CreatedAt.UnixMilli(), ID: item.ID})
	return base64.RawURLEncoding.EncodeToString(encoded)
}

func decodeListCursor(value string) (listCursor, error) {
	content, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return listCursor{}, err
	}
	var cursor listCursor
	if err := json.Unmarshal(content, &cursor); err != nil || cursor.CreatedAt <= 0 || cursor.ID == "" || len(cursor.ID) > 128 {
		return listCursor{}, errors.New("invalid cursor")
	}
	return cursor, nil
}

func (a *API) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := a.store.GetItem(r.Context(), r.PathValue("id"))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.logInternal("get item", err)
		}
		writeAPIError(w, err)
		return
	}
	response, err := a.itemResponse(r, item)
	if err != nil {
		a.logInternal("build item response", err)
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (a *API) deleteItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := a.store.DeleteItem(r.Context(), id); err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.logInternal("delete item", err)
		}
		writeAPIError(w, err)
		return
	}
	a.hub.Publish("deleted", id)
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) updateExpiry(w http.ResponseWriter, r *http.Request) {
	var input struct {
		TTLDays int `json:"ttl_days"`
	}
	if err := decodeJSON(w, r, 4096, &input); err != nil {
		writeAPIError(w, err)
		return
	}
	ttl, ok := allowedTTL(input.TTLDays)
	if !ok {
		writeAPIError(w, &apiError{
			Status: http.StatusBadRequest, Code: "invalid_ttl", Message: "TTL must be 1, 3, or 7 days",
			Details: map[string]any{"allowed_days": []int{1, 3, 7}},
		})
		return
	}
	id := r.PathValue("id")
	expires, err := a.store.UpdateExpiry(r.Context(), id, ttl)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.logInternal("update item expiry", err)
		}
		writeAPIError(w, err)
		return
	}
	a.hub.Publish("expiry_updated", id)
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "expires_at": expires})
}

func (a *API) getText(w http.ResponseWriter, r *http.Request) {
	reader, size, err := a.store.ReadText(r.Context(), r.PathValue("id"))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.logInternal("open item text", err)
		}
		writeAPIError(w, err)
		return
	}
	defer func() { _ = reader.Close() }()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Query().Get("download") == "1" {
		w.Header().Set("Content-Disposition", contentDisposition("attachment", "message-"+r.PathValue("id")+".txt"))
	} else {
		w.Header().Set("Content-Disposition", "inline")
	}
	w.WriteHeader(http.StatusOK)
	if _, err := io.Copy(w, reader); err != nil {
		a.logger.Debug("text response ended", "error", err)
	}
}

func (a *API) getAttachment(w http.ResponseWriter, r *http.Request) {
	file, attachment, err := a.store.OpenAttachment(r.Context(), r.PathValue("id"))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.logInternal("open attachment", err)
		}
		writeAPIError(w, err)
		return
	}
	defer func() { _ = file.Close() }()
	contentType := safeContentType(attachment.MIMEType)
	disposition := "attachment"
	if r.URL.Query().Get("download") != "1" && isInlineMedia(contentType) {
		disposition = "inline"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", contentDisposition(disposition, attachment.OriginalName))
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", fmt.Sprintf("%q", attachment.ID+"-"+strconv.FormatInt(attachment.Size, 10)))
	w.Header().Set("Vary", "Cookie, Authorization, Tailscale-User-Login")
	http.ServeContent(w, r, attachment.OriginalName, attachment.CreatedAt, file)
}

func (a *API) itemResponse(r *http.Request, item store.Item) (itemResponse, error) {
	response := itemResponse{
		ID: item.ID, TextSize: item.TextSize, HasText: item.TextSize > 0, Source: item.Source,
		CreatedAt: item.CreatedAt, ExpiresAt: item.ExpiresAt, TotalSize: item.TotalSize,
		Attachments: make([]attachmentResponse, 0, len(item.Attachments)),
	}
	if item.TextSize > 0 {
		preview, truncated, err := a.textPreview(r.Context(), item)
		if err != nil {
			return itemResponse{}, err
		}
		response.TextPreview = preview
		response.TextTruncated = truncated
		response.FullTextURL = a.cfg.BasePath + "/api/v1/items/" + url.PathEscape(item.ID) + "/text"
	}
	for _, attachment := range item.Attachments {
		attachmentResponse := attachmentResponse{
			ID: attachment.ID, OriginalName: attachment.OriginalName, MIMEType: safeContentType(attachment.MIMEType),
			Size: attachment.Size, Previewable: isPreviewableImage(attachment.MIMEType),
			DownloadURL: a.cfg.BasePath + "/api/v1/attachments/" + url.PathEscape(attachment.ID),
		}
		if isThumbnailableImage(attachment.MIMEType) {
			attachmentResponse.PreviewURL = attachmentResponse.DownloadURL + "/preview"
		}
		response.Attachments = append(response.Attachments, attachmentResponse)
	}
	return response, nil
}

func (a *API) textPreview(ctx context.Context, item store.Item) (string, bool, error) {
	var preview []byte
	if item.TextStorage == "" {
		preview = item.TextInline
		if len(preview) > previewBytes+1 {
			preview = preview[:previewBytes+1]
		}
	} else {
		reader, _, err := a.store.ReadText(ctx, item.ID)
		if err != nil {
			return "", false, err
		}
		preview, err = io.ReadAll(io.LimitReader(reader, previewBytes+1))
		closeErr := reader.Close()
		if err != nil {
			return "", false, err
		}
		if closeErr != nil {
			return "", false, closeErr
		}
	}
	truncated := item.TextSize > previewBytes
	if truncated && len(preview) > previewBytes {
		preview = preview[:previewBytes]
		for !utf8.Valid(preview) && len(preview) > 0 {
			preview = preview[:len(preview)-1]
		}
	}
	return string(preview), truncated, nil
}

func allowedTTL(days int) (time.Duration, bool) {
	switch days {
	case 1, 3, 7:
		return time.Duration(days) * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func safeContentType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		return "application/octet-stream"
	}
	switch strings.ToLower(mediaType) {
	case "text/html", "application/xhtml+xml", "image/svg+xml", "application/xml", "text/xml":
		return "application/octet-stream"
	default:
		return mediaType
	}
}

func isPreviewableImage(value string) bool {
	switch safeContentType(value) {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif":
		return true
	default:
		return false
	}
}

func isInlineMedia(value string) bool {
	if isPreviewableImage(value) {
		return true
	}
	switch safeContentType(value) {
	case "video/mp4", "video/webm", "video/quicktime", "audio/mpeg", "audio/mp4", "audio/ogg", "audio/wav":
		return true
	default:
		return false
	}
}

func contentDisposition(kind, filename string) string {
	filename = strings.ReplaceAll(filename, "\\", "_")
	filename = strings.ReplaceAll(filename, "\"", "_")
	filename = strings.ReplaceAll(filename, "\r", "_")
	filename = strings.ReplaceAll(filename, "\n", "_")
	if filename == "" {
		filename = "download"
	}
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", kind, filename, url.PathEscape(filename))
}

func (a *API) logInternal(operation string, err error) {
	a.logger.Error(operation, "error", err)
}
