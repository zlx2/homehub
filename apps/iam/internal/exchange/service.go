package exchange

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/apps/iam/internal/token"
	"homehub.local/go-sdk/identity"
)

const maxRequestedPermissions = 32

var (
	ErrInvalidClient          = errors.New("invalid client")
	ErrInvalidRequest         = errors.New("invalid token request")
	ErrInsufficientPermission = errors.New("insufficient permission")
)

type Request struct {
	Audience    string   `json:"audience"`
	Permissions []string `json:"permissions"`
}

type Response struct {
	AccessToken string   `json:"access_token"`
	TokenType   string   `json:"token_type"`
	ExpiresIn   int      `json:"expires_in"`
	Audience    string   `json:"audience"`
	Permissions []string `json:"permissions"`
}

type Service struct {
	store         IdentityStore
	authorization AuthorizationChecker
	authzState    storepostgres.AuthorizationState
	signer        TokenSigner
}

type IdentityStore interface {
	AuthenticateAPIKey(context.Context, string) (storepostgres.MachineIdentity, error)
	AudiencePolicy(context.Context, string) (storepostgres.AudiencePolicy, error)
	RecordTokenAudit(context.Context, storepostgres.MachineIdentity, string, string, string, string, map[string]any)
}

type AuthorizationChecker interface {
	Check(context.Context, storepostgres.AuthorizationState, string, string, string) (bool, error)
}

type TokenSigner interface {
	Issue(token.IssueRequest) (string, identity.Claims, error)
}

func New(store IdentityStore, authorization AuthorizationChecker, state storepostgres.AuthorizationState, signer TokenSigner) *Service {
	return &Service{store: store, authorization: authorization, authzState: state, signer: signer}
}

func (service *Service) Exchange(ctx context.Context, credential, requestID string, request Request) (Response, error) {
	if request.Audience == "" || len(request.Permissions) == 0 || len(request.Permissions) > maxRequestedPermissions {
		return Response{}, ErrInvalidRequest
	}
	identity, err := service.store.AuthenticateAPIKey(ctx, credential)
	if err != nil {
		return Response{}, ErrInvalidClient
	}
	policy, err := service.store.AudiencePolicy(ctx, request.Audience)
	if err != nil {
		service.store.RecordTokenAudit(ctx, identity, "token.exchange", "denied", request.Audience, requestID, map[string]any{"reason": "unknown_audience"})
		return Response{}, ErrInvalidRequest
	}

	root, err := service.authorization.Check(ctx, service.authzState, identity.Subject(), "root", "realm:"+identity.Realm)
	if err != nil {
		return Response{}, fmt.Errorf("check root authorization: %w", err)
	}
	for _, permission := range request.Permissions {
		if permission == "system.root" {
			if !root {
				service.denied(ctx, identity, request, requestID, permission)
				return Response{}, ErrInsufficientPermission
			}
			continue
		}
		relation, known := policy.Permissions[permission]
		if !known {
			service.store.RecordTokenAudit(ctx, identity, "token.exchange", "denied", request.Audience, requestID, map[string]any{
				"reason": "unknown_permission", "permission": permission,
			})
			return Response{}, ErrInvalidRequest
		}
		if root {
			continue
		}
		allowed, err := service.authorization.Check(ctx, service.authzState, identity.Subject(), relation, "service:"+policy.ServiceID)
		if err != nil {
			return Response{}, fmt.Errorf("check service authorization: %w", err)
		}
		if !allowed {
			service.denied(ctx, identity, request, requestID, permission)
			return Response{}, ErrInsufficientPermission
		}
	}

	encoded, claims, err := service.signer.Issue(token.IssueRequest{
		Audience: request.Audience, Subject: identity.Subject(), AuthorizedParty: identity.CredentialID,
		Realm: identity.Realm, Permissions: request.Permissions, Authentication: []string{"api_key"},
		AuthenticationAt: time.Now().UTC(),
	})
	if err != nil {
		return Response{}, fmt.Errorf("issue access token: %w", err)
	}
	service.store.RecordTokenAudit(ctx, identity, "token.exchange", "success", request.Audience, requestID, map[string]any{"permissions": claims.Permissions})
	return Response{
		AccessToken: encoded, TokenType: "Bearer", ExpiresIn: int(claims.Expires - claims.IssuedAt),
		Audience: claims.Audience, Permissions: claims.Permissions,
	}, nil
}

func (service *Service) denied(ctx context.Context, identity storepostgres.MachineIdentity, request Request, requestID, permission string) {
	service.store.RecordTokenAudit(ctx, identity, "token.exchange", "denied", request.Audience, requestID, map[string]any{"permission": permission})
}

func HTTPStatus(err error) int {
	switch {
	case errors.Is(err, ErrInvalidClient):
		return http.StatusUnauthorized
	case errors.Is(err, ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, ErrInsufficientPermission):
		return http.StatusForbidden
	default:
		return http.StatusServiceUnavailable
	}
}

func ErrorCode(err error) string {
	switch {
	case errors.Is(err, ErrInvalidClient):
		return "invalid_client"
	case errors.Is(err, ErrInvalidRequest):
		return "invalid_request"
	case errors.Is(err, ErrInsufficientPermission):
		return "insufficient_permission"
	default:
		return "temporarily_unavailable"
	}
}
