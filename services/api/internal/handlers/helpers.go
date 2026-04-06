package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
)

// apiResponse wraps all JSON responses with a consistent structure.
type apiResponse struct {
	Data  any `json:"data,omitempty"`
	Error any `json:"error,omitempty"`
	Meta  any `json:"meta,omitempty"`
}

// respondJSON writes a JSON response with the given status code and data.
func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(apiResponse{Data: data})
	}
}

// respondJSONWithMeta writes a JSON response that includes pagination metadata.
func respondJSONWithMeta(w http.ResponseWriter, status int, data any, meta any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(apiResponse{Data: data, Meta: meta})
}

// respondError maps an error to the appropriate HTTP status and writes an error response.
func respondError(w http.ResponseWriter, err error) {
	appErr, ok := apperrors.IsAppError(err)
	if !ok {
		appErr = apperrors.Internal("internal server error")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.HTTPStatus())
	_ = json.NewEncoder(w).Encode(apiResponse{
		Error: appErr,
	})
}

// decodeJSON decodes the request body into dst. Returns an AppError on failure.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return apperrors.Validation("request body is required", nil)
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return apperrors.Validation("invalid request body: "+err.Error(), nil)
	}
	return nil
}

// parseUUID parses a string as a UUID. Returns an AppError on failure.
func parseUUID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperrors.Validation("invalid UUID: "+s, nil)
	}
	return id, nil
}

// parsePagination extracts limit and offset from query parameters with defaults.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	offset = 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

// parseCursor extracts a cursor UUID from the "before" query parameter.
// Returns nil if the parameter is absent or invalid.
func parseCursor(r *http.Request) *uuid.UUID {
	v := r.URL.Query().Get("before")
	if v == "" {
		return nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil
	}
	return &id
}

// forbiddenErr creates a Forbidden AppError with the given message.
func forbiddenErr(msg string) error {
	return apperrors.Forbidden(msg)
}

// extractClientIP returns the best-effort client IP address for storage in INET columns.
func extractClientIP(r *http.Request) *string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		for _, part := range strings.Split(forwarded, ",") {
			candidate := strings.TrimSpace(part)
			if ip := net.ParseIP(candidate); ip != nil {
				normalized := ip.String()
				return &normalized
			}
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	if ip := net.ParseIP(host); ip != nil {
		normalized := ip.String()
		return &normalized
	}

	return nil
}
