package domain

import (
	"fmt"
	"regexp"
)

const SystemRootPermission = "system.root"

var permissionName = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}\.[a-z][a-z0-9-]{0,62}$`)

type Permission string

func ParsePermission(value string) (Permission, error) {
	if value != SystemRootPermission && !permissionName.MatchString(value) {
		return "", fmt.Errorf("invalid permission %q", value)
	}
	return Permission(value), nil
}

func (permission Permission) String() string {
	return string(permission)
}
