package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"gitee.com/zlx23/homehub/apps/iam/internal/humanauth"
)

const (
	sessionCookieName = "hh_session"
	csrfCookieName    = "hh_csrf"
)

func (server *Server) session(response http.ResponseWriter, request *http.Request) {
	if server.humans == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	setupRequired, err := server.humans.SetupRequired(request.Context())
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	principal, err := server.humanPrincipal(request)
	if err != nil {
		writeJSON(response, http.StatusOK, map[string]any{"authenticated": false, "setup_required": setupRequired})
		return
	}
	administrator, _ := server.humans.IsAdministrator(request.Context(), principal)
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, map[string]any{
		"authenticated": true, "setup_required": false, "principal": principal, "administrator": administrator,
	})
}

func (server *Server) beginSetup(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		BootstrapToken string `json:"bootstrap_token"`
		Username       string `json:"username"`
		DisplayName    string `json:"display_name"`
		Password       string `json:"password"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	setup, err := server.humans.BeginSetup(request.Context(), input.BootstrapToken, input.Username, input.DisplayName, input.Password)
	if err != nil {
		status, code := http.StatusServiceUnavailable, "temporarily_unavailable"
		if errors.Is(err, humanauth.ErrInvalidBootstrap) {
			status, code = http.StatusUnauthorized, "invalid_bootstrap"
		}
		writeJSON(response, status, map[string]string{"error": code})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, setup)
}

func (server *Server) confirmSetup(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		SetupID string `json:"setup_id"`
		TOTP    string `json:"totp_code"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	session, err := server.humans.ConfirmSetup(request.Context(), input.SetupID, input.TOTP, remoteIP(request), request.UserAgent())
	if err != nil {
		status, code := http.StatusServiceUnavailable, "temporarily_unavailable"
		switch {
		case errors.Is(err, humanauth.ErrInvalidTOTP):
			status, code = http.StatusUnauthorized, "invalid_totp"
		case errors.Is(err, humanauth.ErrSetupUnavailable):
			status, code = http.StatusConflict, "setup_unavailable"
		}
		writeJSON(response, status, map[string]string{"error": code})
		return
	}
	server.setSessionCookies(response, session)
	writeJSON(response, http.StatusCreated, map[string]any{"authenticated": true, "principal": session.Principal, "administrator": true})
}

func (server *Server) login(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		Username string `json:"username"`
		Password string `json:"password"`
		TOTP     string `json:"totp_code"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	session, err := server.humans.Login(request.Context(), input.Username, input.Password, input.TOTP, remoteIP(request), request.UserAgent())
	if err != nil {
		if errors.Is(err, humanauth.ErrRateLimited) {
			response.Header().Set("Retry-After", "900")
			writeJSON(response, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
			return
		}
		if errors.Is(err, humanauth.ErrInvalidCredentials) {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
			return
		}
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	server.setSessionCookies(response, session)
	writeJSON(response, http.StatusOK, map[string]any{"authenticated": true, "principal": session.Principal, "administrator": true})
}

func (server *Server) logout(response http.ResponseWriter, request *http.Request) {
	principal, token, ok := server.requireSessionAndCSRF(response, request)
	_ = principal
	if !ok {
		return
	}
	_ = server.humans.Logout(request.Context(), token)
	server.clearSessionCookies(response)
	response.WriteHeader(http.StatusNoContent)
}

func (server *Server) sessionToken(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	var input struct {
		Audience    string   `json:"audience"`
		Permissions []string `json:"permissions"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	issued, err := server.humans.Issue(request.Context(), principal, input.Audience, input.Permissions, false)
	if err != nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, issued)
}

func (server *Server) edgeAuthorize(response http.ResponseWriter, request *http.Request) {
	if server.humans == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	method := strings.ToUpper(strings.TrimSpace(request.Header.Get("X-Forwarded-Method")))
	if method != "" && method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions && !server.validOrigin(request) {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	principal, err := server.humanPrincipal(request)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
		return
	}
	audience := strings.TrimSpace(request.Header.Get("X-HomeHub-Audience"))
	issued, err := server.humans.Issue(request.Context(), principal, audience, nil, true)
	if err != nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
		return
	}
	response.Header().Set("Authorization", "Bearer "+issued.AccessToken)
	response.Header().Set("X-HomeHub-Subject", principal.Subject)
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusNoContent)
}

