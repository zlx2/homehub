package machineadmin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"gitee.com/zlx23/homehub/apps/iam/internal/domain"
	storepostgres "gitee.com/zlx23/homehub/apps/iam/internal/store/postgres"
	"gitee.com/zlx23/homehub/packages/go-sdk/identity"
)

const maxGrants = 16

var (
	ErrInvalidRequest = errors.New("invalid machine identity request")
	ErrConflict       = errors.New("machine identity already exists")
	serviceName       = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	relationName      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

type Grant struct {
	ServiceID string `json:"service_id"`
	Relation  string `json:"relation"`
}

type CreateRequest struct {
	Kind            domain.PrincipalKind `json:"kind"`
	DisplayName     string               `json:"display_name"`
	ExternalSubject string               `json:"external_subject"`
	Grants          []Grant              `json:"grants"`
}

type CreateResponse struct {
	Subject         string  `json:"subject"`
	DisplayName     string  `json:"display_name"`
	ExternalSubject string  `json:"external_subject"`
	Credential      string  `json:"credential"`
	Grants          []Grant `json:"grants"`
}

type Store interface {
	CreateMachineIdentity(context.Context, string, domain.PrincipalKind, string, string, string) (storepostgres.MachineIdentity, error)
	DeleteMachineIdentity(context.Context, string) error
	ServiceRelationExists(context.Context, string, string) (bool, error)
	RecordMachineAdminAudit(context.Context, string, string, string, string, string, map[string]any)
}

type AuthorizationWriter interface {
	WriteRelationship(context.Context, storepostgres.AuthorizationState, string, string, string) error
	DeleteRelationship(context.Context, storepostgres.AuthorizationState, string, string, string) error
}

type Service struct {
	store         Store
	authorization AuthorizationWriter
	state         storepostgres.AuthorizationState
}

func New(store Store, authorization AuthorizationWriter, state storepostgres.AuthorizationState) *Service {
	return &Service{store: store, authorization: authorization, state: state}
}

func (service *Service) Create(ctx context.Context, actor identity.Claims, requestID string, request CreateRequest) (CreateResponse, error) {
	request.DisplayName = strings.TrimSpace(request.DisplayName)
	request.ExternalSubject = strings.TrimSpace(request.ExternalSubject)
	if !request.Kind.Machine() || request.DisplayName == "" || len(request.DisplayName) > 128 ||
		request.ExternalSubject == "" || len(request.Grants) == 0 || len(request.Grants) > maxGrants {
		return CreateResponse{}, ErrInvalidRequest
	}
	grants := make([]Grant, 0, len(request.Grants))
	seen := make(map[string]struct{}, len(request.Grants))
	for _, grant := range request.Grants {
		if !serviceName.MatchString(grant.ServiceID) || !relationName.MatchString(grant.Relation) {
			return CreateResponse{}, ErrInvalidRequest
		}
		key := grant.ServiceID + "\x00" + grant.Relation
		if _, duplicate := seen[key]; duplicate {
			continue
		}
		exists, err := service.store.ServiceRelationExists(ctx, grant.ServiceID, grant.Relation)
		if err != nil {
			return CreateResponse{}, err
		}
		if !exists {
			return CreateResponse{}, ErrInvalidRequest
		}
		seen[key] = struct{}{}
		grants = append(grants, grant)
	}
	credential, err := generateCredential()
	if err != nil {
		return CreateResponse{}, err
	}
	created, err := service.store.CreateMachineIdentity(ctx, actor.Realm, request.Kind, request.DisplayName, request.ExternalSubject, credential)
	if errors.Is(err, storepostgres.ErrMachineIdentityExists) {
		service.store.RecordMachineAdminAudit(ctx, principalUUID(actor.Subject), "machine.create", "denied", request.ExternalSubject, requestID, map[string]any{"reason": "conflict"})
		return CreateResponse{}, ErrConflict
	}
	if err != nil {
		return CreateResponse{}, err
	}
	written := make([]Grant, 0, len(grants))
	for _, grant := range grants {
		if err := service.authorization.WriteRelationship(ctx, service.state, created.Subject(), grant.Relation, "service:"+grant.ServiceID); err != nil {
			for index := len(written) - 1; index >= 0; index-- {
				_ = service.authorization.DeleteRelationship(ctx, service.state, created.Subject(), written[index].Relation, "service:"+written[index].ServiceID)
			}
			_ = service.store.DeleteMachineIdentity(ctx, created.PrincipalID)
			return CreateResponse{}, fmt.Errorf("grant machine service access: %w", err)
		}
		written = append(written, grant)
	}
	service.store.RecordMachineAdminAudit(ctx, principalUUID(actor.Subject), "machine.create", "success", created.Subject(), requestID, map[string]any{
		"kind": request.Kind, "external_subject": request.ExternalSubject, "grants": grants,
	})
	return CreateResponse{
		Subject: created.Subject(), DisplayName: created.DisplayName, ExternalSubject: request.ExternalSubject,
		Credential: credential, Grants: grants,
	}, nil
}

func generateCredential() (string, error) {
	var value [32]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate machine credential: %w", err)
	}
	return "hhm_" + base64.RawURLEncoding.EncodeToString(value[:]), nil
}

func principalUUID(subject string) string {
	_, value, _ := strings.Cut(subject, ":")
	return value
}
