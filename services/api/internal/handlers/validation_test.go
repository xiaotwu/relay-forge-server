package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	apperrors "github.com/relay-forge/relay-forge/services/api/internal/errors"
)

type errorEnvelope struct {
	Error struct {
		Code string `json:"code"`
	} `json:"error"`
}

func withURLParam(req *http.Request, key, value string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func decodeErrorResponse(t *testing.T, recorder *httptest.ResponseRecorder) errorEnvelope {
	t.Helper()

	var response errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}

	return response
}

func TestShouldConcealPasswordResetLookupError(t *testing.T) {
	if !shouldConcealPasswordResetLookupError(apperrors.NotFound("user not found")) {
		t.Fatal("expected not-found app error to be concealed")
	}

	if shouldConcealPasswordResetLookupError(apperrors.Internal("database unavailable")) {
		t.Fatal("expected internal app error to be surfaced")
	}

	if shouldConcealPasswordResetLookupError(nil) {
		t.Fatal("expected nil error not to be concealed")
	}
}

func TestMessageHandlerSendMessageRejectsWhitespaceOnlyContent(t *testing.T) {
	handler := &MessageHandler{}
	channelID := uuid.New().String()

	req := httptest.NewRequest(http.MethodPost, "/channels/"+channelID+"/messages", strings.NewReader(`{"content":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	req = withURLParam(req, "channelID", channelID)

	recorder := httptest.NewRecorder()
	handler.SendMessage(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	response := decodeErrorResponse(t, recorder)
	if response.Error.Code != "validation_error" {
		t.Fatalf("expected validation error code, got %q", response.Error.Code)
	}
}

func TestMessageHandlerSearchRejectsWhitespaceOnlyQuery(t *testing.T) {
	handler := &MessageHandler{}
	channelID := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/channels/"+channelID+"/messages/search", nil)
	req.URL = &url.URL{RawQuery: "q=%20%20%20"}
	req = withURLParam(req, "channelID", channelID)

	recorder := httptest.NewRecorder()
	handler.SearchMessages(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, recorder.Code)
	}

	response := decodeErrorResponse(t, recorder)
	if response.Error.Code != "validation_error" {
		t.Fatalf("expected validation error code, got %q", response.Error.Code)
	}
}
