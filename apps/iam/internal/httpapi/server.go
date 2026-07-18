package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"
)

type Server struct {
	version   string
	readiness func(context.Context) error
	jwkSet    any
}

type Options struct {
	Version   string
	Readiness func(context.Context) error
	JWKSet    any
}

func New(options Options) http.Handler {
	server := &Server{version: options.Version, readiness: options.Readiness, jwkSet: options.JWKSet}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", server.health)
	mux.HandleFunc("GET /health/ready", server.ready)
	mux.HandleFunc("GET /v1/metadata", server.metadata)
	mux.HandleFunc("GET /.well-known/jwks.json", server.jwks)
	return server.middleware(mux)
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
		requestID := request.Header.Get("X-Request-ID")
		if requestID == "" {
			var value [16]byte
			if _, err := rand.Read(value[:]); err == nil {
				requestID = hex.EncodeToString(value[:])
			}
		}
		response.Header().Set("X-Request-ID", requestID)
		response.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(response, request)
	})
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(value)
}
