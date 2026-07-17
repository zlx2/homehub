package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"homehub.local/control/internal/catalog"
	"homehub.local/control/internal/health"
)

type StatusProvider interface {
	Snapshot() map[string]health.Result
}

type Options struct {
	Logger      *slog.Logger
	Services    []catalog.Service
	Statuses    StatusProvider
	Version     string
	Commit      string
	Environment string
}

type server struct {
	logger      *slog.Logger
	services    []catalog.Service
	statuses    StatusProvider
	version     string
	commit      string
	environment string
}

func New(options Options) http.Handler {
	api := &server{
		logger:      options.Logger,
		services:    append([]catalog.Service(nil), options.Services...),
		statuses:    options.Statuses,
		version:     options.Version,
		commit:      options.Commit,
		environment: options.Environment,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", api.live)
	mux.HandleFunc("GET /health/ready", api.ready)
	mux.HandleFunc("GET /api/v1/system", api.system)
	mux.HandleFunc("GET /api/v1/services", api.listServices)
	mux.HandleFunc("GET /api/v1/services/{id}", api.getService)
	return api.recover(api.requestID(api.securityHeaders(api.logRequests(mux))))
}

func (api *server) live(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC(),
	})
}

func (api *server) ready(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"status":   "ready",
		"services": len(api.services),
	})
}

func (api *server) system(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{
		"name":         "HomeHub",
		"version":      api.version,
		"commit":       api.commit,
		"environment":  api.environment,
		"auth_enabled": false,
		"time":         time.Now().UTC(),
	})
}

type serviceResponse struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Icon         string         `json:"icon"`
	Route        string         `json:"route,omitempty"`
	Visibility   string         `json:"visibility"`
	ShareEnabled bool           `json:"share_enabled"`
	Health       healthResponse `json:"health"`
}

type healthResponse struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	LatencyMS int64     `json:"latency_ms"`
}

func (api *server) listServices(writer http.ResponseWriter, _ *http.Request) {
	statuses := api.statuses.Snapshot()
	services := make([]serviceResponse, 0, len(api.services))
	for _, service := range api.services {
		services = append(services, publicService(service, statuses[service.ID]))
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"generated_at": time.Now().UTC(),
		"services":     services,
	})
}

func (api *server) getService(writer http.ResponseWriter, request *http.Request) {
	id := request.PathValue("id")
	statuses := api.statuses.Snapshot()
	for _, service := range api.services {
		if service.ID == id {
			writeJSON(writer, http.StatusOK, publicService(service, statuses[service.ID]))
			return
		}
	}
	writeJSON(writer, http.StatusNotFound, map[string]string{"error": "service_not_found"})
}

func publicService(service catalog.Service, status health.Result) serviceResponse {
	return serviceResponse{
		ID:           service.ID,
		Name:         service.Name,
		Description:  service.Description,
		Icon:         service.Icon,
		Route:        service.Route,
		Visibility:   service.Visibility,
		ShareEnabled: service.ShareEnabled,
		Health: healthResponse{
			Status:    status.Status,
			CheckedAt: status.CheckedAt,
			LatencyMS: status.LatencyMS,
		},
	}
}

func (api *server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := strings.TrimSpace(request.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 {
			requestID = randomID()
		}
		writer.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(writer, request)
	})
}

func (api *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Content-Type", "application/json; charset=utf-8")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(writer, request)
	})
}

func (api *server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(writer, request)
		api.logger.Info("http request",
			"method", request.Method,
			"path", request.URL.Path,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func (api *server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				api.logger.Error("panic recovered", "error", recovered, "stack", string(debug.Stack()))
				writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			}
		}()
		next.ServeHTTP(writer, request)
	})
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.WriteHeader(status)
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		fmt.Fprintln(writer, `{"error":"encode_failed"}`)
	}
}

func randomID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes[:])
}
