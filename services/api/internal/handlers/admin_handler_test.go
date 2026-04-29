package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
	"github.com/relay-forge/relay-forge/services/api/internal/middleware"
)

type fakeAdminUsers struct {
	softDeleted uuid.UUID
}

func (f *fakeAdminUsers) SoftDelete(_ context.Context, id uuid.UUID) error {
	f.softDeleted = id
	return nil
}

type fakeAdminGuilds struct{}

func (f *fakeAdminGuilds) SoftDelete(_ context.Context, _ uuid.UUID) error {
	return nil
}

type fakeAdminSessions struct {
	deletedFor uuid.UUID
	revokedFor uuid.UUID
}

func (f *fakeAdminSessions) DeleteAllForUser(_ context.Context, userID uuid.UUID) error {
	f.deletedFor = userID
	return nil
}

func (f *fakeAdminSessions) RevokeAllRefreshTokensForUser(_ context.Context, userID uuid.UUID) error {
	f.revokedFor = userID
	return nil
}

func TestDisableUserRevokesSessionsAndRefreshTokens(t *testing.T) {
	adminID := uuid.New()
	targetID := uuid.New()
	jwtSvc := auth.NewJWTService("test-secret", time.Hour, time.Hour)
	token, err := jwtSvc.GenerateAccessToken(adminID, []string{"admin"})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	users := &fakeAdminUsers{}
	sessions := &fakeAdminSessions{}
	handler := NewAdminHandler(users, &fakeAdminGuilds{}, sessions, nil)

	router := chi.NewRouter()
	router.With(middleware.AuthRequired(jwtSvc)).Post("/admin/users/{userID}/disable", handler.DisableUser)

	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+targetID.String()+"/disable", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if users.softDeleted != targetID {
		t.Fatalf("soft deleted user = %s, want %s", users.softDeleted, targetID)
	}
	if sessions.deletedFor != targetID {
		t.Fatalf("deleted sessions for = %s, want %s", sessions.deletedFor, targetID)
	}
	if sessions.revokedFor != targetID {
		t.Fatalf("revoked refresh tokens for = %s, want %s", sessions.revokedFor, targetID)
	}
}
