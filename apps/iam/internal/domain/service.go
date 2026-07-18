package domain

import (
	"errors"
	"regexp"
)

var (
	serviceIDName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)
	audienceName  = regexp.MustCompile(`^homehub-[a-z][a-z0-9-]{0,54}$`)
	relationName  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

type ServiceManifest struct {
	Version            int                  `json:"version"`
	ServiceID          string               `json:"service_id"`
	Audience           string               `json:"audience"`
	MaxTokenTTLSeconds int                  `json:"max_token_ttl_seconds"`
	Permissions        []ManifestPermission `json:"permissions"`
}

type ManifestPermission struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	Risk             string `json:"risk"`
	RequiredRelation string `json:"required_relation"`
}

func (manifest ServiceManifest) Validate() error {
	if manifest.Version < 1 || !serviceIDName.MatchString(manifest.ServiceID) ||
		!audienceName.MatchString(manifest.Audience) || manifest.MaxTokenTTLSeconds < 30 ||
		manifest.MaxTokenTTLSeconds > 900 || len(manifest.Permissions) == 0 {
		return errors.New("invalid service manifest")
	}
	seen := make(map[string]struct{}, len(manifest.Permissions))
	for _, permission := range manifest.Permissions {
		parsed, err := ParsePermission(permission.Name)
		if err != nil || parsed.String() == SystemRootPermission || permission.Description == "" ||
			!relationName.MatchString(permission.RequiredRelation) ||
			(permission.Risk != "normal" && permission.Risk != "sensitive" && permission.Risk != "dangerous") {
			return errors.New("invalid service manifest permission")
		}
		if _, duplicate := seen[permission.Name]; duplicate {
			return errors.New("duplicate service manifest permission")
		}
		seen[permission.Name] = struct{}{}
	}
	return nil
}
