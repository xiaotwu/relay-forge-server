package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
)

func TestAuthRequiredRejectsDisabledUserToken(t *testing.T) {
	jwtSvc := auth.NewJWTService("test-secret", time.Hour, time.Hour)
	userID := uuid.New()
	token, err := jwtSvc.GenerateAccessToken(userID, nil)
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	called := false
	handler := AuthRequired(jwtSvc, func(_ context.Context, claims *auth.Claims) error {
		if claims.UserID != userID {
			t.Fatalf("claims user id = %s, want %s", claims.UserID, userID)
		}
		return errors.New("disabled")
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if called {
		t.Fatal("next handler should not be called for a disabled user token")
	}
}

func TestAuthRequiredAllowsEnabledUserToken(t *testing.T) {
	jwtSvc := auth.NewJWTService("test-secret", time.Hour, time.Hour)
	userID := uuid.New()
	token, err := jwtSvc.GenerateAccessToken(userID, []string{"admin"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	handler := AuthRequired(jwtSvc, func(_ context.Context, claims *auth.Claims) error {
		if claims.UserID != userID {
			t.Fatalf("claims user id = %s, want %s", claims.UserID, userID)
		}
		return nil
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := GetUserID(r.Context()); got != userID {
			t.Fatalf("context user id = %s, want %s", got, userID)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
