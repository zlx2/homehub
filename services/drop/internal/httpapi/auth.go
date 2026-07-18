package httpapi

import (
	"context"
	"crypto/subtle"
	"net"
	"net/http"
	"strings"
)

type EntryPoint string

const (
	EntryPublic    EntryPoint = "public"
	EntryTailscale EntryPoint = "tailscale"
	EntryHermes    EntryPoint = "hermes"
	EntryHomeHub   EntryPoint = "homehub"
)

type Role string

const (
	RoleGuest      Role = "guest"
	RoleOwner      Role = "owner"
	RoleHermes     Role = "hermes"
	scopeAgentRoot      = "agent.root"
)

type principal struct {
	Role      Role
	Subject   string
	SessionID int64
	Scopes    []string
}

type principalKey struct{}

func withPrincipal(r *http.Request, value principal) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), principalKey{}, value))
}

func principalFrom(r *http.Request) principal {
	value, _ := r.Context().Value(principalKey{}).(principal)
	return value
}

func (p principal) HasScope(required string) bool {
	for _, scope := range p.Scopes {
		if scope == required {
			return true
		}
	}
	return false
}

func (a *API) authenticate(entry EntryPoint, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var value principal
		switch entry {
		case EntryPublic:
			cookie, err := r.Cookie(a.cfg.CookieName)
			if err != nil {
				writeAPIError(w, unauthorized())
				return
			}
			session, valid, err := a.auth.ValidateSession(r.Context(), cookie.Value, clientIP(r, a.cfg.TrustedPublicProxies))
			if err != nil {
				a.logger.Error("session validation failed", "error", err)
				writeAPIError(w, internalError())
				return
			}
			if !valid {
				writeAPIError(w, unauthorized())
				return
			}
			value = principal{Role: RoleGuest, Subject: "temporary-session", SessionID: session.ID}
		case EntryTailscale:
			if !a.cfg.AllowNonLoopback && !remoteIsLoopback(r.RemoteAddr) {
				writeAPIError(w, unauthorized())
				return
			}
			login := strings.ToLower(strings.TrimSpace(r.Header.Get("Tailscale-User-Login")))
			if _, ok := a.cfg.TailscaleUsers[login]; login == "" || !ok {
				writeAPIError(w, unauthorized())
				return
			}
			value = principal{Role: RoleOwner, Subject: login}
		case EntryHermes:
			if (!a.cfg.AllowNonLoopback && !remoteIsLoopback(r.RemoteAddr)) || a.cfg.HermesToken == "" {
				writeAPIError(w, unauthorized())
				return
			}
			provided := bearerToken(r.Header.Get("Authorization"))
			if len(provided) != len(a.cfg.HermesToken) || subtle.ConstantTimeCompare([]byte(provided), []byte(a.cfg.HermesToken)) != 1 {
				writeAPIError(w, unauthorized())
				return
			}
			value = principal{Role: RoleHermes, Subject: "hermes"}
		default:
			writeAPIError(w, unauthorized())
			return
		}
		next.ServeHTTP(w, withPrincipal(r, value))
	})
}

func requireOwner(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := principalFrom(r).Role
		if role != RoleOwner && role != RoleHermes {
			writeAPIError(w, &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Owner permission required"})
			return
		}
		next(w, r)
	}
}

func unauthorized() *apiError {
	return &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Authentication required"}
}

func internalError() *apiError {
	return &apiError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "Internal server error"}
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}

func remoteIsLoopback(remote string) bool {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}

func clientIP(r *http.Request, trusted []*net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote := net.ParseIP(strings.Trim(host, "[]"))
	if remote == nil || !ipInNetworks(remote, trusted) {
		return host
	}
	parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(parts) - 1; i >= 0; i-- {
		candidate := net.ParseIP(strings.TrimSpace(parts[i]))
		if candidate == nil {
			continue
		}
		if !ipInNetworks(candidate, trusted) {
			return candidate.String()
		}
	}
	return remote.String()
}

func ipInNetworks(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
