package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"homehub.local/go-sdk/httpx"
	"homehub.local/go-sdk/identity"
	"homehub.local/services/ai-gateway/internal/gateway"
)

const maxRequestBody = 4 << 20

type API struct {
	router   *gateway.Router
	logger   *slog.Logger
	modelIDs []string
}

func New(verifier *identity.Verifier, router *gateway.Router, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	api := &API{router: router, logger: logger, modelIDs: router.ModelIDs()}
	protected := http.NewServeMux()
	protected.HandleFunc("GET /v1/models", api.models)
	protected.HandleFunc("POST /v1/chat/completions", api.chatCompletions)
	root := http.NewServeMux()
	root.HandleFunc("GET /health/live", noContent)
	root.HandleFunc("GET /health/ready", noContent)
	root.Handle("/", verifier.Authenticate(nil, protected))
	return root
}

func (api *API) models(writer http.ResponseWriter, request *http.Request) {
	_, models, ok := api.allowedModels(request)
	if !ok {
		writeError(writer, http.StatusForbidden, "invalid_delegation", "AI delegation policy is missing.")
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"object": "list", "data": api.router.Models(models)})
}

func (api *API) chatCompletions(writer http.ResponseWriter, request *http.Request) {
	claims, models, ok := api.allowedModels(request)
	if !ok {
		writeError(writer, http.StatusForbidden, "invalid_delegation", "AI delegation policy is missing.")
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBody)
	body, err := io.ReadAll(request.Body)
	if err != nil {
		writeError(writer, http.StatusBadRequest, "invalid_request", "The request body is invalid or too large.")
		return
	}
	started := time.Now()
	result, err := api.router.Complete(request.Context(), body, models, httpx.RequestIDFromContext(request.Context()))
	if err != nil {
		api.logFailure(request, claims, started, err)
		switch {
		case errors.Is(err, gateway.ErrInvalidRequest):
			writeError(writer, http.StatusBadRequest, "invalid_request", "A model alias and at least one message are required.")
		case errors.Is(err, gateway.ErrModelForbidden):
			writeError(writer, http.StatusForbidden, "model_not_allowed", "The delegated service cannot use this model.")
		case errors.Is(err, gateway.ErrModelNotFound):
			writeError(writer, http.StatusNotFound, "model_not_found", "The requested model alias does not exist.")
		default:
			writeError(writer, http.StatusBadGateway, "provider_unavailable", "The configured AI provider is unavailable.")
		}
		return
	}
	defer gateway.DrainAndClose(result.Response)
	status := result.Response.StatusCode
	if status < 200 || status >= 300 {
		api.logger.Warn("AI provider rejected request",
			"request_id", httpx.RequestIDFromContext(request.Context()),
			"source_service", claims.AuthorizedParty, "subject", claims.Subject,
			"model", result.Alias, "provider", result.Provider, "upstream_status", status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
		if status == http.StatusTooManyRequests {
			writeError(writer, http.StatusTooManyRequests, "upstream_rate_limited", "The AI provider rate limit was reached.")
			return
		}
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			writeError(writer, http.StatusBadGateway, "provider_auth_failed", "The AI provider rejected its configured credential.")
			return
		}
		writeError(writer, http.StatusBadGateway, "provider_error", "The AI provider rejected the request.")
		return
	}
	copyResponseHeaders(writer.Header(), result.Response.Header, result.Stream)
	writer.WriteHeader(status)
	copyErr := copyBody(writer, result.Response.Body, result.Stream)
	api.logger.Info("AI completion",
		"request_id", httpx.RequestIDFromContext(request.Context()),
		"source_service", claims.AuthorizedParty, "subject", claims.Subject,
		"model", result.Alias, "provider", result.Provider, "stream", result.Stream,
		"status", status, "duration_ms", time.Since(started).Milliseconds(), "copy_error", copyErr != nil,
	)
}

func (api *API) allowedModels(request *http.Request) (identity.Claims, []string, bool) {
	claims, ok := identity.FromContext(request.Context())
	if !ok || claims.Audience != "homehub-ai-gateway" || claims.AuthorizedParty == "" {
		return claims, nil, false
	}
	if claims.HasPermission(identity.SystemRootPermission) {
		return claims, append([]string(nil), api.modelIDs...), true
	}
	allowed := make([]string, 0, len(claims.Permissions))
	for _, permission := range claims.Permissions {
		if strings.HasPrefix(permission, "ai.model.") {
			allowed = append(allowed, strings.TrimPrefix(permission, "ai.model."))
		}
	}
	return claims, allowed, len(allowed) != 0
}

func copyResponseHeaders(destination, source http.Header, stream bool) {
	if contentType := source.Get("Content-Type"); contentType != "" {
		destination.Set("Content-Type", contentType)
	} else if stream {
		destination.Set("Content-Type", "text/event-stream")
	} else {
		destination.Set("Content-Type", "application/json")
	}
	if stream {
		destination.Set("Cache-Control", "no-cache")
		destination.Set("X-Accel-Buffering", "no")
	}
}

func copyBody(writer http.ResponseWriter, source io.Reader, stream bool) error {
	if !stream {
		_, err := io.Copy(writer, source)
		return err
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		return errors.New("streaming is unsupported by the response writer")
	}
	buffer := make([]byte, 32<<10)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			if _, err := writer.Write(buffer[:count]); err != nil {
				return err
			}
			flusher.Flush()
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func (api *API) logFailure(request *http.Request, claims identity.Claims, started time.Time, err error) {
	api.logger.Warn("AI completion failed",
		"request_id", httpx.RequestIDFromContext(request.Context()),
		"source_service", claims.AuthorizedParty, "subject", claims.Subject,
		"duration_ms", time.Since(started).Milliseconds(), "error", err,
	)
}

func noContent(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusNoContent) }

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, code, message string) {
	writeJSON(writer, status, map[string]any{"error": map[string]string{
		"message": message, "type": "homehub_gateway_error", "code": code,
	}})
}
