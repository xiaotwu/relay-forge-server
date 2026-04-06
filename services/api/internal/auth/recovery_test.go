package auth

import "testing"

func TestGenerateBackupCodes(t *testing.T) {
	codes, err := GenerateBackupCodes(8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(codes) != 8 {
		t.Fatalf("expected 8 codes, got %d", len(codes))
	}

	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if len(code) != 9 {
			t.Fatalf("expected formatted code length 9, got %q", code)
		}
		if _, exists := seen[code]; exists {
			t.Fatalf("duplicate backup code generated: %q", code)
		}
		seen[code] = struct{}{}
	}
}

func TestBackupCodeMatches(t *testing.T) {
	if !BackupCodeMatches("ABCD-EFGH", "abcdefgh") {
		t.Fatal("expected backup code comparison to ignore casing and dashes")
	}
	if BackupCodeMatches("ABCD-EFGH", "WXYZ-1234") {
		t.Fatal("expected different backup codes not to match")
	}
}

func TestGenerateOpaqueToken(t *testing.T) {
	token, err := GenerateOpaqueToken(32)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected opaque token to be non-empty")
	}
}
