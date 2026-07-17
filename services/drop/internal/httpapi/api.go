package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"drop/internal/auth"
	"drop/internal/config"
	"drop/internal/store"
)

type API struct {
	cfg      config.Config
	store    *store.Store
	auth     *auth.Service
	hub      *Hub
	logger   *slog.Logger
	limiter  *rateLimiter
	identity *identityVerifier
}

func (a *API) EnableHomeHubIdentity() error {
	verifier, err := newIdentityVerifier(a.cfg.IdentityKeyFile)
	if err != nil {
		return err
	}
	a.identity = verifier
	return nil
}

func (a *API) HomeHubHandler() http.Handler {
	protected := http.NewServeMux()
	protected.Handle("/api/v1/items", methods(map[string]http.HandlerFunc{
		http.MethodGet: a.listItems, http.MethodPost: a.createItem,
	}))
	protected.Handle("/api/v1/items/{id}", methods(map[string]http.HandlerFunc{
		http.MethodGet: a.getItem, http.MethodDelete: requireOwner(a.deleteItem),
	}))
	protected.Handle("/api/v1/items/{id}/text", methods(map[string]http.HandlerFunc{http.MethodGet: a.getText}))
	protected.Handle("/api/v1/items/{id}/expiry", methods(map[string]http.HandlerFunc{http.MethodPatch: requireOwner(a.updateExpiry)}))
	protected.Handle("/api/v1/attachments/{id}", methods(map[string]http.HandlerFunc{http.MethodGet: a.getAttachment}))
	protected.Handle("/api/v1/attachments/{id}/preview", methods(map[string]http.HandlerFunc{http.MethodGet: a.getAttachmentPreview}))
	protected.Handle("/api/v1/status", methods(map[string]http.HandlerFunc{http.MethodGet: requireOwner(a.status)}))
	protected.Handle("/api/v1/events", methods(map[string]http.HandlerFunc{http.MethodGet: a.events}))
	protected.Handle("/favicon.ico", methods(map[string]http.HandlerFunc{http.MethodGet: a.asset}))
	protected.Handle("/assets/", methods(map[string]http.HandlerFunc{http.MethodGet: a.asset}))
	protected.Handle("/", methods(map[string]http.HandlerFunc{http.MethodGet: a.appPage}))

	root := http.NewServeMux()
	root.Handle("/health/live", methods(map[string]http.HandlerFunc{http.MethodGet: a.live}))
	root.Handle("/health/ready", methods(map[string]http.HandlerFunc{http.MethodGet: a.ready}))
	root.Handle("/", a.authenticateHomeHub(a.requireAllowedOrigin(protected)))
	return a.measureTraffic(EntryHomeHub, recovery(a.logger, securityHeaders(root)))
}

func New(cfg config.Config, storage *store.Store, authService *auth.Service, hub *Hub, logger *slog.Logger) *API {
	if logger == nil {
		logger = slog.Default()
	}
	if hub == nil {
		hub = NewHub()
	}
	return &API{
		cfg: cfg, store: storage, auth: authService, hub: hub, logger: logger,
		limiter: newRateLimiter(8, 10*time.Second, time.Now),
	}
}

func (a *API) Handler(entry EntryPoint) http.Handler {
	protected := http.NewServeMux()
	protected.Handle("/api/v1/items", methods(map[string]http.HandlerFunc{
		http.MethodGet: a.listItems, http.MethodPost: a.createItem,
	}))
	protected.Handle("/api/v1/items/{id}", methods(map[string]http.HandlerFunc{
		http.MethodGet: a.getItem, http.MethodDelete: requireOwner(a.deleteItem),
	}))
	protected.Handle("/api/v1/items/{id}/text", methods(map[string]http.HandlerFunc{http.MethodGet: a.getText}))
	protected.Handle("/api/v1/items/{id}/expiry", methods(map[string]http.HandlerFunc{http.MethodPatch: requireOwner(a.updateExpiry)}))
	protected.Handle("/api/v1/attachments/{id}", methods(map[string]http.HandlerFunc{http.MethodGet: a.getAttachment}))
	protected.Handle("/api/v1/attachments/{id}/preview", methods(map[string]http.HandlerFunc{http.MethodGet: a.getAttachmentPreview}))
	protected.Handle("/api/v1/auth/codes", methods(map[string]http.HandlerFunc{http.MethodPost: requireOwner(a.createAuthCode)}))
	protected.Handle("/api/v1/auth/sessions", methods(map[string]http.HandlerFunc{
		http.MethodGet: a.listSessions, http.MethodDelete: a.revokeAllSessions,
	}))
	protected.Handle("/api/v1/auth/sessions/{id}", methods(map[string]http.HandlerFunc{http.MethodDelete: a.revokeSession}))
	protected.Handle("/api/v1/status", methods(map[string]http.HandlerFunc{http.MethodGet: requireOwner(a.status)}))
	protected.Handle("/api/v1/events", methods(map[string]http.HandlerFunc{http.MethodGet: a.events}))
	protected.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeAPIError(w, &apiError{Status: http.StatusNotFound, Code: "not_found", Message: "Resource not found"})
	})

	root := http.NewServeMux()
	root.Handle("/health/live", methods(map[string]http.HandlerFunc{http.MethodGet: a.live}))
	root.Handle("/health/ready", methods(map[string]http.HandlerFunc{http.MethodGet: a.ready}))
	if entry == EntryPublic {
		root.Handle("/api/v1/auth/redeem", methods(map[string]http.HandlerFunc{http.MethodPost: a.redeemAuthCode}))
	}
	root.Handle("/api/", a.authenticate(entry, protected))
	if entry == EntryPublic || entry == EntryTailscale {
		root.Handle("/favicon.ico", methods(map[string]http.HandlerFunc{http.MethodGet: a.asset}))
		root.Handle("/assets/", methods(map[string]http.HandlerFunc{http.MethodGet: a.asset}))
	}
	switch entry {
	case EntryPublic:
		root.Handle("/", methods(map[string]http.HandlerFunc{http.MethodGet: a.publicPage}))
	case EntryTailscale:
		root.Handle("/", a.authenticate(entry, methods(map[string]http.HandlerFunc{http.MethodGet: a.appPage})))
	default:
		root.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
			writeAPIError(w, &apiError{Status: http.StatusNotFound, Code: "not_found", Message: "Resource not found"})
		})
	}
	return a.measureTraffic(entry, recovery(a.logger, securityHeaders(root)))
}

func methods(handlers map[string]http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler, ok := handlers[r.Method]
		if !ok {
			for method := range handlers {
				w.Header().Add("Allow", method)
			}
			writeAPIError(w, &apiError{Status: http.StatusMethodNotAllowed, Code: "method_not_allowed", Message: "HTTP method is not allowed"})
			return
		}
		handler(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' blob: data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}
