package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"gitee.com/zlx23/homehub/apps/control/internal/catalog"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

const (
	dashboardRead = "control.dashboard.read"
	nodeRead      = "control.node.read"
)

type TokenVerifier interface {
	Verify(string) (identity.Claims, error)
}

type Options struct {
	Logger       *slog.Logger
	Verifier     TokenVerifier
	Services     []catalog.Service
	HealthClient *http.Client
	Version      string
	Commit       string
	Environment  string
}

type server struct {
	logger       *slog.Logger
	verifier     TokenVerifier
	services     []catalog.Service
	healthClient *http.Client
	version      string
	commit       string
	environment  string
}

func New(options Options) http.Handler {
	api := &server{
		logger: options.Logger, verifier: options.Verifier, services: append([]catalog.Service(nil), options.Services...),
		healthClient: options.HealthClient, version: options.Version, commit: options.Commit, environment: options.Environment,
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", api.live)
	mux.HandleFunc("GET /health/ready", api.ready)
	mux.Handle("GET /v1/whoami", api.authenticate(nil, http.HandlerFunc(api.whoami)))
	mux.Handle("GET /v1/overview", api.authenticate([]string{dashboardRead}, http.HandlerFunc(api.overview)))
	mux.Handle("GET /v1/services", api.authenticate([]string{dashboardRead}, http.HandlerFunc(api.listServices)))
	mux.Handle("GET /v1/nodes", api.authenticate([]string{nodeRead}, http.HandlerFunc(api.listNodes)))
	return api.recover(api.requestID(api.securityHeaders(api.logRequests(mux))))
}

func (api *server) live(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{"status": "ok", "time": time.Now().UTC()})
}

func (api *server) ready(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{"status": "ready", "catalog_services": len(api.services)})
}

func (api *server) whoami(response http.ResponseWriter, request *http.Request) {
	claims, _ := identity.FromContext(request.Context())
	writeJSON(response, http.StatusOK, map[string]any{
		"subject": claims.Subject, "actor": claims.EffectiveActor(), "authorized_party": claims.AuthorizedParty,
		"realm": claims.Realm, "permissions": claims.Permissions, "expires_at": time.Unix(claims.Expires, 0).UTC(),
	})
}

func (api *server) overview(response http.ResponseWriter, request *http.Request) {
	claims, _ := identity.FromContext(request.Context())
	services := catalog.Probe(request.Context(), api.healthClient, api.services)
	healthy := 0
	for _, service := range services {
		if service.Status.State == "healthy" {
			healthy++
		}
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"system":    map[string]any{"name": "HomeHub", "version": api.version, "commit": api.commit, "environment": api.environment, "time": time.Now().UTC()},
		"principal": map[string]any{"subject": claims.Subject, "actor": claims.EffectiveActor()},
		"summary":   map[string]int{"total_services": len(services), "healthy_services": healthy},
		"services":  services,
	})
}

func (api *server) listServices(response http.ResponseWriter, request *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{"services": catalog.Probe(request.Context(), api.healthClient, api.services)})
}

func (api *server) listNodes(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{"nodes": []any{}})
}

func (api *server) authenticate(requiredAny []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		token, err := identity.BearerToken(request)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "invalid_token")
			return
		}
		claims, err := api.verifier.Verify(token)
		if err != nil {
			writeError(response, http.StatusUnauthorized, "invalid_token")
			return
		}
		if len(requiredAny) > 0 {
			allowed := false
			for _, permission := range requiredAny {
				if claims.Allows(permission) {
					allowed = true
					break
				}
			}
			if !allowed {
				writeError(response, http.StatusForbidden, "insufficient_permission")
				return
			}
		}
		next.ServeHTTP(response, request.WithContext(identity.ContextWithClaims(request.Context(), claims)))
	})
}

func (api *server) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		bytes := make([]byte, 12)
		if _, err := rand.Read(bytes); err != nil {
			api.logger.Error("generate request ID", "error", err)
			http.Error(response, "internal error", http.StatusInternalServerError)
			return
		}
		requestID := hex.EncodeToString(bytes)
		response.Header().Set("X-Request-ID", requestID)
		request.Header.Set("X-Request-ID", requestID)
		next.ServeHTTP(response, request)
	})
}

func (api *server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(response, request)
	})
}

func (api *server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(response, request)
		api.logger.Info("request", "request_id", request.Header.Get("X-Request-ID"), "method", request.Method,
			"path", request.URL.Path, "duration_ms", time.Since(started).Milliseconds())
	})
}

func (api *server) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				api.logger.Error("request panic", "request_id", request.Header.Get("X-Request-ID"), "panic", recovered, "stack", string(debug.Stack()))
				writeError(response, http.StatusInternalServerError, "internal_error")
			}
		}()
		next.ServeHTTP(response, request)
	})
}

func writeError(response http.ResponseWriter, status int, code string) {
	writeJSON(response, status, map[string]string{"error": code})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
