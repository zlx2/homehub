package httpapi

import (
	"net/http"
	"strings"

	"homehub.local/go-sdk/identity"
)

func (a *API) authenticateHomeHub(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		claims, err := a.identity.Verify(request.Header.Get(identity.HeaderName))
		if err != nil || !claims.HasAnyScope("portal.view", "admin", "drop.upload") {
			writeAPIError(writer, unauthorized())
			return
		}
		if claims.HasScope("drop.upload") && !claims.HasAnyScope("portal.view", "admin") &&
			(request.Method != http.MethodPost || request.URL.Path != "/api/v1/items") {
			writeAPIError(writer, &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Token is limited to Drop uploads"})
			return
		}
		role := RoleGuest
		if claims.HasScope("admin") {
			role = RoleOwner
		}
		value := principal{Role: role, Subject: claims.Subject, Scopes: append([]string(nil), claims.Scopes...)}
		next.ServeHTTP(writer, withPrincipal(request, value))
	})
}

func (a *API) requireAllowedOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			if principalFrom(request).HasScope("drop.upload") && request.Method == http.MethodPost && request.URL.Path == "/api/v1/items" {
				next.ServeHTTP(writer, request)
				return
			}
			origin := strings.ToLower(strings.TrimSpace(request.Header.Get("Origin")))
			if _, ok := a.cfg.AllowedOrigins[origin]; !ok {
				writeAPIError(writer, &apiError{Status: http.StatusForbidden, Code: "invalid_origin", Message: "Request origin is not allowed"})
				return
			}
		}
		next.ServeHTTP(writer, request)
	})
}
