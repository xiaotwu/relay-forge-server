package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
)

// contextKey is an unexported type used for context keys in this package to
// prevent collisions with keys defined in other packages.
type contextKey int

const (
	ctxKeyUserID contextKey = iota
	ctxKeyUserRoles
)

// AuthRequired returns a chi middleware that enforces Bearer token
// authentication. Requests without a valid JWT access token receive a
// 401 Unauthorized response.
func AuthRequired(jwtSvc *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractClaims(r, jwtSvc)
			if err != nil {
				http.Error(w, `{"code":"unauthorized","message":"invalid or missing token"}`, http.StatusUnauthorized)
				return
			}

			ctx := contextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth returns a chi middleware that attempts Bearer token
// authentication but does not reject requests that lack a token. When a
// valid token is present, user info is placed in the request context.
func OptionalAuth(jwtSvc *auth.JWTService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractClaims(r, jwtSvc)
			if err == nil {
				ctx := contextWithClaims(r.Context(), claims)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID retrieves the authenticated user's ID from the request context.
// Returns uuid.Nil if the user is not authenticated.
func GetUserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id
}

// GetUserRoles retrieves the authenticated user's roles from the request
// context. Returns nil if the user is not authenticated.
func GetUserRoles(ctx context.Context) []string {
	roles, _ := ctx.Value(ctxKeyUserRoles).([]string)
	return roles
}

// extractClaims reads the Authorization header, strips the Bearer prefix, and
// validates the JWT access token.
func extractClaims(r *http.Request, jwtSvc *auth.JWTService) (*auth.Claims, error) {
	header := r.Header.Get("Authorization")
	if header == "" {
		return nil, http.ErrNoCookie // reuse as a sentinel; not a cookie error
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, http.ErrNoCookie
	}

	return jwtSvc.ValidateAccessToken(parts[1])
}

// contextWithClaims stores user ID and roles from the JWT claims into the
// given context.
func contextWithClaims(ctx context.Context, claims *auth.Claims) context.Context {
	ctx = context.WithValue(ctx, ctxKeyUserID, claims.UserID)
	ctx = context.WithValue(ctx, ctxKeyUserRoles, claims.Roles)
	return ctx
}
