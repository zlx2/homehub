package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gitee.com/zlx23/homehub/services/drop/internal/config"
	"gitee.com/zlx23/homehub/services/drop/internal/store"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

const (
	permissionCreate  = "drop.item.create"
	permissionRead    = "drop.item.read"
	permissionList    = "drop.item.list"
	permissionDelete  = "drop.item.delete"
	multipartOverhead = int64(32 << 20)
)

type Storage interface {
	Ready(context.Context) error
	TemporaryDirectory() string
	Create(context.Context, store.CreateInput) (store.Item, error)
	List(context.Context, store.ListOptions) ([]store.Item, error)
	Get(context.Context, string) (store.Item, error)
	OpenAttachment(context.Context, string) (*os.File, store.Attachment, error)
	Delete(context.Context, string) error
}

type TokenVerifier interface {
	Verify(string) (identity.Claims, error)
}

type API struct {
	config    config.Config
	storage   Storage
	verifier  TokenVerifier
	logger    *slog.Logger
	eventsHub *eventHub
}

func New(configuration config.Config, storage Storage, verifier TokenVerifier, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	api := &API{config: configuration, storage: storage, verifier: verifier, logger: logger, eventsHub: newEventHub()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", api.live)
	mux.HandleFunc("GET /health/ready", api.ready)
	mux.Handle("POST /v1/items", api.authenticate(permissionCreate, http.HandlerFunc(api.createItem)))
	mux.Handle("GET /v1/items", api.authenticate(permissionList, http.HandlerFunc(api.listItems)))
	mux.Handle("GET /v1/items/{id}", api.authenticate(permissionRead, http.HandlerFunc(api.getItem)))
	mux.Handle("DELETE /v1/items/{id}", api.authenticate(permissionDelete, http.HandlerFunc(api.deleteItem)))
	mux.Handle("PATCH /v1/items/{id}/expiry", api.authenticate(permissionDelete, http.HandlerFunc(api.updateExpiry)))
	mux.Handle("GET /v1/attachments/{id}", api.authenticate(permissionRead, http.HandlerFunc(api.getAttachment)))
	mux.Handle("GET /v1/status", api.authenticate(permissionList, http.HandlerFunc(api.status)))
	mux.Handle("GET /v1/events", api.authenticate(permissionList, http.HandlerFunc(api.events)))
	mux.Handle("GET /", webHandler())
	return api.requestID(api.securityHeaders(api.logging(mux)))
}

func (api *API) live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (api *API) ready(response http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), time.Second)
	defer cancel()
	if err := api.storage.Ready(ctx); err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]string{"status": "ready"})
}

func (api *API) authenticate(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		encoded, err := identity.BearerToken(request)
		if err != nil || api.verifier == nil {
			writeError(response, http.StatusUnauthorized, "invalid_token")
			return
		}
		claims, err := api.verifier.Verify(encoded)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "invalid_token")
			return
		}
		if !claims.Allows(permission) {
			writeError(response, http.StatusForbidden, "insufficient_permission")
			return
		}
		next.ServeHTTP(response, request.WithContext(identity.ContextWithClaims(request.Context(), claims)))
	})
}

