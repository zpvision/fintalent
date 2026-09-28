package main

import (
	"strings"
	"testing"
)

func TestResetOutboxEncryptedAndAuthenticated(t *testing.T) {
	t.Setenv("PASSWORD_RESET_SECRET", strings.Repeat("a", 32))
	sealed, err := encryptResetCode("123456")
	if err != nil {
		t.Fatal(err)
	}
	if sealed == "123456" || !strings.HasPrefix(sealed, "v1:") {
		t.Fatal("reset code stored without envelope")
	}
	plain, err := decryptResetCode(sealed)
	if err != nil || plain != "123456" {
		t.Fatal("round trip failed")
	}
	second, err := encryptResetCode("123456")
	if err != nil {
		t.Fatal(err)
	}
	if second == sealed {
		t.Fatal("nonce reused")
	}
	if _, err = decryptResetCode("123456"); err == nil {
		t.Fatal("plaintext envelope accepted")
	}
	t.Setenv("PASSWORD_RESET_SECRET", strings.Repeat("b", 32))
	if _, err = decryptResetCode(sealed); err == nil {
		t.Fatal("wrong key accepted")
	}
}
