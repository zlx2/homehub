package httpapi

import (
	"encoding/json"
	"net/http"

	"homehub.local/go-sdk/identity"
)

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func New(verifier *identity.Verifier) http.Handler {
	protected := http.NewServeMux()
	protected.HandleFunc("GET /v1/models", models)
	protected.HandleFunc("POST /v1/chat/completions", chatCompletions)
	root := http.NewServeMux()
	root.HandleFunc("GET /health/live", noContent)
	root.HandleFunc("GET /health/ready", noContent)
	root.Handle("/", verifier.Authenticate([]string{"admin", "ai.use"}, protected))
	return root
}

func models(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"object": "list", "data": []any{}})
}

func chatCompletions(writer http.ResponseWriter, request *http.Request) {
	request.Body = http.MaxBytesReader(writer, request.Body, 1<<20)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input chatRequest
	if err := decoder.Decode(&input); err != nil || input.Model == "" || len(input.Messages) == 0 {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	writeJSON(writer, http.StatusServiceUnavailable, map[string]string{
		"error":   "provider_unconfigured",
		"message": "No AI provider has been configured for this model alias.",
	})
}

func noContent(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}
