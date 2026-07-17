package httpapi

import (
	"net/http"
	"strings"

	"homehub.local/go-sdk/identity"
)

func (a *API) authenticateHomeHub(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		claims, err := a.identity.Verify(request.Header.Get(identity.HeaderName))
		if err != nil || !claims.HasAnyScope("portal.view", "admin") {
			writeAPIError(writer, unauthorized())
			return
		}
		role := RoleGuest
		if claims.HasScope("admin") {
			role = RoleOwner
		}
		value := principal{Role: role, Subject: claims.Subject}
		next.ServeHTTP(writer, withPrincipal(request, value))
	})
}

func (a *API) requireAllowedOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			origin := strings.ToLower(strings.TrimSpace(request.Header.Get("Origin")))
			if _, ok := a.cfg.AllowedOrigins[origin]; !ok {
				writeAPIError(writer, &apiError{Status: http.StatusForbidden, Code: "invalid_origin", Message: "Request origin is not allowed"})
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}
