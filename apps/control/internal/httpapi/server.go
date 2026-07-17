package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"homehub.local/control/internal/auth"
	"homehub.local/control/internal/catalog"
	"homehub.local/control/internal/health"
)

type StatusProvider interface {
	Snapshot() map[string]health.Result
}

type Options struct {
	Logger              *slog.Logger
	Services            []catalog.Service
	Statuses            StatusProvider
	Version             string
	Commit              string
	Environment         string
	Auth                *auth.Service
	AllowedOrigins      []string
	SecureCookies       bool
	DisableAuthForTests bool
}

type server struct {
	logger              *slog.Logger
	services            []catalog.Service
	statuses            StatusProvider
	version             string
	commit              string
	environment         string
	auth                *auth.Service
	allowedOrigins      map[string]struct{}
	secureCookies       bool
	disableAuthForTests bool
}

type principalContextKey struct{}

func New(options Options) http.Handler {
	api := &server{
		logger:              options.Logger,
		services:            append([]catalog.Service(nil), options.Services...),
		statuses:            options.Statuses,
		version:             options.Version,
		commit:              options.Commit,
		environment:         options.Environment,
		auth:                options.Auth,
		allowedOrigins:      make(map[string]struct{}, len(options.AllowedOrigins)),
		secureCookies:       options.SecureCookies,
		disableAuthForTests: options.DisableAuthForTests,
	}
	for _, origin := range options.AllowedOrigins {
		api.allowedOrigins[origin] = struct{}{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", api.live)
	mux.HandleFunc("GET /health/ready", api.ready)
	mux.HandleFunc("GET /api/v1/auth/session", api.authSession)
	mux.HandleFunc("GET /api/v1/auth/check", api.authCheck)
	mux.HandleFunc("POST /api/v1/auth/login", api.login)
	mux.HandleFunc("POST /api/v1/auth/logout", api.logout)
	mux.HandleFunc("POST /api/v1/setup/begin", api.beginSetup)
	mux.HandleFunc("POST /api/v1/setup/confirm", api.confirmSetup)
	mux.Handle("GET /api/v1/system", api.requireAuth(http.HandlerFunc(api.system)))
	mux.Handle("GET /api/v1/services", api.requireAuth(http.HandlerFunc(api.listServices)))
	mux.Handle("GET /api/v1/services/{id}", api.requireAuth(http.HandlerFunc(api.getService)))
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
		"auth_enabled": true,
		"time":         time.Now().UTC(),
	})
}

func (api *server) authSession(writer http.ResponseWriter, request *http.Request) {
	setupRequired, err := api.auth.SetupRequired(request.Context())
	if err != nil {
		api.logger.Error("check setup state", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	principal, err := api.authenticate(request)
	if err != nil {
		writeJSON(writer, http.StatusOK, map[string]any{"authenticated": false, "setup_required": setupRequired})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"authenticated": true, "setup_required": false, "principal": principal})
}

func (api *server) authCheck(writer http.ResponseWriter, request *http.Request) {
	principal, err := api.authenticate(request)
	if err != nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
		return
	}
	if !forwardAuthAllowed(principal, request.Header.Get("X-Forwarded-Uri")) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "insufficient_scope"})
		return
	}
	writer.Header().Set("X-HomeHub-Principal-ID", principal.ID)
	writer.Header().Set("X-HomeHub-Principal", principal.Username)
	writer.Header().Set("X-HomeHub-Email", principal.Username+"@homehub.local")
	if isServerPanelURI(request.Header.Get("X-Forwarded-Uri")) {
		writer.Header().Set("X-HomeHub-Beszel-Email", "owner@homehub.local")
	}
	writer.Header().Set("X-HomeHub-Scopes", strings.Join(principal.Scopes, " "))
	writer.WriteHeader(http.StatusNoContent)
}

func forwardAuthAllowed(principal auth.Principal, forwardedURI string) bool {
	if !isServerPanelURI(forwardedURI) {
		return true
	}
	for _, scope := range principal.Scopes {
		if scope == "admin" {
			return true
		}
	}
	return false
}

func isServerPanelURI(forwardedURI string) bool {
	path := strings.TrimSpace(strings.SplitN(forwardedURI, "?", 2)[0])
	return path == "/server" || strings.HasPrefix(path, "/server/")
}

