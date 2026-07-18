package telegram

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCallsBotAPIAndDownloadsFile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/botbot-secret/getUpdates":
			body, _ := io.ReadAll(request.Body)
			if !strings.Contains(string(body), `"offset":44`) {
				t.Fatalf("getUpdates body = %s", body)
			}
			_, _ = writer.Write([]byte(`{"ok":true,"result":[{"update_id":44,"message":{"message_id":9,"chat":{"id":23,"type":"private"},"text":"hello"}}]}`))
		case "/botbot-secret/getFile":
			_, _ = writer.Write([]byte(`{"ok":true,"result":{"file_id":"f","file_size":3,"file_path":"docs/a.txt"}}`))
		case "/file/botbot-secret/docs/a.txt":
			_, _ = writer.Write([]byte("abc"))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	client := NewClient(server.URL, "bot-secret", 5*time.Second)
	updates, err := client.GetUpdates(context.Background(), 44, 50*time.Second)
	if err != nil || len(updates) != 1 || updates[0].Content().Text != "hello" {
		t.Fatalf("updates=%#v err=%v", updates, err)
	}
	file, err := client.GetFile(context.Background(), "f")
	if err != nil || file.FilePath != "docs/a.txt" {
		t.Fatalf("file=%#v err=%v", file, err)
	}
	body, err := client.Download(context.Background(), file.FilePath)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	if string(data) != "abc" {
		t.Fatalf("download = %q", data)
	}
}

func TestTransportErrorDoesNotExposeBotToken(t *testing.T) {
	client := NewClient("http://127.0.0.1:1", "super-secret-token", 100*time.Millisecond)
	_, err := client.GetMe(context.Background())
	if err == nil || strings.Contains(err.Error(), "super-secret-token") {
		t.Fatalf("error leaked token: %v", err)
	}
}