func (api *API) createItem(response http.ResponseWriter, request *http.Request) {
	if request.ContentLength > api.config.MaxItemBytes+multipartOverhead {
		writeError(response, http.StatusRequestEntityTooLarge, "item_too_large")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, api.config.MaxItemBytes+multipartOverhead)
	reader, err := request.MultipartReader()
	if err != nil {
		writeError(response, http.StatusBadRequest, "multipart_required")
		return
	}
	var text string
	ttlDays := 1
	seenText, seenTTL := false, false
	var pending []store.PendingAttachment
	defer func() {
		for _, attachment := range pending {
			_ = os.Remove(attachment.TempPath)
		}
	}()
	var total int64
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeError(response, http.StatusBadRequest, "invalid_multipart")
			return
		}
		switch {
		case part.FormName() == "text" && part.FileName() == "" && !seenText:
			seenText = true
			contents, readErr := io.ReadAll(io.LimitReader(part, api.config.MaxItemBytes+1))
			_ = part.Close()
			if readErr != nil || int64(len(contents)) > api.config.MaxItemBytes || !utf8.Valid(contents) {
				writeError(response, http.StatusBadRequest, "invalid_text")
				return
			}
			text = string(contents)
			total += int64(len(contents))
		case part.FormName() == "ttl_days" && part.FileName() == "" && !seenTTL:
			seenTTL = true
			contents, _ := io.ReadAll(io.LimitReader(part, 16))
			_ = part.Close()
			ttlDays, err = strconv.Atoi(strings.TrimSpace(string(contents)))
			if err != nil {
				writeError(response, http.StatusBadRequest, "invalid_ttl")
				return
			}
		case part.FormName() == "files" && part.FileName() != "":
			if len(pending) >= api.config.MaxAttachments {
				_ = part.Close()
				writeError(response, http.StatusRequestEntityTooLarge, "too_many_attachments")
				return
			}
			attachment, stageErr := api.stageAttachment(part)
			if stageErr != nil {
				writeAPIError(response, stageErr)
				return
			}
			pending = append(pending, attachment)
			total += attachment.Size
		default:
			_ = part.Close()
			writeError(response, http.StatusBadRequest, "unexpected_multipart_field")
			return
		}
		if total > api.config.MaxItemBytes {
			writeError(response, http.StatusRequestEntityTooLarge, "item_too_large")
			return
		}
	}
	if text == "" && len(pending) == 0 {
		writeError(response, http.StatusBadRequest, "empty_item")
		return
	}
	if ttlDays != 1 && ttlDays != 3 && ttlDays != 7 {
		writeError(response, http.StatusBadRequest, "invalid_ttl")
		return
	}
	claims, _ := identity.FromContext(request.Context())
	var idempotencyHash []byte
	if key := strings.TrimSpace(request.Header.Get("Idempotency-Key")); key != "" {
		if len(key) < 16 || len(key) > 200 {
			writeError(response, http.StatusBadRequest, "invalid_idempotency_key")
			return
		}
		hash := sha256.Sum256([]byte(claims.EffectiveActor() + "\x00" + key))
		idempotencyHash = hash[:]
	}
	item, err := api.storage.Create(request.Context(), store.CreateInput{
		Text: text, CreatorSubject: claims.Subject, ActorSubject: claims.EffectiveActor(),
		TTL: time.Duration(ttlDays) * 24 * time.Hour, IdempotencyKey: idempotencyHash, Attachments: pending,
	})
	if err != nil {
		writeStoreError(response, err)
		return
	}
	api.eventsHub.publish()
	writeJSON(response, http.StatusCreated, api.itemResponse(item))
}

func (api *API) stageAttachment(part *multipart.Part) (store.PendingAttachment, *apiError) {
	file, err := os.CreateTemp(api.storage.TemporaryDirectory(), "upload-*")
	if err != nil {
		_ = part.Close()
		return store.PendingAttachment{}, &apiError{Status: http.StatusServiceUnavailable, Code: "storage_unavailable"}
	}
	name := file.Name()
	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(file, hash), io.LimitReader(part, api.config.MaxAttachmentBytes+1))
	closeErr := errors.Join(file.Close(), part.Close())
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(name)
		return store.PendingAttachment{}, &apiError{Status: http.StatusBadRequest, Code: "upload_failed"}
	}
	if written > api.config.MaxAttachmentBytes {
		_ = os.Remove(name)
		return store.PendingAttachment{}, &apiError{Status: http.StatusRequestEntityTooLarge, Code: "attachment_too_large"}
	}
	mediaType := strings.TrimSpace(part.Header.Get("Content-Type"))
	if mediaType == "" || mediaType == "application/octet-stream" {
		mediaType = detectType(name)
	}
	return store.PendingAttachment{
		TempPath: name, OriginalName: sanitizeFilename(part.FileName()), MediaType: safeMediaType(mediaType),
		Size: written, SHA256: hash.Sum(nil),
	}, nil
}

