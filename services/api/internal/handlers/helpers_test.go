package handlers

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
)

func TestParsePagination(t *testing.T) {
	t.Run("default values", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: ""}}
		limit, offset := parsePagination(req)
		if limit != 50 {
			t.Fatalf("expected default limit 50, got %d", limit)
		}
		if offset != 0 {
			t.Fatalf("expected default offset 0, got %d", offset)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: "limit=25&offset=10"}}
		limit, offset := parsePagination(req)
		if limit != 25 {
			t.Fatalf("expected limit 25, got %d", limit)
		}
		if offset != 10 {
			t.Fatalf("expected offset 10, got %d", offset)
		}
	})

	t.Run("invalid values use defaults", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: "limit=999&offset=-5"}}
		limit, offset := parsePagination(req)
		// limit=999 exceeds 100, so it stays at default 50
		if limit != 50 {
			t.Fatalf("expected limit to stay at default 50 for out-of-range value, got %d", limit)
		}
		// offset=-5 is negative, so it stays at default 0
		if offset != 0 {
			t.Fatalf("expected offset to stay at default 0 for negative value, got %d", offset)
		}
	})

	t.Run("non-numeric values use defaults", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: "limit=abc&offset=xyz"}}
		limit, offset := parsePagination(req)
		if limit != 50 {
			t.Fatalf("expected default limit 50, got %d", limit)
		}
		if offset != 0 {
			t.Fatalf("expected default offset 0, got %d", offset)
		}
	})
}

func TestParseUUID(t *testing.T) {
	t.Run("valid UUID parses", func(t *testing.T) {
		expected := uuid.New()
		parsed, err := parseUUID(expected.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed != expected {
			t.Fatalf("expected %s, got %s", expected, parsed)
		}
	})

	t.Run("invalid string returns error", func(t *testing.T) {
		_, err := parseUUID("not-a-uuid")
		if err == nil {
			t.Fatal("expected error for invalid UUID string")
		}
	})
}

func TestParseCursor(t *testing.T) {
	t.Run("valid UUID returns pointer", func(t *testing.T) {
		id := uuid.New()
		req := &http.Request{URL: &url.URL{RawQuery: "before=" + id.String()}}
		cursor := parseCursor(req)
		if cursor == nil {
			t.Fatal("expected non-nil cursor for valid UUID")
		}
		if *cursor != id {
			t.Fatalf("expected %s, got %s", id, *cursor)
		}
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: ""}}
		cursor := parseCursor(req)
		if cursor != nil {
			t.Fatal("expected nil cursor for empty before param")
		}
	})

	t.Run("invalid returns nil", func(t *testing.T) {
		req := &http.Request{URL: &url.URL{RawQuery: "before=not-valid"}}
		cursor := parseCursor(req)
		if cursor != nil {
			t.Fatal("expected nil cursor for invalid UUID")
		}
	})
}

func TestExtractClientIP(t *testing.T) {
	t.Run("extracts host from remote addr", func(t *testing.T) {
		req := &http.Request{RemoteAddr: "127.0.0.1:54321", Header: make(http.Header)}
		ip := extractClientIP(req)
		if ip == nil {
			t.Fatal("expected IP to be extracted")
		}
		if *ip != "127.0.0.1" {
			t.Fatalf("expected 127.0.0.1, got %s", *ip)
		}
	})

	t.Run("prefers first valid forwarded IP", func(t *testing.T) {
		req := &http.Request{
			RemoteAddr: "127.0.0.1:54321",
			Header: http.Header{
				"X-Forwarded-For": []string{"bad-value, 203.0.113.5, 198.51.100.2"},
			},
		}
		ip := extractClientIP(req)
		if ip == nil {
			t.Fatal("expected forwarded IP to be extracted")
		}
		if *ip != "203.0.113.5" {
			t.Fatalf("expected 203.0.113.5, got %s", *ip)
		}
	})

	t.Run("returns nil for invalid input", func(t *testing.T) {
		req := &http.Request{RemoteAddr: "not-an-ip", Header: make(http.Header)}
		ip := extractClientIP(req)
		if ip != nil {
			t.Fatalf("expected nil IP, got %s", *ip)
		}
	})
}
