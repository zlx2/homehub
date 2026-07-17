package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"drop/internal/store"
)

type trustedSessionResponse struct {
	ID         int64     `json:"id"`
	DeviceName string    `json:"device_name"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	LastIP     string    `json:"last_ip,omitempty"`
	Current    bool      `json:"current"`
}

func (a *API) listSessions(w http.ResponseWriter, r *http.Request) {
	principal := principalFrom(r)
	sessions, err := a.store.ListSessions(r.Context())
	if err != nil {
		a.logInternal("list trusted sessions", err)
		writeAPIError(w, err)
		return
	}
	response := make([]trustedSessionResponse, 0, len(sessions))
	for _, session := range sessions {
		if principal.Role == RoleGuest && session.ID != principal.SessionID {
			continue
		}
		response = append(response, sessionResponse(session, session.ID == principal.SessionID))
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": response})
}

func (a *API) revokeSession(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeAPIError(w, store.ErrNotFound)
		return
	}
	principal := principalFrom(r)
	if principal.Role == RoleGuest && principal.SessionID != id {
		writeAPIError(w, &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Owner permission required"})
		return
	}
	revoked, err := a.store.RevokeSession(r.Context(), id)
	if err != nil {
		a.logInternal("revoke trusted session", err)
		writeAPIError(w, err)
		return
	}
	if !revoked {
		writeAPIError(w, store.ErrNotFound)
		return
	}
	if principal.SessionID == id {
		a.clearSessionCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) revokeAllSessions(w http.ResponseWriter, r *http.Request) {
	if principalFrom(r).Role == RoleGuest {
		writeAPIError(w, &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Owner permission required"})
		return
	}
	if _, err := a.store.RevokeAllSessions(r.Context()); err != nil {
		a.logInternal("revoke all trusted sessions", err)
		writeAPIError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: a.cfg.CookieName, Value: "", Path: "/", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func sessionResponse(session store.TrustedSession, current bool) trustedSessionResponse {
	return trustedSessionResponse{
		ID: session.ID, DeviceName: session.DeviceName, CreatedAt: session.CreatedAt,
		LastSeenAt: session.LastSeenAt, ExpiresAt: session.ExpiresAt, LastIP: session.LastIP, Current: current,
	}
}