func (api *API) listItems(response http.ResponseWriter, request *http.Request) {
	limit := 50
	if raw := request.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			writeError(response, http.StatusBadRequest, "invalid_limit")
			return
		}
		limit = parsed
	}
	var before time.Time
	var beforeID string
	if raw := request.URL.Query().Get("cursor"); raw != "" {
		decoded, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			writeError(response, http.StatusBadRequest, "invalid_cursor")
			return
		}
		var cursor struct {
			CreatedAt time.Time `json:"created_at"`
			ID        string    `json:"id"`
		}
		if json.Unmarshal(decoded, &cursor) != nil || cursor.CreatedAt.IsZero() || cursor.ID == "" {
			writeError(response, http.StatusBadRequest, "invalid_cursor")
			return
		}
		before, beforeID = cursor.CreatedAt, cursor.ID
	}
	items, err := api.storage.List(request.Context(), store.ListOptions{Limit: limit, Before: before, BeforeID: beforeID})
	if err != nil {
		writeStoreError(response, err)
		return
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, api.itemResponse(item))
	}
	nextCursor := ""
	if len(items) == limit {
		contents, _ := json.Marshal(map[string]any{"created_at": items[len(items)-1].CreatedAt, "id": items[len(items)-1].ID})
		nextCursor = base64.RawURLEncoding.EncodeToString(contents)
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": result, "next_cursor": nextCursor})
}

func (api *API) getItem(response http.ResponseWriter, request *http.Request) {
	item, err := api.storage.Get(request.Context(), request.PathValue("id"))
	if err != nil {
		writeStoreError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, api.itemResponse(item))
}

func (api *API) deleteItem(response http.ResponseWriter, request *http.Request) {
	if err := api.storage.Delete(request.Context(), request.PathValue("id")); err != nil {
		writeStoreError(response, err)
		return
	}
	api.eventsHub.publish()
	response.WriteHeader(http.StatusNoContent)
}

type expiryStorage interface {
	UpdateExpiry(context.Context, string, time.Duration) (store.Item, error)
}
type statsStorage interface {
	Stats(context.Context) (store.Stats, error)
}

func (api *API) updateExpiry(response http.ResponseWriter, request *http.Request) {
	storage, ok := api.storage.(expiryStorage)
	if !ok {
		writeError(response, http.StatusNotImplemented, "expiry_unavailable")
		return
	}
	var body struct {
		TTLDays int `json:"ttl_days"`
	}
	if json.NewDecoder(http.MaxBytesReader(response, request.Body, 1024)).Decode(&body) != nil || (body.TTLDays != 1 && body.TTLDays != 3 && body.TTLDays != 7) {
		writeError(response, http.StatusBadRequest, "invalid_ttl")
		return
	}
	item, err := storage.UpdateExpiry(request.Context(), request.PathValue("id"), time.Duration(body.TTLDays)*24*time.Hour)
	if err != nil {
		writeStoreError(response, err)
		return
	}
	api.eventsHub.publish()
	writeJSON(response, http.StatusOK, api.itemResponse(item))
}

func (api *API) status(response http.ResponseWriter, request *http.Request) {
	storage, ok := api.storage.(statsStorage)
	if !ok {
		writeError(response, http.StatusNotImplemented, "status_unavailable")
		return
	}
	stats, err := storage.Stats(request.Context())
	if err != nil {
		writeStoreError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"status": "ok", "storage": stats})
}

