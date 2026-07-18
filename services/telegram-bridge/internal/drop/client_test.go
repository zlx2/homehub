package drop

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type staticTokens struct {
	token string
	calls int
}

func (source *staticTokens) Token(context.Context) (string, error) {
	source.calls++
	return source.token, nil
}

func TestCreateStreamsAuthenticatedMultipart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/items" || request.Method != http.MethodPost {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer drop-secret" {
			t.Fatal("missing bearer token")
		}
		if request.Header.Get("Idempotency-Key") != "telegram:123:456:789" {
			t.Fatal("missing idempotency key")
		}
		reader, err := request.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{}
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(part)
			fields[part.FormName()] = part.FileName() + "|" + part.Header.Get("Content-Type") + "|" + string(body)
		}
		if fields["text"] != "||hello" || fields["ttl_days"] != "||3" || fields["files"] != "image.png|image/png|original-bytes" {
			t.Fatalf("multipart fields = %#v", fields)
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(`{"id":"drop-item"}`))
	}))
	defer server.Close()

	tokens := &staticTokens{token: "drop-secret"}
	client := NewClient(server.URL, tokens, 5*time.Second)
	item, err := client.Create(context.Background(), CreateInput{
		Text: "hello", TTL: 3, IdempotencyKey: "telegram:123:456:789",
		Attachment: &Attachment{Name: "image.png", ContentType: "image/png", Reader: strings.NewReader("original-bytes")},
	})
	if err != nil || item.ID != "drop-item" || tokens.calls != 1 {
		t.Fatalf("item=%#v err=%v", item, err)
	}
}

func TestCreateReturnsDropError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, `{"error":{"code":"denied"}}`, http.StatusForbidden)
	}))
	defer server.Close()
	client := NewClient(server.URL, &staticTokens{token: "secret"}, time.Second)
	_, err := client.Create(context.Background(), CreateInput{Text: "x", TTL: 1, IdempotencyKey: "1234567890123456"})
	if err == nil || !strings.Contains(err.Error(), "HTTP 403") {
		t.Fatalf("error = %v", err)
	}
}
