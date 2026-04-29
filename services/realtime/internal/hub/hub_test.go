package hub

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/relay-forge/relay-forge/services/realtime/internal/config"
)

func TestNormalizeEventTypeUsesUppercaseExternalContract(t *testing.T) {
	cases := map[string]string{
		"message.create":    EventMessage,
		"MESSAGE_CREATE":    EventMessage,
		"dm.message.create": "DM_MESSAGE_CREATE",
		"typing.start":      EventTypingStart,
		"read_state.update": EventReadState,
		"presence.update":   EventPresence,
	}

	for input, want := range cases {
		if got := normalizeEventType(input); got != want {
			t.Fatalf("normalizeEventType(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestBuildEnvelopeIncludesSequencedEventData(t *testing.T) {
	h := New(&config.Config{}, nil)
	payload := json.RawMessage(`{"message_id":"m1","channel_id":"c1"}`)

	raw, err := h.buildEnvelope("message.delete", payload)
	if err != nil {
		t.Fatalf("build envelope: %v", err)
	}

	var envelope outboundEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}

	if envelope.Type != EventMessageDelete {
		t.Fatalf("type = %q, want %q", envelope.Type, EventMessageDelete)
	}
	if envelope.Seq != 1 {
		t.Fatalf("seq = %d, want 1", envelope.Seq)
	}
	if string(envelope.Data) != string(payload) {
		t.Fatalf("data = %s, want %s", envelope.Data, payload)
	}
	if envelope.Timestamp == "" {
		t.Fatal("timestamp must be set")
	}
}

func TestValidateTokenRejectsDisabledUser(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID.String(),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	h := New(&config.Config{JWTSecret: secret}, nil)
	h.userActive = func(_ context.Context, got uuid.UUID) (bool, error) {
		if got != userID {
			t.Fatalf("user id = %s, want %s", got, userID)
		}
		return false, nil
	}

	if _, err := h.validateToken(context.Background(), signed); err == nil {
		t.Fatal("validateToken should reject a disabled user")
	}
}
