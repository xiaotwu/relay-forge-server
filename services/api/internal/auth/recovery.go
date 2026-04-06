package auth

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"strings"
)

func GenerateBackupCodes(count int) ([]string, error) {
	if count <= 0 {
		count = 8
	}

	encoder := base32.StdEncoding.WithPadding(base32.NoPadding)
	codes := make([]string, 0, count)
	seen := make(map[string]struct{}, count)

	for len(codes) < count {
		buf := make([]byte, 5)
		if _, err := rand.Read(buf); err != nil {
			return nil, fmt.Errorf("generate backup code: %w", err)
		}

		raw := strings.ToUpper(encoder.EncodeToString(buf))
		if len(raw) < 8 {
			continue
		}

		code := fmt.Sprintf("%s-%s", raw[:4], raw[4:8])
		if _, exists := seen[code]; exists {
			continue
		}

		seen[code] = struct{}{}
		codes = append(codes, code)
	}

	return codes, nil
}

func NormalizeBackupCode(code string) string {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	normalized = strings.ReplaceAll(normalized, "-", "")
	normalized = strings.ReplaceAll(normalized, " ", "")
	return normalized
}

func BackupCodeMatches(storedCode, providedCode string) bool {
	return NormalizeBackupCode(storedCode) == NormalizeBackupCode(providedCode)
}

func GenerateOpaqueToken(byteLength int) (string, error) {
	if byteLength <= 0 {
		byteLength = 32
	}

	buf := make([]byte, byteLength)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}
