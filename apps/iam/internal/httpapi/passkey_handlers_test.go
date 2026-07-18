package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasskeyLoginRejectsUnknownOrigin(t *testing.T) {
	handler := New(Options{AllowedOrigins: []string{"https://zlx2.com"}, SecureCookies: true})
	request := httptest.NewRequest(http.MethodPost, "/v1/passkeys/login/begin", nil)
	request.Header.Set("Origin", "https://attacker.invalid")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "invalid_origin") {
		t.Fatalf("expected invalid origin, got status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestPasskeyCeremonyCookieIsHostOnlySecureAndScoped(t *testing.T) {
	server := &Server{secureCookies: true}
	response := httptest.NewRecorder()
	server.setPasskeyCeremonyCookie(response, "ceremony-token")
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected one cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != passkeyCeremonyCookieName || cookie.Value != "ceremony-token" || cookie.Path != "/api/iam/v1/passkeys/" {
		t.Fatalf("unexpected cookie: %#v", cookie)
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Domain != "" {
		t.Fatalf("ceremony cookie must be secure, HttpOnly, Strict, and host-only: %#v", cookie)
	}
}
