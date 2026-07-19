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

	"gitee.com/zlx23/homehub/apps/iam/internal/exchange"
	"gitee.com/zlx23/homehub/apps/iam/internal/humanauth"
	"gitee.com/zlx23/homehub/apps/iam/internal/machineadmin"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

const (
	principalManagePermission = "iam.principal.manage"
	grantManagePermission     = "iam.grant.manage"
)

type TokenVerifier interface {
	Verify(string) (identity.Claims, error)
}

type MachineAdministrator interface {
	Create(context.Context, identity.Claims, string, machineadmin.CreateRequest) (machineadmin.CreateResponse, error)
}

type Server struct {
	version       string
	readiness     func(context.Context) error
	jwkSet        any
	exchanger     *exchange.Service
	verifier      TokenVerifier
	machines      MachineAdministrator
	humans        *humanauth.Service
	origins       map[string]struct{}
	secureCookies bool
}

type Options struct {
	Version        string
	Readiness      func(context.Context) error
	JWKSet         any
	Exchanger      *exchange.Service
	Verifier       TokenVerifier
	Machines       MachineAdministrator
	Humans         *humanauth.Service
	AllowedOrigins []string
	SecureCookies  bool
}

func New(options Options) http.Handler {
	server := &Server{
		version: options.Version, readiness: options.Readiness, jwkSet: options.JWKSet, exchanger: options.Exchanger,
		verifier: options.Verifier, machines: options.Machines,
		humans: options.Humans, origins: make(map[string]struct{}, len(options.AllowedOrigins)), secureCookies: options.SecureCookies,
	}
	for _, origin := range options.AllowedOrigins {
		server.origins[origin] = struct{}{}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.health)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /v1/metadata", server.metadata)
	mux.HandleFunc("GET /.well-known/jwks.json", server.jwks)
	mux.HandleFunc("POST /v1/tokens/exchange", server.exchangeToken)
	mux.HandleFunc("GET /v1/session", server.session)
	mux.HandleFunc("POST /v1/setup/begin", server.beginSetup)
	mux.HandleFunc("POST /v1/setup/confirm", server.confirmSetup)
	mux.HandleFunc("POST /v1/login", server.login)
	mux.HandleFunc("POST /v1/passkeys/login/begin", server.beginPasskeyLogin)
	mux.HandleFunc("POST /v1/passkeys/login/finish", server.finishPasskeyLogin)
	mux.HandleFunc("POST /v1/passkeys/registration/begin", server.beginPasskeyRegistration)
	mux.HandleFunc("POST /v1/passkeys/registration/finish", server.finishPasskeyRegistration)
	mux.HandleFunc("GET /v1/passkeys", server.listPasskeys)
	mux.HandleFunc("DELETE /v1/passkeys/{id}", server.deletePasskey)
	mux.HandleFunc("POST /v1/logout", server.logout)
	mux.HandleFunc("POST /v1/session/tokens", server.sessionToken)
	mux.HandleFunc("/v1/edge/authorize", server.edgeAuthorize)
	mux.HandleFunc("GET /v1/shares", server.listShares)
	mux.HandleFunc("POST /v1/shares", server.createShare)
	mux.HandleFunc("DELETE /v1/shares/{id}", server.revokeShare)
	mux.HandleFunc("POST /v1/shares/redeem", server.redeemShare)
	mux.Handle("POST /v1/machine-identities", server.authenticate([]string{principalManagePermission, grantManagePermission}, http.HandlerFunc(server.createMachineIdentity)))
	return server.middleware(mux)
}

func (server *Server) createMachineIdentity(response http.ResponseWriter, request *http.Request) {
	if server.machines == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 32<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input machineadmin.CreateRequest
	if err := decoder.Decode(&input); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	actor, _ := identity.FromContext(request.Context())
	result, err := server.machines.Create(request.Context(), actor, request.Header.Get("X-Request-ID"), input)
	if err != nil {
		switch {
		case errors.Is(err, machineadmin.ErrInvalidRequest):
			writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		case errors.Is(err, machineadmin.ErrConflict):
			writeJSON(response, http.StatusConflict, map[string]string{"error": "identity_exists"})
		default:
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		}
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, result)
}

func (server *Server) authenticate(requiredAll []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if server.verifier == nil {
			writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
			return
		}
		encoded, err := identity.BearerToken(request)
		if err != nil {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
			return
		}
		claims, err := server.verifier.Verify(encoded)
		if err != nil {
			writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
			return
		}
		for _, permission := range requiredAll {
			if !claims.Allows(permission) {
				writeJSON(response, http.StatusForbidden, map[string]string{"error": "insufficient_permission"})
				return
			}
		}
		next.ServeHTTP(response, request.WithContext(identity.ContextWithClaims(request.Context(), claims)))
	})
}

func (server *Server) exchangeToken(response http.ResponseWriter, request *http.Request) {
	if server.exchanger == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "temporarily_unavailable"})
		return
	}
	credential, err := identity.BearerToken(request)
	if err != nil {
		writeJSON(response, http.StatusUnauthorized, map[string]string{"error": "invalid_client"})
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, 16<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var input exchange.Request
	if err := decoder.Decode(&input); err != nil {
		writeJSON(response, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	result, err := server.exchanger.Exchange(request.Context(), credential, request.Header.Get("X-Request-ID"), input)
	if err != nil {
		writeJSON(response, exchange.HTTPStatus(err), map[string]string{"error": exchange.ErrorCode(err)})
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusOK, result)
}

func (server *Server) jwks(response http.ResponseWriter, _ *http.Request) {
	if server.jwkSet == nil {
		writeJSON(response, http.StatusServiceUnavailable, map[string]string{"error": "signing_keys_unavailable"})
		return
	}
	response.Header().Set("Cache-Control", "public, max-age=60, stale-if-error=300")
	writeJSON(response, http.StatusOK, server.jwkSet)
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

func (server *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) metadata(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]any{
		"service":         "homehub-iam",
		"version":         server.version,
		"realm":           "homehub",
		"principal_kinds": []string{"human", "guest", "device", "node", "workload", "agent"},
		"permission_form": "<service>.<resource>.<action>",
	})
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