func (api *server) beginSetup(writer http.ResponseWriter, request *http.Request) {
	if !api.validOrigin(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		BootstrapToken string `json:"bootstrap_token"`
		Username       string `json:"username"`
		Password       string `json:"password"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	setup, err := api.auth.BeginSetup(request.Context(), input.BootstrapToken, input.Username, input.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidBootstrap):
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid_bootstrap_token"})
		case errors.Is(err, auth.ErrSetupUnavailable):
			writeJSON(writer, http.StatusConflict, map[string]string{"error": "setup_unavailable"})
		default:
			if strings.Contains(err.Error(), "must") {
				writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_input", "message": err.Error()})
				return
			}
			api.logger.Error("begin owner setup", "error", err)
			writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		}
		return
	}
	writeJSON(writer, http.StatusCreated, setup)
}

func (api *server) confirmSetup(writer http.ResponseWriter, request *http.Request) {
	if !api.validOrigin(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		SetupID  string `json:"setup_id"`
		TOTPCode string `json:"totp_code"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	session, err := api.auth.ConfirmSetup(request.Context(), input.SetupID, input.TOTPCode, remoteIP(request), request.UserAgent())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidTOTP) {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid_totp"})
			return
		}
		if errors.Is(err, auth.ErrSetupUnavailable) {
			writeJSON(writer, http.StatusConflict, map[string]string{"error": "setup_unavailable"})
			return
		}
		api.logger.Error("confirm owner setup", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	api.setSessionCookies(writer, session)
	writeJSON(writer, http.StatusCreated, map[string]any{"authenticated": true, "principal": session.Principal})
}

func (api *server) login(writer http.ResponseWriter, request *http.Request) {
	if !api.validOrigin(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTPCode string `json:"totp_code"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	session, err := api.auth.Login(request.Context(), input.Username, input.Password, input.TOTPCode, remoteIP(request), request.UserAgent())
	if err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			writer.Header().Set("Retry-After", "900")
			writeJSON(writer, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
			return
		}
		api.logger.Error("owner login", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	api.setSessionCookies(writer, session)
	writeJSON(writer, http.StatusOK, map[string]any{"authenticated": true, "principal": session.Principal})
}

func (api *server) logout(writer http.ResponseWriter, request *http.Request) {
	if !api.validOrigin(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	sessionToken := api.sessionToken(request)
	csrfCookie, err := request.Cookie(api.csrfCookieName())
	if err != nil || !api.auth.ValidateCSRF(request.Context(), sessionToken, request.Header.Get("X-CSRF-Token")) || request.Header.Get("X-CSRF-Token") != csrfCookie.Value {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf"})
		return
	}
	if err := api.auth.Logout(request.Context(), sessionToken); err != nil {
		api.logger.Error("logout", "error", err)
	}
	api.clearSessionCookies(writer)
	writer.WriteHeader(http.StatusNoContent)
}

func (api *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if api.disableAuthForTests {
			next.ServeHTTP(writer, request)
			return
		}
		principal, err := api.authenticate(request)
		if err != nil {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
			return
		}
		next.ServeHTTP(writer, request.WithContext(context.WithValue(request.Context(), principalContextKey{}, principal)))
	})
}

func (api *server) authenticate(request *http.Request) (auth.Principal, error) {
	return api.auth.Authenticate(request.Context(), api.sessionToken(request))
}

func (api *server) sessionToken(request *http.Request) string {
	cookie, err := request.Cookie(api.sessionCookieName())
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (api *server) setSessionCookies(writer http.ResponseWriter, session auth.Session) {
	http.SetCookie(writer, &http.Cookie{Name: api.sessionCookieName(), Value: session.Token, Path: "/", MaxAge: 604800, HttpOnly: true, Secure: api.secureCookies, SameSite: http.SameSiteStrictMode})
	http.SetCookie(writer, &http.Cookie{Name: api.csrfCookieName(), Value: session.CSRF, Path: "/", MaxAge: 604800, HttpOnly: false, Secure: api.secureCookies, SameSite: http.SameSiteStrictMode})
}

func (api *server) clearSessionCookies(writer http.ResponseWriter) {
	for _, name := range []string{api.sessionCookieName(), api.csrfCookieName()} {
		http.SetCookie(writer, &http.Cookie{Name: name, Value: "", Path: "/", MaxAge: -1, HttpOnly: name == api.sessionCookieName(), Secure: api.secureCookies, SameSite: http.SameSiteStrictMode})
	}
}

func (api *server) sessionCookieName() string {
	if api.secureCookies {
		return "__Host-homehub_session"
	}
	return "homehub_session"
}
func (api *server) csrfCookieName() string {
	if api.secureCookies {
		return "__Host-homehub_csrf"
	}
	return "homehub_csrf"
}

func (api *server) validOrigin(request *http.Request) bool {
	_, ok := api.allowedOrigins[strings.TrimSpace(request.Header.Get("Origin"))]
	return ok
}

func remoteIP(request *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}
	host := request.RemoteAddr
	if colon := strings.LastIndex(host, ":"); colon >= 0 {
		host = strings.Trim(host[:colon], "[]")
	}
	return host
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 16*1024)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return false
	}
	return true
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