func (server *Server) createShare(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok || !server.requireAdministrator(response, request, principal) {
		return
	}
	var input struct {
		Grants    []humanauth.Grant `json:"grants"`
		ExpiresAt time.Time         `json:"expires_at"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	share, err := server.humans.CreateShare(request.Context(), principal, input.Grants, input.ExpiresAt, remoteIP(request))
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_share"})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, share)
}

func (server *Server) listShares(response http.ResponseWriter, request *http.Request) {
	principal, err := server.humanPrincipal(request)
	if err != nil || !server.requireAdministrator(response, request, principal) {
		return
	}
	shares, err := server.humans.ListShares(request.Context())
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"shares": shares})
}

func (server *Server) revokeShare(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok || !server.requireAdministrator(response, request, principal) {
		return
	}
	revoked, err := server.humans.RevokeShare(request.Context(), principal, request.PathValue("id"), remoteIP(request))
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	if !revoked {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (server *Server) redeemShare(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	session, err := server.humans.RedeemShare(request.Context(), input.Token, remoteIP(request), request.UserAgent())
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_share"})
		return
	}
	server.setSessionCookies(response, session)
	writeJSON(response, http.StatusCreated, map[string]any{"authenticated": true, "principal": session.Principal, "administrator": false})
}

func (server *Server) humanPrincipal(request *http.Request) (humanauth.Principal, error) {
	cookie, err := request.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" || server.humans == nil {
		return humanauth.Principal{}, humanauth.ErrInvalidSession
	}
	return server.humans.Authenticate(request.Context(), cookie.Value)
}

func (server *Server) requireSessionAndCSRF(response http.ResponseWriter, request *http.Request) (humanauth.Principal, string, bool) {
	if !server.validOrigin(request) {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return humanauth.Principal{}, "", false
	}
	sessionCookie, err1 := request.Cookie(sessionCookieName)
	csrfCookie, err2 := request.Cookie(csrfCookieName)
	header := request.Header.Get("X-CSRF-Token")
	if err1 != nil || err2 != nil || subtle.ConstantTimeCompare([]byte(csrfCookie.Value), []byte(header)) != 1 || !server.humans.ValidateCSRF(request.Context(), sessionCookie.Value, header) {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_csrf"})
		return humanauth.Principal{}, "", false
	}
	principal, err := server.humans.Authenticate(request.Context(), sessionCookie.Value)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
		return humanauth.Principal{}, "", false
	}
	return principal, sessionCookie.Value, true
}

func (server *Server) requireAdministrator(response http.ResponseWriter, request *http.Request, principal humanauth.Principal) bool {
	allowed, err := server.humans.IsAdministrator(request.Context(), principal)
	if err != nil || !allowed {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "administrator_required"})
		return false
	}
	return true
}

func (server *Server) validOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	_, ok := server.origins[origin]
	return origin != "" && ok
}

func (server *Server) setSessionCookies(response http.ResponseWriter, session humanauth.Session) {
	common := http.Cookie{Path: "/", Secure: server.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: int((7 * 24 * time.Hour).Seconds())}
	sessionCookie := common
	sessionCookie.Name, sessionCookie.Value, sessionCookie.HttpOnly = sessionCookieName, session.Token, true
	csrfCookie := common
	csrfCookie.Name, csrfCookie.Value, csrfCookie.HttpOnly = csrfCookieName, session.CSRF, false
	http.SetCookie(response, &sessionCookie)
	http.SetCookie(response, &csrfCookie)
}

func (server *Server) clearSessionCookies(response http.ResponseWriter) {
	for _, name := range []string{sessionCookieName, csrfCookieName} {
		http.SetCookie(response, &http.Cookie{Name: name, Value: "", Path: "/", Secure: server.secureCookies, HttpOnly: name == sessionCookieName, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	}
}

func decodeJSON(response http.ResponseWriter, request *http.Request, target any) bool {
	request.Body = http.MaxBytesReader(response, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return false
	}
	return true
}

func remoteIP(request *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0]); net.ParseIP(forwarded) != nil {
		return forwarded
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && net.ParseIP(host) != nil {
		return host
	}
	return ""
}
