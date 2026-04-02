package auth

import (
	"strings"
	"testing"
)

func TestGenerateTOTPSecret(t *testing.T) {
	secret, url, err := GenerateTOTPSecret("RelayForge", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secret == "" {
		t.Fatal("expected non-empty secret")
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
}

func TestTOTPSecretFormat(t *testing.T) {
	_, url, err := GenerateTOTPSecret("RelayForge", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(url, "otpauth://") {
		t.Fatalf("expected URL to start with otpauth://, got %s", url)
	}
}
