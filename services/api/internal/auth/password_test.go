package auth

import (
	"strings"
	"testing"
)

func TestHashPassword(t *testing.T) {
	t.Run("hash is non-empty", func(t *testing.T) {
		hash, err := HashPassword("MyP@ssw0rd!")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash == "" {
			t.Fatal("expected non-empty hash")
		}
	})

	t.Run("different hashes for same password due to different salts", func(t *testing.T) {
		hash1, err := HashPassword("MyP@ssw0rd!")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		hash2, err := HashPassword("MyP@ssw0rd!")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash1 == hash2 {
			t.Fatal("expected different hashes for the same password (different salts)")
		}
	})
}

func TestVerifyPassword(t *testing.T) {
	password := "SecureP@ss1"
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatalf("unexpected error hashing: %v", err)
	}

	t.Run("correct password returns true", func(t *testing.T) {
		ok, err := VerifyPassword(password, hash)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected true for correct password")
		}
	})

	t.Run("wrong password returns false", func(t *testing.T) {
		ok, err := VerifyPassword("WrongP@ss1", hash)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected false for wrong password")
		}
	})
}

func TestVerifyPasswordFormat(t *testing.T) {
	hash, err := HashPassword("MyP@ssw0rd!")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Fatalf("expected hash to start with $argon2id$, got %s", hash)
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	minLen := 8

	t.Run("too short fails", func(t *testing.T) {
		err := ValidatePasswordStrength("Ab1!", minLen)
		if err == nil {
			t.Fatal("expected error for short password")
		}
	})

	t.Run("no uppercase fails", func(t *testing.T) {
		err := ValidatePasswordStrength("abcdefg1!", minLen)
		if err == nil {
			t.Fatal("expected error for missing uppercase")
		}
	})

	t.Run("no digit fails", func(t *testing.T) {
		err := ValidatePasswordStrength("Abcdefgh!", minLen)
		if err == nil {
			t.Fatal("expected error for missing digit")
		}
	})

	t.Run("valid password passes", func(t *testing.T) {
		err := ValidatePasswordStrength("Abcdefg1!", minLen)
		if err != nil {
			t.Fatalf("expected no error for valid password, got: %v", err)
		}
	})
}
