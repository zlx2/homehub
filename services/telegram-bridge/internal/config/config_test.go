package config

import "testing"

func TestParseIDsAndAllowed(t *testing.T) {
	users, err := parseIDs("23, 42")
	if err != nil {
		t.Fatal(err)
	}
	chats, err := parseIDs("-100123")
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{AllowedUserIDs: users, AllowedChatIDs: chats}
	if !cfg.Allowed(23, 999) || !cfg.Allowed(999, -100123) || cfg.Allowed(7, 8) {
		t.Fatal("allowlist decision is incorrect")
	}
}

func TestParseIDsRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"abc", "0", "1,,nope"} {
		if _, err := parseIDs(value); err == nil {
			t.Fatalf("parseIDs(%q) succeeded", value)
		}
	}
}