func (api *API) getAttachment(response http.ResponseWriter, request *http.Request) {
	file, attachment, err := api.storage.OpenAttachment(request.Context(), request.PathValue("id"))
	if err != nil {
		writeStoreError(response, err)
		return
	}
	defer file.Close()
	disposition := "attachment"
	if request.URL.Query().Get("download") != "1" && strings.HasPrefix(attachment.MediaType, "image/") && attachment.MediaType != "image/svg+xml" {
		disposition = "inline"
	}
	response.Header().Set("Content-Type", safeMediaType(attachment.MediaType))
	response.Header().Set("Content-Disposition", contentDisposition(disposition, attachment.OriginalName))
	response.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'")
	response.Header().Set("Digest", "sha-256="+base64.StdEncoding.EncodeToString(attachment.SHA256))
	response.Header().Set("Cache-Control", "private, no-cache")
	response.Header().Set("Vary", "Authorization")
	http.ServeContent(response, request, attachment.OriginalName, attachment.CreatedAt, file)
}

func (api *API) itemResponse(item store.Item) map[string]any {
	attachments := make([]map[string]any, 0, len(item.Attachments))
	for _, attachment := range item.Attachments {
		attachments = append(attachments, map[string]any{
			"id": attachment.ID, "original_name": attachment.OriginalName, "media_type": attachment.MediaType,
			"size": attachment.Size, "sha256": hex.EncodeToString(attachment.SHA256),
			"content_url": "/v1/attachments/" + url.PathEscape(attachment.ID),
		})
	}
	return map[string]any{
		"id": item.ID, "text": item.Text, "creator_subject": item.CreatorSubject, "actor_subject": item.ActorSubject,
		"created_at": item.CreatedAt, "expires_at": item.ExpiresAt, "total_size": item.TotalSize, "attachments": attachments,
	}
}

type apiError struct {
	Status int
	Code   string
}

func (err *apiError) Error() string { return err.Code }

func writeStoreError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(response, http.StatusNotFound, "not_found")
	case errors.Is(err, store.ErrInvalidInput):
		writeError(response, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, store.ErrQuotaExceeded):
		writeError(response, http.StatusInsufficientStorage, "storage_quota_exceeded")
	default:
		writeError(response, http.StatusInternalServerError, "internal_error")
	}
}

func writeAPIError(response http.ResponseWriter, err *apiError) {
	writeError(response, err.Status, err.Code)
}
func writeError(response http.ResponseWriter, status int, code string) {
	writeJSON(response, status, map[string]string{"error": code})
}
func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}

func detectType(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return "application/octet-stream"
	}
	defer file.Close()
	buffer := make([]byte, 512)
	read, _ := file.Read(buffer)
	return http.DetectContentType(buffer[:read])
}

func safeMediaType(value string) string {
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		return "application/octet-stream"
	}
	switch strings.ToLower(mediaType) {
	case "text/html", "application/xhtml+xml", "image/svg+xml", "application/xml", "text/xml":
		return "application/octet-stream"
	default:
		return strings.ToLower(mediaType)
	}
}

func sanitizeFilename(value string) string {
	value = path.Base(strings.ReplaceAll(value, "\\", "/"))
	value = strings.Map(func(character rune) rune {
		if character == 0 || unicode.IsControl(character) {
			return '_'
		}
		return character
	}, value)
	value = strings.TrimSpace(value)
	if value == "" || value == "." {
		value = "file"
	}
	for len(value) > 255 {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func contentDisposition(kind, filename string) string {
	filename = strings.NewReplacer("\\", "_", "\"", "_", "\r", "_", "\n", "_").Replace(filename)
	return fmt.Sprintf("%s; filename=\"%s\"; filename*=UTF-8''%s", kind, filename, url.PathEscape(filename))
}

func (api *API) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var value [12]byte
		_, _ = rand.Read(value[:])
		requestID := hex.EncodeToString(value[:])
		request.Header.Set("X-Request-ID", requestID)
		response.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(response, request)
	})
}

func (api *API) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(response, request)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (recorder *statusRecorder) WriteHeader(status int) {
	recorder.status = status
	recorder.ResponseWriter.WriteHeader(status)
}

func (api *API) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: response, status: http.StatusOK}
		next.ServeHTTP(recorder, request)
		api.logger.Info("request", "request_id", request.Header.Get("X-Request-ID"), "method", request.Method,
			"path", request.URL.Path, "status", recorder.status, "duration_ms", time.Since(started).Milliseconds())
	})
}
