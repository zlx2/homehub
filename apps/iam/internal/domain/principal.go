package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type PrincipalKind string

const (
	PrincipalHuman    PrincipalKind = "human"
	PrincipalGuest    PrincipalKind = "guest"
	PrincipalDevice   PrincipalKind = "device"
	PrincipalNode     PrincipalKind = "node"
	PrincipalWorkload PrincipalKind = "workload"
	PrincipalAgent    PrincipalKind = "agent"
)

var (
	errInvalidPrincipal = errors.New("invalid principal identifier")
	principalLocalID    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
)

func (kind PrincipalKind) Valid() bool {
	switch kind {
	case PrincipalHuman, PrincipalGuest, PrincipalDevice, PrincipalNode, PrincipalWorkload, PrincipalAgent:
		return true
	default:
		return false
	}
}

type PrincipalID struct {
	Kind PrincipalKind
	ID   string
}

func NewPrincipalID(kind PrincipalKind, id string) (PrincipalID, error) {
	if !kind.Valid() || !principalLocalID.MatchString(id) {
		return PrincipalID{}, errInvalidPrincipal
	}
	return PrincipalID{Kind: kind, ID: id}, nil
}

func ParsePrincipalID(value string) (PrincipalID, error) {
	kind, id, ok := strings.Cut(value, ":")
	if !ok || strings.ContainsRune(id, ':') {
		return PrincipalID{}, fmt.Errorf("%w: %q", errInvalidPrincipal, value)
	}
	principal, err := NewPrincipalID(PrincipalKind(kind), id)
	if err != nil {
		return PrincipalID{}, fmt.Errorf("%w: %q", errInvalidPrincipal, value)
	}
	return principal, nil
}

func (principal PrincipalID) String() string {
	return string(principal.Kind) + ":" + principal.ID
}
