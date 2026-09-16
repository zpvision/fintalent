package main

import (
	"regexp"
	"testing"
)

func TestResetHMACSeparatesPurposeAndValue(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	first := resetHMAC(secret, "code", "user@example.com:123456")
	if first != resetHMAC(secret, "code", "user@example.com:123456") {
		t.Fatal("same input must produce the same digest")
	}
	if first == resetHMAC(secret, "token", "user@example.com:123456") {
		t.Fatal("different purposes must produce different digests")
	}
}

func TestPasswordResetSecretFallsBackToSMTPPassword(t *testing.T) {
	t.Setenv("PASSWORD_RESET_SECRET", "")
	t.Setenv("SMTP_PASSWORD", "smtp-secret")
	secret, err := passwordResetSecret()
	if err != nil {
		t.Fatal(err)
	}
	if len(secret) != 32 {
		t.Fatalf("derived secret length = %d, want 32", len(secret))
	}
}

func TestNewResetCodeAlwaysHasSixDigits(t *testing.T) {
	pattern := regexp.MustCompile(`^\d{6}$`)
	for range 100 {
		code, err := newResetCode()
		if err != nil {
			t.Fatal(err)
		}
		if !pattern.MatchString(code) {
			t.Fatalf("invalid reset code %q", code)
		}
	}
}

func TestNewResetTokenIsUniqueAndURLSafe(t *testing.T) {
	first, err := newResetToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newResetToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("tokens must be unique")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`).MatchString(first) {
		t.Fatalf("token is not raw URL-safe base64: %q", first)
	}
}
