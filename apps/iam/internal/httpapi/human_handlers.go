package httpapi

import (
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/apps/iam/internal/humanauth"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

// ── Session ──

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
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, map[string]any{
		"authenticated": true, "setup_required": false, "principal": principal, "administrator": true,
	})
}

// ── Sessions management ──

func (server *Server) listSessions(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	sessions, err := server.humans.ListSessions(request.Context(), principal)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"sessions": sessions, "current_session_id": principal.SessionID})
}

func (server *Server) revokeSession(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	sessionID := request.PathValue("id")
	revoked, err := server.humans.RevokeSessionByID(request.Context(), principal, sessionID)
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

func (server *Server) revokeOtherSessions(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	count, err := server.humans.RevokeOtherSessions(request.Context(), principal, principal.SessionID)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"revoked": count})
}

// ── Setup and Login ──

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
	_, token, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	_ = server.humans.Logout(request.Context(), token)
	server.clearSessionCookies(response)
	response.WriteHeader(http.StatusNoContent)
}

// ── API Keys ──

func (server *Server) listAPIKeys(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	keys, err := server.humans.ListAPIKeys(request.Context(), principal)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"api_keys": keys})
}

func (server *Server) createAPIKey(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	var input struct {
		Name    string   `json:"name"`
		Kind    string   `json:"kind"`
		Scopes  []string `json:"scopes"`
		ExpiresInDays *int `json:"expires_in_days,omitempty"`
	}
	if !decodeJSON(response, request, &input) {
		return
	}
	var expiresAt *time.Time
	if input.ExpiresInDays != nil && *input.ExpiresInDays > 0 {
		t := time.Now().UTC().Add(time.Duration(*input.ExpiresInDays) * 24 * time.Hour)
		expiresAt = &t
	}
	keyID, tokenValue, err := server.humans.CreateAPIKey(request.Context(), principal, input.Name, input.Kind, input.Scopes, expiresAt)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_api_key"})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, map[string]any{
		"id":    keyID,
		"token": tokenValue,
		"name":  input.Name,
		"kind":  input.Kind,
	})
}

func (server *Server) revokeAPIKey(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	revoked, err := server.humans.RevokeAPIKey(request.Context(), principal, request.PathValue("id"))
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

// ── Shares ──

func (server *Server) listShares(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	shares, err := server.humans.ListShares(request.Context(), principal)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"shares": shares})
}

func (server *Server) createShare(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	var input humanauth.ShareInput
	if !decodeJSON(response, request, &input) {
		return
	}
	shareID, token, err := server.humans.CreateShare(request.Context(), principal, input)
	if err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_share"})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, map[string]any{
		"id":    shareID,
		"token": token,
		"share_type": input.ShareType,
	})
}

func (server *Server) revokeShare(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	revoked, err := server.humans.RevokeShare(request.Context(), principal, request.PathValue("id"))
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

// ── Passkey handlers ──

func (server *Server) beginPasskeyLogin(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	options, token, err := server.humans.BeginPasskeyLogin(request.Context())
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"publicKey": options, "ceremony_token": token})
}

func (server *Server) finishPasskeyLogin(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	ceremonyToken := request.Header.Get("X-HomeHub-Ceremony-Token")
	if ceremonyToken == "" {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "missing_ceremony_token"})
		return
	}
	session, err := server.humans.FinishPasskeyLogin(request.Context(), ceremonyToken, remoteIP(request), request.UserAgent(), request)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_passkey"})
		return
	}
	server.setSessionCookies(response, session)
	writeJSON(response, http.StatusOK, map[string]any{"authenticated": true, "principal": session.Principal, "administrator": true})
}

func (server *Server) beginPasskeyRegistration(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	options, token, err := server.humans.BeginPasskeyRegistration(request.Context(), principal)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"publicKey": options, "ceremony_token": token})
}

func (server *Server) finishPasskeyRegistration(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	name := request.Header.Get("X-HomeHub-Passkey-Name")
	if name == "" {
		name = "Bitwarden Passkey"
	}
	ceremonyToken := request.Header.Get("X-HomeHub-Ceremony-Token")
	if err := server.humans.FinishPasskeyRegistration(request.Context(), principal, ceremonyToken, name, request); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_passkey"})
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"registered": true})
}

func (server *Server) listPasskeys(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	passkeys, err := server.humans.ListPasskeys(request.Context(), principal)
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"passkeys": passkeys})
}

func (server *Server) deletePasskey(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	deleted, err := server.humans.DeletePasskey(request.Context(), principal, request.PathValue("id"))
	if err != nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	if !deleted {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

// ── Edge Authorize (Traefik ForwardAuth) ──

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

	audience := strings.TrimSpace(request.Header.Get("X-HomeHub-Audience"))
	if audience == "" {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "missing_audience"})
		return
	}

	// Try API key first (Bearer token)
	bearerToken, _ := identity.BearerToken(request)
	if bearerToken != "" && strings.HasPrefix(bearerToken, "hh_") {
		tokenHash := storepostgres.HashCredential(bearerToken)
		key, err := server.humans.AuthenticateAPIKey(request.Context(), tokenHash)
		if err == nil {
			issued, err := server.humans.IssueAPIKeyJWT(request.Context(), key, audience)
			if err != nil {
				writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
				return
			}
			response.Header().Set("Authorization", "Bearer "+issued.AccessToken)
			response.Header().Set("X-HomeHub-Subject", "human:"+key.OwnerID)
			response.Header().Set("Cache-Control", "no-store")
			response.WriteHeader(http.StatusNoContent)
			return
		}
	}

	// Try session cookie
	principal, err := server.humanPrincipal(request)
	if err != nil {
		// Try share session
		cookie, cookieErr := request.Cookie(sessionCookieName)
		if cookieErr == nil && cookie.Value != "" {
			// Check if it's a share session (guest principal)
			principal, err = server.humans.Authenticate(request.Context(), cookie.Value)
			if err != nil {
				writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
				return
			}
		} else {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
			return
		}
	}

	issued, err := server.humans.IssueJWT(request.Context(), principal, audience, nil)
	if err != nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
		return
	}
	response.Header().Set("Authorization", "Bearer "+issued.AccessToken)
	response.Header().Set("X-HomeHub-Subject", "human:"+principal.ID)
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(http.StatusNoContent)
}

// ── Token exchange compat (legacy) ──

func (server *Server) tokenExchangeCompat(response http.ResponseWriter, request *http.Request) {
	credential, err := identity.BearerToken(request)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	// Only support API key exchange now
	if !strings.HasPrefix(credential, "hh_") {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	tokenHash := storepostgres.HashCredential(credential)
	key, err := server.humans.AuthenticateAPIKey(request.Context(), tokenHash)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	var input struct {
		Audience    string   `json:"audience"`
		Permissions []string `json:"permissions"`
	}
	decodeJSON(response, request, &input)
	issued, err := server.humans.IssueAPIKeyJWT(request.Context(), key, input.Audience)
	if err != nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, issued)
}

// ── Session token (for frontend) ──

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
	issued, err := server.humans.IssueJWT(request.Context(), principal, input.Audience, input.Permissions)
	if err != nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, issued)
}

// ── Helpers ──

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

func (server *Server) validOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		// Allow same-origin requests (no Origin header)
		return true
	}
	_, ok := server.origins[origin]
	return ok
}

func (server *Server) setSessionCookies(response http.ResponseWriter, session humanauth.Session) {
	maxAge := int((180 * 24 * time.Hour).Seconds())
	common := http.Cookie{Path: "/", Secure: server.secureCookies, SameSite: http.SameSiteStrictMode, MaxAge: maxAge}
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
