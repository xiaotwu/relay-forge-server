package auth

import (
	"fmt"

	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new TOTP secret for the given issuer and
// account. It returns the base32-encoded secret, the otpauth:// provisioning
// URL (suitable for QR code generation), and any error encountered.
func GenerateTOTPSecret(issuer, account string) (secret string, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
	})
	if err != nil {
		return "", "", fmt.Errorf("generating TOTP key: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

// ValidateTOTP checks whether the provided TOTP code is valid for the given
// base32-encoded secret. It uses the default validation options which allow
// for a +/- 1 period skew to accommodate clock drift.
func ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}
