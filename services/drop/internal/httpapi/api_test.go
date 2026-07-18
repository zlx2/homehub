package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gitee.com/zlx23/homehub/services/drop/internal/config"
	"gitee.com/zlx23/homehub/services/drop/internal/store"
	"homehub.local/go-sdk/identity"
)

type fakeVerifier struct {
	claims identity.Claims
	err    error
}

func (verifier fakeVerifier) Verify(string) (identity.Claims, error) {
	return verifier.claims, verifier.err
}

type fakeStorage struct {
	temporary      string
	created        store.CreateInput
	items          []store.Item
	attachmentPath string
}

func (storage *fakeStorage) Ready(context.Context) error { return nil }
func (storage *fakeStorage) TemporaryDirectory() string  { return storage.temporary }
func (storage *fakeStorage) Create(_ context.Context, input store.CreateInput) (store.Item, error) {
	storage.created = input
	for _, attachment := range input.Attachments {
		if _, err := os.ReadFile(attachment.TempPath); err != nil {
			return store.Item{}, err
		}
	}
	return store.Item{ID: "item-1", Text: input.Text, CreatorSubject: input.CreatorSubject, ActorSubject: input.ActorSubject,
		CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour), Attachments: []store.Attachment{}}, nil
}
func (storage *fakeStorage) List(context.Context, store.ListOptions) ([]store.Item, error) {
	return storage.items, nil
}
func (storage *fakeStorage) Get(context.Context, string) (store.Item, error) {
	if len(storage.items) == 0 {
		return store.Item{}, store.ErrNotFound
	}
	return storage.items[0], nil
}
func (storage *fakeStorage) OpenAttachment(context.Context, string) (*os.File, store.Attachment, error) {
	file, err := os.Open(storage.attachmentPath)
	if err != nil {
		return nil, store.Attachment{}, err
	}
	contents, _ := os.ReadFile(storage.attachmentPath)
	hash := sha256.Sum256(contents)
	return file, store.Attachment{ID: "file-1", OriginalName: "原图.png", MediaType: "image/png", Size: int64(len(contents)), SHA256: hash[:], CreatedAt: time.Now()}, nil
}
func (*fakeStorage) Delete(context.Context, string) error { return nil }

func testConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{MaxItemBytes: 1 << 20, MaxAttachmentBytes: 1 << 20, MaxAttachments: 3}
}

func TestPermissionEnforcement(t *testing.T) {
	t.Parallel()
	storage := &fakeStorage{temporary: t.TempDir()}
	tests := []struct {
		name      string
		verifier  TokenVerifier
		authorize bool
		status    int
	}{
		{name: "missing", verifier: fakeVerifier{}, status: http.StatusUnauthorized},
		{name: "invalid", verifier: fakeVerifier{err: errors.New("invalid")}, authorize: true, status: http.StatusUnauthorized},
		{name: "create only", verifier: fakeVerifier{claims: identity.Claims{Permissions: []string{permissionCreate}}}, authorize: true, status: http.StatusForbidden},
		{name: "list", verifier: fakeVerifier{claims: identity.Claims{Permissions: []string{permissionList}}}, authorize: true, status: http.StatusOK},
		{name: "root", verifier: fakeVerifier{claims: identity.Claims{Permissions: []string{identity.SystemRootPermission}}}, authorize: true, status: http.StatusOK},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			handler := New(testConfig(t), storage, test.verifier, slog.New(slog.NewTextHandler(io.Discard, nil)))
			request := httptest.NewRequest(http.MethodGet, "/v1/items", nil)
			if test.authorize {
				request.Header.Set("Authorization", "Bearer token")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
			}
		})
	}
}

func TestCreateStagesOriginalBytesAndAttribution(t *testing.T) {
	t.Parallel()
	storage := &fakeStorage{temporary: t.TempDir()}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("text", "hello")
	_ = writer.WriteField("ttl_days", "3")
	part, _ := writer.CreateFormFile("files", "截图.png")
	original := []byte{0x89, 'P', 'N', 'G', 0x00, 0x01, 0xff}
	_, _ = part.Write(original)
	_ = writer.Close()
	handler := New(testConfig(t), storage, fakeVerifier{claims: identity.Claims{
		Subject: "human:luna", Actor: &identity.Actor{Subject: "agent:hermes"}, Permissions: []string{permissionCreate},
	}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodPost, "/v1/items", &body)
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Idempotency-Key", "telegram:1:2:3456")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if storage.created.CreatorSubject != "human:luna" || storage.created.ActorSubject != "agent:hermes" || storage.created.TTL != 72*time.Hour {
		t.Fatalf("unexpected attribution: %+v", storage.created)
	}
	if len(storage.created.Attachments) != 1 || !bytes.Equal(storage.created.Attachments[0].SHA256, sha256Sum(original)) {
		t.Fatal("original attachment hash was not preserved")
	}
}

func TestAttachmentReturnsOriginalInline(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	filename := filepath.Join(directory, "original")
	original := []byte("original-image-bytes")
	if err := os.WriteFile(filename, original, 0o600); err != nil {
		t.Fatal(err)
	}
	storage := &fakeStorage{temporary: directory, attachmentPath: filename}
	handler := New(testConfig(t), storage, fakeVerifier{claims: identity.Claims{Permissions: []string{permissionRead}}}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	request := httptest.NewRequest(http.MethodGet, "/v1/attachments/file-1", nil)
	request.Header.Set("Authorization", "Bearer token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), original) {
		t.Fatalf("status=%d body=%q", response.Code, response.Body.Bytes())
	}
	if disposition := response.Header().Get("Content-Disposition"); len(disposition) < 6 || disposition[:6] != "inline" {
		t.Fatalf("disposition=%q", disposition)
	}
}

func sha256Sum(value []byte) []byte { sum := sha256.Sum256(value); return sum[:] }
