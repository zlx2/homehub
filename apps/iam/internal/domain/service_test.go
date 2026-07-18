package domain

import "testing"

func TestServiceManifestValidation(t *testing.T) {
	t.Parallel()
	manifest := ServiceManifest{
		Version: 1, ServiceID: "drop", Audience: "homehub-drop", MaxTokenTTLSeconds: 120,
		Permissions: []ManifestPermission{{
			Name: "drop.item.create", Description: "Create an item", Risk: "normal", RequiredRelation: "caller",
		}},
	}
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	manifest.Permissions[0].Name = "drop.*"
	if err := manifest.Validate(); err == nil {
		t.Fatal("invalid permission unexpectedly passed validation")
	}
}
