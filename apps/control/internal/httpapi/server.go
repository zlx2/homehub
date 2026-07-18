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
	"net/url"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"homehub.local/control/internal/auth"
	"homehub.local/control/internal/catalog"
	"homehub.local/control/internal/health"
)

type StatusProvider interface {
	Snapshot() map[string]health.Result
}

type IdentityIssuer interface {
	Issue(subject, name string, scopes []string, audience string) (string, error)
	IssueAI(subject, name, sourceService string, scopes, models []string) (string, error)
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
	IdentityIssuer      IdentityIssuer
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
	identityIssuer      IdentityIssuer
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
		identityIssuer:      options.IdentityIssuer,
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
	mux.HandleFunc("POST /api/v1/invitations/redeem", api.redeemInvitation)
	mux.Handle("GET /api/v1/system", api.requireAuth(http.HandlerFunc(api.system)))
	mux.Handle("GET /api/v1/services", api.requireAuth(http.HandlerFunc(api.listServices)))
	mux.Handle("GET /api/v1/services/{id}", api.requireAuth(http.HandlerFunc(api.getService)))
	mux.Handle("GET /api/v1/admin/principals", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.listPrincipals))))
	mux.Handle("GET /api/v1/admin/service-grants", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.listServiceGrants))))
	mux.Handle("POST /api/v1/admin/service-grants", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.createServiceGrant))))
	mux.Handle("DELETE /api/v1/admin/service-grants/{id}", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.deleteServiceGrant))))
	mux.Handle("GET /api/v1/admin/invitations", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.listInvitations))))
	mux.Handle("POST /api/v1/admin/invitations", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.createInvitation))))
	mux.Handle("DELETE /api/v1/admin/invitations/{id}", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.deleteInvitation))))
	mux.Handle("GET /api/v1/admin/api-tokens", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.listAPITokens))))
	mux.Handle("POST /api/v1/admin/api-tokens", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.createAPIToken))))
	mux.Handle("DELETE /api/v1/admin/api-tokens/{id}", api.requireAuth(api.requireAdmin(http.HandlerFunc(api.deleteAPIToken))))
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
	principal, tokenIdentity, err := api.authenticateForwardRequest(request)
	if err != nil {
		writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
		return
	}
	service, matched := catalog.MatchRoute(api.services, request.Header.Get("X-Forwarded-Uri"))
	if !matched {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "service_route_unregistered"})
		return
	}
	allowed := false
	if tokenIdentity != nil {
		allowed = apiTokenRequestAllowed(*tokenIdentity, service, request.Header.Get("X-Forwarded-Method"), request.Header.Get("X-Forwarded-Uri"))
	} else {
		allowed, err = api.serviceAllowed(request.Context(), principal, service)
		if err != nil {
			api.logger.Error("authorize service", "service_id", service.ID, "error", err)
			writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
			return
		}
	}
	if !allowed {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "insufficient_scope"})
		return
	}
	writer.Header().Set("X-HomeHub-Principal-ID", principal.ID)
	writer.Header().Set("X-HomeHub-Principal", principal.Username)
	writer.Header().Set("X-HomeHub-Email", principal.Username+"@homehub.local")
	if service.ID == "server-monitor" {
		writer.Header().Set("X-HomeHub-Beszel-Email", "owner@homehub.local")
	}
	writer.Header().Set("X-HomeHub-Scopes", strings.Join(principal.Scopes, " "))
	if err := api.setServiceIdentity(writer, principal, service); err != nil {
		api.logger.Error("issue service identity", "service_id", service.ID, "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	if err := api.setAIIdentity(writer, principal, service); err != nil {
		api.logger.Error("issue AI delegation", "service_id", service.ID, "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (api *server) setAIIdentity(writer http.ResponseWriter, principal auth.Principal, service catalog.Service) error {
	if !service.AIEnabled {
		return nil
	}
	if api.identityIssuer == nil {
		return errors.New("AI identity issuer is not configured")
	}
	token, err := api.identityIssuer.IssueAI(
		principal.ID, principal.DisplayName, service.ID, principal.Scopes, service.AIModels,
	)
	if err != nil {
		return err
	}
	writer.Header().Set("X-HomeHub-AI-Identity", token)
	return nil
}

func (api *server) setServiceIdentity(writer http.ResponseWriter, principal auth.Principal, service catalog.Service) error {
	if !service.IdentityEnabled {
		return nil
	}
	if api.identityIssuer == nil {
		return errors.New("service identity issuer is not configured")
	}
	token, err := api.identityIssuer.Issue(principal.ID, principal.DisplayName, principal.Scopes, service.ID)
	if err != nil {
		return err
	}
	writer.Header().Set("X-HomeHub-Identity", token)
	return nil
}

func (api *server) serviceAllowed(ctx context.Context, principal auth.Principal, service catalog.Service) (bool, error) {
	if serviceAccessAllowed(principal, service, false) {
		return true, nil
	}
	if service.Visibility != "shared" || !service.ShareEnabled {
		return false, nil
	}
	serviceIDs, err := api.auth.ActiveServiceIDs(ctx, principal.ID)
	if err != nil {
		return false, err
	}
	_, allowed := serviceIDs[service.ID]
	return serviceAccessAllowed(principal, service, allowed), nil
}

func serviceAccessAllowed(principal auth.Principal, service catalog.Service, hasGrant bool) bool {
	if hasAdminAccess(principal) {
		return true
	}
	return service.Visibility == "shared" && service.ShareEnabled && hasGrant
}

func hasAdminAccess(principal auth.Principal) bool {
	return auth.HasScope(principal, "admin") || auth.HasScope(principal, auth.ScopeAgentRoot)
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

func (api *server) redeemInvitation(writer http.ResponseWriter, request *http.Request) {
	if !api.validOrigin(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	session, err := api.auth.RedeemInvitation(request.Context(), input.Token, remoteIP(request), request.UserAgent())
	if err != nil {
		if errors.Is(err, auth.ErrInvalidInvitation) {
			writeJSON(writer, http.StatusUnauthorized, map[string]string{"error": "invalid_invitation"})
			return
		}
		api.logger.Error("redeem share link", "error", err)
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

func (api *server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, ok := request.Context().Value(principalContextKey{}).(auth.Principal)
		if !ok || !hasAdminAccess(principal) {
			writeJSON(writer, http.StatusForbidden, map[string]string{"error": "admin_required"})
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (api *server) authenticate(request *http.Request) (auth.Principal, error) {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if header != "" {
		const prefix = "Bearer "
		if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
			return auth.Principal{}, auth.ErrInvalidCredentials
		}
		identity, err := api.auth.AuthenticateAPIToken(request.Context(), strings.TrimSpace(header[len(prefix):]))
		if err != nil || identity.ServiceID != auth.APITokenServiceAll || !auth.HasScope(identity.Principal, auth.ScopeAgentRoot) {
			return auth.Principal{}, auth.ErrInvalidCredentials
		}
		return identity.Principal, nil
	}
	return api.auth.Authenticate(request.Context(), api.sessionToken(request))
}

func (api *server) authenticateForwardRequest(request *http.Request) (auth.Principal, *auth.APITokenIdentity, error) {
	header := strings.TrimSpace(request.Header.Get("Authorization"))
	if header != "" {
		const prefix = "Bearer "
		if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
			return auth.Principal{}, nil, auth.ErrInvalidCredentials
		}
		identity, err := api.auth.AuthenticateAPIToken(request.Context(), strings.TrimSpace(header[len(prefix):]))
		if err != nil {
			return auth.Principal{}, nil, err
		}
		return identity.Principal, &identity, nil
	}
	principal, err := api.authenticate(request)
	return principal, nil, err
}

func apiTokenRequestAllowed(identity auth.APITokenIdentity, service catalog.Service, method, rawURI string) bool {
	if identity.ServiceID == auth.APITokenServiceAll && auth.HasScope(identity.Principal, auth.ScopeAgentRoot) {
		return true
	}
	if identity.ServiceID != service.ID || !auth.HasScope(identity.Principal, auth.ScopeDropUpload) {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(method)) != http.MethodPost {
		return false
	}
	parsed, err := url.ParseRequestURI(rawURI)
	return err == nil && parsed.Path == "/drop/api/v1/items"
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

func (api *server) listServices(writer http.ResponseWriter, request *http.Request) {
	statuses := api.statuses.Snapshot()
	services := make([]serviceResponse, 0, len(api.services))
	principal, hasPrincipal := request.Context().Value(principalContextKey{}).(auth.Principal)
	for _, service := range api.services {
		if hasPrincipal {
			allowed, err := api.serviceAllowed(request.Context(), principal, service)
			if err != nil {
				api.logger.Error("filter service catalog", "service_id", service.ID, "error", err)
				writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
				return
			}
			if !allowed {
				continue
			}
		}
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
			if principal, ok := request.Context().Value(principalContextKey{}).(auth.Principal); ok {
				allowed, err := api.serviceAllowed(request.Context(), principal, service)
				if err != nil {
					api.logger.Error("authorize service detail", "service_id", service.ID, "error", err)
					writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
					return
				}
				if !allowed {
					writeJSON(writer, http.StatusNotFound, map[string]string{"error": "service_not_found"})
					return
				}
			}
			writeJSON(writer, http.StatusOK, publicService(service, statuses[service.ID]))
			return
		}
	}
	writeJSON(writer, http.StatusNotFound, map[string]string{"error": "service_not_found"})
}

func (api *server) listPrincipals(writer http.ResponseWriter, request *http.Request) {
	principals, err := api.auth.ListPrincipals(request.Context())
	if err != nil {
		api.logger.Error("list principals", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"principals": principals})
}

func (api *server) listServiceGrants(writer http.ResponseWriter, request *http.Request) {
	grants, err := api.auth.ListServiceGrants(request.Context())
	if err != nil {
		api.logger.Error("list service grants", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"grants": grants})
}

func (api *server) createServiceGrant(writer http.ResponseWriter, request *http.Request) {
	if !api.validMutation(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf_or_origin"})
		return
	}
	var input struct {
		PrincipalID string     `json:"principal_id"`
		ServiceID   string     `json:"service_id"`
		ExpiresAt   *time.Time `json:"expires_at"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC()) {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "expiry_must_be_future"})
		return
	}
	service, exists := api.findService(input.ServiceID)
	if !exists || service.Visibility != "shared" || !service.ShareEnabled {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "service_not_shareable"})
		return
	}
	principalExists, err := api.auth.PrincipalExists(request.Context(), input.PrincipalID)
	if err != nil {
		api.logger.Error("validate grant principal", "error", err)
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_principal_id"})
		return
	}
	if !principalExists {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "principal_not_found"})
		return
	}
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	grant, err := api.auth.GrantService(request.Context(), actor.ID, input.PrincipalID, input.ServiceID, input.ExpiresAt, remoteIP(request))
	if err != nil {
		api.logger.Error("grant service", "service_id", input.ServiceID, "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusCreated, grant)
}

func (api *server) deleteServiceGrant(writer http.ResponseWriter, request *http.Request) {
	if !api.validMutation(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf_or_origin"})
		return
	}
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	revoked, err := api.auth.RevokeServiceGrant(request.Context(), actor.ID, request.PathValue("id"), remoteIP(request))
	if err != nil {
		api.logger.Error("revoke service grant", "error", err)
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_grant_id"})
		return
	}
	if !revoked {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "grant_not_found"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (api *server) listInvitations(writer http.ResponseWriter, request *http.Request) {
	invitations, err := api.auth.ListInvitations(request.Context())
	if err != nil {
		api.logger.Error("list invitations", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"invitations": invitations})
}

func (api *server) createInvitation(writer http.ResponseWriter, request *http.Request) {
	if !api.validMutation(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf_or_origin"})
		return
	}
	var input struct {
		ServiceIDs []string   `json:"service_ids"`
		ExpiresAt  *time.Time `json:"expires_at"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	serviceIDs, err := api.validateInvitationServices(input.ServiceIDs)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_service_selection", "message": err.Error()})
		return
	}
	expiresAt, err := normalizeInvitationExpiry(time.Now().UTC(), input.ExpiresAt)
	if err != nil {
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_expiry", "message": err.Error()})
		return
	}
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	invitation, err := api.auth.CreateInvitation(request.Context(), actor.ID, serviceIDs, expiresAt, remoteIP(request))
	if err != nil {
		api.logger.Error("create invitation", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusCreated, invitation)
}

func (api *server) deleteInvitation(writer http.ResponseWriter, request *http.Request) {
	if !api.validMutation(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf_or_origin"})
		return
	}
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	revoked, err := api.auth.RevokeInvitation(request.Context(), actor.ID, request.PathValue("id"), remoteIP(request))
	if err != nil {
		api.logger.Error("revoke invitation", "error", err)
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_invitation_id"})
		return
	}
	if !revoked {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "invitation_not_found"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (api *server) listAPITokens(writer http.ResponseWriter, request *http.Request) {
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	tokens, err := api.auth.ListAPITokens(request.Context(), actor.ID)
	if err != nil {
		api.logger.Error("list API tokens", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{"tokens": tokens})
}

func (api *server) createAPIToken(writer http.ResponseWriter, request *http.Request) {
	if !api.validMutation(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf_or_origin"})
		return
	}
	var input struct {
		Name      string    `json:"name"`
		ServiceID string    `json:"service_id"`
		Scopes    []string  `json:"scopes"`
		ExpiresAt time.Time `json:"expires_at"`
	}
	if !decodeJSON(writer, request, &input) {
		return
	}
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	created, err := api.auth.CreateAPIToken(request.Context(), actor.ID, input.Name, input.ServiceID, input.Scopes, input.ExpiresAt, remoteIP(request))
	if err != nil {
		if errors.Is(err, auth.ErrInvalidAPIToken) || errors.Is(err, auth.ErrTooManyAPITokens) {
			writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_api_token", "message": err.Error()})
			return
		}
		api.logger.Error("create API token", "error", err)
		writeJSON(writer, http.StatusInternalServerError, map[string]string{"error": "internal_error"})
		return
	}
	writeJSON(writer, http.StatusCreated, created)
}

func (api *server) deleteAPIToken(writer http.ResponseWriter, request *http.Request) {
	if !api.validMutation(request) {
		writeJSON(writer, http.StatusForbidden, map[string]string{"error": "invalid_csrf_or_origin"})
		return
	}
	actor := request.Context().Value(principalContextKey{}).(auth.Principal)
	revoked, err := api.auth.RevokeAPIToken(request.Context(), actor.ID, request.PathValue("id"), remoteIP(request))
	if err != nil {
		api.logger.Error("revoke API token", "error", err)
		writeJSON(writer, http.StatusBadRequest, map[string]string{"error": "invalid_api_token_id"})
		return
	}
	if !revoked {
		writeJSON(writer, http.StatusNotFound, map[string]string{"error": "api_token_not_found"})
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (api *server) validateInvitationServices(input []string) ([]string, error) {
	if len(input) == 0 {
		return nil, fmt.Errorf("select at least one shareable service")
	}
	seen := make(map[string]struct{}, len(input))
	serviceIDs := make([]string, 0, len(input))
	for _, serviceID := range input {
		if _, duplicate := seen[serviceID]; duplicate {
			return nil, fmt.Errorf("service %q was selected more than once", serviceID)
		}
		service, exists := api.findService(serviceID)
		if !exists || service.Visibility != "shared" || !service.ShareEnabled {
			return nil, fmt.Errorf("service %q is not shareable", serviceID)
		}
		seen[serviceID] = struct{}{}
		serviceIDs = append(serviceIDs, serviceID)
	}
	sort.Strings(serviceIDs)
	return serviceIDs, nil
}

func normalizeInvitationExpiry(now time.Time, requested *time.Time) (time.Time, error) {
	expiresAt := now.Add(24 * time.Hour)
	if requested != nil {
		expiresAt = requested.UTC()
	}
	if expiresAt.Before(now.Add(5 * time.Minute)) {
		return time.Time{}, fmt.Errorf("invitation must remain valid for at least 5 minutes")
	}
	if expiresAt.After(now.Add(7 * 24 * time.Hour)) {
		return time.Time{}, fmt.Errorf("invitation cannot remain valid for more than 7 days")
	}
	return expiresAt, nil
}

func (api *server) findService(id string) (catalog.Service, bool) {
	for _, service := range api.services {
		if service.ID == id {
			return service, true
		}
	}
	return catalog.Service{}, false
}

func (api *server) validMutation(request *http.Request) bool {
	if principal, ok := request.Context().Value(principalContextKey{}).(auth.Principal); ok && auth.HasScope(principal, auth.ScopeAgentRoot) {
		return true
	}
	if !api.validOrigin(request) {
		return false
	}
	csrfCookie, err := request.Cookie(api.csrfCookieName())
	if err != nil {
		return false
	}
	csrf := request.Header.Get("X-CSRF-Token")
	return csrf != "" && csrf == csrfCookie.Value && api.auth.ValidateCSRF(request.Context(), api.sessionToken(request), csrf)
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
