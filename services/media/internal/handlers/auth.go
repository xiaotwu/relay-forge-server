package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey int

const ctxKeyUserID contextKey = iota

func AuthRequired(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, err := validateTokenString(extractBearerToken(r), jwtSecret)
			if err != nil {
				http.Error(w, `{"error":"invalid or missing token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func OptionalAuth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearerToken(r)
			if tokenStr == "" {
				tokenStr = strings.TrimSpace(r.URL.Query().Get("token"))
			}

			if tokenStr != "" {
				if userID, err := validateTokenString(tokenStr, jwtSecret); err == nil {
					ctx := context.WithValue(r.Context(), ctxKeyUserID, userID)
					r = r.WithContext(ctx)
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetUserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ctxKeyUserID).(uuid.UUID)
	return id
}

func extractBearerToken(r *http.Request) string {
	header := r.Header.Get("Authorization")
	if header == "" {
		return ""
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return strings.TrimSpace(parts[1])
}

func validateTokenString(tokenStr, secret string) (uuid.UUID, error) {
	if tokenStr == "" {
		return uuid.Nil, fmt.Errorf("missing token")
	}

	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, fmt.Errorf("invalid token claims")
	}

	subject, err := claims.GetSubject()
	if err != nil {
		return uuid.Nil, err
	}

	return uuid.Parse(subject)
}
