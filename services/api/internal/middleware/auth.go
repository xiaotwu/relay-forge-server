package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/relay-forge/relay-forge/services/api/internal/auth"
)

// contextKey is an unexported type used for context keys in this package to
// prevent collisions with keys defined in other packages.
type contextKey int

const (
	ctxKeyUserID contextKey = iota
	ctxKeyUserRoles
)

// TokenValidator performs server-side checks that must remain true after the
// access token was issued, such as user disabled status.
type TokenValidator func(context.Context, *auth.Claims) error

// AuthRequired returns a chi middleware that enforces Bearer token
// authentication. Requests without a valid JWT access token receive a
// 401 Unauthorized response.
func AuthRequired(jwtSvc *auth.JWTService, validators ...TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractClaims(r, jwtSvc)
			if err != nil {
				http.Error(w, `{"code":"unauthorized","message":"invalid or missing token"}`, http.StatusUnauthorized)
				return
			}
			if err := validateClaims(r.Context(), claims, validators); err != nil {
				http.Error(w, `{"code":"unauthorized","message":"token no longer valid"}`, http.StatusUnauthorized)
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
func OptionalAuth(jwtSvc *auth.JWTService, validators ...TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := extractClaims(r, jwtSvc)
			if err == nil && validateClaims(r.Context(), claims, validators) == nil {
				ctx := contextWithClaims(r.Context(), claims)
				r = r.WithContext(ctx)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// UserEnabledValidator rejects tokens for deleted or disabled users. This makes
// admin disable take effect before the access token's natural expiry.
func UserEnabledValidator(pool *pgxpool.Pool) TokenValidator {
	if pool == nil {
		return nil
	}
	return func(ctx context.Context, claims *auth.Claims) error {
		var active bool
		err := pool.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM users WHERE id = $1 AND is_disabled = false
			)`,
			claims.UserID,
		).Scan(&active)
		if err != nil {
			return err
		}
		if !active {
			return fmt.Errorf("user disabled or not found")
		}
		return nil
	}
}

func validateClaims(ctx context.Context, claims *auth.Claims, validators []TokenValidator) error {
	for _, validator := range validators {
		if validator == nil {
			continue
		}
		if err := validator(ctx, claims); err != nil {
			return err
		}
	}
	return nil
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
