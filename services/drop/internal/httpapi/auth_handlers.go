package httpapi

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"drop/internal/store"
	"github.com/skip2/go-qrcode"
)

func (a *API) createAuthCode(w http.ResponseWriter, r *http.Request) {
	code, err := a.auth.GenerateCode(r.Context())
	if err != nil {
		a.logInternal("create authorization code", err)
		writeAPIError(w, err)
		return
	}
	response := map[string]any{
		"code": code.Value, "expires_at": code.ExpiresAt,
		"session_ttl_seconds": int64(a.cfg.SessionTTL.Seconds()),
	}
	if a.cfg.PublicURL != "" {
		redeemURL := strings.TrimRight(a.cfg.PublicURL, "/") + "/#code=" + url.PathEscape(code.Value)
		png, err := qrcode.Encode(redeemURL, qrcode.Medium, 320)
		if err != nil {
			a.logInternal("create authorization QR code", err)
			writeAPIError(w, internalError())
			return
		}
		response["redeem_url"] = redeemURL
		response["qr_data_url"] = "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
	}
	writeJSON(w, http.StatusCreated, response)
}

func (a *API) redeemAuthCode(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r, a.cfg.TrustedPublicProxies)
	if !a.limiter.Allow(ip) {
		w.Header().Set("Retry-After", "10")
		writeAPIError(w, &apiError{Status: http.StatusTooManyRequests, Code: "authorization_rate_limited", Message: "Too many authorization attempts"})
		return
	}
	var input struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(w, r, 8192, &input); err != nil {
		writeAPIError(w, err)
		return
	}
	session, err := a.auth.RedeemCode(r.Context(), input.Code, store.SessionMetadata{
		DeviceName: describeDevice(r.UserAgent()),
		LastIP:     ip,
	})
	if err != nil {
		if !errors.Is(err, store.ErrCodeInvalid) {
			a.logInternal("redeem authorization code", err)
		}
		writeAPIError(w, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: a.cfg.CookieName, Value: session.Token, Path: "/", Expires: session.ExpiresAt,
		MaxAge: int(a.cfg.SessionTTL.Seconds()), HttpOnly: true, Secure: a.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
	writeJSON(w, http.StatusOK, map[string]any{"expires_at": session.ExpiresAt})
}

func describeDevice(userAgent string) string {
	lower := strings.ToLower(userAgent)
	device := "设备"
	switch {
	case strings.Contains(lower, "iphone"):
		device = "iPhone"
	case strings.Contains(lower, "ipad"):
		device = "iPad"
	case strings.Contains(lower, "android"):
		device = "Android"
	case strings.Contains(lower, "windows"):
		device = "Windows"
	case strings.Contains(lower, "macintosh") || strings.Contains(lower, "mac os"):
		device = "Mac"
	case strings.Contains(lower, "linux"):
		device = "Linux"
	}
	browser := "浏览器"
	switch {
	case strings.Contains(lower, "via"):
		browser = "Via"
	case strings.Contains(lower, "crios") || strings.Contains(lower, "chrome"):
		browser = "Chrome"
	case strings.Contains(lower, "fxios") || strings.Contains(lower, "firefox"):
		browser = "Firefox"
	case strings.Contains(lower, "edgios") || strings.Contains(lower, "edg/"):
		browser = "Edge"
	case strings.Contains(lower, "safari"):
		browser = "Safari"
	}
	return browser + " · " + device
}

func (a *API) status(w http.ResponseWriter, r *http.Request) {
	usage, err := a.store.Usage(r.Context())
	if err != nil {
		a.logInternal("read status", err)
		writeAPIError(w, err)
		return
	}
	traffic, err := a.store.TrafficReport(r.Context(), time.Now())
	if err != nil {
		a.logInternal("read traffic report", err)
		writeAPIError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"storage": map[string]any{
			"used_bytes": usage.UsedBytes, "quota_bytes": usage.QuotaBytes,
			"item_count": usage.ItemCount, "attachment_count": usage.AttachmentCount,
		},
		"traffic":      traffic,
		"traffic_note": "Application response body bytes only; provider-billed traffic also includes transport, encryption, and retransmission overhead.",
		"sse_clients":  a.hub.ClientCount(),
	})
}

func (a *API) events(w http.ResponseWriter, r *http.Request) {
	if err := serveEvents(w, r, a.hub, 20*time.Second); err != nil && r.Context().Err() == nil {
		a.logger.Debug("SSE connection ended", "error", err)
	}
}
