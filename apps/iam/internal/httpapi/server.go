package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"gitee.com/zlx23/homehub/apps/iam/internal/humanauth"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

const (
	sessionCookieName = "hh_session"
	csrfCookieName    = "hh_csrf"
)

type TokenVerifier interface {
	Verify(string) (identity.Claims, error)
}

type Server struct {
	version       string
	readiness     func(context.Context) error
	jwkSet        any
	verifier      TokenVerifier
	humans        *humanauth.Service
	origins       map[string]struct{}
	secureCookies bool
}

type Options struct {
	Version        string
	Readiness      func(context.Context) error
	JWKSet         any
	Verifier       TokenVerifier
	Humans         *humanauth.Service
	AllowedOrigins []string
	SecureCookies  bool
}

func New(options Options) http.Handler {
	server := &Server{
		version: options.Version, readiness: options.Readiness, jwkSet: options.JWKSet,
		verifier: options.Verifier, humans: options.Humans,
		origins: make(map[string]struct{}, len(options.AllowedOrigins)), secureCookies: options.SecureCookies,
	}
	for _, origin := range options.AllowedOrigins {
		server.origins[origin] = struct{}{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.health)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /v1/metadata", server.metadata)
	mux.HandleFunc("GET /.well-known/jwks.json", server.jwks)

	// Setup and login
	mux.HandleFunc("POST /v1/setup/begin", server.beginSetup)
	mux.HandleFunc("POST /v1/setup/confirm", server.confirmSetup)
	mux.HandleFunc("POST /v1/login", server.login)
	mux.HandleFunc("POST /v1/logout", server.logout)

	// Session
	mux.HandleFunc("GET /v1/session", server.session)
	mux.HandleFunc("GET /v1/sessions", server.listSessions)
	mux.HandleFunc("DELETE /v1/sessions/{id}", server.revokeSession)
	mux.HandleFunc("DELETE /v1/sessions", server.revokeOtherSessions)

	// Passkeys
	mux.HandleFunc("POST /v1/passkeys/login/begin", server.beginPasskeyLogin)
	mux.HandleFunc("POST /v1/passkeys/login/finish", server.finishPasskeyLogin)
	mux.HandleFunc("POST /v1/passkeys/registration/begin", server.beginPasskeyRegistration)
	mux.HandleFunc("POST /v1/passkeys/registration/finish", server.finishPasskeyRegistration)
	mux.HandleFunc("GET /v1/passkeys", server.listPasskeys)
	mux.HandleFunc("DELETE /v1/passkeys/{id}", server.deletePasskey)

	// API Keys
	mux.HandleFunc("GET /v1/api-keys", server.listAPIKeys)
	mux.HandleFunc("POST /v1/api-keys", server.createAPIKey)
	mux.HandleFunc("DELETE /v1/api-keys/{id}", server.revokeAPIKey)

	// Shares
	mux.HandleFunc("GET /v1/shares", server.listShares)
	mux.HandleFunc("POST /v1/shares", server.createShare)
	mux.HandleFunc("DELETE /v1/shares/{id}", server.revokeShare)
	mux.HandleFunc("POST /v1/shares/redeem", server.redeemShare)

	// Edge authorize for Traefik ForwardAuth
	mux.HandleFunc("GET /v1/edge/authorize", server.edgeAuthorize)
	// Also handle POST (Traefik can send POST for non-GET methods)
	mux.HandleFunc("POST /v1/edge/authorize", server.edgeAuthorize)

	// Legacy token exchange (deprecated but kept for compatibility)
	mux.HandleFunc("POST /v1/tokens/exchange", server.tokenExchangeCompat)

	// Session token issue for frontend
	mux.HandleFunc("POST /v1/session/tokens", server.sessionToken)

	return server.middleware(mux)
}

func (server *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) ready(response http.ResponseWriter, request *http.Request) {
	if server.readiness != nil {
		ctx, cancel := context.WithTimeout(request.Context(), time.Second)
		defer cancel()
		if err := server.readiness(ctx); err != nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
	}
	server.health(response, request)
}

func (server *Server) metadata(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{
		"service":  "homehub-iam",
		"version":  server.version,
		"realm":    "homehub",
		"features": []string{"passkeys", "api_keys", "shares"},
	})
}

func (server *Server) jwks(response http.ResponseWriter, _ *http.Request) {
	if server.jwkSet == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "signing_keys_unavailable"})
		return
	}
	response.Header().Set("Cache-Control", "public, max-age=60, stale-if-error=300")
	writeJSON(response, http.StatusOK, server.jwkSet)
}

func (server *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		var value [16]byte
		requestID := ""
		if _, err := rand.Read(value[:]); err == nil {
			requestID = hex.EncodeToString(value[:])
		}
		response.Header().Set("X-Request-ID", requestID)
		request.Header.Set("X-Request-ID", requestID)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
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
