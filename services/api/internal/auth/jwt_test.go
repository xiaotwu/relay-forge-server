package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestJWTService() *JWTService {
	return NewJWTService("test-secret-key-at-least-32-chars!", 15*time.Minute, 7*24*time.Hour)
}

func TestGenerateAccessToken(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	token, err := svc.GenerateAccessToken(userID, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}
}

func TestValidateAccessToken(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	roles := []string{"user", "admin"}

	token, err := svc.GenerateAccessToken(userID, roles)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	claims, err := svc.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected UserID %s, got %s", userID, claims.UserID)
	}
	if len(claims.Roles) != 2 || claims.Roles[0] != "user" || claims.Roles[1] != "admin" {
		t.Fatalf("expected roles [user admin], got %v", claims.Roles)
	}
}

func TestExpiredAccessToken(t *testing.T) {
	svc := NewJWTService("test-secret-key-at-least-32-chars!", -1*time.Second, 7*24*time.Hour)
	userID := uuid.New()

	token, err := svc.GenerateAccessToken(userID, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	_, err = svc.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestInvalidSignature(t *testing.T) {
	svc1 := NewJWTService("secret-one-at-least-32-characters!", 15*time.Minute, 7*24*time.Hour)
	svc2 := NewJWTService("secret-two-at-least-32-characters!", 15*time.Minute, 7*24*time.Hour)

	userID := uuid.New()
	token, err := svc1.GenerateAccessToken(userID, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	_, err = svc2.ValidateAccessToken(token)
	if err == nil {
		t.Fatal("expected error for token signed with wrong secret")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	sessionID := uuid.New()

	token, err := svc.GenerateRefreshToken(userID, sessionID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty refresh token")
	}
}

func TestValidateRefreshToken(t *testing.T) {
	svc := newTestJWTService()
	userID := uuid.New()
	sessionID := uuid.New()

	token, err := svc.GenerateRefreshToken(userID, sessionID)
	if err != nil {
		t.Fatalf("unexpected error generating token: %v", err)
	}

	claims, err := svc.ValidateRefreshToken(token)
	if err != nil {
		t.Fatalf("unexpected error validating token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.SessionID != sessionID {
		t.Fatalf("expected SessionID %s, got %s", sessionID, claims.SessionID)
	}
}
