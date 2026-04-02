package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestNotFoundHTTPStatus(t *testing.T) {
	appErr := NotFound("not found")
	if status := appErr.HTTPStatus(); status != http.StatusNotFound {
		t.Fatalf("expected %d, got %d", http.StatusNotFound, status)
	}
}

func TestUnauthorizedHTTPStatus(t *testing.T) {
	appErr := Unauthorized("unauthorized")
	if status := appErr.HTTPStatus(); status != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, status)
	}
}

func TestForbiddenHTTPStatus(t *testing.T) {
	appErr := Forbidden("forbidden")
	if status := appErr.HTTPStatus(); status != http.StatusForbidden {
		t.Fatalf("expected %d, got %d", http.StatusForbidden, status)
	}
}

func TestValidationHTTPStatus(t *testing.T) {
	appErr := Validation("invalid input", nil)
	if status := appErr.HTTPStatus(); status != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, status)
	}
}

func TestConflictHTTPStatus(t *testing.T) {
	appErr := Conflict("already exists")
	if status := appErr.HTTPStatus(); status != http.StatusConflict {
		t.Fatalf("expected %d, got %d", http.StatusConflict, status)
	}
}

func TestInternalHTTPStatus(t *testing.T) {
	appErr := Internal("something went wrong")
	if status := appErr.HTTPStatus(); status != http.StatusInternalServerError {
		t.Fatalf("expected %d, got %d", http.StatusInternalServerError, status)
	}
}

func TestIsAppError(t *testing.T) {
	t.Run("works with AppError", func(t *testing.T) {
		appErr := NotFound("test resource")
		recovered, ok := IsAppError(appErr)
		if !ok {
			t.Fatal("expected IsAppError to return true for AppError")
		}
		if recovered.Code != "not_found" {
			t.Fatalf("expected code not_found, got %s", recovered.Code)
		}
	})

	t.Run("wrapped AppError", func(t *testing.T) {
		appErr := NotFound("test resource")
		wrapped := fmt.Errorf("outer: %w", appErr)
		recovered, ok := IsAppError(wrapped)
		if !ok {
			t.Fatal("expected IsAppError to return true for wrapped AppError")
		}
		if recovered.Code != "not_found" {
			t.Fatalf("expected code not_found, got %s", recovered.Code)
		}
	})

	t.Run("returns false for regular errors", func(t *testing.T) {
		regularErr := errors.New("just a plain error")
		_, ok := IsAppError(regularErr)
		if ok {
			t.Fatal("expected IsAppError to return false for regular error")
		}
	})
}
