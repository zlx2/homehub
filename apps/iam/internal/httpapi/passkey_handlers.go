package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"gitee.com/zlx23/homehub/apps/iam/internal/humanauth"
)

const passkeyCeremonyCookieName = "hh_passkey"

func (server *Server) beginPasskeyRegistration(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	options, ceremony, err := server.humans.BeginPasskeyRegistration(request.Context(), principal)
	if err != nil {
		server.writePasskeyError(response, err)
		return
	}
	server.setPasskeyCeremonyCookie(response, ceremony)
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, options)
}

func (server *Server) finishPasskeyRegistration(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	ceremony, err := request.Cookie(passkeyCeremonyCookieName)
	if err != nil || ceremony.Value == "" {
		server.writePasskeyError(response, humanauth.ErrPasskey)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 128<<10)
	name := strings.TrimSpace(request.Header.Get("X-HomeHub-Passkey-Name"))
	err = server.humans.FinishPasskeyRegistration(request.Context(), principal, ceremony.Value, name, request)
	server.clearPasskeyCeremonyCookie(response)
	if err != nil {
		server.writePasskeyError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]bool{"registered": true})
}

func (server *Server) beginPasskeyLogin(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	options, ceremony, err := server.humans.BeginPasskeyLogin(request.Context())
	if err != nil {
		server.writePasskeyError(response, err)
		return
	}
	server.setPasskeyCeremonyCookie(response, ceremony)
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, options)
}

func (server *Server) finishPasskeyLogin(response http.ResponseWriter, request *http.Request) {
	if !server.validOrigin(request) || server.humans == nil {
		writeJSON(response, http.StatusForbidden, map[string]string{"error": "invalid_origin"})
		return
	}
	ceremony, err := request.Cookie(passkeyCeremonyCookieName)
	if err != nil || ceremony.Value == "" {
		server.writePasskeyError(response, humanauth.ErrPasskey)
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 128<<10)
	session, err := server.humans.FinishPasskeyLogin(request.Context(), ceremony.Value, remoteIP(request), request.UserAgent(), request)
	server.clearPasskeyCeremonyCookie(response)
	if err != nil {
		server.writePasskeyError(response, err)
		return
	}
	administrator, _ := server.humans.IsAdministrator(request.Context(), session.Principal)
	server.setSessionCookies(response, session)
	writeJSON(response, http.StatusOK, map[string]any{"authenticated": true, "principal": session.Principal, "administrator": administrator})
}

func (server *Server) listPasskeys(response http.ResponseWriter, request *http.Request) {
	principal, err := server.humanPrincipal(request)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "authentication_required"})
		return
	}
	passkeys, err := server.humans.ListPasskeys(request.Context(), principal)
	if err != nil {
		server.writePasskeyError(response, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, map[string]any{"passkeys": passkeys})
}

func (server *Server) deletePasskey(response http.ResponseWriter, request *http.Request) {
	principal, _, ok := server.requireSessionAndCSRF(response, request)
	if !ok {
		return
	}
	deleted, err := server.humans.DeletePasskey(request.Context(), principal, request.PathValue("id"))
	if err != nil {
		server.writePasskeyError(response, err)
		return
	}
	if !deleted {
		writeJSON(response, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	response.WriteHeader(http.StatusNoContent)
}

func (server *Server) setPasskeyCeremonyCookie(response http.ResponseWriter, value string) {
	http.SetCookie(response, &http.Cookie{
		Name: passkeyCeremonyCookieName, Value: value, Path: "/api/iam/v1/passkeys/",
		Secure: server.secureCookies, HttpOnly: true, SameSite: http.SameSiteStrictMode,
		MaxAge: int((5 * time.Minute).Seconds()),
	})
}

func (server *Server) clearPasskeyCeremonyCookie(response http.ResponseWriter) {
	http.SetCookie(response, &http.Cookie{
		Name: passkeyCeremonyCookieName, Value: "", Path: "/api/iam/v1/passkeys/",
		Secure: server.secureCookies, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

func (server *Server) writePasskeyError(response http.ResponseWriter, err error) {
	status, code := http.StatusBadRequest, "passkey_failed"
	switch {
	case errors.Is(err, humanauth.ErrForbidden):
		status, code = http.StatusForbidden, "forbidden"
	case errors.Is(err, humanauth.ErrInvalidCredentials):
		status, code = http.StatusUnauthorized, "invalid_credentials"
	case !errors.Is(err, humanauth.ErrPasskey):
		status, code = http.StatusServiceUnavailable, "temporarily_unavailable"
	}
	writeJSON(response, status, map[string]string{"error": code})
}
