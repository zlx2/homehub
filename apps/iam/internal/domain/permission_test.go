package domain

import "testing"

func TestParsePermission(t *testing.T) {
	t.Parallel()

	valid := []string{
		"drop.item.create",
		"server.command.execute",
		"ai.model.invoke",
		SystemRootPermission,
	}
	for _, value := range valid {
		if _, err := ParsePermission(value); err != nil {
			t.Errorf("ParsePermission(%q) error = %v", value, err)
		}
	}

	invalid := []string{"admin", "drop.upload", "drop.*.*", "Drop.item.read", "drop.item", "drop.item.read.extra"}
	for _, value := range invalid {
		if _, err := ParsePermission(value); err == nil {
			t.Errorf("ParsePermission(%q) unexpectedly succeeded", value)
		}
	}
}
