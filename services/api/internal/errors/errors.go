package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors for common application error conditions.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrValidation   = errors.New("validation error")
	ErrConflict     = errors.New("resource conflict")
	ErrInternal     = errors.New("internal server error")
)

// AppError represents a structured application error with an HTTP-friendly code,
// a human-readable message, and optional details for validation errors.
type AppError struct {
	// Code is a machine-readable error code (e.g. "not_found", "validation_error").
	Code string `json:"code"`
	// Message is a human-readable description of the error.
	Message string `json:"message"`
	// Details holds optional structured data, typically validation field errors.
	Details any `json:"details,omitempty"`
	// Err is the underlying error (not serialized to JSON).
	Err error `json:"-"`
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying error for use with errors.Is / errors.As.
func (e *AppError) Unwrap() error {
	return e.Err
}

// HTTPStatus maps the AppError to an appropriate HTTP status code.
func (e *AppError) HTTPStatus() int {
	if e.Err != nil {
		return HTTPStatusFromError(e.Err)
	}
	return HTTPStatusFromCode(e.Code)
}

// HTTPStatusFromError maps a sentinel error to an HTTP status code.
func HTTPStatusFromError(err error) int {
	switch {
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrUnauthorized):
		return http.StatusUnauthorized
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, ErrConflict):
		return http.StatusConflict
	case errors.Is(err, ErrInternal):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// HTTPStatusFromCode maps an error code string to an HTTP status code.
func HTTPStatusFromCode(code string) int {
	switch code {
	case "not_found":
		return http.StatusNotFound
	case "unauthorized":
		return http.StatusUnauthorized
	case "forbidden":
		return http.StatusForbidden
	case "validation_error":
		return http.StatusBadRequest
	case "conflict":
		return http.StatusConflict
	case "internal_error":
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// New creates a new AppError wrapping a sentinel error.
func New(err error, message string) *AppError {
	return &AppError{
		Code:    codeFromError(err),
		Message: message,
		Err:     err,
	}
}

// Newf creates a new AppError with a formatted message.
func Newf(err error, format string, args ...any) *AppError {
	return &AppError{
		Code:    codeFromError(err),
		Message: fmt.Sprintf(format, args...),
		Err:     err,
	}
}

// WithDetails returns a copy of the AppError with the given details attached.
func (e *AppError) WithDetails(details any) *AppError {
	return &AppError{
		Code:    e.Code,
		Message: e.Message,
		Details: details,
		Err:     e.Err,
	}
}

// NotFound creates a not-found AppError.
func NotFound(message string) *AppError {
	return New(ErrNotFound, message)
}

// Unauthorized creates an unauthorized AppError.
func Unauthorized(message string) *AppError {
	return New(ErrUnauthorized, message)
}

// Forbidden creates a forbidden AppError.
func Forbidden(message string) *AppError {
	return New(ErrForbidden, message)
}

// Validation creates a validation AppError with optional field details.
func Validation(message string, details any) *AppError {
	return &AppError{
		Code:    "validation_error",
		Message: message,
		Details: details,
		Err:     ErrValidation,
	}
}

// Conflict creates a conflict AppError.
func Conflict(message string) *AppError {
	return New(ErrConflict, message)
}

// Internal creates an internal server error AppError.
func Internal(message string) *AppError {
	return New(ErrInternal, message)
}

// codeFromError derives a machine-readable code from a sentinel error.
func codeFromError(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return "not_found"
	case errors.Is(err, ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, ErrForbidden):
		return "forbidden"
	case errors.Is(err, ErrValidation):
		return "validation_error"
	case errors.Is(err, ErrConflict):
		return "conflict"
	case errors.Is(err, ErrInternal):
		return "internal_error"
	default:
		return "internal_error"
	}
}

// IsAppError checks whether an error is an AppError and returns it.
func IsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
