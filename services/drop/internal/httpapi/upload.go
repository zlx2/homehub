package httpapi

import (
	"crypto/sha256"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"drop/internal/store"
)

const multipartOverheadAllowance = int64(32 << 20)

func (a *API) createItem(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	var idempotencyHash []byte
	if idempotencyKey != "" {
		if !validIdempotencyKey(idempotencyKey) {
			writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_idempotency_key", Message: "Idempotency-Key must contain 16 to 200 visible ASCII characters"})
			return
		}
		sum := sha256.Sum256([]byte(string(principal.Role) + "\x00" + principal.Subject + "\x00" + idempotencyKey))
		idempotencyHash = sum[:]
	}
	if r.ContentLength > 0 && r.ContentLength > a.cfg.MaxItemBytes+multipartOverheadAllowance {
		writeAPIError(w, tooLarge("item_too_large", "Message exceeds the maximum total size", a.cfg.MaxItemBytes))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxItemBytes+multipartOverheadAllowance)
	reader, err := r.MultipartReader()
	if err != nil {
		writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "multipart_required", Message: "Request must use multipart/form-data"})
		return
	}

	var input store.CreateItemInput
	var staged []string
	defer func() {
		for _, file := range staged {
			_ = os.Remove(file)
		}
	}()
	var total int64
	var ttlDays = 1
	seenText := false
	seenTTL := false

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_multipart", Message: "Malformed multipart request"})
			return
		}
		formName := part.FormName()
		switch {
		case formName == "text" && part.FileName() == "":
			if seenText {
				_ = part.Close()
				writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "duplicate_text", Message: "Text may be supplied once"})
				return
			}
			seenText = true
			stagedPath, size, err := a.stagePart(part, "text-*", a.cfg.MaxTextBytes)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			if size > 0 {
				input.TextTempPath = stagedPath
				input.TextSize = size
				staged = append(staged, stagedPath)
			} else {
				_ = os.Remove(stagedPath)
			}
			total += size
		case formName == "ttl_days" && part.FileName() == "":
			if seenTTL {
				_ = part.Close()
				writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "duplicate_ttl", Message: "TTL may be supplied once"})
				return
			}
			seenTTL = true
			value, err := io.ReadAll(io.LimitReader(part, 32))
			_ = part.Close()
			if err != nil {
				writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_ttl", Message: "TTL is invalid"})
				return
			}
			ttlDays, err = strconv.Atoi(strings.TrimSpace(string(value)))
			if err != nil {
				writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "invalid_ttl", Message: "TTL must be 1, 3, or 7 days"})
				return
			}
		case formName == "files" && part.FileName() != "":
			if len(input.Attachments) >= a.cfg.MaxAttachments {
				_ = part.Close()
				writeAPIError(w, &apiError{
					Status: http.StatusRequestEntityTooLarge, Code: "too_many_attachments", Message: "Too many attachments",
					Details: map[string]any{"max_attachments": a.cfg.MaxAttachments},
				})
				return
			}
			stagedPath, size, err := a.stagePart(part, "upload-*", a.cfg.MaxAttachmentBytes)
			if err != nil {
				writeAPIError(w, err)
				return
			}
			staged = append(staged, stagedPath)
			contentType := strings.TrimSpace(part.Header.Get("Content-Type"))
			if contentType == "" || contentType == "application/octet-stream" {
				contentType = detectFileType(stagedPath)
			}
			input.Attachments = append(input.Attachments, store.PendingAttachment{
				TempPath: stagedPath, OriginalName: sanitizeFilename(part.FileName()), MIMEType: contentType, Size: size,
			})
			total += size
		default:
			_ = part.Close()
			writeAPIError(w, &apiError{Status: http.StatusBadRequest, Code: "unexpected_multipart_field", Message: "Unexpected multipart field"})
			return
		}
		if total > a.cfg.MaxItemBytes {
			writeAPIError(w, tooLarge("item_too_large", "Message exceeds the maximum total size", a.cfg.MaxItemBytes))
			return
		}
	}

	role := principal.Role
	if role == RoleGuest && seenTTL && ttlDays != 1 {
		writeAPIError(w, &apiError{Status: http.StatusForbidden, Code: "ttl_not_allowed", Message: "Temporary guests may only create 24-hour messages"})
		return
	}
	ttl, ok := allowedTTL(ttlDays)
	if !ok {
		writeAPIError(w, &apiError{
			Status: http.StatusBadRequest, Code: "invalid_ttl", Message: "TTL must be 1, 3, or 7 days",
			Details: map[string]any{"allowed_days": []int{1, 3, 7}},
		})
		return
	}
	input.TTL = ttl
	input.Source = string(role)
	input.IdempotencyKey = idempotencyHash
	item, err := a.store.CreateItem(r.Context(), input)
	if err != nil {
		if !errors.Is(err, store.ErrInvalidInput) && !errors.Is(err, store.ErrQuotaExceeded) {
			a.logInternal("create item", err)
		}
		writeAPIError(w, err)
		return
	}
	a.hub.Publish("created", item.ID)
	response, err := a.itemResponse(r, item)
	if err != nil {
		a.logInternal("build created item response", err)
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func validIdempotencyKey(value string) bool {
	if len(value) < 16 || len(value) > 200 {
		return false
	}
	for i := range len(value) {
		if value[i] < 0x21 || value[i] > 0x7e {
			return false
		}
	}
	return true
}

func (a *API) stagePart(part *multipart.Part, pattern string, limit int64) (string, int64, error) {
	file, err := os.CreateTemp(a.store.TmpDir(), pattern)
	if err != nil {
		_ = part.Close()
		return "", 0, internalError()
	}
	path := file.Name()
	buffer := make([]byte, 64<<10)
	written, copyErr := io.CopyBuffer(file, io.LimitReader(part, limit+1), buffer)
	closeErr := file.Close()
	partCloseErr := part.Close()
	if copyErr != nil || closeErr != nil || partCloseErr != nil {
		_ = os.Remove(path)
		return "", 0, &apiError{Status: http.StatusBadRequest, Code: "upload_failed", Message: "Upload stream failed"}
	}
	if written > limit {
		_ = os.Remove(path)
		return "", 0, tooLarge("part_too_large", "A multipart part exceeds its size limit", limit)
	}
	return path, written, nil
}

func tooLarge(code, message string, limit int64) *apiError {
	return &apiError{Status: http.StatusRequestEntityTooLarge, Code: code, Message: message, Details: map[string]any{"limit_bytes": limit}}
}

func detectFileType(filename string) string {
	file, err := os.Open(filename)
	if err != nil {
		return "application/octet-stream"
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, 512)
	read, _ := file.Read(buffer)
	return http.DetectContentType(buffer[:read])
}

func sanitizeFilename(value string) string {
	value = strings.ReplaceAll(value, "\\", "/")
	value = path.Base(value)
	value = strings.Map(func(char rune) rune {
		if char == 0 || unicode.IsControl(char) {
			return '_'
		}
		return char
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
