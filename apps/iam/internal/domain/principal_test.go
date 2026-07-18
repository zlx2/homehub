package domain

import "testing"

func TestParsePrincipalID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		valid bool
	}{
		{value: "human:luna", valid: true},
		{value: "guest:01JZK6A4", valid: true},
		{value: "device:iphone-17", valid: true},
		{value: "node:hermes", valid: true},
		{value: "workload:telegram-bridge", valid: true},
		{value: "agent:hermes", valid: true},
		{value: "user:luna", valid: false},
		{value: "human:", valid: false},
		{value: "human:luna:extra", valid: false},
		{value: "human:contains space", valid: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()
			principal, err := ParsePrincipalID(test.value)
			if test.valid && err != nil {
				t.Fatalf("ParsePrincipalID() error = %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("ParsePrincipalID() unexpectedly succeeded")
			}
			if test.valid && principal.String() != test.value {
				t.Fatalf("String() = %q, want %q", principal.String(), test.value)
			}
		})
	}
}
