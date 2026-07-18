package humanauth

import (
	"testing"
	"time"
)

func TestValidateTOTP(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if !validateTOTP(secret, "287082", time.Unix(59, 0)) {
		t.Fatal("expected RFC 6238-derived code to validate")
	}
	if validateTOTP(secret, "287083", time.Unix(59, 0)) {
		t.Fatal("accepted an invalid code")
	}
	if validateTOTP(secret, "not-six-digits", time.Unix(59, 0)) {
		t.Fatal("accepted malformed code")
	}
}

func TestPasswordHashAndVerify(t *testing.T) {
	service := &Service{hashSlots: make(chan struct{}, 1)}
	encoded, err := service.hashPassword("a-long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if !service.verifyPassword("a-long-test-password", encoded) {
		t.Fatal("password did not verify")
	}
	if service.verifyPassword("a-different-password", encoded) {
		t.Fatal("wrong password verified")
	}
	if service.verifyPassword("a-long-test-password", "$argon2id$broken") {
		t.Fatal("malformed hash verified")
	}
}
